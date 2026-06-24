package test

import (
	"os"
	"path/filepath"
	"testing"

	"MusicLeCLI/bridge"
	"MusicLeCLI/state"
)

func TestExtractMetadata_NoFile(t *testing.T) {
	result := extractMetadataViaBridge(t, filepath.Join(t.TempDir(), "nonexistent.mp3"))
	if result.Status != "error" {
		t.Fatalf("expected error for missing file, got status=%q", result.Status)
	}
}

func TestExtractMetadata_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	os.WriteFile(fpath, []byte("not an audio file"), 0644)

	result := extractMetadataViaBridge(t, fpath)
	if result.Format != "" {
		t.Fatalf("expected empty format for unsupported extension, got %q", result.Format)
	}
}

func TestExtractMetadata_BasicFallback(t *testing.T) {
	// Create a minimal valid WAV file (no tags, but valid format)
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.wav")
	writeMinimalWAV(t, fpath)

	result := extractMetadataViaBridge(t, fpath)
	if result.Status != "ok" {
		t.Fatalf("expected ok, got status=%q error=%q", result.Status, result.Error)
	}
	if result.Format != "WAV" {
		t.Errorf("format = %q, want WAV", result.Format)
	}
}

func TestAddLocal_NonExistent(t *testing.T) {
	dir := t.TempDir()
	playlistDir := filepath.Join(dir, "pl")
	result := addLocalViaBridge(t, filepath.Join(dir, "nonexistent.mp3"), playlistDir)
	if result.Status != "error" {
		t.Fatalf("expected error for missing file, got %q", result.Status)
	}
}

func TestAddLocal_InvalidExtension(t *testing.T) {
	dir := t.TempDir()
	playlistDir := filepath.Join(dir, "pl")
	src := filepath.Join(dir, "test.txt")
	os.WriteFile(src, []byte("hello"), 0644)

	result := addLocalViaBridge(t, src, playlistDir)
	if result.Status != "error" {
		t.Fatalf("expected error for non-mp3, got %q", result.Status)
	}
}

func TestAddLocal_MP3(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "test.mp3")
	writeMinimalMP3(t, src)

	playlistDir := filepath.Join(dir, "pl")

	result := addLocalViaBridge(t, src, playlistDir)
	if result.Status != "ok" {
		t.Fatalf("addLocal failed: status=%q error=%q", result.Status, result.Error)
	}
	if result.Title == "" {
		t.Error("expected non-empty title")
	}

	// Verify file was copied
	dest := filepath.Join(playlistDir, "test.mp3")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("mp3 file was not copied to playlist dir")
	}

	// Verify song_list.txt was created
	listPath := filepath.Join(playlistDir, "song_list.txt")
	if _, err := os.Stat(listPath); os.IsNotExist(err) {
		t.Error("song_list.txt was not created")
	}

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 1 {
		t.Fatalf("expected 1 song in list, got %d", len(songs))
	}
	if songs[0].Filename != "test.mp3" {
		t.Errorf("filename = %q", songs[0].Filename)
	}
}

func TestAddLocal_MP3_Collision(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "song.mp3")
	writeMinimalMP3(t, src)

	playlistDir := filepath.Join(dir, "pl")

	// First import
	addLocalViaBridge(t, src, playlistDir)

	// Second import should deduplicate filename
	result := addLocalViaBridge(t, src, playlistDir)
	if result.Status != "ok" {
		t.Fatalf("second import failed: %q", result.Error)
	}

	listPath := filepath.Join(playlistDir, "song_list.txt")
	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(songs))
	}
	if songs[1].Filename != "song_1.mp3" {
		t.Errorf("expected collided name 'song_1.mp3', got %q", songs[1].Filename)
	}

	// Verify actual file exists
	if _, err := os.Stat(filepath.Join(playlistDir, "song_1.mp3")); os.IsNotExist(err) {
		t.Error("collided file not found")
	}
}

