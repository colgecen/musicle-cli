package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type SettingsModel struct {
	width  int
	height int

	langIdx int
	focus   int
}

func NewSettingsModel() *SettingsModel {
	m := &SettingsModel{}
	if state.Current.Language == state.LangTurkish {
		m.langIdx = 1
	}
	return m
}

func (m *SettingsModel) Init() tea.Cmd { return nil }

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.langIdx = (m.langIdx + 1) % 2
		case "down", "j":
			m.langIdx = (m.langIdx + 1) % 2
		case "enter":
			lang := state.LangEnglish
			if m.langIdx == 1 {
				lang = state.LangTurkish
			}
			state.Current.Language = lang
			_ = state.Current.SaveConfig()
		}
	}
	return m, nil
}

func (m *SettingsModel) cycleFocus() bool {
	m.focus = (m.focus + 1) % 2
	return m.focus == 0
}

func (m *SettingsModel) View() string {
	if m.width <= 0 {
		m.width = 120
		m.height = 40
	}

	langOpts := "  English"
	if m.langIdx == 1 {
		langOpts = "  Turkce"
	}
	if m.focus == 0 {
		if m.langIdx == 0 {
			langOpts = ui.AccentStyle.Render("> English") + "\n  Turkce"
		} else {
			langOpts = "  English\n" + ui.AccentStyle.Render("> Turkce")
		}
	}

	rootDir := state.Current.RootDir
	if rootDir == "" {
		rootDir = langT("Not set", "Ayarlanmamis")
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Language", "Dil")+" "),
		"",
		"  "+langOpts,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Music Directory", "Muzik Dizini")+" "),
		"",
		"  "+ui.WhiteStyle.Render(rootDir),
		"",
		ui.DimStyle.Render("  "+langT("[^v] Change  [Enter] Save", "[^v] Degistir  [Enter] Kaydet")),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("General Settings", "Genel Ayarlar") + " ")
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + boxContent)

	return box
}
