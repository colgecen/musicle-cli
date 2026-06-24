package bridge

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"MusicLeCLI/state"
)

// downloadYouTube downloads a YouTube URL using yt-dlp.
func downloadYouTube(url, outputDir string) *Result {
	if !strings.HasPrefix(url, "http") {
		return &Result{Status: "error", Error: "invalid URL"}
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("create dir: %v", err)}
	}

	outTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")
	ytdl := findExternalCmd([]string{"yt-dlp", "yt-dlp.exe", "youtube-dl", "youtube-dl.exe"})
	if ytdl == "" {
		ytdl = "python"
	}

	var args []string
	if ytdl == "python" {
		args = []string{"-m", "yt_dlp"}
	} else {
		args = []string{}
	}

	args = append(args,
		url,
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "192K",
		"--output", outTemplate,
		"--no-playlist",
		"--print", "after_move:filepath",
		"--newline",
		"--add-metadata",
		"--parse-metadata", "%(uploader)s:%(artist)s",
		"--embed-thumbnail",
	)

	result := runProgressCommand(ytdl, args)
	if result.Status == "error" {
		return result
	}

	filepathStr := strings.TrimSpace(result.Message)
	if filepathStr == "" || !fileExists(filepathStr) {
		filepathStr = latestFile(outputDir, ".mp3")
	}
	if filepathStr == "" {
		return &Result{Status: "error", Error: "downloaded file not found"}
	}

	return finalizeDownload(filepathStr, outputDir)
}

// downloadSpotify downloads a Spotify URL using spotdl.
func downloadSpotify(url, outputDir string) *Result {
	if !strings.HasPrefix(url, "http") {
		return &Result{Status: "error", Error: "invalid URL"}
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("create dir: %v", err)}
	}

	// Snapshot existing mp3s
	before := listMP3s(outputDir)

	outTemplate := filepath.Join(outputDir, "{title} - {artist}.{ext}")
	spotdl := findExternalCmd([]string{"spotdl", "spotdl.exe"})

	var cmd *exec.Cmd
	if spotdl == "" {
		// Try python -m spotdl
		args := []string{"-m", "spotdl", url, "--output", outTemplate, "--format", "mp3", "--bitrate", "192k"}
		cmd = exec.Command("python", args...)
	} else {
		args := []string{url, "--output", outTemplate, "--format", "mp3", "--bitrate", "192k"}
		cmd = exec.Command(spotdl, args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("spotdl failed: %v\n%s", err, string(output))}
	}

	// Find new files
	after := listMP3s(outputDir)
	newFiles := diffStrings(after, before)
	if len(newFiles) == 0 {
		return &Result{Status: "error", Error: "downloaded file not found"}
	}

	var songs []Result
	for _, fname := range newFiles {
		fpath := filepath.Join(outputDir, fname)
		meta := finalizeDownload(fpath, outputDir)
		songs = append(songs, *meta)
	}

	return &Result{
		Status:  "ok",
		Message: fmt.Sprintf("Downloaded %d song(s)", len(songs)),
		Songs:   songs,
	}
}

func finalizeDownload(filepathStr, outputDir string) *Result {
	meta := extractMetadata(filepathStr)
	if meta.Status == "error" {
		meta = &Result{
			Title:  strings.TrimSuffix(filepath.Base(filepathStr), filepath.Ext(filepathStr)),
			Artist: "Unknown",
		}
	}

	filename := filepath.Base(filepathStr)
	durStr := fmtDuration(meta.Duration)

	listPath := filepath.Join(outputDir, "song_list.txt")
	_ = state.AppendSong(listPath, filename, meta.Title, meta.Artist, durStr)

	return &Result{
		Status:   "ok",
		Filename: filename,
		Title:    meta.Title,
		Artist:   meta.Artist,
		Duration: meta.Duration,
		ArtPath:  meta.ArtPath,
	}
}

func runProgressCommand(bin string, args []string) *Result {
	cmd := exec.Command(bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("stdout pipe: %v", err)}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("stderr pipe: %v", err)}
	}

	if err := cmd.Start(); err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("start: %v", err)}
	}

	CurrentDownload.Set(true, 0, "Starting...")
	re := regexp.MustCompile(`(\d+\.?\d*)%`)
	var lastLine string

	// Read stderr for progress
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			pct := 0.0
			fmt.Sscanf(m[1], "%f", &pct)
			CurrentDownload.Set(true, pct, fmt.Sprintf("%.0f%%", pct))
		}
	}

	// Read stdout for final filepath
	scanOut := bufio.NewScanner(stdout)
	for scanOut.Scan() {
		lastLine = scanOut.Text()
	}

	err = cmd.Wait()
	CurrentDownload.Set(false, 100, "Done")
	if err != nil {
		CurrentDownload.Set(false, 0, "Error")
		return &Result{Status: "error", Error: fmt.Sprintf("exit: %v", err)}
	}

	return &Result{Status: "ok", Message: lastLine}
}

func findExternalCmd(names []string) string {
	for _, name := range names {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func latestFile(dir, ext string) string {
	var newest string
	var newestMod int64
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ext) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mod := info.ModTime().UnixNano()
		if mod > newestMod {
			newestMod = mod
			newest = filepath.Join(dir, e.Name())
		}
	}
	return newest
}

func listMP3s(dir string) []string {
	var files []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp3") {
			files = append(files, e.Name())
		}
	}
	return files
}

func diffStrings(after, before []string) []string {
	beforeSet := make(map[string]bool)
	for _, s := range before {
		beforeSet[s] = true
	}
	var diff []string
	for _, s := range after {
		if !beforeSet[s] {
			diff = append(diff, s)
		}
	}
	return diff
}
