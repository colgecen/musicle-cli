package playlist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"MusicLeCLI/bridge/download/music"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// PlaylistEntry is a single track from a playlist.
type PlaylistEntry struct {
	VideoID string
	Title   string
	Index   int
}

// fetchPlaylistPage fetches a YouTube playlist page.
func fetchPlaylistPage(playlistID string) (string, error) {
	url := fmt.Sprintf("https://www.youtube.com/playlist?list=%s", playlistID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create req: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get playlist: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return string(body), nil
}

// These regexes match video entries in ytInitialData JSON on playlist pages.
var (
	// Matches: "videoId":"XXXXXXXXXXX"
	videoIDRe  = regexp.MustCompile(`"videoId"\s*:\s*"([a-zA-Z0-9_-]{11})"`)
	// Matches title runs within playlist video renderer context
	titleBlockRe = regexp.MustCompile(`"title"\s*:\s*\{\s*"runs"\s*:\s*\[\s*\{[^}]*"text"\s*:\s*"([^"]+)"`)
)

// extractPlaylistEntries extracts video IDs and titles from a playlist page.
func extractPlaylistEntries(html string) ([]PlaylistEntry, error) {
	// Find ytInitialData JSON
	re := regexp.MustCompile(`window\[['"]ytInitialData['"]\]\s*=\s*({.*?});`)
	match := re.FindStringSubmatch(html)
	if len(match) < 2 {
		return nil, fmt.Errorf("ytInitialData not found in playlist page")
	}

	raw := match[1]
	// Unescape JSON if needed
	if strings.HasPrefix(raw, `"`) {
		var unescaped string
		if err := json.Unmarshal([]byte(raw), &unescaped); err == nil {
			raw = unescaped
		}
	}

	// Try to parse structured data first
	type playlistVideoRenderer struct {
		VideoID string `json:"videoId"`
		Title   struct {
			Runs []struct {
				Text string `json:"text"`
			} `json:"runs"`
		} `json:"title"`
	}

	type playlistVideoListRenderer struct {
		Contents []struct {
			PlaylistVideoRenderer *playlistVideoRenderer `json:"playlistVideoRenderer"`
		} `json:"contents"`
	}

	var data struct {
		Contents *struct {
			TwoColumnBrowseResultsRenderer *struct {
				Tabs []struct {
					TabRenderer *struct {
						Content *struct {
							SectionListRenderer *struct {
								Contents []struct {
									ItemSectionRenderer *struct {
										Contents []struct {
											PlaylistVideoListRenderer *playlistVideoListRenderer `json:"playlistVideoListRenderer"`
										} `json:"contents"`
									} `json:"itemSectionRenderer"`
								} `json:"contents"`
							} `json:"sectionListRenderer"`
						} `json:"content"`
					} `json:"tabRenderer"`
				} `json:"tabs"`
			} `json:"twoColumnBrowseResultsRenderer"`
		} `json:"contents"`
	}

	var entries []PlaylistEntry
	seen := make(map[string]bool)

	if err := json.Unmarshal([]byte(raw), &data); err == nil {
		if data.Contents != nil && data.Contents.TwoColumnBrowseResultsRenderer != nil {
			for _, tab := range data.Contents.TwoColumnBrowseResultsRenderer.Tabs {
				if tab.TabRenderer == nil || tab.TabRenderer.Content == nil ||
					tab.TabRenderer.Content.SectionListRenderer == nil {
					continue
				}
				for _, sec := range tab.TabRenderer.Content.SectionListRenderer.Contents {
					if sec.ItemSectionRenderer == nil {
						continue
					}
					for _, item := range sec.ItemSectionRenderer.Contents {
						if item.PlaylistVideoListRenderer == nil {
							continue
						}
						for _, v := range item.PlaylistVideoListRenderer.Contents {
							if v.PlaylistVideoRenderer == nil {
								continue
							}
							id := v.PlaylistVideoRenderer.VideoID
							if id == "" || seen[id] {
								continue
							}
							seen[id] = true
							title := ""
							if len(v.PlaylistVideoRenderer.Title.Runs) > 0 {
								title = v.PlaylistVideoRenderer.Title.Runs[0].Text
							}
							entries = append(entries, PlaylistEntry{
								VideoID: id,
								Title:   title,
								Index:   len(entries) + 1,
							})
						}
					}
				}
			}
		}
	}

	// Fallback: extract via regex
	if len(entries) == 0 {
		ids := videoIDRe.FindAllStringSubmatch(raw, -1)
		titles := titleBlockRe.FindAllStringSubmatch(raw, -1)
		for i, m := range ids {
			id := m[1]
			if seen[id] {
				continue
			}
			seen[id] = true
			title := ""
			if i < len(titles) {
				title = titles[i][1]
			}
			entries = append(entries, PlaylistEntry{
				VideoID: id,
				Title:   title,
				Index:   len(entries) + 1,
			})
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no videos found in playlist")
	}

	return entries, nil
}

// extractPlaylistName tries to get the playlist name from the page.
func extractPlaylistName(html string) string {
	re := regexp.MustCompile(`"title"\s*:\s*\{\s*"runs"\s*:\s*\[\s*\{[^}]*"text"\s*:\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ParseYouTubePlaylistURL extracts the playlist ID from various URL formats.
func ParseYouTubePlaylistURL(rawURL string) (string, error) {
	if strings.Contains(rawURL, "list=") {
		for _, part := range strings.Split(rawURL, "&") {
			if strings.HasPrefix(part, "list=") {
				id := strings.TrimPrefix(part, "list=")
				if id != "" && len(id) >= 13 {
					return id, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no playlist ID found in URL: %s", rawURL)
}

// FetchYouTubePlaylist fetches a YouTube playlist and returns all entries.
func FetchYouTubePlaylist(playlistURL string) (string, []PlaylistEntry, error) {
	playlistID, err := ParseYouTubePlaylistURL(playlistURL)
	if err != nil {
		return "", nil, err
	}

	html, err := fetchPlaylistPage(playlistID)
	if err != nil {
		return "", nil, err
	}

	name := extractPlaylistName(html)
	if name == "" {
		name = playlistID
	}

	entries, err := extractPlaylistEntries(html)
	if err != nil {
		return "", nil, err
	}

	return name, entries, nil
}

const (
	playlistMaxRetriesPerTrack = 2
	defaultConcurrency         = 3
)

// PlaylistConcurrency controls the number of concurrent downloads in parallel mode.
var PlaylistConcurrency = defaultConcurrency

// DownloadYouTubePlaylist downloads all tracks in a YouTube playlist (sequential).
// Each track is saved as "{Artist} - {Title}.mp3" in outputDir.
// Skips files that already exist (resume-friendly).
func DownloadYouTubePlaylist(playlistURL, outputDir string, progress func(pct int, msg string)) ([]string, error) {
	return DownloadYouTubePlaylistParallel(playlistURL, outputDir, 1, progress)
}

// DownloadYouTubePlaylistParallel downloads tracks concurrently with a worker pool.
func DownloadYouTubePlaylistParallel(playlistURL, outputDir string, workers int, progress func(pct int, msg string)) ([]string, error) {
	name, entries, err := FetchYouTubePlaylist(playlistURL)
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

	type result struct {
		file     string
		index    int
		err      error
		entryIdx int
	}

	jobs := make(chan struct {
		entry PlaylistEntry
		idx   int
	}, total)
	results := make(chan result, total)

	// Worker pool
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				entry, idx := job.entry, job.idx
				videoURL := "https://www.youtube.com/watch?v=" + entry.VideoID

				// Skip if already downloaded
				safeName := music.SafeFilename(entry.Title)
				existing := filepath.Join(outputDir, safeName+".mp3")
				if _, statErr := os.Stat(existing); statErr == nil {
					results <- result{file: existing, index: idx, entryIdx: job.idx}
					continue
				}

				var lastErr error
				for attempt := 0; attempt <= playlistMaxRetriesPerTrack; attempt++ {
					file, dlErr := music.DownloadYouTubeToFile(videoURL, outputDir, nil)
					if dlErr == nil {
						err2 := music.ReTagMP3(file, name, entry.Index)
						if err2 != nil {
							lastErr = err2
						} else {
							results <- result{file: file, index: idx, entryIdx: job.idx}
							lastErr = nil
							break
						}
					}
					lastErr = dlErr
				}

				if lastErr != nil {
					results <- result{err: lastErr, index: idx, entryIdx: job.idx}
				}
			}
		}()
	}

	// Send jobs
	for i, entry := range entries {
		jobs <- struct {
			entry PlaylistEntry
			idx   int
		}{entry, i + 1}
	}
	close(jobs)

	// Close results when all workers done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	files := make([]string, total)
	errs := make(map[int]string)
	completed := 0
	doneIdx := make(map[int]bool)

	for res := range results {
		if res.err != nil {
			errs[res.entryIdx] = fmt.Sprintf("[%d/%d] Error: %v", res.index, total, res.err)
		} else {
			files[res.entryIdx-1] = res.file
		}
		doneIdx[res.entryIdx] = true
		completed = len(doneIdx)

		if progress != nil {
			pct := completed * 100 / total
			if pct > 100 {
				pct = 100
			}
			progress(pct, fmt.Sprintf("[%d/%d] %d downloaded, %d errors", completed, total, len(files)-len(errs), len(errs)))
		}
	}

	// Filter out empty entries (failed downloads)
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


