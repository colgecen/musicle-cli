package main

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

var themeNames = func() []string {
	names := make([]string, 0, len(ui.ThemeColors))
	for n := range ui.ThemeColors {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}()

type SettingsModel struct {
	width  int
	height int

	langIdx   int
	themeIdx  int
	focus     int // 0=lang, 1=theme
}

func NewSettingsModel() *SettingsModel {
	m := &SettingsModel{}
	if state.Current.Language == state.LangTurkish {
		m.langIdx = 1
	}
	for i, n := range themeNames {
		if n == state.Current.Theme {
			m.themeIdx = i
			break
		}
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
		case "tab":
			m.focus = (m.focus + 1) % 2
		case "shift+tab":
			m.focus = (m.focus - 1 + 2) % 2
		case "up", "k":
			if m.focus == 0 {
				m.langIdx = (m.langIdx + 1) % 2
			} else if m.focus == 1 {
				m.themeIdx = (m.themeIdx + 1) % len(themeNames)
			}
		case "down", "j":
			if m.focus == 0 {
				m.langIdx = (m.langIdx + 1) % 2
			} else if m.focus == 1 {
				m.themeIdx = (m.themeIdx + 1) % len(themeNames)
			}
		case "enter":
			if m.focus == 0 {
				lang := state.LangEnglish
				if m.langIdx == 1 {
					lang = state.LangTurkish
				}
				state.Current.Language = lang
				_ = state.Current.SaveConfig()
			} else if m.focus == 1 {
				theme := themeNames[m.themeIdx]
				state.Current.Theme = theme
				_ = state.Current.SaveConfig()
				ui.ApplyTheme(theme)
			}
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
	}
	if m.height <= 0 {
		m.height = 40
	}

	// Language picker (side by side)
	enText := "English"
	trText := "Turkce"
	if m.focus == 0 {
		if m.langIdx == 0 {
			enText = ui.AccentStyle.Render("> English")
			trText = "  Turkce"
		} else {
			enText = "  English"
			trText = ui.AccentStyle.Render("> Turkce")
		}
	} else {
		if m.langIdx == 0 {
			enText = ui.WhiteStyle.Render("English")
			trText = ui.DimStyle.Render("Turkce")
		} else {
			enText = ui.DimStyle.Render("English")
			trText = ui.WhiteStyle.Render("Turkce")
		}
	}
	langOpts := lipgloss.JoinHorizontal(lipgloss.Left, enText, "    ", trText)

	// Theme picker
	var themeLines []string
	for i, n := range themeNames {
		prefix := "  "
		if m.focus == 1 && i == m.themeIdx {
			prefix = "> "
		}
		colorHex := ui.ThemeColors[n]
		colorSample := lipgloss.NewStyle().Foreground(lipgloss.Color(colorHex)).Render("###")
		line := prefix + colorSample + "  " + n
		if m.focus == 1 && i == m.themeIdx {
			line = ui.AccentStyle.Render(line)
		}
		themeLines = append(themeLines, line)
	}
	themeOpts := strings.Join(themeLines, "\n")

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
		ui.SectionTitleStyle.Render(" "+langT("Theme", "Tema")+" "),
		"",
		themeOpts,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Music Directory", "Muzik Dizini")+" "),
		"",
		"  "+ui.WhiteStyle.Render(rootDir),
		"",
		ui.DimStyle.Render("  "+langT("[^v] Change  [Enter] Save", "[^v] Degistir  [Enter] Kaydet")),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("General Settings", "Genel Ayarlar") + " ")
	box := ui.BorderStyle.
		Width(75).
		Render(title + "\n" + boxContent)

	return box
}
