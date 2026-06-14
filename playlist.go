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

	focus       int
	lastProfile string
}

func NewPlaylistModel() *PlaylistModel {
	return &PlaylistModel{
		artInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Art Path:  "
			ti.Placeholder = "optional"
			ti.Width = 60
			return ti
		}(),
		plNameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Playlist Name:  "
			ti.Placeholder = "My Playlist"
			ti.Width = 60
			return ti
		}(),
		plBioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Description:  "
			ti.Placeholder = "My favorite songs"
			ti.Width = 60
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
		id := cp.FolderName + "/" + state.Current.CurrentPlaylist.FolderName
		if id != m.lastProfile {
			m.lastProfile = id
			pl := state.Current.CurrentPlaylist
			if pl != nil {
				m.plNameInput.SetValue(pl.Name)
				m.plNameInput.SetCursor(len(pl.Name))
				m.plBioInput.SetValue(pl.Bio)
				m.plBioInput.SetCursor(len(pl.Bio))
			}
		}
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
			if m.focus == 0 {
				if m.playlistDropIdx > 0 {
					m.playlistDropIdx--
					m.refreshOptions()
				}
			} else if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		case "down", "j":
			if m.focus == 0 {
				if m.playlistDropIdx < len(m.playlistOptions)-1 {
					m.playlistDropIdx++
					m.refreshOptions()
				}
			} else if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		case "tab":
			m.setFocus((m.focus + 1) % 6)
		case "shift+tab":
			m.setFocus((m.focus - 1 + 6) % 6)
		case "enter":
			if m.focus == 4 {
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
				_ = state.Current.ScanProfiles()
				for i, p := range state.Current.Profiles {
					if p.FolderName == cp.FolderName {
						state.Current.CurrentProfile = &state.Current.Profiles[i]
						if m.playlistDropIdx < len(p.Playlists) {
							state.Current.CurrentPlaylist = &p.Playlists[m.playlistDropIdx]
						}
						break
					}
				}
				m.refreshOptions()
				m.playlistStatus = ui.AccentStyle.Render("  v " + langT("Saved!", "Kaydedildi!"))
			} else if m.focus == 5 {
				cp := state.Current.CurrentProfile
				pl := state.Current.CurrentPlaylist
				if cp == nil || pl == nil {
					return m, nil
				}
				_ = state.Current.DeletePlaylist(cp.FolderName, pl.FolderName)
				_ = state.Current.ScanProfiles()
				for i, p := range state.Current.Profiles {
					if p.FolderName == cp.FolderName {
						state.Current.CurrentProfile = &state.Current.Profiles[i]
						if len(p.Playlists) > 0 {
							state.Current.CurrentPlaylist = &p.Playlists[0]
						} else {
							state.Current.CurrentPlaylist = nil
						}
						break
					}
				}
				m.refreshOptions()
				if m.playlistDropIdx >= len(m.playlistOptions) {
					m.playlistDropIdx = 0
				}
				m.playlistStatus = ui.DimStyle.Render("  " + langT("Deleted", "Silindi"))
			}
		case "delete":
			if m.focus == 5 {
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
			}
		case "esc":
			m.focus = 0
		case "ctrl+v":
			if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
				*inputs[m.focus-1], _ = inputs[m.focus-1].Update(textinput.Paste())
				return m, nil
			}
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
	if idx < 0 || idx >= 6 {
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
	m.setFocus((m.focus + 1) % 6)
	return m.focus == 0
}

func (m *PlaylistModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	plV := "-"
	if len(m.playlistOptions) > 0 && m.playlistDropIdx < len(m.playlistOptions) {
		plV = m.playlistOptions[m.playlistDropIdx]
	}
	if m.focus == 0 {
		plV = ui.AccentStyle.Render("> " + plV)
	} else {
		plV = "  " + ui.WhiteStyle.Render(plV)
	}

	artVal := m.artInput.Value()
	if artVal == "" {
		artVal = m.artInput.Placeholder
	}
	var artV string
	if m.focus == 1 {
		artV = m.artInput.View()
	} else {
		artV = "  Art Path:  " + ui.WhiteStyle.Render(artVal)
	}

	plNameVal := m.plNameInput.Value()
	if plNameVal == "" {
		plNameVal = m.plNameInput.Placeholder
	}
	var plNameV string
	if m.focus == 2 {
		plNameV = m.plNameInput.View()
	} else {
		plNameV = "  Playlist Name:  " + ui.WhiteStyle.Render(plNameVal)
	}

	plBioVal := m.plBioInput.Value()
	if plBioVal == "" {
		plBioVal = m.plBioInput.Placeholder
	}
	var plBioV string
	if m.focus == 3 {
		plBioV = m.plBioInput.View()
	} else {
		plBioV = "  Description:  " + ui.WhiteStyle.Render(plBioVal)
	}

	saveBtn := ui.AccentButtonStyle.Render(langT("  Save  ", "  Kaydet  "))
	deleteBtn := ui.ErrorButtonStyle.Render(langT("  Delete  ", "  Sil  "))
	if m.focus == 4 {
		saveBtn = ui.FocusedButtonStyle.Render(langT("  Save  ", "  Kaydet  "))
	}
	if m.focus == 5 {
		deleteBtn = ui.FocusedButtonStyle.Render(langT("  Delete  ", "  Sil  "))
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Playlist", "Playlist")+": ") + plV,
		"",
		ui.SectionTitleStyle.Render(" Art Image "),
		"",
		artV,
		"",
		ui.SectionTitleStyle.Render(" Playlist Name "),
		"",
		plNameV,
		"",
		ui.SectionTitleStyle.Render(" Description "),
		"",
		plBioV,
		"",
		m.playlistStatus,
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, saveBtn, "  ", deleteBtn),
	)

	title := ui.SectionTitleStyle.Render(langT(" Playlist Settings", " Playlist Ayarlari"))
	box := ui.BorderStyle.
		Width(75).
		Render(title + "\n" + boxContent)

	return box
}
