package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

func RenderHeader(width int, activeView string) string {
	divStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 2)

	logoText := ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le")
	logoDiv := divStyle.Render(logoText)

	tabBase := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 2).
		Width(14).
		Align(lipgloss.Center)

	activeStyle := tabBase.
		BorderForeground(ui.ColorAccent).
		Background(ui.ColorAccent).
		Foreground(ui.ColorBlack).
		Bold(true)
	inactiveStyle := tabBase.
		BorderForeground(ui.ColorPrimary)

	type tabItem struct{ id, label string }
	tabDefs := []tabItem{
		{"home", " Home "},
		{"profile", " Profile "},
		{"playlist", " Playlist "},
		{"settings", " Settings "},
	}
	var tabs []string
	for _, t := range tabDefs {
		if activeView == t.id {
			tabs = append(tabs, activeStyle.Render(t.label))
		} else {
			tabs = append(tabs, inactiveStyle.Render(t.label))
		}
	}
	tabsJoined := lipgloss.JoinHorizontal(lipgloss.Center, tabs...)

	netColor := ui.ColorAccent
	if !state.Current.NetworkOnline {
		netColor = lipgloss.Color("#666666")
	}
	netIndicator := lipgloss.NewStyle().Foreground(netColor).Render("o")
	clock := time.Now().Format("15:04")
	lang := state.T(state.Current.Language, "EN", "TR")
	statusDiv := divStyle.Render(fmt.Sprintf("%s %s %s", netIndicator, clock, lang))

	logoW := lipgloss.Width(logoDiv)
	tabsW := lipgloss.Width(tabsJoined)
	statusW := lipgloss.Width(statusDiv)
	avail := width - 2
	remaining := avail - logoW - tabsW - statusW - 4
	if remaining < 0 {
		remaining = 0
	}
	left := remaining / 2
	right := remaining - left

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		"  ",
		logoDiv,
		strings.Repeat(" ", left),
		tabsJoined,
		strings.Repeat(" ", right),
		statusDiv,
		"  ",
	)

	outer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Width(width - 2)

	return outer.Render(row)
}
