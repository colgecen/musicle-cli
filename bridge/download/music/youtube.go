package music

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"MusicLeCLI/bridge/download"
)

var (
	youtubeRL    *rateLimiter
	spotifyRL    *rateLimiter
	rlOnce       sync.Once
)

func initRateLimiters() {
	youtubeRL = newRateLimiter(8, 4)   // 8 req/s, burst 4
	spotifyRL = newRateLimiter(15, 8) // 15 req/s, burst 8
}

type rateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	burst    int
	rate     float64 // tokens per nanosecond
	last     time.Time
}

func newRateLimiter(ratePerSec, burst int) *rateLimiter {
	return &rateLimiter{
		tokens: float64(burst),
		burst:  burst,
		rate:   float64(ratePerSec) / float64(time.Second),
		last:   time.Now(),
	}
}

func (rl *rateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.last)
	rl.last = now

	rl.tokens += elapsed.Seconds() * rl.rate
	if rl.tokens > float64(rl.burst) {
		rl.tokens = float64(rl.burst)
	}

	if rl.tokens >= 1 {
		rl.tokens--
		return
	}

	// Wait for next token
	waitDur := time.Duration(float64(time.Second) / rl.rate * (1 - rl.tokens))
	rl.mu.Unlock()
	time.Sleep(waitDur)
	rl.mu.Lock()
	rl.tokens = float64(rl.burst) - 1
	rl.last = time.Now()
}

// httpGetWithYouTubeRL performs an HTTP GET with YouTube rate limiting.
func httpGetWithYouTubeRL(urlStr string) (*http.Response, error) {
	rlOnce.Do(initRateLimiters)
	youtubeRL.Wait()
	return http.Get(urlStr)
}

// httpGetWithSpotifyRL performs an HTTP GET with Spotify rate limiting.
func httpGetWithSpotifyRL(urlStr string) (*http.Response, error) {
	rlOnce.Do(initRateLimiters)
	spotifyRL.Wait()
	return http.Get(urlStr)
}

// httpDoWithYouTubeRL performs an HTTP request with YouTube rate limiting.
func httpDoWithYouTubeRL(req *http.Request) (*http.Response, error) {
	rlOnce.Do(initRateLimiters)
	youtubeRL.Wait()
	return http.DefaultClient.Do(req)
}

// WaitYouTube blocks until a YouTube API request is allowed (rate limiter).
func WaitYouTube() {
	rlOnce.Do(initRateLimiters)
	youtubeRL.Wait()
}

