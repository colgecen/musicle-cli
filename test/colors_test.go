package test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"MusicLeCLI/ui"
)

func TestVolumeColor(t *testing.T) {
	tests := []struct {
		vol  float64
		want string
	}{
		{0, string(ui.ColorAccent)},
		{0.2, string(ui.ColorAccent)},
		{0.33, string(ui.ColorAccent)},
		{0.34, string(ui.ColorOrange)},
		{0.5, string(ui.ColorOrange)},
		{0.66, string(ui.ColorOrange)},
		{0.67, string(ui.ColorError)},
		{1.0, string(ui.ColorError)},
	}
	for _, tc := range tests {
		got := ui.VolumeColor(tc.vol)
		if string(got) != tc.want {
			t.Errorf("VolumeColor(%v) = %v; want %v", tc.vol, got, tc.want)
		}
	}
}

func TestVolumeBar(t *testing.T) {
	bar := ui.VolumeBar(0.5, 10)
	if utf8.RuneCountInString(bar) != 10 {
		t.Errorf("VolumeBar length = %d chars; want 10", utf8.RuneCountInString(bar))
	}
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
		t.Errorf("VolumeBar should contain both filled (█) and empty (░) chars")
	}

	empty := ui.VolumeBar(0, 5)
	if empty != "░░░░░" {
		t.Errorf("VolumeBar(0, 5) = %q; want %q", empty, "░░░░░")
	}

	full := ui.VolumeBar(1, 5)
	if full != "█████" {
		t.Errorf("VolumeBar(1, 5) = %q; want %q", full, "█████")
	}

	clamped := ui.VolumeBar(-0.5, 5)
	if clamped != "░░░░░" {
		t.Errorf("VolumeBar(-0.5, 5) = %q; want %q", clamped, "░░░░░")
	}

	zeroWidth := ui.VolumeBar(0.5, 0)
	if utf8.RuneCountInString(zeroWidth) != 10 {
		t.Errorf("VolumeBar with width 0 should default to 10, got %d chars", utf8.RuneCountInString(zeroWidth))
	}
}

func TestProgressBar(t *testing.T) {
	bar := ui.ProgressBar(0, 100, 20)
	if !strings.Contains(bar, "o") {
		t.Errorf("ProgressBar(0) should show the thumb at start")
	}

	bar50 := ui.ProgressBar(50, 100, 20)
	if !strings.Contains(bar50, "o") {
		t.Errorf("ProgressBar(50) should show the thumb")
	}

	complete := ui.ProgressBar(100, 100, 20)
	if strings.Contains(complete, "o") {
		t.Errorf("ProgressBar(100) thumb should be at very end")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		secs float64
		want string
	}{
		{0, "00:00"},
		{5, "00:05"},
		{60, "01:00"},
		{90, "01:30"},
		{3600, "60:00"},
		{3661, "61:01"},
	}
	for _, tc := range tests {
		got := ui.FormatDuration(tc.secs)
		if got != tc.want {
			t.Errorf("FormatDuration(%v) = %s; want %s", tc.secs, got, tc.want)
		}
	}
}
