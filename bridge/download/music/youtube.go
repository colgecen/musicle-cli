package music

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"MusicLeCLI/bridge/download"
)

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

	client := &http.Client{}
	resp, err := client.Do(req)
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

// DownloadStream downloads raw audio bytes from a stream URL.
func DownloadStream(streamURL string, contentLen int64, cb download.ProgressCallback) ([]byte, error) {
	if cb != nil {
		cb(0, "Downloading stream...")
	}

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create stream request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://www.youtube.com/")
	req.Header.Set("Origin", "https://www.youtube.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stream get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("stream http status %d", resp.StatusCode)
	}

	if contentLen <= 0 {
		contentLen = resp.ContentLength
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read stream: %w", err)
	}

	if cb != nil {
		cb(100, "Downloaded")
	}
	return data, nil
}

// DownloadYouTubeTrack fetches a YouTube page, parses it, selects the best audio
// stream, downloads it, and returns track info + raw audio bytes.
func DownloadYouTubeTrack(urlOrID string, cb download.ProgressCallback) (*download.TrackInfo, []byte, error) {
	if cb != nil {
		cb(0, "Fetching page...")
	}

	html, err := FetchYouTubePage(urlOrID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch page: %w", err)
	}

	if cb != nil {
		cb(10, "Parsing player response...")
	}

	pr, err := ParsePlayerResponse(html)
	if err != nil {
		return nil, nil, fmt.Errorf("parse response: %w", err)
	}

	stream := BestAudioStream(pr.Streams)
	if stream == nil {
		return nil, nil, fmt.Errorf("no suitable audio stream found")
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
		return nil, nil, fmt.Errorf("download stream: %w", err)
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
func SearchYouTubeTrack(query string) (videoID string, info *download.TrackInfo, err error) {
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))

	req, err2 := http.NewRequest("GET", searchURL, nil)
	if err2 != nil {
		return "", nil, fmt.Errorf("create search req: %w", err2)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err2 := http.DefaultClient.Do(req)
	if err2 != nil {
		return "", nil, fmt.Errorf("search: %w", err2)
	}
	defer resp.Body.Close()

	body, err2 := io.ReadAll(resp.Body)
	if err2 != nil {
		return "", nil, fmt.Errorf("read search: %w", err2)
	}
	html := string(body)

	re := regexp.MustCompile(`"/watch\?v=([a-zA-Z0-9_-]{11})"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) < 2 {
		return "", nil, fmt.Errorf("no YouTube results for: %s", query)
	}
	videoID = matches[1]

	// Get full track info from video page (without downloading audio)
	pr, err := ParsePlayerResponseFromID(videoID)
	if err != nil {
		return videoID, nil, nil
	}

	thumb := ""
	if pr.ThumbnailURL != "" {
		thumb = pr.ThumbnailURL
	}

	info = &download.TrackInfo{
		Title:       pr.Title,
		Artist:      pr.Author,
		Album:       pr.Author,
		DurationSec: pr.DurationSec,
		Thumbnail:   thumb,
	}
	return videoID, info, nil
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

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

