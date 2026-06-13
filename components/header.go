package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

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

	activeStyle := tabBase.Background(ui.ColorAccent).Foreground(ui.ColorBlack).Bold(true)
	inactiveStyle := tabBase.Background(lipgloss.Color("#282828")).Foreground(ui.ColorPrimary)

	homeTab := inactiveStyle.Render(" Home ")
	settingsTab := activeStyle.Render(" Settings ")
	if activeView == "home" {
		homeTab = activeStyle.Render(" Home ")
		settingsTab = inactiveStyle.Render(" Settings ")
	}

	logoW := lipgloss.Width(logoDiv)
	tabs := lipgloss.JoinHorizontal(lipgloss.Left, homeTab, "  ", settingsTab)
	tabsW := lipgloss.Width(tabs)
	innerW := width - 2
	spacer := (innerW - logoW - tabsW) / 2
	if spacer < 2 {
		spacer = 2
	}
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, logoDiv, strings.Repeat(" ", spacer), tabs, strings.Repeat(" ", spacer))
	return ui.BorderStyle.Width(width - 2).Render(headerLine)
}
