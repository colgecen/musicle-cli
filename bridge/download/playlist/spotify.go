package playlist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"MusicLeCLI/bridge/download"
	"MusicLeCLI/bridge/download/music"
)

type spotifyArtist struct {
	Name string `json:"name"`
}

type spotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type spotifyAlbumJSON struct {
	Name   string         `json:"name"`
	Images []spotifyImage `json:"images"`
}

type spotifyTrackJSON struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Artists []spotifyArtist  `json:"artists"`
	Album   spotifyAlbumJSON `json:"album"`
	DurationMs int           `json:"duration_ms"`
	TrackNumber int          `json:"track_number"`
}

type spotifyTracksResp struct {
	Items []struct {
		Track *spotifyTrackJSON `json:"track"`
	} `json:"items"`
	Next  string `json:"next"`
	Total int    `json:"total"`
}

var spClientID, spClientSecret, spToken string

func spAuth() error {
	if spToken != "" {
		return nil
	}
	if spClientID == "" {
		spClientID = os.Getenv("SPOTIFY_CLIENT_ID")
		spClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	}
	if spClientID == "" {
		return fmt.Errorf("Spotify credentials not set (SPOTIFY_CLIENT_ID / SPOTIFY_CLIENT_SECRET)")
	}

	body := url.Values{"grant_type": {"client_credentials"}}.Encode()
	req, _ := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(body))
	req.SetBasicAuth(spClientID, spClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var t struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(raw, &t) != nil || t.AccessToken == "" {
		return fmt.Errorf("Spotify auth failed: %s", string(raw))
	}
	spToken = t.AccessToken
	return nil
}

func spGet(endpoint string) ([]byte, error) {
	if err := spAuth(); err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", "https://api.spotify.com/v1"+endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+spToken)

	music.WaitSpotify()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Spotify %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func parseSpotifyID(rawURL string) (entity, id string, err error) {
	u, e := url.Parse(rawURL)
	if e != nil {
		return "", "", e
	}
	if u.Scheme == "spotify" {
		parts := strings.Split(u.Opaque, ":")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("invalid spotify URI: %s", rawURL)
	}
	if strings.Contains(u.Host, "spotify") {
		seg := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, s := range seg {
			if s == "track" || s == "playlist" || s == "album" {
				if i+1 < len(seg) {
					return seg[i], seg[i+1], nil
				}
				break
			}
		}
		if len(seg) >= 2 {
			return seg[0], seg[1], nil
		}
	}
	return "", "", fmt.Errorf("unsupported URL: %s", rawURL)
}

// joinArtists joins all artist names from a list.
func joinArtists(artists []spotifyArtist) string {
	var names []string
	for _, a := range artists {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, ", ")
}

// bestImage picks the largest image from a list.
func bestImage(imgs []spotifyImage) string {
	if len(imgs) == 0 {
		return ""
	}
	best := imgs[0]
	for _, img := range imgs[1:] {
		if img.Width > best.Width || (img.Width == 0 && img.Height > best.Height) {
			best = img
		}
	}
	return best.URL
}

// SpotifyPlaylistEntry is a single track from a Spotify playlist with stored audio bytes.
type SpotifyPlaylistEntry struct {
	download.TrackInfo
	Index int
}

// FetchSpotifyPlaylist fetches a Spotify playlist's track metadata.
func FetchSpotifyPlaylist(playlistURL string) (name string, entries []SpotifyPlaylistEntry, err error) {
	entity, id, err := parseSpotifyID(playlistURL)
	if err != nil {
		return "", nil, err
	}
	if entity != "playlist" {
		return "", nil, fmt.Errorf("expected playlist, got %s", entity)
	}

	// Get playlist metadata
	raw, err := spGet("/playlists/" + id)
	if err != nil {
		return "", nil, err
	}
	var pl struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Images      []spotifyImage `json:"images"`
		Owner       struct {
			DisplayName string `json:"display_name"`
		} `json:"owner"`
		Followers struct {
			Total int `json:"total"`
		} `json:"followers"`
		Tracks struct {
			Total int `json:"total"`
		} `json:"tracks"`
	}
	json.Unmarshal(raw, &pl)

	// Get tracks (paginated)
	next := "/playlists/" + id + "/tracks?limit=50"
	for next != "" {
		endpoint := next
		if strings.HasPrefix(next, "https://") {
			u, _ := url.Parse(next)
			endpoint = u.RequestURI()
		}

		raw, err = spGet(endpoint)
		if err != nil {
			return "", nil, err
		}

		var resp spotifyTracksResp
		if err := json.Unmarshal(raw, &resp); err != nil {
			return "", nil, err
		}

		for _, item := range resp.Items {
			if item.Track == nil {
				continue
			}
			t := item.Track
			entries = append(entries, SpotifyPlaylistEntry{
				TrackInfo: download.TrackInfo{
					Title:       t.Name,
					Artist:      joinArtists(t.Artists),
					Album:       t.Album.Name,
					DurationSec: float64(t.DurationMs) / 1000,
					Thumbnail:   bestImage(t.Album.Images),
					Playlist:    pl.Name,
					TrackNum:    t.TrackNumber,
				},
				Index: len(entries) + 1,
			})
		}
		next = resp.Next
	}

	return pl.Name, entries, nil
}

