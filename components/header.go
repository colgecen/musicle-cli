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
	logoText := ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le")
	logoDiv := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 2).
		Render(logoText)

	tabBase := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 2).
		Width(16).
		Align(lipgloss.Center)

	activeStyle := tabBase.
		Background(ui.ColorAccent).
		BorderForeground(ui.ColorAccent).
		Foreground(ui.ColorBlack).
		Bold(true)
	inactiveStyle := tabBase.
		Foreground(ui.ColorPrimary)

	homeTab := inactiveStyle.Render(" Home ")
	settingsTab := activeStyle.Render(" Settings ")
	if activeView == "home" {
		homeTab = activeStyle.Render(" Home ")
		settingsTab = inactiveStyle.Render(" Settings ")
	}

	// Status area: network indicator, clock, language
	netColor := ui.ColorAccent
	netChar := "●"
	if !state.Current.NetworkOnline {
		netColor = lipgloss.Color("#666666")
	}
	netIndicator := lipgloss.NewStyle().Foreground(netColor).Render(netChar)
	clock := time.Now().Format("15:04")
	lang := state.T(state.Current.Language, "EN", "TR")
	statusText := fmt.Sprintf("%s %s %s", netIndicator, clock, lang)
	statusW := lipgloss.Width(statusText)

	// Pad status to 3 lines (match logoDiv height) so it aligns on the middle line
	statusDiv := "\n" + statusText + "\n"

	logoW := lipgloss.Width(logoDiv)
	tabs := lipgloss.JoinHorizontal(lipgloss.Left, homeTab, "  ", settingsTab)
	tabsW := lipgloss.Width(tabs)
	innerW := width - 2
	totalContentW := logoW + tabsW + statusW
	spacer := (innerW - totalContentW) / 2
	if spacer < 2 {
		spacer = 2
	}
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, logoDiv, strings.Repeat(" ", spacer), tabs, strings.Repeat(" ", spacer), statusDiv)
	return ui.BorderStyle.Width(width - 2).Render(headerLine)
}
