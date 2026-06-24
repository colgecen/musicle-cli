package bridge

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

	ytdl, err := ensureTool()
	if err != nil {
		return &Result{Status: "error", Error: err.Error()}
	}

	outTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")
	args := []string{
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
	}

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

// downloadSpotify downloads a Spotify URL using yt-dlp (auto-downloaded) or spotdl if installed.
func downloadSpotify(url, outputDir string) *Result {
	if !strings.HasPrefix(url, "http") {
		return &Result{Status: "error", Error: "invalid URL"}
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("create dir: %v", err)}
	}

	// Try spotdl first (if user has it installed separately)
	if p, err := exec.LookPath("spotdl"); err == nil {
		return downloadSpotifyWithSpotdl(url, outputDir, p)
	}
	if p, err := exec.LookPath("spotdl.exe"); err == nil {
		return downloadSpotifyWithSpotdl(url, outputDir, p)
	}

	// Fall back to yt-dlp (auto-downloaded)
	ytdl, err := ensureTool()
	if err != nil {
		return &Result{Status: "error", Error: "spotdl not found, and yt-dlp could not be downloaded: " + err.Error()}
	}

	outTemplate := filepath.Join(outputDir, "%(title)s.%(ext)s")
	args := []string{
		url,
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "192K",
		"--output", outTemplate,
		"--no-playlist",
		"--print", "after_move:filepath",
		"--newline",
	}

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

func downloadSpotifyWithSpotdl(url, outputDir, spotdlBin string) *Result {
	before := listMP3s(outputDir)
	outTemplate := filepath.Join(outputDir, "{title} - {artist}.{ext}")

	var cmd *exec.Cmd
	if strings.Contains(spotdlBin, "python") {
		args := []string{"-m", "spotdl", url, "--output", outTemplate, "--format", "mp3", "--bitrate", "192k"}
		cmd = exec.Command("python", args...)
	} else {
		args := []string{url, "--output", outTemplate, "--format", "mp3", "--bitrate", "192k"}
		cmd = exec.Command(spotdlBin, args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &Result{Status: "error", Error: fmt.Sprintf("spotdl failed: %v\n%s", err, string(output))}
	}

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

// ensureTool ensures yt-dlp is available: checks PATH, then cached binary, then downloads.
func ensureTool() (string, error) {
	// 1. Check PATH
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("yt-dlp.exe"); err == nil {
		return p, nil
	}

	// 2. Check cached binary in config dir
	binDir := filepath.Join(state.Current.ConfigDir, "bin")
	binName := "yt-dlp"
	if runtime.GOOS == "windows" {
		binName = "yt-dlp.exe"
	}
	cached := filepath.Join(binDir, binName)
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	// 3. Download yt-dlp
	_ = os.MkdirAll(binDir, 0755)

	url := ytDlpDownloadURL()
	if err := downloadFile(cached, url); err != nil {
		return "", fmt.Errorf("could not download yt-dlp: %w", err)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		_ = os.Chmod(cached, 0755)
	}

	return cached, nil
}

func ytDlpDownloadURL() string {
	base := "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	switch runtime.GOOS {
	case "windows":
		return base + ".exe"
	case "darwin":
		return base + "_macos"
	default:
		return base
	}
}

func downloadFile(path, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
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

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); m != nil {
			pct := 0.0
			fmt.Sscanf(m[1], "%f", &pct)
			CurrentDownload.Set(true, pct, fmt.Sprintf("%.0f%%", pct))
		}
	}

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
