package test

import (
	"os"
	"path/filepath"
	"testing"

	"MusicLeCLI/bridge"
	"MusicLeCLI/state"
)

func TestRunScript_UnknownAction(t *testing.T) {
	result, err := bridge.RunScript(bridge.Action{Action: "nonexistent"})
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
}

func TestRunScript_UpdateSong(t *testing.T) {
	dir := t.TempDir()
	listPath := filepath.Join(dir, "song_list.txt")
	writeSongListFile(t, listPath, []string{
		"s1.mp3|Old Title|Old Artist|2024-01-01|03:00",
	})

	value := map[string]interface{}{
		"title":    "New Title",
		"artist":   "New Artist",
		"duration": "04:30",
	}
	action := bridge.Action{
		Action: "update_song",
		File:   listPath,
		Path:   "s1.mp3",
		Value:  value,
	}

	result, err := bridge.RunScript(action)
	if err != nil {
		t.Fatalf("RunScript update_song: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}

	songs, _ := state.ReadSongs(listPath)
	if songs[0].Title != "New Title" || songs[0].Duration != "04:30" {
		t.Errorf("song not updated: %+v", songs[0])
	}
}

func TestRunScript_UpdateSong_NotFound(t *testing.T) {
	dir := t.TempDir()
	listPath := filepath.Join(dir, "song_list.txt")
	writeSongListFile(t, listPath, []string{
		"s1.mp3|Title|Artist|2024-01-01|03:00",
	})

	action := bridge.Action{
		Action: "update_song",
		File:   listPath,
		Path:   "nonexistent.mp3",
		Value:  map[string]interface{}{"title": "New"},
	}

	result, err := bridge.RunScript(action)
	if err == nil {
		t.Fatal("expected error for nonexistent song")
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
}

func TestRunScript_RemoveSong(t *testing.T) {
	dir := t.TempDir()
	listPath := filepath.Join(dir, "song_list.txt")
	writeSongListFile(t, listPath, []string{
		"s1.mp3|One|A|2024-01-01|03:00",
		"s2.mp3|Two|B|2024-01-02|04:00",
	})

	action := bridge.Action{
		Action: "remove_song",
		File:   listPath,
		Path:   "s1.mp3",
	}

	result, err := bridge.RunScript(action)
	if err != nil {
		t.Fatalf("RunScript remove_song: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}

	songs, _ := state.ReadSongs(listPath)
	if len(songs) != 1 || songs[0].Filename != "s2.mp3" {
		t.Errorf("remaining songs: %+v", songs)
	}
}

func TestRunScript_RemoveSong_NotFound(t *testing.T) {
	dir := t.TempDir()
	listPath := filepath.Join(dir, "song_list.txt")
	writeSongListFile(t, listPath, []string{
		"s1.mp3|One|A|2024-01-01|03:00",
	})

	action := bridge.Action{
		Action: "remove_song",
		File:   listPath,
		Path:   "nope.mp3",
	}

	result, err := bridge.RunScript(action)
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Status != "error" {
		t.Fatalf("expected error status")
	}
}

func TestRunScriptDownload_UnknownAction(t *testing.T) {
	result, err := bridge.RunScriptDownload(bridge.Action{Action: "nonexistent"})
	if err != nil {
		t.Fatalf("RunScriptDownload: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
}

func TestPlayerCall_UnknownAction(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "nonexistent"})
	if err != nil {
		t.Fatalf("PlayerCall: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
}

func TestPlayerCall_StopWhenIdle(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "stop"})
	if err != nil {
		t.Fatalf("PlayerCall stop: %v", err)
	}
	if result.Status != "stopped" {
		t.Fatalf("expected stopped, got %q", result.Status)
	}
}

func TestPlayerCall_StatusWhenIdle(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "status"})
	if err != nil {
		t.Fatalf("PlayerCall status: %v", err)
	}
	if result.Status != "idle" {
		t.Fatalf("expected idle, got %q", result.Status)
	}
}

func TestPlayerCall_Volume(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "volume", Value: 0.5})
	if err != nil {
		t.Fatalf("PlayerCall volume: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}
	if result.Volume != 0.5 {
		t.Errorf("volume = %v, want 0.5", result.Volume)
	}
}

func TestPlayerCall_Volume_Clamp(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "volume", Value: 2.0})
	if err != nil {
		t.Fatalf("PlayerCall volume: %v", err)
	}
	if result.Volume > 1.0 {
		t.Errorf("expected volume clamped to 1.0, got %v", result.Volume)
	}

	result, err = bridge.PlayerCall(bridge.Action{Action: "volume", Value: -1.0})
	if err != nil {
		t.Fatalf("PlayerCall volume: %v", err)
	}
	if result.Volume < 0 {
		t.Errorf("expected volume clamped to 0, got %v", result.Volume)
	}
}

func TestActionJSONTags(t *testing.T) {
	// Verify Action struct has proper JSON tags for common fields
	action := bridge.Action{
		Action: "test",
		File:   "f",
		URL:    "u",
		Output: "o",
		Value:  42,
		Path:   "p",
	}
	if action.Action != "test" || action.File != "f" {
		t.Errorf("Action fields: %+v", action)
	}
}

// This integration test writes then reads via RunScript add_local -> ReadSongs
func TestAddLocalIntegration(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "song.mp3")
	writeMinimalMP3(t, src)

	plDir := filepath.Join(dir, "pl")

	result := addLocalViaBridge(t, src, plDir)
	if result.Status != "ok" {
		t.Fatalf("addLocal failed: %q", result.Error)
	}
	if result.Filename != "song.mp3" {
		t.Errorf("filename = %q", result.Filename)
	}
	if result.Title == "" {
		t.Error("expected title from metadata")
	}
}

func writeSongListFile(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write song_list: %v", err)
	}
}