// WaitSpotify blocks until a Spotify API request is allowed (rate limiter).
func WaitSpotify() {
	rlOnce.Do(initRateLimiters)
	spotifyRL.Wait()
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// extractVideoID parses a YouTube URL and returns the video ID.
// Supports:
//
//	https://www.youtube.com/watch?v=VIDEO_ID
//	https://youtu.be/VIDEO_ID
//	https://m.youtube.com/watch?v=VIDEO_ID
//	https://youtube.com/watch?v=VIDEO_ID
func extractVideoID(url string) (string, error) {
	// youtu.be short links
	if strings.Contains(url, "youtu.be") {
		parts := strings.Split(url, "/")
		for i, p := range parts {
			if strings.Contains(p, "youtu.be") && i+1 < len(parts) {
				id := parts[i+1]
				// strip query params
				if idx := strings.Index(id, "?"); idx >= 0 {
					id = id[:idx]
				}
				if idx := strings.Index(id, "&"); idx >= 0 {
					id = id[:idx]
				}
				if len(id) == 11 {
					return id, nil
				}
			}
		}
		return "", fmt.Errorf("cannot extract video ID from youtu.be URL: %s", url)
	}

	// /watch?v=VIDEO_ID
	if strings.Contains(url, "watch?v=") {
		idx := strings.Index(url, "watch?v=")
		id := url[idx+8:]
		if amp := strings.Index(id, "&"); amp >= 0 {
			id = id[:amp]
		}
		if len(id) == 11 {
			return id, nil
		}
		return "", fmt.Errorf("invalid video ID length in /watch URL: %s", url)
	}

	return "", fmt.Errorf("unsupported YouTube URL format: %s", url)
}

// FetchYouTubePage fetches the HTML page for a YouTube video.
// Accepts a full URL or video ID.
func FetchYouTubePage(urlOrID string) (string, error) {
	videoID := urlOrID
	if strings.HasPrefix(urlOrID, "http") {
		var err error
		videoID, err = extractVideoID(urlOrID)
		if err != nil {
			return "", err
		}
	} else if len(videoID) != 11 {
		return "", fmt.Errorf("invalid video ID: %q", videoID)
	}

	watchURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	req, err := http.NewRequest("GET", watchURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	rlOnce.Do(initRateLimiters)
	youtubeRL.Wait()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	return string(body), nil
}

// downloadMaxRetries is the number of times to retry a failed stream download.
const downloadMaxRetries = 3

// DownloadStream downloads raw audio bytes from a stream URL with retry + progress.
func DownloadStream(streamURL string, contentLen int64, cb download.ProgressCallback) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < downloadMaxRetries; attempt++ {
		if attempt > 0 {
			if cb != nil {
				cb(0, fmt.Sprintf("Retry %d/%d...", attempt+1, downloadMaxRetries))
			}
		}

		req, err := http.NewRequest("GET", streamURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("create request: %w", err)
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Referer", "https://www.youtube.com/")
		req.Header.Set("Origin", "https://www.youtube.com")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http get (attempt %d): %w", attempt+1, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			if contentLen <= 0 {
				contentLen = resp.ContentLength
			}

			// Read with progress tracking
			var buf bytes.Buffer
			read := int64(0)
			tmp := make([]byte, 32768)

			for {
				n, rErr := resp.Body.Read(tmp)
				if n > 0 {
					buf.Write(tmp[:n])
					read += int64(n)
					if cb != nil && contentLen > 0 {
						pct := int(read * 100 / contentLen)
						if pct > 100 {
							pct = 100
						}
						cb(pct, fmt.Sprintf("Downloading %d / %d KB", read/1024, contentLen/1024))
					}
				}
				if rErr == io.EOF {
					break
				}
				if rErr != nil {
					lastErr = rErr
					break
				}
			}
			resp.Body.Close()

			if lastErr == nil {
				if cb != nil {
					cb(100, "Downloaded")
				}
				return buf.Bytes(), nil
			}
			// Error during read, retry
			continue
		}

		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d (attempt %d)", resp.StatusCode, attempt+1)
	}

	return nil, fmt.Errorf("download failed after %d retries: %w", downloadMaxRetries, lastErr)
}

// classifyYouTubeError wraps errors with user-friendly messages.
func classifyYouTubeError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "not playable") || strings.Contains(errStr, "unavailable"):
		return fmt.Errorf("video unavailable (age-restricted or blocked): %w", err)
	case strings.Contains(errStr, "private"):
		return fmt.Errorf("video is private: %w", err)
	case strings.Contains(errStr, "no audio") || strings.Contains(errStr, "no suitable"):
		return fmt.Errorf("no audio stream available: %w", err)
	case strings.Contains(errStr, "404") || strings.Contains(errStr, "410"):
		return fmt.Errorf("video not found (deleted or removed): %w", err)
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection refused"):
		return fmt.Errorf("network error: %w", err)
	default:
		return err
	}
}

