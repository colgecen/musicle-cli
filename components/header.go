package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
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
	netIndicator := lipgloss.NewStyle().Foreground(netColor).Render("●")
	clock := time.Now().Format("15:04")
	lang := state.T(state.Current.Language, "EN", "TR")
	statusDiv := divStyle.Render(fmt.Sprintf("%s %s %s", netIndicator, clock, lang))

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		"  ",
		logoDiv,
		"  ",
		tabsJoined,
		"  ",
		statusDiv,
		"  ",
	)

	outer := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Width(width - 2)

	return outer.Render(row)
}
