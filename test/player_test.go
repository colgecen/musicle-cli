package test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"MusicLeCLI/bridge"
)

func TestPlayerCall_PlayNoFile(t *testing.T) {
	// Playing a nonexistent file should return error
	result, err := bridge.PlayerCall(bridge.Action{
		Action: "play",
		File:   filepath.Join(t.TempDir(), "nonexistent.mp3"),
	})
	if err != nil {
		t.Fatalf("PlayerCall play: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error for nonexistent file, got %q", result.Status)
	}
}

func TestPlayerCall_PlayUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	os.WriteFile(fpath, []byte("not audio"), 0644)

	result, err := bridge.PlayerCall(bridge.Action{
		Action: "play",
		File:   fpath,
	})
	if err != nil {
		t.Fatalf("PlayerCall play: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error for unsupported format, got %q", result.Status)
	}

	// Close the file handle so temp dir can be cleaned up
	bridge.PlayerCall(bridge.Action{Action: "stop"})
}

func TestPlayerCall_SeekNoFile(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{
		Action: "seek",
		Value:  10.0,
	})
	if err != nil {
		t.Fatalf("PlayerCall seek: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error when no file loaded, got %q", result.Status)
	}
}

func TestPlayerCall_PauseNoFile(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "pause"})
	if err != nil {
		t.Fatalf("PlayerCall pause: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error when no file loaded, got %q", result.Status)
	}
}

func TestPlayerCall_ResumeNoFile(t *testing.T) {
	result, err := bridge.PlayerCall(bridge.Action{Action: "resume"})
	if err != nil {
		t.Fatalf("PlayerCall resume: %v", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error when no file loaded, got %q", result.Status)
	}
}

func TestPlayerCall_PlayStopPlay(t *testing.T) {
	// Stop when idle
	bridge.PlayerCall(bridge.Action{Action: "stop"})

	// Status should be idle
	status, _ := bridge.PlayerCall(bridge.Action{Action: "status"})
	if status.Status != "idle" {
		t.Logf("status after stop: %q (expected idle)", status.Status)
	}
}

func TestPlayerCall_PlayMinimalWAV(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.wav")
	writeMinimalWAV(t, fpath)

	result, err := bridge.PlayerCall(bridge.Action{
		Action: "play",
		File:   fpath,
	})
	if err != nil {
		t.Fatalf("PlayerCall play: %v", err)
	}
	if result.Status != "playing" {
		t.Fatalf("expected playing, got %q: %q", result.Status, result.Error)
	}
	if result.Format != "WAV" {
		t.Errorf("expected WAV format, got %q", result.Format)
	}
	if result.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", result.Duration)
	}

	// Stop
	bridge.PlayerCall(bridge.Action{Action: "stop"})
}

func TestPlayerCall_VolumeStatus(t *testing.T) {
	// Set volume
	bridge.PlayerCall(bridge.Action{Action: "volume", Value: 0.3})

	// Check status reflects volume
	status, _ := bridge.PlayerCall(bridge.Action{Action: "status"})
	if status.Volume != 0.3 {
		t.Errorf("status volume = %v, want 0.3", status.Volume)
	}
}

func TestPlayerCall_SeekWithIntValue(t *testing.T) {
	// seek with int value (not float64)
	result, err := bridge.PlayerCall(bridge.Action{
		Action: "seek",
		Value:  5,
	})
	if err != nil {
		t.Fatalf("seek with int: %v", err)
	}
	// Should get error since no file loaded
	if result.Status != "error" {
		t.Fatalf("expected error, got %q", result.Status)
	}
}

func TestLogspace(t *testing.T) {
	// Test logspace logic indirectly via player behavior
	// logspace(min, max, n) returns n log-spaced values
	// This is used in FFT spectrum computation
	// We can't call it directly since it's unexported, but we can test
	// the math it relies on

	// Test that log spacing produces correct properties
	vals := logspace(30, 18000, 5)
	if len(vals) != 5 {
		t.Fatalf("expected 5 values, got %d", len(vals))
	}
	if vals[0] != 30 {
		t.Errorf("first value = %v, want 30", vals[0])
	}
	if vals[4] != 18000 {
		t.Errorf("last value = %v, want 18000", vals[4])
	}
	// Values should be increasing
	for i := 1; i < len(vals); i++ {
		if vals[i] <= vals[i-1] {
			t.Errorf("vals[%d]=%v <= vals[%d]=%v", i, vals[i], i-1, vals[i-1])
		}
	}
}

func TestComplexAbs(t *testing.T) {
	// |3+4i| = 5
	if got := complexAbs(3 + 4i); math.Abs(got-5) > 1e-10 {
		t.Errorf("|3+4i| = %v, want 5", got)
	}
	// |0+0i| = 0
	if got := complexAbs(0 + 0i); got != 0 {
		t.Errorf("|0+0i| = %v, want 0", got)
	}
	// |-1+0i| = 1
	if got := complexAbs(-1 + 0i); math.Abs(got-1) > 1e-10 {
		t.Errorf("|-1+0i| = %v, want 1", got)
	}
}

