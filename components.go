package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type InputField struct {
	textinput.Model
	label string
}

func NewInputField(label, placeholder string) InputField {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Prompt = label
	ti.PromptStyle = ui.AccentStyle
	ti.TextStyle = ui.WhiteStyle
	ti.PlaceholderStyle = ui.DimStyle
	ti.Cursor.Style = lipgloss.NewStyle().
		Background(lipgloss.Color("#1DB954")).
		Foreground(lipgloss.Color("#000000"))
	ti.Width = 50
	ti.CharLimit = 200
	return InputField{Model: ti, label: label}
}

func (i InputField) FocusedView() string {
	return i.Model.View()
}

func (i InputField) BlurredView() string {
	v := i.Model.View()
	return lipgloss.NewStyle().Foreground(ui.ColorSecondary).Render(v)
}

func langT(en, tr string) string {
	return state.T(state.Current.Language, en, tr)
}

func renderLogo() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		ui.LogoStyle.Render("Music"),
		ui.LogoAccentStyle.Render("Le"),
	)
}

type Option struct {
	Text  string
	Value string
}

func handleGlobalKeys(msg tea.KeyMsg, m *MainModel) (*MainModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "f1":
		if m.view != ViewSetup {
			m.view = ViewHome
			m.activeNav = "home"
		}
	case "f2":
		if m.view != ViewSetup {
			m.view = ViewSettings
			m.activeNav = "settings"
		}
	}
	return m, nil
}

// Shared header component — used by Home and Settings views
func renderHeader(width int, activeView string) string {
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

// Shared player bar component — used by Home view
func renderPlayerBar(width int, sectionFocused bool) string {
	ps := state.Current.Player
	title := ui.DimStyle.Render("No track playing")
	artist := ""
	posStr := "00:00"
	durStr := "00:00"
	progress := ui.ProgressBar(0, 1, 28)
	if ps.CurrentSong != nil {
		t := ps.CurrentSong.Title
		if len(t) > 28 {
			t = t[:26] + "…"
		}
		title = ui.WhiteStyle.Bold(true).Render(t)
		a := ps.CurrentSong.Artist
		if len(a) > 28 {
			a = a[:26] + "…"
		}
		artist = "  " + ui.DimStyle.Render(a)
		posStr = ui.FormatDuration(ps.Position)
		durStr = ui.FormatDuration(ps.Duration)
		progress = ui.ProgressBar(ps.Position, ps.Duration, 28)
	}
	statusIcon := ui.AccentStyle.Render("▶")
	if ps.IsPaused {
		statusIcon = ui.AccentStyle.Render("⏸")
	} else if !ps.IsPlaying {
		statusIcon = ui.DimStyle.Render("⏹")
	}
	volColor := ui.ColorAccent
	if ps.Volume > 0.66 {
		volColor = ui.ColorError
	} else if ps.Volume > 0.33 {
		volColor = ui.ColorOrange
	}
	volStr := lipgloss.NewStyle().Foreground(volColor).Render(ui.VolumeBar(ps.Volume, 8))
	line1 := fmt.Sprintf("  %s  %s%s", statusIcon, title, artist)
	line2 := fmt.Sprintf("  %s  %s  %s / %s   %s %s", ui.DimStyle.Render(posStr), ui.AccentStyle.Render(progress), ui.DimStyle.Render(posStr), ui.DimStyle.Render(durStr), ui.FaintStyle.Render("VOL"), volStr)
	if ps.StatusMsg != "" {
		c := ui.AccentStyle
		if ps.IsError {
			c = ui.ErrorStyle
		}
		line1 = "  " + c.Render(ps.StatusMsg)
		line2 = ""
	}
	bar := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
	border := ui.BorderStyle
	if sectionFocused {
		border = ui.AccentBorderStyle
	}
	return border.Width(width - 2).Render(bar)
}
