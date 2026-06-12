package main

import (
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