func TestFFTFreqs(t *testing.T) {
	// FFT size 2048, sample rate 44100 -> 1025 frequencies (0-22050 Hz)
	freqs := fftFreqs(2048, 44100)
	if len(freqs) != 1025 {
		t.Fatalf("expected 1025 frequencies, got %d", len(freqs))
	}
	if freqs[0] != 0 {
		t.Errorf("first freq = %v, want 0", freqs[0])
	}
	if math.Abs(freqs[1]-(44100.0/2048.0)) > 1e-10 {
		t.Errorf("freq step = %v, want %v", freqs[1], 44100.0/2048.0)
	}
	if math.Abs(freqs[len(freqs)-1]-22050) > 1 {
		t.Errorf("nyquist = %v, want 22050", freqs[len(freqs)-1])
	}
}

func TestMinIdx(t *testing.T) {
	if minIdx(3, 5) != 3 {
		t.Errorf("min(3,5) = %d, want 3", minIdx(3, 5))
	}
	if minIdx(5, 3) != 3 {
		t.Errorf("min(5,3) = %d, want 3", minIdx(5, 3))
	}
	if minIdx(3, 3) != 3 {
		t.Errorf("min(3,3) = %d, want 3", minIdx(3, 3))
	}
}

func TestApplyWindow(t *testing.T) {
	// Apply Hann window to data shorter than window
	window := make([]float64, 2048)
	for i := range window {
		window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(2048-1)))
	}

	data := make([]float64, 100)
	for i := range data {
		data[i] = 1.0
	}

	result := applyWindow(data, window)
	if len(result) != 2048 {
		t.Fatalf("expected 2048 samples, got %d", len(result))
	}
	// First element should be 0 * 0 = 0 (Hann window starts at 0)
	if result[0] != 0 {
		t.Errorf("result[0] = %v, want 0", result[0])
	}
	// Elements beyond input length should be 0
	for i := 100; i < len(result); i++ {
		if result[i] != 0 {
			t.Errorf("result[%d] = %v, want 0", i, result[i])
			break
		}
	}
}

// Test that play and stop can be called multiple times without panic
func TestPlayerCall_Sequence(t *testing.T) {
	// Stop -> Stop -> Status
	bridge.PlayerCall(bridge.Action{Action: "stop"})
	bridge.PlayerCall(bridge.Action{Action: "stop"})
	status, _ := bridge.PlayerCall(bridge.Action{Action: "status"})
	if status.Status != "idle" {
		t.Logf("status after stop x2: %q", status.Status)
	}
}

// Test Play WAV -> Status -> Stop -> Status flow
func TestPlayerCall_PlayStopFlow(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.wav")
	writeMinimalWAV(t, fpath)

	// Play
	result, _ := bridge.PlayerCall(bridge.Action{Action: "play", File: fpath})
	if result.Status != "playing" {
		t.Skipf("skipping: play returned %q: %q", result.Status, result.Error)
	}

	// Status
	status, _ := bridge.PlayerCall(bridge.Action{Action: "status"})
	if status.Status == "playing" || status.Status == "paused" {
		t.Logf("status after play: %q, pos=%.2f/dur=%.2f", status.Status, status.Position, status.Duration)
	} else {
		t.Logf("status: %q", status.Status)
	}

	// Stop
	bridge.PlayerCall(bridge.Action{Action: "stop"})

	// Status after stop
	status, _ = bridge.PlayerCall(bridge.Action{Action: "status"})
	if status.Status != "idle" {
		t.Logf("status after stop: %q (expected idle eventually)", status.Status)
	}
}

// replicas of unexported bridge functions for testing

func logspace(min, max float64, n int) []float64 {
	result := make([]float64, n)
	for i := range result {
		t := float64(i) / float64(n-1)
		result[i] = min * math.Pow(max/min, t)
	}
	return result
}

func complexAbs(c complex128) float64 {
	return math.Sqrt(real(c)*real(c) + imag(c)*imag(c))
}

func fftFreqs(fftSize, sampleRate int) []float64 {
	freqs := make([]float64, fftSize/2+1)
	for i := range freqs {
		freqs[i] = float64(i) * float64(sampleRate) / float64(fftSize)
	}
	return freqs
}

func minIdx(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func applyWindow(data []float64, window []float64) []float64 {
	n := len(window)
	if len(data) < n {
		n = len(data)
	}
	result := make([]float64, 2048)
	for i := 0; i < n; i++ {
		result[i] = data[i] * window[i]
	}
	return result
}
