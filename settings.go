package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/components"
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

func (m *SettingsModel) View() string {
	if m.width <= 0 {
		m.width = 120
		m.height = 40
	}

	header := components.RenderHeader(m.width, "settings")
	playerBar := components.RenderPlayerBar(m.width, false)
	headerH := lipgloss.Height(header)
	barH := lipgloss.Height(playerBar)
	bodyH := m.height - headerH - barH
	if bodyH < 5 {
		bodyH = 5
	}

	langOpts := "  English"
	if m.langIdx == 1 {
		langOpts = "  Türkçe"
	}
	if m.focus == 0 {
		if m.langIdx == 0 {
			langOpts = ui.AccentStyle.Render("▸ English") + "\n  Türkçe"
		} else {
			langOpts = "  English\n" + ui.AccentStyle.Render("▸ Türkçe")
		}
	}

	rootDir := state.Current.RootDir
	if rootDir == "" {
		rootDir = langT("Not set", "Ayarlanmamış")
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Language", "Dil")+" "),
		"",
		"  "+langOpts,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Music Directory", "Müzik Dizini")+" "),
		"",
		"  "+ui.WhiteStyle.Render(rootDir),
		"",
		ui.DimStyle.Render("  "+langT("[↑↓] Change  [Enter] Save", "[↑↓] Değiştir  [Enter] Kaydet")),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("General Settings", "Genel Ayarlar") + " ")
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + boxContent)

	boxH := lipgloss.Height(box)
	if boxH < bodyH {
		box = lipgloss.JoinVertical(lipgloss.Left, box, strings.Repeat("\n", bodyH-boxH))
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, box, playerBar)
}
