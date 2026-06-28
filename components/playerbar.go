package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

func RenderPlayerBar(width int, sectionFocused bool) string {
	ps := state.Current.Player
	title := ui.DimStyle.Render("No track playing")
	artist := ""
	posStr := "00:00"
	durStr := "00:00"
	progress := ui.ProgressBar(0, 1, 28)
	if ps.CurrentSong != nil {
		t := ps.CurrentSong.Title
		if len(t) > 28 {
			t = t[:26] + "..."
		}
		title = ui.WhiteStyle.Bold(true).Render(t)
		a := ps.CurrentSong.Artist
		if len(a) > 28 {
			a = a[:26] + "..."
		}
		artist = "  " + ui.DimStyle.Render(a)
		posStr = ui.FormatDuration(ps.Position)
		durStr = ui.FormatDuration(ps.Duration)
		progress = ui.ProgressBar(ps.Position, ps.Duration, 28)
	}
	statusIcon := ui.AccentStyle.Render(">")
	if ps.IsPaused {
		statusIcon = ui.AccentStyle.Render("||")
	} else if !ps.IsPlaying {
		statusIcon = ui.DimStyle.Render("#")
	}
	volColor := ui.ColorAccent
	if ps.Volume > 0.66 {
		volColor = ui.ColorError
	} else if ps.Volume > 0.33 {
		volColor = ui.ColorOrange
	}
	volStr := lipgloss.NewStyle().Foreground(volColor).Render(ui.VolumeBar(ps.Volume, 8))
	inner := width - 2

	// Adaptive segments for line2 — prevent overflow
	barCount := 40
	metaW := 24
	if inner < 80 {
		barCount = 28
		metaW = 18
	}
	if inner < 60 {
		barCount = 16
		metaW = 14
	}
	if inner < 40 {
		barCount = 8
		metaW = 8
	}
	mw := inner - barCount - metaW
	if mw < 4 {
		mw = 4
		barCount = (inner - 4) / 2
		if barCount < 4 {
			barCount = 4
		}
		metaW = 8
		mw = inner - barCount - metaW
		if mw < 4 {
			mw = 4
		}
	}

	// Line 1: status + title + artist (MaxWidth prevents overflow)
	center := lipgloss.NewStyle().MaxWidth(inner).Width(inner).Align(lipgloss.Center)
	line1 := center.Render(fmt.Sprintf("  %s  %s%s", statusIcon, title, artist))

	// Line 2: Volume bars (left) | progress + pos + vol | metadata (right)
	barStr := strings.Repeat(" ", barCount)
	if ps.CurrentSong != nil {
		level := ps.AudioLevelL
		if level > 0 {
			level = level*7 + 0.3
			if level > 1 {
				level = 1
			}
		}
		barStr = ui.VolumeBars(level, barCount)
	}

	// Metadata (right side) — truncated via MaxWidth
	metaRaw := ""
	if ps.Format != "" {
		metaParts := ps.Format
		if ps.SampleRate > 0 {
			metaParts += fmt.Sprintf(" %dHz", ps.SampleRate)
		}
		if ps.Bitrate > 0 {
			metaParts += fmt.Sprintf(" %dkbps", ps.Bitrate)
		}
		metaRaw = metaParts
	}
	metaStr := lipgloss.NewStyle().MaxWidth(metaW).Width(metaW).Align(lipgloss.Right).Render(ui.FaintStyle.Render(metaRaw))

	// Main content: position + progress + volume (truncated via MaxWidth)
	mainContent := fmt.Sprintf("  %s  %s  %s / %s   %s %s",
		ui.DimStyle.Render(posStr), ui.AccentStyle.Render(progress),
		ui.DimStyle.Render(posStr), ui.DimStyle.Render(durStr),
		ui.FaintStyle.Render("VOL"), volStr)
	mainRendered := lipgloss.NewStyle().MaxWidth(mw).Width(mw).Align(lipgloss.Center).Render(strings.TrimSpace(mainContent))

	line2 := fmt.Sprintf("%s%s%s", barStr, mainRendered, metaStr)

	if ps.StatusMsg != "" {
		c := ui.AccentStyle
		if ps.IsError {
			c = ui.ErrorStyle
		}
		line1 = center.Render("  " + c.Render(ps.StatusMsg))
		line2 = center.Render("")
	}
	bar := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
	border := ui.BorderStyle
	if sectionFocused {
		border = ui.AccentBorderStyle
	}
	return border.Width(width - 2).Render(bar)
}
