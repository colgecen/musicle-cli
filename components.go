package main

import (
	"github.com/charmbracelet/bubbles/textinput"
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
	ti.Width = 60
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
