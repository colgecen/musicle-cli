package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type PlaylistModel struct {
	width  int
	height int

	playlistDropIdx int
	playlistOptions []string
	artInput        textinput.Model
	plNameInput     textinput.Model
	plBioInput      textinput.Model
	playlistStatus  string

	focus int
}

func NewPlaylistModel() *PlaylistModel {
	return &PlaylistModel{
		artInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Art Path:  "
			ti.Placeholder = "optional"
			ti.Width = 50
			return ti
		}(),
		plNameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Playlist Name:  "
			ti.Placeholder = "My Playlist"
			ti.Width = 50
			return ti
		}(),
		plBioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Description:  "
			ti.Placeholder = "My favorite songs"
			ti.Width = 50
			return ti
		}(),
	}
}

func (m *PlaylistModel) Init() tea.Cmd { return nil }

func (m *PlaylistModel) refreshOptions() {
	cp := state.Current.CurrentProfile
	if cp == nil {
		return
	}
	opts := make([]string, len(cp.Playlists))
	for i, pl := range cp.Playlists {
		opts[i] = pl.Name
	}
	m.playlistOptions = opts
	if m.playlistDropIdx >= len(opts) {
		m.playlistDropIdx = 0
	}
	if len(opts) > 0 {
		state.Current.CurrentPlaylist = &cp.Playlists[m.playlistDropIdx]
	}
}

func (m *PlaylistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.refreshOptions()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.focus == 0 && m.playlistDropIdx > 0 {
				m.playlistDropIdx--
				m.refreshOptions()
			}
			m.setFocus((m.focus - 1 + 4) % 4)
		case "down", "j":
			if m.focus == 0 && m.playlistDropIdx < len(m.playlistOptions)-1 {
				m.playlistDropIdx++
				m.refreshOptions()
			}
			m.setFocus((m.focus + 1) % 4)
		case "tab":
			m.setFocus((m.focus + 1) % 4)
		case "shift+tab":
			m.setFocus((m.focus - 1 + 4) % 4)
		case "enter":
			cp := state.Current.CurrentProfile
			if cp == nil {
				return m, nil
			}
			pl := state.Current.CurrentPlaylist
			if pl == nil {
				return m, nil
			}
			name := strings.TrimSpace(m.plNameInput.Value())
			if name == "" {
				name = pl.FolderName
			}
			bio := strings.TrimSpace(m.plBioInput.Value())
			artSrc := strings.TrimSpace(m.artInput.Value())
			if err := state.Current.SavePlaylistMeta(cp.FolderName, pl.FolderName, name, bio); err != nil {
				m.playlistStatus = ui.ErrorStyle.Render("  x " + err.Error())
				return m, nil
			}
			if artSrc != "" {
				plDir := state.Current.PlaylistDir(cp.FolderName, pl.FolderName)
				ext := ".jpg"
				if strings.HasSuffix(strings.ToLower(artSrc), ".png") {
					ext = ".png"
				}
				_ = state.CopyFile(artSrc, plDir+"/playlist_art/art"+ext)
			}
			pl.Name = name
			pl.Bio = bio
			_ = state.Current.ScanProfiles()
			cp = state.Current.CurrentProfile
			if cp != nil && m.playlistDropIdx < len(cp.Playlists) {
				state.Current.CurrentPlaylist = &cp.Playlists[m.playlistDropIdx]
			}
			m.refreshOptions()
			m.playlistStatus = ui.AccentStyle.Render("  v " + langT("Saved!", "Kaydedildi!"))
		case "delete":
			cp := state.Current.CurrentProfile
			pl := state.Current.CurrentPlaylist
			if cp == nil || pl == nil {
				return m, nil
			}
			_ = state.Current.DeletePlaylist(cp.FolderName, pl.FolderName)
			_ = state.Current.ScanProfiles()
			cp = state.Current.CurrentProfile
			if cp != nil && len(cp.Playlists) > 0 {
				state.Current.CurrentPlaylist = &cp.Playlists[0]
			}
			m.refreshOptions()
			if m.playlistDropIdx >= len(m.playlistOptions) {
				m.playlistDropIdx = 0
			}
			m.playlistStatus = ui.DimStyle.Render("  " + langT("Deleted", "Silindi"))
		case "esc":
			m.focus = 0
		default:
			if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *PlaylistModel) setFocus(idx int) {
	if idx < 0 || idx >= 4 {
		return
	}
	m.focus = idx
	inputs := []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
	for i, inp := range inputs {
		if i+1 == idx {
			inp.Focus()
		} else {
			inp.Blur()
		}
	}
}

func (m *PlaylistModel) cycleFocus() bool {
	m.setFocus((m.focus + 1) % 4)
	return m.focus == 0
}

func (m *PlaylistModel) View() string {
	if m.width <= 0 {
		m.width = 120
		m.height = 40
	}

	plV := "-"
	if len(m.playlistOptions) > 0 && m.playlistDropIdx < len(m.playlistOptions) {
		plV = m.playlistOptions[m.playlistDropIdx]
	}
	inputV1 := ui.FaintStyle.Render(m.artInput.Value())
	inputV2 := ui.FaintStyle.Render(m.plNameInput.Value())
	inputV3 := ui.FaintStyle.Render(m.plBioInput.Value())
	if m.focus == 1 { inputV1 = m.artInput.View() }
	if m.focus == 2 { inputV2 = m.plNameInput.View() }
	if m.focus == 3 { inputV3 = m.plBioInput.View() }

	saveBtn := ui.AccentButtonStyle.Render(langT("  Save  ", "  Kaydet  "))
	deleteBtn := ui.ErrorButtonStyle.Render(langT("  Delete  ", "  Sil  "))

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" Playlist: ") + ui.WhiteStyle.Render(plV),
		"",
		inputV1,
		"",
		inputV2,
		"",
		inputV3,
		"",
		m.playlistStatus,
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, saveBtn, "  ", deleteBtn),
	)

	title := ui.SectionTitleStyle.Render(langT(" Playlist Settings", " Playlist Ayarlari"))
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + boxContent)

	return box
}