// DownloadYouTubeTrack fetches a YouTube page, parses it, selects the best audio
// stream, downloads it, and returns track info + raw audio bytes.
func DownloadYouTubeTrack(urlOrID string, cb download.ProgressCallback) (*download.TrackInfo, []byte, error) {
	if cb != nil {
		cb(0, "Fetching page...")
	}

	html, err := FetchYouTubePage(urlOrID)
	if err != nil {
		return nil, nil, classifyYouTubeError(fmt.Errorf("fetch page: %w", err))
	}

	if cb != nil {
		cb(10, "Parsing player response...")
	}

	pr, err := ParsePlayerResponse(html)
	if err != nil {
		return nil, nil, classifyYouTubeError(fmt.Errorf("parse response: %w", err))
	}

	stream := BestAudioStream(pr.Streams)
	if stream == nil {
		return nil, nil, classifyYouTubeError(fmt.Errorf("no suitable audio stream found"))
	}

	if cb != nil {
		cb(20, fmt.Sprintf("Selected stream: itag=%d (%s)", stream.ITag, stream.Format))
	}

	rawAudio, err := DownloadStream(stream.URL, stream.ContentLen, func(pct int, msg string) {
		if cb != nil {
			cb(20+pct*60/100, msg)
		}
	})
	if err != nil {
		return nil, nil, classifyYouTubeError(fmt.Errorf("download stream: %w", err))
	}

	if cb != nil {
		cb(90, "Decoding audio...")
	}

	track := &download.TrackInfo{
		Title:       pr.Title,
		Artist:      pr.Author,
		Album:       pr.Author,
		DurationSec: pr.DurationSec,
		StreamURL:   stream.URL,
		Format:      stream.Format,
		ContentLen:  int64(len(rawAudio)),
		Thumbnail:   pr.ThumbnailURL,
	}

	if cb != nil {
		cb(100, "Done")
	}
	return track, rawAudio, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchYouTubeTrack searches YouTube for a track (e.g. "artist - title")
// and returns the video ID + basic metadata from the video page.
// YouTubeSearchResult holds info about a single YouTube search result.
type YouTubeSearchResult struct {
	VideoID     string
	Title       string
	Channel     string
	DurationSec float64
	Thumbnail   string
}

// youtubeAPIKey is the YouTube Data API v3 key (optional, from YOUTUBE_API_KEY env).
var youtubeAPIKey = os.Getenv("YOUTUBE_API_KEY")

// searchYouTubeAPI uses the YouTube Data API v3 to search for tracks.
// Returns up to limit results, filtered by duration (videoDuration).
func searchYouTubeAPI(query string, limit int, videoDuration string) ([]YouTubeSearchResult, error) {
	if youtubeAPIKey == "" {
		return nil, fmt.Errorf("no YouTube API key")
	}
	apiURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&type=video&q=%s&maxResults=%d&key=%s",
		url.QueryEscape(query), limit, youtubeAPIKey)
	if videoDuration != "" {
		apiURL += "&videoDuration=" + videoDuration
	}

	resp, err := httpGetWithYouTubeRL(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API search: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("YouTube API %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title       string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails struct {
					Default struct { URL string `json:"url"` } `json:"default"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse API response: %w", err)
	}

	var results []YouTubeSearchResult
	for _, item := range result.Items {
		if item.ID.VideoID == "" {
			continue
		}
		results = append(results, YouTubeSearchResult{
			VideoID:   item.ID.VideoID,
			Title:     item.Snippet.Title,
			Channel:   item.Snippet.ChannelTitle,
			Thumbnail: item.Snippet.Thumbnails.Default.URL,
		})
	}
	return results, nil
}

// searchYouTubeHTML scrapes YouTube search results page for video IDs.
func searchYouTubeHTML(query string, minResults int) ([]YouTubeSearchResult, error) {
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create search req: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := httpDoWithYouTubeRL(req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read search: %w", err)
	}

	// Extract video IDs from the JSON embedded in the page
	html := string(body)
	idRe := regexp.MustCompile(`videoId["']:\s*["']([a-zA-Z0-9_-]{11})["']`)
	idMatches := idRe.FindAllStringSubmatch(html, minResults)

	var results []YouTubeSearchResult
	seen := make(map[string]bool)
	for _, m := range idMatches {
		id := m[1]
		if seen[id] {
			continue
		}
		seen[id] = true
		results = append(results, YouTubeSearchResult{VideoID: id})
		if len(results) >= minResults {
			break
		}
	}

	if len(results) == 0 {
		// Fallback: use old regex
		re := regexp.MustCompile(`"/watch\?v=([a-zA-Z0-9_-]{11})"`)
		matches := re.FindStringSubmatch(html)
		if len(matches) >= 2 {
			results = append(results, YouTubeSearchResult{VideoID: matches[1]})
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no YouTube results for: %s", query)
	}
	return results, nil
}

// enrichSearchResult fetches video page to get title, duration, and thumbnail.
func enrichSearchResult(r *YouTubeSearchResult) error {
	if r.Title != "" {
		return nil // already has metadata from API
	}
	pr, err := ParsePlayerResponseFromID(r.VideoID)
	if err != nil {
		return err
	}
	r.Title = pr.Title
	r.Channel = pr.Author
	r.DurationSec = pr.DurationSec
	r.Thumbnail = pr.ThumbnailURL
	return nil
}

// SearchYouTubeTrack searches for a track on YouTube and returns the best match.
// Uses YouTube Data API if YOUTUBE_API_KEY is set, otherwise scrapes HTML.
func SearchYouTubeTrack(query string) (videoID string, info *download.TrackInfo, err error) {
	// Try API first
	var results []YouTubeSearchResult
	if youtubeAPIKey != "" {
		results, err = searchYouTubeAPI(query, 3, "medium")
		if err != nil {
			// Fall through to HTML
			results = nil
		}
	}

	if len(results) == 0 {
		results, err = searchYouTubeHTML(query, 3)
		if err != nil {
			return "", nil, err
		}
	}

	// Enrich the best result
	best := &results[0]
	if err := enrichSearchResult(best); err != nil {
		return best.VideoID, nil, nil
	}

	info = &download.TrackInfo{
		Title:       best.Title,
		Artist:      best.Channel,
		Album:       best.Channel,
		DurationSec: best.DurationSec,
		Thumbnail:   best.Thumbnail,
	}
	return best.VideoID, info, nil
}

// ParsePlayerResponseFromID fetches a page and parses player response (no download).
func ParsePlayerResponseFromID(videoID string) (*ParseResult, error) {
	html, err := FetchYouTubePage(videoID)
	if err != nil {
		return nil, err
	}
	return ParsePlayerResponse(html)
}

// SaveRawAsMP3 converts raw audio bytes to MP3 with metadata, writes to file.
func SaveRawAsMP3(rawAudio []byte, track *download.TrackInfo, outputDir string, cb download.ProgressCallback) (string, error) {
	if cb != nil {
		cb(0, "Converting to MP3...")
	}

	mp3Data, err := download.WebMToMP3(rawAudio, "192k", track.Artist, func(pct int, msg string) {
		if cb != nil {
			cb(pct*50/100, msg)
		}
	})
	if err != nil {
		return "", fmt.Errorf("convert: %w", err)
	}

	if cb != nil {
		cb(50, "Writing ID3 tag...")
	}

	tagged, err := download.WriteID3Tag(mp3Data, track)
	if err != nil {
		return "", fmt.Errorf("tag: %w", err)
	}

	artist := sanitizeFilename(track.Artist)
	title := sanitizeFilename(track.Title)
	if artist == "" {
		artist = "Unknown"
	}
	if title == "" {
		title = "Unknown"
	}
	filename := fmt.Sprintf("%s - %s.mp3", artist, title)
	filePath := filepath.Join(outputDir, filename)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if err := os.WriteFile(filePath, tagged, 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if cb != nil {
		cb(100, fmt.Sprintf("Saved: %s", filename))
	}

	return filePath, nil
}

// DownloadYouTubeToFile is the full pipeline: YouTube URL → WebM → PCM → MP3 → ID3 → .mp3 file.
func DownloadYouTubeToFile(url, outputDir string, cb download.ProgressCallback) (string, error) {
	if cb != nil {
		cb(0, "Starting...")
	}

	track, rawAudio, err := DownloadYouTubeTrack(url, func(pct int, msg string) {
		if cb != nil {
			cb(pct*40/100, msg)
		}
	})
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	return SaveRawAsMP3(rawAudio, track, outputDir, func(pct int, msg string) {
		if cb != nil {
			cb(40+pct*60/100, msg)
		}
	})
}

// ReTagMP3 updates the playlist name and track number in an existing MP3 file's ID3 tag.
func ReTagMP3(filePath, playlistName string, trackNum int) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read for retag: %w", err)
	}
	// Find existing ID3 tag and strip it if present
	var audioData []byte
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		tagSize := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
		audioData = data[10+tagSize:]
	} else {
		audioData = data
	}
	// Build minimal TrackInfo with playlist + track number
	ti := &download.TrackInfo{
		Playlist:  playlistName,
		TrackNum:  trackNum,
		Title:     "", // preserve existing — we don't parse the old tag here
	}
	// For simplicity, re-tag by reading format info from file path
	// Actually we need to parse the title from existing ID3 — skip for now
	// Instead, write a TXXX frame for playlist and TRCK for track number
	// More robust: just write minimal tag
	_ = audioData
	// TODO full re-tag without re-encoding: parse existing ID3 then update
	// For now, use a simpler approach: write new tag over existing
	// Read existing tag to extract title/artist/album
	title, artist, album := extractID3Fields(data)
	if title == "" {
		// Derive title from filename
		base := filepath.Base(filePath)
		base = strings.TrimSuffix(base, ".mp3")
		if idx := strings.Index(base, " - "); idx >= 0 {
			title = base[idx+3:]
		} else {
			title = base
		}
	}
	ti.Title = title
	if artist != "" {
		ti.Artist = artist
	} else {
		ti.Artist = "Unknown"
	}
	if album != "" {
		ti.Album = album
	} else {
		ti.Album = playlistName
	}

	tagged, err := download.WriteID3Tag(audioData, ti)
	if err != nil {
		return fmt.Errorf("re-tag: %w", err)
	}
	return os.WriteFile(filePath, tagged, 0644)
}

// extractID3Fields extracts title, artist, album from an ID3v2.3 tag.
func extractID3Fields(data []byte) (title, artist, album string) {
	if len(data) < 10 || string(data[:3]) != "ID3" {
		return
	}
	tagSize := int(data[6])<<21 | int(data[7])<<14 | int(data[8])<<7 | int(data[9])
	end := 10 + tagSize
	if end > len(data) {
		end = len(data)
	}
	pos := 10
	for pos+10 <= end {
		fid := string(data[pos : pos+4])
		fSize := int(data[pos+4])<<24 | int(data[pos+5])<<16 | int(data[pos+6])<<8 | int(data[pos+7])
		// flags
		// data starts at pos+10
		dataStart := pos + 10
		if dataStart+fSize > end {
			break
		}
		fieldData := data[dataStart : dataStart+fSize]
		switch fid {
		case "TIT2":
			if len(fieldData) > 1 {
				title = string(fieldData[1:])
			}
		case "TPE1":
			if len(fieldData) > 1 {
				artist = string(fieldData[1:])
			}
		case "TALB":
			if len(fieldData) > 1 {
				album = string(fieldData[1:])
			}
		}
		pos += 10 + fSize
	}
	return
}

// SafeFilename sanitizes a string for use as a filename.
func SafeFilename(name string) string {
	invalid := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	s := invalid.ReplaceAllString(name, "_")
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	if s == "" {
		s = "unknown"
	}
	return s
}

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