func TestImportDirectory_NonMP3Rejected(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "songs")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "song.mp3"), []byte("fake"), 0644)
	os.WriteFile(filepath.Join(srcDir, "song.flac"), []byte("fake"), 0644)

	playlistDir := filepath.Join(dir, "pl")
	result := addLocalViaBridge(t, srcDir, playlistDir)
	if result.Status != "error" {
		t.Fatalf("expected error for non-mp3 in directory, got status=%q", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestImportDirectory_AllMP3(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "songs")
	os.MkdirAll(srcDir, 0755)
	writeMinimalMP3(t, filepath.Join(srcDir, "a.mp3"))
	writeMinimalMP3(t, filepath.Join(srcDir, "b.mp3"))

	playlistDir := filepath.Join(dir, "pl")
	result := addLocalViaBridge(t, srcDir, playlistDir)
	if result.Status != "ok" {
		t.Fatalf("import dir failed: status=%q error=%q", result.Status, result.Error)
	}

	listPath := filepath.Join(playlistDir, "song_list.txt")
	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 2 {
		t.Fatalf("expected 2 songs, got %d", len(songs))
	}
}

func TestFmtDuration(t *testing.T) {
	// bridge.fmtDuration is unexported, test via importSingleFile metadata parsing
	// which writes duration to song_list.txt
	dir := t.TempDir()
	src := filepath.Join(dir, "test.mp3")
	writeMinimalMP3(t, src)

	playlistDir := filepath.Join(dir, "pl")
	result := addLocalViaBridge(t, src, playlistDir)
	if result.Status != "ok" {
		t.Fatalf("addLocal: %q", result.Error)
	}

	listPath := filepath.Join(playlistDir, "song_list.txt")
	songs, _ := state.ReadSongs(listPath)
	if len(songs) > 0 && songs[0].Duration == "" {
		t.Error("expected non-empty duration")
	}
}

func TestDownloadProgress(t *testing.T) {
	var dp bridge.DownloadProgress

	dp.Set(true, 50.0, "50%")
	active, pct, msg := dp.Get()
	if !active || pct != 50.0 || msg != "50%" {
		t.Errorf("Get after Set: active=%v pct=%v msg=%q", active, pct, msg)
	}

	dp.Set(false, 100, "Done")
	active, pct, msg = dp.Get()
	if active || pct != 100 || msg != "Done" {
		t.Errorf("Get after Done: active=%v pct=%v msg=%q", active, pct, msg)
	}
}

func TestFmtDurationInDownload(t *testing.T) {
	// bridge.fmtDuration is used in finalizeDownload for song_list.txt entries
	dir := t.TempDir()
	src := filepath.Join(dir, "test.mp3")
	writeMinimalMP3(t, src)

	playlistDir := filepath.Join(dir, "pl")
	result := addLocalViaBridge(t, src, playlistDir)
	if result.Status != "ok" {
		t.Fatalf("addLocal: %q", result.Error)
	}

	listPath := filepath.Join(playlistDir, "song_list.txt")
	songs, _ := state.ReadSongs(listPath)
	if len(songs) > 0 && songs[0].Duration == "" {
		t.Error("expected duration in song entry")
	}
}

// helpers

func extractMetadataViaBridge(t *testing.T, filePath string) *bridge.Result {
	t.Helper()
	action := bridge.Action{Action: "metadata", File: filePath}
	result, err := bridge.RunScript(action)
	if err != nil {
		t.Fatalf("RunScript metadata: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	return result
}

func addLocalViaBridge(t *testing.T, sourcePath, playlistDir string) *bridge.Result {
	t.Helper()
	action := bridge.Action{Action: "add_local", File: sourcePath, Output: playlistDir}
	result, err := bridge.RunScript(action)
	if err != nil {
		t.Fatalf("RunScript add_local: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	return result
}

func writeMinimalWAV(t *testing.T, path string) {
	t.Helper()
	// 44-byte WAV header + 8 bytes of silence = 52 bytes
	data := make([]byte, 52)
	// RIFF header
	copy(data[0:4], "RIFF")
	// File size - 8
	data[4] = 44
	data[5] = 0
	data[6] = 0
	data[7] = 0
	// WAVE
	copy(data[8:12], "WAVE")
	// fmt chunk
	copy(data[12:16], "fmt ")
	// chunk size = 16
	data[16] = 16
	data[17] = 0
	data[18] = 0
	data[19] = 0
	// audio format = 1 (PCM)
	data[20] = 1
	data[21] = 0
	// num channels = 1
	data[22] = 1
	data[23] = 0
	// sample rate = 8000
	data[24] = 0x40
	data[25] = 0x1F
	data[26] = 0
	data[27] = 0
	// byte rate = 8000
	data[28] = 0x40
	data[29] = 0x1F
	data[30] = 0
	data[31] = 0
	// block align = 1
	data[32] = 1
	data[33] = 0
	// bits per sample = 8
	data[34] = 8
	data[35] = 0
	// data chunk
	copy(data[36:40], "data")
	// data size = 8
	data[40] = 8
	data[41] = 0
	data[42] = 0
	data[43] = 0
	// 8 bytes of silence (all zeros already)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write WAV: %v", err)
	}
}

func writeMinimalMP3(t *testing.T, path string) {
	t.Helper()
	// Minimal valid MP3 frame: sync word 0xFF 0xFB + 4 bytes header + 413 bytes data
	// MPEG1 Layer3, 128kbps, 44100Hz, stereo, no padding
	frame := make([]byte, 417)
	frame[0] = 0xFF
	frame[1] = 0xFB
	frame[2] = 0x90 // bitrate index=9 (128kbps), sample rate=00 (44100Hz)
	frame[3] = 0xC0 // no padding, stereo
	// Rest is zero-filled (silence)
	if err := os.WriteFile(path, frame, 0644); err != nil {
		t.Fatalf("write MP3: %v", err)
	}
}
