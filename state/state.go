package state

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Language codes
type Language string

const (
	LangEnglish Language = "en"
	LangTurkish Language = "tr"
)

// T returns localized text (en or tr)
func T(lang Language, en, tr string) string {
	if lang == LangTurkish {
		return tr
	}
	return en
}

// Song represents a single audio track
type Song struct {
	Filename  string
	Title     string
	Artist    string
	DateAdded string
	Duration  string
	FilePath  string
}

// Playlist represents a named collection of songs
type Playlist struct {
	FolderName string
	Name       string
	Bio        string
	ArtPath    string
	Songs      []Song
	IsPrivate  bool
}

// Profile represents a user profile
type Profile struct {
	FolderName  string
	DisplayName string
	Bio         string
	Language    Language
	AvatarPath  string
	Playlists   []Playlist
}

// PlayerState tracks audio playback state
type PlayerState struct {
	IsPlaying   bool
	IsPaused    bool
	CurrentSong *Song
	Position    float64 // seconds
	Duration    float64 // seconds
	Volume      float64 // 0.0-1.0
	IsShuffled  bool
	IsPrivate   bool
	StatusMsg   string
	IsError     bool
	Format      string  // e.g. "MP3", "FLAC"
	SampleRate  int     // e.g. 44100
	Bitrate     int     // e.g. 320 (kbps)
	AudioLevelL float64 // 0.0-1.0 VU left
	AudioLevelR float64 // 0.0-1.0 VU right
}

// AppState is the central singleton state
type AppState struct {
	RootDir         string
	Language        Language
	Profiles        []Profile
	CurrentProfile  *Profile
	CurrentPlaylist *Playlist
	Player          PlayerState
	IsFirstLaunch   bool
	ConfigDir       string
	NetworkOnline   bool
	Theme           string // accent color theme name
}

// Current is the global app state
var Current = &AppState{
	Player:   PlayerState{Volume: 0.7},
	Language: LangEnglish,
	Theme:    "green",
}

// savedConfig is the on-disk persistent config format
type savedConfig struct {
	RootDir  string   `json:"root_dir"`
	Language Language `json:"language"`
	LastUser string   `json:"last_user"`
	Theme    string   `json:"theme"`
}

func (a *AppState) configPath() string {
	return filepath.Join(a.ConfigDir, "config.json")
}

// LoadConfig reads config.json from disk
func (a *AppState) LoadConfig() error {
	data, err := os.ReadFile(a.configPath())
	if err != nil {
		return err
	}
	var cfg savedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	a.RootDir = cfg.RootDir
	a.Language = cfg.Language
	a.Theme = cfg.Theme
	if a.Theme == "" {
		a.Theme = "green"
	}
	return nil
}

// SaveConfig writes config.json to disk
func (a *AppState) SaveConfig() error {
	if err := os.MkdirAll(a.ConfigDir, 0755); err != nil {
		return err
	}
	cfg := savedConfig{RootDir: a.RootDir, Language: a.Language, Theme: a.Theme}
	if a.CurrentProfile != nil {
		cfg.LastUser = a.CurrentProfile.FolderName
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath(), data, 0644)
}

// InitializeBaseDirs creates the root music and profiles directory early
func (a *AppState) InitializeBaseDirs(rootDir string) error {
	a.RootDir = rootDir
	return os.MkdirAll(a.ProfilesDir(), 0755)
}

// ProfilesDir returns the profiles/ root directory
func (a *AppState) ProfilesDir() string {
	return filepath.Join(a.RootDir, "profiles")
}

// ScanProfiles re-scans the profiles/ directory and populates a.Profiles
func (a *AppState) ScanProfiles() error {
	dir := a.ProfilesDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	a.Profiles = nil
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := loadProfile(filepath.Join(dir, e.Name()), e.Name())
		if err == nil {
			a.Profiles = append(a.Profiles, p)
		}
	}
	return nil
}

func loadProfile(dir, folderName string) (Profile, error) {
	p := Profile{
		FolderName:  folderName,
		DisplayName: readTxt(filepath.Join(dir, "name.txt"), folderName),
		Bio:         readTxt(filepath.Join(dir, "bio.txt"), ""),
		Language:    Language(readTxt(filepath.Join(dir, "lang.txt"), "en")),
	}
	// Find avatar image
	avatarDir := filepath.Join(dir, "avatar")
	if entries, err := os.ReadDir(avatarDir); err == nil {
		for _, e := range entries {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				p.AvatarPath = filepath.Join(avatarDir, e.Name())
				break
			}
		}
	}
	// Scan playlists
	plDir := filepath.Join(dir, "playlists")
	if entries, err := os.ReadDir(plDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pl, err := LoadPlaylist(filepath.Join(plDir, e.Name()), e.Name())
			if err == nil {
				p.Playlists = append(p.Playlists, pl)
			}
		}
	}
	return p, nil
}

