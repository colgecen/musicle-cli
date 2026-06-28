package music

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"MusicLeCLI/bridge/download"
)

// Spotify credentials source: env vars SPOTIFY_CLIENT_ID / SPOTIFY_CLIENT_SECRET,
// or a config file at the path in SPOTIFY_CONFIG (default: config dir).

const spotifyTokenURL = "https://accounts.spotify.com/api/token"
const spotifyAPIBase = "https://api.spotify.com/v1"

type spotifyTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type spotifyArtist struct {
	Name string `json:"name"`
}

type spotifyAlbumResp struct {
	Name       string `json:"name"`
	Images     []spotifyImage `json:"images"`
	ReleaseDate string `json:"release_date"`
	TotalTracks int    `json:"total_tracks"`
}

type spotifyImage struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// spotifyTrackExternalIDs holds external IDs like ISRC.
type spotifyTrackExternalIDs struct {
	ISRC string `json:"isrc"`
}

type spotifyTrackResp struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Artists     []spotifyArtist         `json:"artists"`
	Album       spotifyAlbumResp        `json:"album"`
	DurationMs  int                     `json:"duration_ms"`
	TrackNumber int                     `json:"track_number"`
	DiscNumber  int                     `json:"disc_number"`
	Explicit    bool                    `json:"explicit"`
	ExternalIDs spotifyTrackExternalIDs `json:"external_ids"`
	Popularity  int                     `json:"popularity"`
}

type spotifySearchResp struct {
	Tracks struct {
		Items []spotifyTrackResp `json:"items"`
	} `json:"tracks"`
}

var (
	spotifyClientID     string
	spotifyClientSecret string
	spotifyToken        string
	spotifyTokenExpiry  time.Time
)

func initSpotifyCredentials() error {
	if spotifyClientID != "" && spotifyClientSecret != "" {
		return nil
	}
	spotifyClientID = os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	if spotifyClientID != "" && spotifyClientSecret != "" {
		return nil
	}
	// Try config file
	cfgDir := os.Getenv("MUSICLECLI_CONFIG_DIR")
	if cfgDir == "" {
		home, _ := os.UserConfigDir()
		if home != "" {
			cfgDir = home
		}
	}
	if cfgDir != "" {
		data, err := os.ReadFile(cfgDir + "/spotify.json")
		if err == nil {
			var cfg struct {
				ClientID     string `json:"client_id"`
				ClientSecret string `json:"client_secret"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				spotifyClientID = cfg.ClientID
				spotifyClientSecret = cfg.ClientSecret
			}
		}
	}
	if spotifyClientID == "" || spotifyClientSecret == "" {
		return fmt.Errorf("Spotify credentials not found. Set SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET env vars, or create a spotify.json in config dir")
	}
	return nil
}

func spotifyAuth() error {
	if spotifyToken != "" && time.Now().Before(spotifyTokenExpiry) {
		return nil
	}
	if err := initSpotifyCredentials(); err != nil {
		return err
	}

	data := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequest("POST", spotifyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("create auth req: %w", err)
	}
	req.SetBasicAuth(spotifyClientID, spotifyClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth req: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read auth resp: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed (%d): %s", resp.StatusCode, string(body))
	}

	var tr spotifyTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("parse auth resp: %w", err)
	}

	spotifyToken = tr.AccessToken
	spotifyTokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return nil
}

func spotifyGet(endpoint string) ([]byte, error) {
	if err := spotifyAuth(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", spotifyAPIBase+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create req: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+spotifyToken)

	WaitSpotify()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", endpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Spotify API %s returned %d: %s", endpoint, resp.StatusCode, string(body))
	}
	return body, nil
}

// joinArtists joins all artist names from a Spotify track response.
func joinArtists(artists []spotifyArtist) string {
	var names []string
	for _, a := range artists {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, ", ")
}

// bestImage picks the largest image (best quality) from an image list.
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

// parseSpotifyID extracts the track/album/playlist ID from various Spotify URL formats.
func parseSpotifyID(rawURL string) (entity string, id string, err error) {
	// https://open.spotify.com/track/ID?si=xxx
	// https://open.spotify.com/album/ID
	// https://open.spotify.com/playlist/ID
	// spotify:track:ID
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "spotify" {
		// spotify:track:ID
		parts := strings.Split(u.Opaque, ":")
		if len(parts) >= 2 {
			return parts[0], parts[1], nil
		}
		return "", "", fmt.Errorf("invalid spotify URI: %s", rawURL)
	}

	if strings.Contains(u.Host, "spotify") {
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		for i, seg := range segments {
			if seg == "track" || seg == "playlist" || seg == "album" {
				if i+1 < len(segments) {
					return segments[i], segments[i+1], nil
				}
				break
			}
		}
		if len(segments) >= 2 {
			return segments[0], segments[1], nil
		}
	}

	return "", "", fmt.Errorf("unsupported Spotify URL: %s", rawURL)
}

var spotifyTrackIDRe = regexp.MustCompile(`[a-zA-Z0-9]{22}`)

// FetchSpotifyTrack fetches track metadata from Spotify by URL.
func FetchSpotifyTrack(rawURL string) (*download.TrackInfo, error) {
	entity, id, err := parseSpotifyID(rawURL)
	if err != nil {
		return nil, err
	}
	if entity != "track" {
		return nil, fmt.Errorf("expected track, got %s", entity)
	}

	body, err := spotifyGet("/tracks/" + id)
	if err != nil {
		return nil, err
	}

	var tr spotifyTrackResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("parse track: %w", err)
	}

	artist := joinArtists(tr.Artists)
	thumb := bestImage(tr.Album.Images)

	return &download.TrackInfo{
		Title:       tr.Name,
		Artist:      artist,
		Album:       tr.Album.Name,
		DurationSec: float64(tr.DurationMs) / 1000,
		Thumbnail:   thumb,
		TrackNum:    tr.TrackNumber,
	}, nil
}

// SearchSpotifyTrack searches Spotify for a track matching query (e.g. "artist title").
// Returns the best matching track.
func SearchSpotifyTrack(query string) (*download.TrackInfo, error) {
	body, err := spotifyGet("/search?q=" + url.QueryEscape(query) + "&type=track&limit=1")
	if err != nil {
		return nil, err
	}

	var sr spotifySearchResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("parse search: %w", err)
	}

	if len(sr.Tracks.Items) == 0 {
		return nil, fmt.Errorf("no Spotify results for: %s", query)
	}

	t := sr.Tracks.Items[0]
	artist := joinArtists(t.Artists)
	thumb := bestImage(t.Album.Images)

	return &download.TrackInfo{
		Title:       t.Name,
		Artist:      artist,
		Album:       t.Album.Name,
		DurationSec: float64(t.DurationMs) / 1000,
		Thumbnail:   thumb,
		TrackNum:    t.TrackNumber,
	}, nil
}
