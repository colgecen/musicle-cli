package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
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
	center := lipgloss.NewStyle().Width(inner).Align(lipgloss.Center)
	line1 := center.Render(fmt.Sprintf("  %s  %s%s", statusIcon, title, artist))
	line2 := center.Render(fmt.Sprintf("  %s  %s  %s / %s   %s %s", ui.DimStyle.Render(posStr), ui.AccentStyle.Render(progress), ui.DimStyle.Render(posStr), ui.DimStyle.Render(durStr), ui.FaintStyle.Render("VOL"), volStr))
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