// LoadPlaylist loads a playlist from its directory
func LoadPlaylist(dir, folderName string) (Playlist, error) {
	pl := Playlist{
		FolderName: folderName,
		Name:       readTxt(filepath.Join(dir, "playlist_name.txt"), folderName),
		Bio:        readTxt(filepath.Join(dir, "playlist_bio.txt"), ""),
	}
	pl.ArtPath = findArtFile(dir)
	if pl.ArtPath == "" {
		pl.ArtPath = findArtFile(filepath.Join(dir, "playlist_art"))
	}
	pl.Songs = parseSongList(filepath.Join(dir, "song_list.txt"), dir)
	return pl, nil
}

func findArtFile(dir string) string {
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func parseSongList(listPath, plDir string) []Song {
	data, err := os.ReadFile(listPath)
	if err != nil {
		return nil
	}
	var songs []Song
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}
		songs = append(songs, Song{
			Filename:  parts[0],
			Title:     parts[1],
			Artist:    parts[2],
			DateAdded: parts[3],
			Duration:  strings.TrimSpace(parts[4]),
			FilePath:  filepath.Join(plDir, parts[0]),
		})
	}
	return songs
}

// CreateProfileStructure writes the full directory/file scaffold for a new profile
func (a *AppState) CreateProfileStructure(folderName, displayName, bio, avatarSrc string, lang Language) error {
	profileDir := filepath.Join(a.ProfilesDir(), folderName)
	for _, d := range []string{profileDir, filepath.Join(profileDir, "avatar"), filepath.Join(profileDir, "playlists")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	if err := writeTxt(filepath.Join(profileDir, "name.txt"), displayName); err != nil {
		return err
	}
	if err := writeTxt(filepath.Join(profileDir, "bio.txt"), bio); err != nil {
		return err
	}
	if err := writeTxt(filepath.Join(profileDir, "lang.txt"), string(lang)); err != nil {
		return err
	}
	if avatarSrc != "" {
		ext := filepath.Ext(avatarSrc)
		if ext == "" {
			ext = ".jpg"
		}
		_ = CopyFile(avatarSrc, filepath.Join(profileDir, "avatar", "avatar"+ext))
	}
	return nil
}

// CreatePlaylistStructure writes the full directory/file scaffold for a new playlist
func (a *AppState) CreatePlaylistStructure(profileFolder, plFolder, plName, plBio, artSrc string) error {
	plDir := filepath.Join(a.ProfilesDir(), profileFolder, "playlists", plFolder)
	if err := os.MkdirAll(plDir, 0755); err != nil {
		return err
	}
	if err := writeTxt(filepath.Join(plDir, "playlist_name.txt"), plName); err != nil {
		return err
	}
	if err := writeTxt(filepath.Join(plDir, "playlist_bio.txt"), plBio); err != nil {
		return err
	}
	if artSrc != "" {
		ext := filepath.Ext(artSrc)
		if ext == "" {
			ext = ".jpg"
		}
		_ = CopyFile(artSrc, filepath.Join(plDir, "art"+ext))
	}
	return nil
}

// AppendSong appends a song entry line to song_list.txt
func AppendSong(listPath, filename, title, artist, duration string) error {
	entry := strings.Join([]string{filename, title, artist, time.Now().Format("2006-01-02"), duration}, "|") + "\n"
	f, err := os.OpenFile(listPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// SaveProfileMeta updates name.txt and bio.txt
func (a *AppState) SaveProfileMeta(folderName, displayName, bio string) error {
	dir := filepath.Join(a.ProfilesDir(), folderName)
	if err := writeTxt(filepath.Join(dir, "name.txt"), displayName); err != nil {
		return err
	}
	return writeTxt(filepath.Join(dir, "bio.txt"), bio)
}

// SavePlaylistMeta updates playlist_name.txt and playlist_bio.txt
func (a *AppState) SavePlaylistMeta(profileFolder, plFolder, name, bio string) error {
	dir := filepath.Join(a.ProfilesDir(), profileFolder, "playlists", plFolder)
	if err := writeTxt(filepath.Join(dir, "playlist_name.txt"), name); err != nil {
		return err
	}
	return writeTxt(filepath.Join(dir, "playlist_bio.txt"), bio)
}

// DeletePlaylist removes a playlist directory entirely
func (a *AppState) DeletePlaylist(profileFolder, plFolder string) error {
	return os.RemoveAll(filepath.Join(a.ProfilesDir(), profileFolder, "playlists", plFolder))
}

// SongListPath returns the path to song_list.txt for the given playlist
func (a *AppState) SongListPath(profileFolder, plFolder string) string {
	return filepath.Join(a.ProfilesDir(), profileFolder, "playlists", plFolder, "song_list.txt")
}

// PlaylistDir returns the directory for the given playlist
func (a *AppState) PlaylistDir(profileFolder, plFolder string) string {
	return filepath.Join(a.ProfilesDir(), profileFolder, "playlists", plFolder)
}

// ---- helpers ----

func readTxt(path, def string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	return strings.TrimSpace(string(data))
}

func writeTxt(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// CopyFile copies src file to dst (creates parent dirs)
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}
