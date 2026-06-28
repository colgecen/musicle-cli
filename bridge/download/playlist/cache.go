package playlist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// CachedTrack holds metadata for a downloaded track in a playlist.
type CachedTrack struct {
	VideoID    string  `json:"video_id,omitempty"`
	SpotifyID  string  `json:"spotify_id,omitempty"`
	Title      string  `json:"title"`
	Artist     string  `json:"artist"`
	Album      string  `json:"album"`
	DurationSec float64 `json:"duration_sec"`
	TrackNum   int     `json:"track_num"`
	Filename   string  `json:"filename"`
}

// PlaylistCache holds metadata for an entire downloaded playlist.
type PlaylistCache struct {
	mu         sync.Mutex
	Path       string         `json:"-"`
	Name       string         `json:"name"`
	SourceURL  string         `json:"source_url"`
	Tracks     []CachedTrack  `json:"tracks"`
	TrackByID  map[string]int `json:"-"` // videoID -> index
}

// LoadPlaylistCache loads or creates a playlist cache file.
func LoadPlaylistCache(cachePath string) (*PlaylistCache, error) {
	pc := &PlaylistCache{
		Path:      cachePath,
		TrackByID: make(map[string]int),
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return pc, nil
		}
		return nil, fmt.Errorf("read cache: %w", err)
	}
	if err := json.Unmarshal(data, pc); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	for i, t := range pc.Tracks {
		if t.VideoID != "" {
			pc.TrackByID[t.VideoID] = i
		}
	}
	return pc, nil
}

// HasVideoID checks if a video ID was already downloaded.
func (pc *PlaylistCache) HasVideoID(videoID string) bool {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	_, ok := pc.TrackByID[videoID]
	return ok
}

// AddTrack adds a track to the cache.
func (pc *PlaylistCache) AddTrack(track CachedTrack) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.Tracks = append(pc.Tracks, track)
	if track.VideoID != "" {
		pc.TrackByID[track.VideoID] = len(pc.Tracks) - 1
	}
}

// Save writes the cache to disk.
func (pc *PlaylistCache) Save() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(pc.Path), 0755); err != nil {
		return fmt.Errorf("mkdir cache: %w", err)
	}
	return os.WriteFile(pc.Path, data, 0644)
}

// TrackCount returns the number of cached tracks.
func (pc *PlaylistCache) TrackCount() int {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return len(pc.Tracks)
}
