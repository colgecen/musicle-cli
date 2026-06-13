package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestVolumeColor(t *testing.T) {
	tests := []struct {
		vol  float64
		want lipgloss.Color
	}{
		{0, ColorAccent},
		{0.2, ColorAccent},
		{0.33, ColorAccent},
		{0.34, ColorOrange},
		{0.5, ColorOrange},
		{0.66, ColorOrange},
		{0.67, ColorError},
		{1.0, ColorError},
	}
	for _, tc := range tests {
		got := VolumeColor(tc.vol)
		if got != tc.want {
			t.Errorf("VolumeColor(%v) = %v; want %v", tc.vol, got, tc.want)
		}
	}
}

func TestVolumeBar(t *testing.T) {
	bar := VolumeBar(0.5, 10)
	if utf8.RuneCountInString(bar) != 10 {
		t.Errorf("VolumeBar length = %d chars; want 10", utf8.RuneCountInString(bar))
	}
	if !strings.Contains(bar, "#") || !strings.Contains(bar, ".") {
		t.Errorf("VolumeBar should contain both filled and empty chars")
	}

	empty := VolumeBar(0, 5)
	if empty != "....." {
		t.Errorf("VolumeBar(0, 5) = %q; want %q", empty, ".....")
	}

	full := VolumeBar(1, 5)
	if full != "#####" {
		t.Errorf("VolumeBar(1, 5) = %q; want %q", full, "#####")
	}

	clamped := VolumeBar(-0.5, 5)
	if clamped != "....." {
		t.Errorf("VolumeBar(-0.5, 5) = %q; want %q", clamped, ".....")
	}

	zeroWidth := VolumeBar(0.5, 0)
	if utf8.RuneCountInString(zeroWidth) != 10 {
		t.Errorf("VolumeBar with width 0 should default to 10, got %d chars", utf8.RuneCountInString(zeroWidth))
	}
}

func TestProgressBar(t *testing.T) {
	bar := ProgressBar(0, 100, 20)
	if !		strings.Contains(bar, "o") {
		t.Errorf("ProgressBar(0) should show the thumb at start")
	}

	bar50 := ProgressBar(50, 100, 20)
	if !		strings.Contains(bar50, "o") {
		t.Errorf("ProgressBar(50) should show the thumb")
	}

	complete := ProgressBar(100, 100, 20)
	if 		strings.Contains(complete, "o") {
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
		got := FormatDuration(tc.secs)
		if got != tc.want {
			t.Errorf("FormatDuration(%v) = %s; want %s", tc.secs, got, tc.want)
		}
	}
}
