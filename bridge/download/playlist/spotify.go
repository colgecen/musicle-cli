package playlist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

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

// DownloadSpotifyPlaylist downloads a Spotify playlist: searches each track on
// YouTube, downloads from YouTube, tags with Spotify metadata.
func DownloadSpotifyPlaylist(playlistURL, outputDir string, progress func(pct int, msg string)) ([]string, error) {
	_, entries, err := FetchSpotifyPlaylist(playlistURL)
	if err != nil {
		return nil, fmt.Errorf("fetch playlist: %w", err)
	}

	var files []string
	total := len(entries)
	for i, entry := range entries {
		idx := i + 1
		query := entry.Artist + " - " + entry.Title

		if progress != nil {
			progress((idx-1)*100/total, fmt.Sprintf("[%d/%d] Searching: %s", idx, total, query))
		}

		videoID, _, err := music.SearchYouTubeTrack(query)
		if err != nil {
			if progress != nil {
				progress(idx*100/total, fmt.Sprintf("[%d/%d] Search failed: %v", idx, total, err))
			}
			continue
		}

		if progress != nil {
			progress((idx-1)*100/total+5, fmt.Sprintf("[%d/%d] Downloading...", idx, total))
		}

		// Download raw audio from YouTube
		_, rawAudio, err := music.DownloadYouTubeTrack(videoID, nil)
		if err != nil {
			if progress != nil {
				progress(idx*100/total, fmt.Sprintf("[%d/%d] Download failed: %v", idx, total, err))
			}
			continue
		}

		if progress != nil {
			progress((idx-1)*100/total+30, fmt.Sprintf("[%d/%d] Converting...", idx, total))
		}

		// Use Spotify metadata for tagging
		entry.TrackInfo.StreamURL = "https://www.youtube.com/watch?v=" + videoID
		entry.TrackInfo.Format = "webm"
		entry.TrackInfo.ContentLen = int64(len(rawAudio))

		file, err := music.SaveRawAsMP3(rawAudio, &entry.TrackInfo, outputDir, func(pct int, msg string) {
			if progress != nil {
				overall := ((idx-1)*100/total) + 30 + (pct*30/100)
				if overall > 100 {
					overall = 100
				}
				progress(overall, fmt.Sprintf("[%d/%d] %s", idx, total, msg))
			}
		})
		if err != nil {
			if progress != nil {
				progress(idx*100/total, fmt.Sprintf("[%d/%d] Error: %v", idx, total, err))
			}
			continue
		}
		files = append(files, file)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no tracks downloaded from playlist")
	}
	return files, nil
}
