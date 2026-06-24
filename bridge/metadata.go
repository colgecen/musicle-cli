package bridge

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
)

var supportedAudioExt = map[string]string{
	".mp3":  "MP3",
	".flac": "FLAC",
	".m4a":  "AAC",
	".mp4":  "AAC",
	".aac":  "AAC",
	".ogg":  "OGG",
	".wav":  "WAV",
	".opus": "Opus",
}

// extractMetadata reads audio file tags and returns a Result with title, artist, album, etc.
func extractMetadata(filePath string) *Result {
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	ext := strings.ToLower(filepath.Ext(filePath))
	result := &Result{
		Status:   "ok",
		Filename: filepath.Base(filePath),
		Title:    base,
		Artist:   "Unknown",
		Album:    "",
		Format:   supportedAudioExt[ext],
	}

	f, err := os.Open(filePath)
	if err != nil {
		result.Status = "error"
		result.Error = fmt.Sprintf("open file: %v", err)
		return result
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		// Return basic result without tags
		return result
	}

	if title := meta.Title(); title != "" {
		result.Title = title
	}
	if artist := meta.Artist(); artist != "" {
		result.Artist = artist
	}
	if album := meta.Album(); album != "" {
		result.Album = album
	}

	// Album art
	if pic := meta.Picture(); pic != nil {
		artPath := saveAlbumArt(pic.Data, filePath)
		if artPath != "" {
			result.ArtPath = artPath
		}
	}

	return result
}

// saveAlbumArt saves album art bytes alongside the audio file and returns the path.
func saveAlbumArt(data []byte, audioPath string) string {
	h := fmt.Sprintf("%x", md5.Sum(data[:min64(len(data))]))[:8]
	artDir := filepath.Join(filepath.Dir(audioPath), "_art")
	if err := os.MkdirAll(artDir, 0755); err != nil {
		return ""
	}
	artPath := filepath.Join(artDir, "art_"+h+".jpg")
	if _, err := os.Stat(artPath); err == nil {
		return artPath
	}
	if err := os.WriteFile(artPath, data, 0644); err != nil {
		return ""
	}
	return artPath
}

func min64(n int) int {
	if n > 64 {
		return 64
	}
	return n
}
