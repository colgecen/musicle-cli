package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"MusicLeCLI/ui"
)

type DownloadsModel struct {
	width  int
	height int
	focus  int
}

func NewDownloadsModel() *DownloadsModel {
	return &DownloadsModel{}
}

func (m *DownloadsModel) Init() tea.Cmd { return nil }

func (m *DownloadsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *DownloadsModel) cycleFocus() bool {
	return true
}

func (m *DownloadsModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 30
	}

	title := ui.AccentStyle.Bold(true).Render("Downloads")
	placeholder := ui.DimStyle.Render("Download manager coming soon...")

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(lipgloss.JoinVertical(lipgloss.Center, title, "", placeholder))
}