// downloadSpotifyWorker handles one Spotify track: search YouTube, download, convert, save.
func downloadSpotifyWorker(entry SpotifyPlaylistEntry, outputDir string) (file string, err error) {
	query := entry.Artist + " - " + entry.Title
	videoID, _, sErr := music.SearchYouTubeTrack(query)
	if sErr != nil {
		return "", fmt.Errorf("search failed: %w", sErr)
	}

	_, rawAudio, dErr := music.DownloadYouTubeTrack(videoID, nil)
	if dErr != nil {
		return "", fmt.Errorf("download failed: %w", dErr)
	}

	entry.TrackInfo.StreamURL = "https://www.youtube.com/watch?v=" + videoID
	entry.TrackInfo.Format = "webm"
	entry.TrackInfo.ContentLen = int64(len(rawAudio))

	f, sErr := music.SaveRawAsMP3(rawAudio, &entry.TrackInfo, outputDir, nil)
	if sErr != nil {
		return "", fmt.Errorf("save failed: %w", sErr)
	}
	return f, nil
}

// DownloadSpotifyPlaylist downloads a Spotify playlist sequentially.
func DownloadSpotifyPlaylist(playlistURL, outputDir string, progress func(pct int, msg string)) ([]string, error) {
	return DownloadSpotifyPlaylistParallel(playlistURL, outputDir, 1, progress)
}

// DownloadSpotifyPlaylistParallel downloads a Spotify playlist with a concurrent worker pool.
func DownloadSpotifyPlaylistParallel(playlistURL, outputDir string, workers int, progress func(pct int, msg string)) ([]string, error) {
	_, entries, err := FetchSpotifyPlaylist(playlistURL)
	if err != nil {
		return nil, fmt.Errorf("fetch playlist: %w", err)
	}

	total := len(entries)
	if workers <= 0 {
		workers = PlaylistConcurrency
	}
	if workers > total {
		workers = total
	}

	type spResult struct {
		file     string
		err      error
		entryIdx int
	}

	jobs := make(chan struct {
		entry SpotifyPlaylistEntry
		idx   int
	}, total)
	results := make(chan spResult, total)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Skip if already downloaded
				safeName := music.SafeFilename(job.entry.Title)
				existing := filepath.Join(outputDir, safeName+".mp3")
				if _, statErr := os.Stat(existing); statErr == nil {
					results <- spResult{file: existing, entryIdx: job.idx}
					continue
				}

				f, dErr := downloadSpotifyWorker(job.entry, outputDir)
				if dErr != nil {
					results <- spResult{err: dErr, entryIdx: job.idx}
				} else {
					results <- spResult{file: f, entryIdx: job.idx}
				}
			}
		}()
	}

	for i, entry := range entries {
		jobs <- struct {
			entry SpotifyPlaylistEntry
			idx   int
		}{entry, i + 1}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	files := make([]string, total)
	errs := make(map[int]string)
	doneIdx := make(map[int]bool)

	for res := range results {
		if res.err != nil {
			errs[res.entryIdx] = fmt.Sprintf("[%d/%d] Error: %v", res.entryIdx, total, res.err)
		} else {
			files[res.entryIdx-1] = res.file
		}
		doneIdx[res.entryIdx] = true
		completed := len(doneIdx)

		if progress != nil {
			pct := completed * 100 / total
			if pct > 100 {
				pct = 100
			}
			progress(pct, fmt.Sprintf("[%d/%d] %d done, %d errors", completed, total, completed-len(errs), len(errs)))
		}
	}

	var resultFiles []string
	for _, f := range files {
		if f != "" {
			resultFiles = append(resultFiles, f)
		}
	}

	if len(resultFiles) == 0 {
		return nil, fmt.Errorf("no tracks downloaded from playlist")
	}
	if len(errs) > 0 && progress != nil {
		progress(100, fmt.Sprintf("Completed with %d errors", len(errs)))
	}
	return resultFiles, nil
}
