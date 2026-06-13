package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

func RenderHeader(width int, activeView string) string {
	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 2).
		Width(width - 4)

	logoText := ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le")

	tabBase := lipgloss.NewStyle().Width(14).Align(lipgloss.Center)

	activeStyle := tabBase.
		Background(ui.ColorAccent).
		Foreground(ui.ColorBlack).
		Bold(true)
	inactiveStyle := tabBase.
		Foreground(ui.ColorPrimary)

	type tabItem struct{ id, label string }
	tabDefs := []tabItem{
		{"home", " Home "},
		{"profile", " Profile "},
		{"playlist", " Playlist "},
		{"settings", " Settings "},
	}
	var tabRenders []string
	for _, t := range tabDefs {
		if activeView == t.id {
			tabRenders = append(tabRenders, activeStyle.Render(t.label))
		} else {
			tabRenders = append(tabRenders, inactiveStyle.Render(t.label))
		}
	}
	tabs := strings.Join(tabRenders, "  ")

	netColor := ui.ColorAccent
	if !state.Current.NetworkOnline {
		netColor = lipgloss.Color("#666666")
	}
	netIndicator := lipgloss.NewStyle().Foreground(netColor).Render("●")
	clock := time.Now().Format("15:04")
	lang := state.T(state.Current.Language, "EN", "TR")
	statusText := fmt.Sprintf("%s %s %s", netIndicator, clock, lang)

	logoW := lipgloss.Width(logoText)
	tabsW := lipgloss.Width(tabs)
	statusW := lipgloss.Width(statusText)
	totalContentW := logoW + tabsW + statusW
	remaining := width - 4 - totalContentW
	if remaining < 4 {
		remaining = 4
	}
	leftSpacer := remaining / 3
	rightSpacer := remaining - leftSpacer

	headerLine := lipgloss.JoinHorizontal(lipgloss.Center,
		logoText,
		strings.Repeat(" ", leftSpacer),
		tabs,
		strings.Repeat(" ", rightSpacer),
		statusText,
	)

	return headerStyle.Render(headerLine)
}
