package playlist

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"MusicLeCLI/bridge/download"
	"MusicLeCLI/bridge/download/music"
)

// SpotifyPlaylistEntry is a single track from a Spotify playlist with stored audio bytes.
type SpotifyPlaylistEntry struct {
	download.TrackInfo
	Index int
}

// FetchSpotifyPlaylist fetches a Spotify playlist's track metadata via web scraping.
func FetchSpotifyPlaylist(playlistURL string) (name string, entries []SpotifyPlaylistEntry, err error) {
	plName, tracks, err := music.FetchSpotifyPlaylistMetadata(playlistURL)
	if err != nil {
		return "", nil, err
	}

	entries = make([]SpotifyPlaylistEntry, 0, len(tracks))
	for i, t := range tracks {
		t.Playlist = plName
		entries = append(entries, SpotifyPlaylistEntry{
			TrackInfo: t,
			Index:     i + 1,
		})
	}
	return plName, entries, nil
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
