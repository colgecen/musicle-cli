package music

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"

	"MusicLeCLI/bridge/download"
)

// Scraped Spotify metadata — no API, no credentials.

var (
	ldjsonRe   = regexp.MustCompile(`<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	nextDataRe = regexp.MustCompile(`<script[^>]*id=["']__NEXT_DATA__["'][^>]*type=["']application/json["'][^>]*>(.*?)</script>`)

	spotifyClient *http.Client
)

func init() {
	jar, _ := cookiejar.New(nil)
	spotifyClient = &http.Client{
		Timeout: 15 * time.Second,
		Jar:     jar,
	}
}

func extractMeta(html string, prop string) string {
	re := regexp.MustCompile(`<meta[^>]+(?:property|name)=["']` + regexp.QuoteMeta(prop) + `["'][^>]+content=["']([^"']*)["']`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		return htmlUnescape(m[1])
	}
	return ""
}

func htmlUnescape(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

type ldMusicRecording struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	Duration    string `json:"duration"`
	Image       string `json:"image"`
	TrackNumber int    `json:"trackNumber"`
	ByArtist    struct {
		Name string `json:"name"`
	} `json:"byArtist"`
	Album struct {
		Name string `json:"name"`
	} `json:"album"`
}

type ldMusicPlaylist struct {
	Type      string `json:"@type"`
	Name      string `json:"name"`
	NumTracks int    `json:"numTracks"`
	Track     []struct {
		Type     string `json:"@type"`
		Name     string `json:"name"`
		Duration string `json:"duration"`
		Image    string `json:"image"`
		ByArtist struct {
			Name string `json:"name"`
		} `json:"byArtist"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
	} `json:"track"`
}

type nextDataEntity struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	URI      string `json:"uri"`
	Duration int    `json:"duration"`
	Artists  []struct {
		Name string `json:"name"`
		URI  string `json:"uri"`
	} `json:"artists"`
	Album   *struct {
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
	TrackNumber    int    `json:"track_number"`
	VisualIdentity *struct {
		Image []struct {
			URL       string `json:"url"`
			MaxHeight int    `json:"maxHeight"`
			MaxWidth  int    `json:"maxWidth"`
		} `json:"image"`
	} `json:"visualIdentity"`
	Items     []nextDataPlaylistItem `json:"items"`
	TrackList *struct {
		Items []nextDataPlaylistItem `json:"items"`
	} `json:"trackList"`
}

type nextDataPlaylistItem struct {
	Track *nextDataEntity `json:"track"`
}

type nextDataState struct {
	Data *struct {
		Entity *nextDataEntity `json:"entity"`
	} `json:"data"`
}

func extractTracksFromEntity(e *nextDataEntity) ([]download.TrackInfo, string) {
	var tracks []download.TrackInfo
	collectionArtist := ""
	if len(e.Artists) > 0 {
		collectionArtist = e.Artists[0].Name
	}

	var items []nextDataPlaylistItem
	if len(e.Items) > 0 {
		items = e.Items
	} else if e.TrackList != nil && len(e.TrackList.Items) > 0 {
		items = e.TrackList.Items
	}

	for _, item := range items {
		t := item.Track
		if t == nil || t.Name == "" {
			continue
		}
		artist := ""
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}
		if artist == "" {
			artist = collectionArtist
		}
		albumName := ""
		if t.Album != nil {
			albumName = t.Album.Name
		}
		thumbnail := getEntityThumbnail(t)
		tracks = append(tracks, download.TrackInfo{
			Title:       htmlUnescape(t.Name),
			Artist:      htmlUnescape(artist),
			Album:       htmlUnescape(albumName),
			DurationSec: float64(t.Duration) / 1000.0,
			Thumbnail:   thumbnail,
			TrackNum:    t.TrackNumber,
		})
	}
	return tracks, collectionArtist
}

func getEntityThumbnail(e *nextDataEntity) string {
	if e.Album != nil && len(e.Album.Images) > 0 {
		return e.Album.Images[0].URL
	}
	if e.VisualIdentity != nil && len(e.VisualIdentity.Image) > 0 {
		return e.VisualIdentity.Image[0].URL
	}
	return ""
}

func fetchSpotifyPage(pageURL string) (string, error) {
	WaitSpotify()

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://open.spotify.com/")
	req.Header.Set("DNT", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := spotifyClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return string(body), nil
}

func extractNextData(html string) *nextDataState {
	m := nextDataRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil
	}
	var parsed struct {
		Props *struct {
			PageProps *struct {
				State *nextDataState `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
		return nil
	}
	if parsed.Props != nil && parsed.Props.PageProps != nil {
		return parsed.Props.PageProps.State
	}
	return nil
}

func parseISO8601(dur string) float64 {
	if dur == "" {
		return 0
	}
	total := 0.0
	idx := 0
	n := len(dur)
	for idx < n {
		if dur[idx] == 'P' || dur[idx] == 'T' {
			idx++
			continue
		}
		start := idx
		for idx < n && ((dur[idx] >= '0' && dur[idx] <= '9') || dur[idx] == '.') {
			idx++
		}
		if idx == start {
			idx++
			continue
		}
		val := 0.0
		fmt.Sscanf(dur[start:idx], "%f", &val)
		if idx >= n {
			break
		}
		switch dur[idx] {
		case 'H':
			total += val * 3600
		case 'M':
			total += val * 60
		case 'S':
			total += val
		}
		idx++
	}
	return total
}

func extractLDJSON[T any](html string) []T {
	var results []T
	matches := ldjsonRe.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		raw := strings.TrimSpace(m[1])
		// Try single object
		var obj T
		if json.Unmarshal([]byte(raw), &obj) == nil {
			results = append(results, obj)
			continue
		}
		// Try array
		var arr []T
		if json.Unmarshal([]byte(raw), &arr) == nil {
			results = append(results, arr...)
		}
	}
	return results
}

// parseSpotifyID extracts the track/album/playlist ID from various Spotify URL formats.
func parseSpotifyID(rawURL string) (entity string, id string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	if u.Scheme == "spotify" {
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

// FetchSpotifyTrack fetches track metadata from Spotify by scraping the embed page.
func FetchSpotifyTrack(rawURL string) (*download.TrackInfo, error) {
	entity, id, err := parseSpotifyID(rawURL)
	if err != nil {
		return nil, err
	}
	if entity != "track" {
		return nil, fmt.Errorf("expected track, got %s", entity)
	}

	pageURL := "https://open.spotify.com/embed/track/" + id
	html, err := fetchSpotifyPage(pageURL)
	if err != nil {
		return nil, fmt.Errorf("fetch track page: %w", err)
	}

	// Try __NEXT_DATA__ first (embed page has SSR data)
	if state := extractNextData(html); state != nil && state.Data != nil && state.Data.Entity != nil {
		e := state.Data.Entity
		if e.Name != "" {
			artist := ""
			if len(e.Artists) > 0 {
				artist = e.Artists[0].Name
			}
			return &download.TrackInfo{
				Title:       htmlUnescape(e.Name),
				Artist:      htmlUnescape(artist),
				DurationSec: float64(e.Duration) / 1000.0,
				Thumbnail:   getEntityThumbnail(e),
			}, nil
		}
	}

	// Fallback: ld+json
	records := extractLDJSON[ldMusicRecording](html)
	for _, r := range records {
		if r.Type == "MusicRecording" && r.Name != "" {
			artist := r.ByArtist.Name
			album := r.Album.Name
			if album == "" {
				if desc := extractMeta(html, "og:description"); desc != "" {
					if parts := strings.SplitN(desc, " · ", 2); len(parts) >= 1 {
						artist = strings.TrimSpace(parts[0])
					}
				}
			}
			return &download.TrackInfo{
				Title:       htmlUnescape(r.Name),
				Artist:      htmlUnescape(artist),
				Album:       htmlUnescape(album),
				DurationSec: parseISO8601(r.Duration),
				Thumbnail:   r.Image,
				TrackNum:    r.TrackNumber,
			}, nil
		}
	}

	// Last resort: Open Graph / meta tags
	title := extractMeta(html, "og:title")
	if title == "" {
		return nil, fmt.Errorf("could not extract track metadata from page")
	}
	desc := extractMeta(html, "og:description")
	thumbnail := extractMeta(html, "og:image")
	durStr := extractMeta(html, "music:duration")

	artist := ""
	album := ""
	if desc != "" {
		if parts := strings.SplitN(desc, " · ", 2); len(parts) >= 1 {
			artist = strings.TrimSpace(parts[0])
		}
		if parts := strings.SplitN(desc, " · ", 2); len(parts) == 2 {
			album = strings.TrimSpace(parts[1])
		}
	}

	durationSec := 0.0
	if durStr != "" {
		fmt.Sscanf(durStr, "%f", &durationSec)
	}

	return &download.TrackInfo{
		Title:       htmlUnescape(title),
		Artist:      htmlUnescape(artist),
		Album:       htmlUnescape(album),
		DurationSec: durationSec,
		Thumbnail:   thumbnail,
	}, nil
}

// SearchSpotifyTrack is not available without API; use YouTube search instead.
func SearchSpotifyTrack(query string) (*download.TrackInfo, error) {
	return nil, fmt.Errorf("Spotify search requires API credentials; use YouTube search instead")
}

type ldMusicAlbum struct {
	Type      string `json:"@type"`
	Name      string `json:"name"`
	ByArtist  struct {
		Name string `json:"name"`
	} `json:"byArtist"`
	Image string `json:"image"`
	Track []struct {
		Type     string `json:"@type"`
		Name     string `json:"name"`
		Duration string `json:"duration"`
		Image    string `json:"image"`
		ByArtist struct {
			Name string `json:"name"`
		} `json:"byArtist"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
	} `json:"track"`
}

// FetchSpotifyPlaylistMetadata scrapes a Spotify playlist or album page for name and track list.
func FetchSpotifyPlaylistMetadata(collectionURL string) (name string, tracks []download.TrackInfo, err error) {
	entity, id, err := parseSpotifyID(collectionURL)
	if err != nil {
		return "", nil, err
	}
	if entity != "playlist" && entity != "album" {
		return "", nil, fmt.Errorf("expected playlist or album, got %s", entity)
	}

	var pageURL, fallbackURL string
	if entity == "album" {
		pageURL = "https://open.spotify.com/embed/album/" + id
		fallbackURL = "https://open.spotify.com/album/" + id
	} else {
		pageURL = "https://open.spotify.com/embed/playlist/" + id
		fallbackURL = "https://open.spotify.com/playlist/" + id
	}

	html, fetchErr := fetchSpotifyPage(pageURL)
	if fetchErr != nil {
		html, fetchErr = fetchSpotifyPage(fallbackURL)
		if fetchErr != nil {
			return "", nil, fmt.Errorf("fetch page: %w", fetchErr)
		}
	}

	// Try ld+json: MusicPlaylist or MusicAlbum with track list
	playlists := extractLDJSON[ldMusicPlaylist](html)
	for _, pl := range playlists {
		if pl.Type == "MusicPlaylist" && pl.Name != "" {
			plName := htmlUnescape(pl.Name)
			tracks := make([]download.TrackInfo, 0, len(pl.Track))
			for _, t := range pl.Track {
				if t.Type != "MusicRecording" || t.Name == "" {
					continue
				}
				tracks = append(tracks, download.TrackInfo{
					Title:       htmlUnescape(t.Name),
					Artist:      htmlUnescape(t.ByArtist.Name),
					Album:       htmlUnescape(t.Album.Name),
					DurationSec: parseISO8601(t.Duration),
					Thumbnail:   t.Image,
					Playlist:    plName,
				})
			}
			return plName, tracks, nil
		}
	}

	albums := extractLDJSON[ldMusicAlbum](html)
	for _, al := range albums {
		if al.Type == "MusicAlbum" && al.Name != "" {
			alName := htmlUnescape(al.Name)
			alArtist := htmlUnescape(al.ByArtist.Name)
			tracks := make([]download.TrackInfo, 0, len(al.Track))
			for _, t := range al.Track {
				if t.Type != "MusicRecording" || t.Name == "" {
					continue
				}
				artist := htmlUnescape(t.ByArtist.Name)
				if artist == "" {
					artist = alArtist
				}
				alb := htmlUnescape(t.Album.Name)
				if alb == "" {
					alb = alName
				}
				tracks = append(tracks, download.TrackInfo{
					Title:       htmlUnescape(t.Name),
					Artist:      artist,
					Album:       alb,
					DurationSec: parseISO8601(t.Duration),
					Thumbnail:   t.Image,
					Playlist:    alName,
				})
			}
			return alName, tracks, nil
		}
	}

	// Fallback: MusicRecording entries not wrapped in a parent type
	records := extractLDJSON[ldMusicRecording](html)
	if len(records) > 0 {
		tracks := make([]download.TrackInfo, 0, len(records))
		for _, r := range records {
			if r.Type == "MusicRecording" && r.Name != "" {
				tracks = append(tracks, download.TrackInfo{
					Title:       htmlUnescape(r.Name),
					Artist:      htmlUnescape(r.ByArtist.Name),
					Album:       htmlUnescape(r.Album.Name),
					DurationSec: parseISO8601(r.Duration),
					Thumbnail:   r.Image,
				})
			}
		}
		if len(tracks) > 0 {
			plName := extractMeta(html, "og:title")
			return plName, tracks, nil
		}
	}

	// Try __NEXT_DATA__ fallback
	if state := extractNextData(html); state != nil && state.Data != nil && state.Data.Entity != nil {
		e := state.Data.Entity
		if (e.Type == "playlist" || e.Type == "album") && e.Name != "" {
			plName := htmlUnescape(e.Name)
			if len(e.Artists) > 0 && e.Album != nil {
				plName = htmlUnescape(e.Album.Name)
			}
			tracks, _ := extractTracksFromEntity(e)
			if len(tracks) > 0 {
				// Ensure tracks missing an album inherit the collection name
				for i := range tracks {
					if tracks[i].Album == "" {
						tracks[i].Album = plName
					}
				}
				return plName, tracks, nil
			}
		}
	}

	return "", nil, fmt.Errorf("could not extract collection data from page")
}
