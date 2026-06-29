package bridge

import (
	"fmt"
	"path/filepath"
	"strings"

	"MusicLeCLI/bridge/download/music"
	"MusicLeCLI/bridge/download/playlist"
)

// downloadYouTube downloads a YouTube URL using the pure Go pipeline.
func downloadYouTube(url, outputDir string) *Result {
	if url == "" {
		return &Result{Status: "error", Error: "invalid URL"}
	}

	CurrentDownload.Set(true, 0, "Bridge: Starting YouTube download...")

	filePath, err := music.DownloadYouTubeToFile(url, outputDir, func(pct int, msg string) {
		CurrentDownload.Set(true, float64(pct), fmt.Sprintf("YouTube: %s", msg))
	})
	if err != nil {
		CurrentDownload.Set(false, 0, fmt.Sprintf("Error: %v", err))
		return &Result{Status: "error", Error: fmt.Sprintf("YouTube download failed: %v", err)}
	}

	CurrentDownload.Set(false, 100, fmt.Sprintf("Saved: %s", filepath.Base(filePath)))

	meta := extractMetadata(filePath)
	if meta.Status == "error" {
		return &Result{
			Status:   "ok",
			Filename: filepath.Base(filePath),
			Title:    strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath)),
			Artist:   "Unknown",
		}
	}
	return meta
}

// downloadSpotify downloads a Spotify URL. Handles both track and playlist URLs.
func downloadSpotify(url, outputDir string) *Result {
	if url == "" {
		return &Result{Status: "error", Error: "invalid URL"}
	}

	CurrentDownload.Set(true, 0, "Starting...")

	// Detect URL type
	urlLower := strings.ToLower(url)
	isPlaylist := strings.Contains(urlLower, "/playlist/") || strings.Contains(urlLower, "spotify:playlist:")
	isAlbum := strings.Contains(urlLower, "/album/") || strings.Contains(urlLower, "spotify:album:")
	isTrack := strings.Contains(urlLower, "/track/") || strings.Contains(urlLower, "spotify:track:")

	if isPlaylist || isAlbum {
		return downloadSpotifyPlaylist(url, outputDir)
	}

	if !isTrack {
		// Try collection (playlist/album) detection as fallback
		return downloadSpotifyPlaylist(url, outputDir)
	}

	// Single track: fetch Spotify metadata, search YouTube, download
	spTrack, err := music.FetchSpotifyTrack(url)
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: fmt.Sprintf("Spotify metadata: %v", err)}
	}

	query := spTrack.Artist + " - " + spTrack.Title
	CurrentDownload.Set(true, 5, fmt.Sprintf("Searching YouTube: %s", query))

	videoID, _, err := music.SearchYouTubeTrack(query)
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: fmt.Sprintf("YouTube search: %v", err)}
	}

	CurrentDownload.Set(true, 10, "Downloading from YouTube...")

	// Download raw audio from YouTube
	_, rawAudio, err := music.DownloadYouTubeTrack(videoID, func(pct int, msg string) {
		CurrentDownload.Set(true, 10+float64(pct)*30/100, msg)
	})
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: fmt.Sprintf("YouTube download: %v", err)}
	}

	CurrentDownload.Set(true, 40, "Converting to MP3...")

	// Save with Spotify metadata
	spTrack.StreamURL = "https://www.youtube.com/watch?v=" + videoID
	spTrack.Format = "webm"
	spTrack.ContentLen = int64(len(rawAudio))

	filePath, err := music.SaveRawAsMP3(rawAudio, spTrack, outputDir, func(pct int, msg string) {
		CurrentDownload.Set(true, 40+float64(pct)*60/100, msg)
	})
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: fmt.Sprintf("convert: %v", err)}
	}

	CurrentDownload.Set(false, 100, "Done")

	return &Result{
		Status:   "ok",
		Filename: filepath.Base(filePath),
		Title:    spTrack.Title,
		Artist:   spTrack.Artist,
		Duration: spTrack.DurationSec,
	}
}

func downloadSpotifyPlaylist(spotifyURL, outputDir string) *Result {
	files, err := playlist.DownloadSpotifyPlaylist(spotifyURL, outputDir, func(pct int, msg string) {
		CurrentDownload.Set(true, float64(pct), msg)
	})
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: err.Error()}
	}

	CurrentDownload.Set(false, 100, fmt.Sprintf("Downloaded %d songs", len(files)))

	songs := make([]Result, 0, len(files))
	for _, f := range files {
		meta := extractMetadata(f)
		if meta.Status == "ok" {
			songs = append(songs, *meta)
		}
	}

	return &Result{
		Status:  "ok",
		Message: fmt.Sprintf("Downloaded %d song(s)", len(songs)),
		Songs:   songs,
	}
}
