package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sqweek/dialog"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

type ArtFileSelectedMsg struct {
	Path string
}

type ArtFileTooLargeMsg struct{}

type PlaylistModel struct {
	width  int
	height int

	leftColWidth int

	playlistFocusIdx int
	playlistOffset   int
	playlistOptions  []string

	artPath     string
	plNameInput textinput.Model
	plBioInput  textinput.Model

	addMode bool

	playlistStatus string

	focus       int
	lastProfile string
	selectAll   bool
}

func NewPlaylistModel() *PlaylistModel {
	return &PlaylistModel{
		leftColWidth: 30,
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
		opts[i] = fmt.Sprintf("%d. %s", i+1, pl.Name)
	}
	m.playlistOptions = opts
	if m.playlistFocusIdx >= len(opts) {
		m.playlistFocusIdx = 0
		m.playlistOffset = 0
	}
	pl := m.selectedPlaylist()
	if pl != nil {
		id := cp.FolderName + "/" + pl.FolderName
		if id != m.lastProfile {
			m.lastProfile = id
			m.plNameInput.SetValue(pl.Name)
			m.plNameInput.SetCursor(len(pl.Name))
			m.plBioInput.SetValue(pl.Bio)
			m.plBioInput.SetCursor(len(pl.Bio))
		}
	}
}

func (m *PlaylistModel) selectedPlaylist() *state.Playlist {
	cp := state.Current.CurrentProfile
	if cp == nil {
		return nil
	}
	if m.playlistFocusIdx >= 0 && m.playlistFocusIdx < len(cp.Playlists) {
		return &cp.Playlists[m.playlistFocusIdx]
	}
	return nil
}

func (m *PlaylistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.refreshOptions()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ArtFileSelectedMsg:
		m.artPath = msg.Path
		m.playlistStatus = ui.WhiteStyle.Render("  " + langT("Art selected", "Resim secildi"))

	case ArtFileTooLargeMsg:
		m.playlistStatus = ui.ErrorStyle.Render("  x " + langT("Image must be under 1MB", "Resim 1MB altinda olmali"))

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.focus == 0 {
				if m.playlistFocusIdx > 0 {
					m.playlistFocusIdx--
				}
			} else if m.focus >= 2 && m.focus <= 3 {
				m.selectAll = false
				inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-2], cmd = inputs[m.focus-2].Update(msg)
				return m, cmd
			}
		case "down", "j":
			if m.focus == 0 {
				if m.playlistFocusIdx < len(m.playlistOptions)-1 {
					m.playlistFocusIdx++
				}
			} else if m.focus >= 2 && m.focus <= 3 {
				m.selectAll = false
				inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-2], cmd = inputs[m.focus-2].Update(msg)
				return m, cmd
			}
		case "tab":
			m.selectAll = false
			m.setFocus((m.focus + 1) % 8)
		case "shift+tab":
			m.selectAll = false
			m.setFocus((m.focus - 1 + 8) % 8)
		case "enter":
			if m.focus == 0 {
				m.selectPlaylist()
			} else if m.focus == 1 {
				return m, m.openArtDialog()
			} else if m.focus == 4 {
				if m.addMode {
					return m.addNewPlaylist()
				}
				return m.savePlaylist()
			} else if m.focus == 5 {
				return m.deleteCurrentPlaylist()
			} else if m.focus == 6 {
				m.enterAddMode()
			} else if m.focus == 7 {
				// Reset playlist image
				if pl := m.selectedPlaylist(); pl != nil {
					if pl.ArtPath != "" {
						_ = os.Remove(pl.ArtPath)
						pl.ArtPath = ""
						state.Current.CurrentPlaylist.ArtPath = ""
						m.playlistStatus = ui.AccentStyle.Render("  v " + langT("Image reset", "Resim sifirlandi"))
						_ = state.Current.ScanProfiles()
					}
					m.setFocus(0)
				}
			}
		case "delete":
			if m.focus == 5 {
				return m.deleteCurrentPlaylist()
			}
		case "esc":
			if m.addMode {
				m.cancelAddMode()
			} else {
				m.setFocus(0)
			}
		case "ctrl+v":
			if m.focus >= 2 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
				*inputs[m.focus-2], _ = inputs[m.focus-2].Update(textinput.Paste())
				return m, nil
			}
		case "ctrl+a":
			if m.focus >= 2 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
				if inputs[m.focus-2].Value() != "" {
					m.selectAll = true
				}
			}
			return m, nil
		default:
			if m.focus >= 2 && m.focus <= 3 {
				if m.selectAll {
					inp := []*textinput.Model{&m.plNameInput, &m.plBioInput}[m.focus-2]
					s := msg.String()
					if len(s) == 1 || s == "backspace" || s == "delete" {
						inp.SetValue("")
						inp.SetCursor(0)
						m.selectAll = false
					} else {
						m.selectAll = false
					}
				}
				inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
				var cmd tea.Cmd
				*inputs[m.focus-2], cmd = inputs[m.focus-2].Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *PlaylistModel) setFocus(idx int) {
	if idx < 0 || idx >= 8 {
		return
	}
	m.focus = idx
	inputs := []*textinput.Model{&m.plNameInput, &m.plBioInput}
	for i, inp := range inputs {
		if i+2 == idx {
			inp.Focus()
		} else {
			inp.Blur()
		}
	}
}

func (m *PlaylistModel) cycleFocus() bool {
	if m.focus == 0 {
		m.setFocus(4)
	} else {
		if m.addMode {
			m.cancelAddMode()
		}
		m.setFocus(0)
	}
	return false
}

func (m *PlaylistModel) selectPlaylist() {
	cp := state.Current.CurrentProfile
	if cp == nil {
		return
	}
	if m.playlistFocusIdx >= 0 && m.playlistFocusIdx < len(cp.Playlists) {
		state.Current.CurrentPlaylist = &cp.Playlists[m.playlistFocusIdx]
		m.cancelAddMode()
		m.refreshOptions()
	}
}

func (m *PlaylistModel) enterAddMode() {
	m.addMode = true
	m.artPath = ""
	m.plNameInput.SetValue("")
	m.plBioInput.SetValue("")
	m.playlistStatus = ""
	m.setFocus(2)
}

func (m *PlaylistModel) cancelAddMode() {
	m.addMode = false
	pl := m.selectedPlaylist()
	if pl != nil {
		m.plNameInput.SetValue(pl.Name)
		m.plNameInput.SetCursor(len(pl.Name))
		m.plBioInput.SetValue(pl.Bio)
		m.plBioInput.SetCursor(len(pl.Bio))
		m.artPath = ""
	}
	m.playlistStatus = ""
	m.setFocus(0)
}

func (m *PlaylistModel) addNewPlaylist() (tea.Model, tea.Cmd) {
	cp := state.Current.CurrentProfile
	if cp == nil {
		m.playlistStatus = ui.ErrorStyle.Render("  x " + langT("No profile", "Profil yok"))
		return m, nil
	}
	name := strings.TrimSpace(m.plNameInput.Value())
	if name == "" {
		m.playlistStatus = ui.ErrorStyle.Render("  x " + langT("Name is required", "İsim gerekli"))
		return m, nil
	}
	for _, pl := range cp.Playlists {
		if pl.Name == name {
			m.playlistStatus = ui.ErrorStyle.Render("  x " + langT("Name already exists", "Bu isim zaten var"))
			return m, nil
		}
	}
	folder := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	bio := strings.TrimSpace(m.plBioInput.Value())
	artSrc := strings.TrimSpace(m.artPath)
	if err := state.Current.CreatePlaylistStructure(cp.FolderName, folder, name, bio, artSrc); err != nil {
		m.playlistStatus = ui.ErrorStyle.Render("  x " + err.Error())
		return m, nil
	}
	_ = state.Current.ScanProfiles()
	for i, p := range state.Current.Profiles {
		if p.FolderName == cp.FolderName {
			state.Current.CurrentProfile = &state.Current.Profiles[i]
			for j, pl := range p.Playlists {
				if pl.FolderName == folder {
					state.Current.CurrentPlaylist = &p.Playlists[j]
					m.playlistFocusIdx = j
					break
				}
			}
			break
		}
	}
	m.addMode = false
	m.refreshOptions()
	m.playlistStatus = ui.AccentStyle.Render("  v " + langT("Playlist created!", "Playlist oluşturuldu!"))
	m.setFocus(0) // return to first button after creation
	return m, nil
}

func (m *PlaylistModel) savePlaylist() (tea.Model, tea.Cmd) {
	cp := state.Current.CurrentProfile
	if cp == nil {
		return m, nil
	}
	pl := m.selectedPlaylist()
	if pl == nil {
		return m, nil
	}
	name := strings.TrimSpace(m.plNameInput.Value())
	if name == "" {
		name = pl.FolderName
	}
	bio := strings.TrimSpace(m.plBioInput.Value())
	artSrc := strings.TrimSpace(m.artPath)
	if err := state.Current.SavePlaylistMeta(cp.FolderName, pl.FolderName, name, bio); err != nil {
		m.playlistStatus = ui.ErrorStyle.Render("  x " + err.Error())
		return m, nil
	}
	if artSrc != "" {
		plDir := state.Current.PlaylistDir(cp.FolderName, pl.FolderName)
		artDir := filepath.Join(plDir, "playlist_avatar")
		ext := ".jpg"
		if strings.HasSuffix(strings.ToLower(artSrc), ".png") {
			ext = ".png"
		}
		destPath := filepath.Join(artDir, "avatar"+ext)
		if err := state.CopyFile(artSrc, destPath); err != nil {
			m.playlistStatus = ui.ErrorStyle.Render("  x " + err.Error())
			return m, nil
		}
		pl.ArtPath = destPath
	}
	_ = state.Current.ScanProfiles()
	for i, p := range state.Current.Profiles {
		if p.FolderName == cp.FolderName {
			state.Current.CurrentProfile = &state.Current.Profiles[i]
			if m.playlistFocusIdx < len(p.Playlists) {
				state.Current.CurrentPlaylist = &p.Playlists[m.playlistFocusIdx]
			}
			break
		}
	}
	m.refreshOptions()
	m.playlistStatus = ui.AccentStyle.Render("  v " + langT("Saved!", "Kaydedildi!"))
	return m, nil
}

func (m *PlaylistModel) openArtDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := dialog.File().
			Filter(langT("Image Files", "Resim Dosyalari"), "jpg", "jpeg", "png").
			Title(langT("Select Art Image", "Resim Sec")).
			Load()
		if err != nil || selectedPath == "" {
			return nil
		}
		info, err := os.Stat(selectedPath)
		if err != nil {
			return nil
		}
		if info.Size() > 1024*1024 {
			return ArtFileTooLargeMsg{}
		}
		return ArtFileSelectedMsg{Path: selectedPath}
	}
}

func (m *PlaylistModel) deleteCurrentPlaylist() (tea.Model, tea.Cmd) {
	cp := state.Current.CurrentProfile
	pl := m.selectedPlaylist()
	if cp == nil || pl == nil {
		return m, nil
	}
	_ = state.Current.DeletePlaylist(cp.FolderName, pl.FolderName)
	_ = state.Current.ScanProfiles()
	for i, p := range state.Current.Profiles {
		if p.FolderName == cp.FolderName {
			state.Current.CurrentProfile = &state.Current.Profiles[i]
			if len(p.Playlists) > 0 {
				idx := m.playlistFocusIdx
				if idx >= len(p.Playlists) {
					idx = len(p.Playlists) - 1
				}
				state.Current.CurrentPlaylist = &p.Playlists[idx]
				m.playlistFocusIdx = idx
			} else {
				state.Current.CurrentPlaylist = nil
				m.playlistFocusIdx = 0
			}
			break
		}
	}
	m.refreshOptions()
	if m.playlistFocusIdx >= len(m.playlistOptions) {
		m.playlistFocusIdx = 0
	}
	m.playlistStatus = ui.DimStyle.Render("  " + langT("Deleted", "Silindi"))
	return m, nil
}

func (m *PlaylistModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	leftW := m.leftColWidth
	if leftW < 20 {
		leftW = 20
	}
	rightW := m.width - leftW - 4
	if rightW < 40 {
		rightW = 40
	}

	rightPanel := m.renderRightPanel(rightW)
	rightH := lipgloss.Height(rightPanel)
	leftPanel := m.renderLeftPanel(leftW, rightH)

	leftH := lipgloss.Height(leftPanel)
	if leftH < rightH {
		leftPanel += strings.Repeat("\n", rightH-leftH)
	} else if rightH < leftH {
		rightPanel += strings.Repeat("\n", leftH-rightH)
	}

	joined := lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		"  ",
		rightPanel,
	)

	return joined
}

func (m *PlaylistModel) renderLeftPanel(w, maxH int) string {
	innerH := maxH - 4
	if innerH < 3 {
		innerH = 3
	}
	maxVisible := innerH - 1
	if maxVisible < 1 {
		maxVisible = 1
	}

	total := len(m.playlistOptions)

	if m.playlistFocusIdx < m.playlistOffset {
		m.playlistOffset = m.playlistFocusIdx
	}
	if m.playlistFocusIdx >= m.playlistOffset+maxVisible {
		m.playlistOffset = m.playlistFocusIdx - maxVisible + 1
	}
	if m.playlistOffset > total-maxVisible && m.playlistOffset > 0 {
		m.playlistOffset = total - maxVisible
	}
	if m.playlistOffset < 0 {
		m.playlistOffset = 0
	}

	var lines []string
	if total == 0 {
		lines = append(lines, ui.DimStyle.Render("  "+langT("No playlists", "Playlist yok")))
	} else {
		end := m.playlistOffset + maxVisible
		if end > total {
			end = total
		}
		for i := m.playlistOffset; i < end; i++ {
			item := "  " + m.playlistOptions[i]
			if m.focus == 0 && i == m.playlistFocusIdx {
				item = ui.AccentStyle.Render("> " + m.playlistOptions[i])
			}
			lines = append(lines, item)
		}
	}

	content := strings.Join(lines, "\n")
	contentH := lipgloss.Height(content)
	if contentH < innerH {
		content += strings.Repeat("\n", innerH-contentH)
	}

	title := ui.SectionTitleStyle.Render(langT(" Playlists", " Playlistler"))
	box := ui.BorderStyle.
		Width(w).
		Height(maxH - 2).
		Render(title + "\n" + content)

	return box
}

func (m *PlaylistModel) renderRightPanel(w int) string {
	plV := "-"
	pl := m.selectedPlaylist()
	if pl != nil {
		plV = pl.Name
	}

	titlePrefix := langT(" Playlist", " Playlist")
	if m.addMode {
		titlePrefix = langT(" New Playlist", " Yeni Playlist")
		plV = langT("(creating new)", "(yeni oluşturuluyor)")
	}

	artVal := m.artPath
	if artVal == "" {
		artVal = langT("(click to select image)", "(dosya secmek icin tikla)")
	}
	var artV string
	if m.focus == 1 {
		if m.artPath == "" {
			artV = ui.AccentBorderStyle.Render("  Art Path:  " + ui.DimStyle.Render(artVal))
		} else {
			artV = ui.AccentBorderStyle.Render("  Art Path:  " + ui.WhiteStyle.Render(artVal))
		}
	} else {
		artV = "  Art Path:  " + ui.WhiteStyle.Render(artVal)
	}

	plNameVal := m.plNameInput.Value()
	if plNameVal == "" && !m.addMode {
		plNameVal = m.plNameInput.Placeholder
	}
	var plNameV string
	if m.focus == 2 {
		plNameV = m.plNameInput.View()
	} else {
		plNameV = "  Playlist Name:  " + ui.WhiteStyle.Render(plNameVal)
	}

	plBioVal := m.plBioInput.Value()
	if plBioVal == "" && !m.addMode {
		plBioVal = m.plBioInput.Placeholder
	}
	var plBioV string
	if m.focus == 3 {
		plBioV = m.plBioInput.View()
	} else {
		plBioV = "  Description:  " + ui.WhiteStyle.Render(plBioVal)
	}

	saveLabel := langT("  Save  ", "  Kaydet  ")
	if m.addMode {
		saveLabel = langT("  Create  ", "  Oluştur  ")
	}
	saveBtn := ui.AccentButtonStyle.Render(saveLabel)
	deleteBtn := ui.ErrorButtonStyle.Render(langT("  Playlist Sil  ", "  Playlist Sil  "))
	addBtn := ui.ButtonStyle.Render(langT("  Playlist Ekle  ", "  Playlist Ekle  "))
	resetBtn := ui.ButtonStyle.Render(langT("  Resmi Sıfırla  ", "  Resmi Sıfırla  "))

	if m.focus == 4 {
		saveBtn = ui.FocusedButtonStyle.Render(saveLabel)
	}
	if m.focus == 5 {
		deleteBtn = ui.FocusedButtonStyle.Render(langT("  Playlist Sil  ", "  Playlist Sil  "))
	}
	if m.focus == 6 {
		addBtn = ui.FocusedButtonStyle.Render(langT("  Playlist Ekle  ", "  Playlist Ekle  "))
	}
	// Reset image button (focus index 7)
	if m.focus == 7 {
		resetBtn = ui.FocusedButtonStyle.Render(langT("  Resmi Sıfırla  ", "  Resmi Sıfırla  "))
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" "+titlePrefix+": ")+plV,
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
		lipgloss.JoinHorizontal(lipgloss.Left, saveBtn, "  ", deleteBtn, "  ", addBtn),
		resetBtn,
	)

	title := ui.SectionTitleStyle.Render(langT(" Playlist Settings", " Playlist Ayarlari"))
	box := ui.BorderStyle.
		Width(w).
		Render(title + "\n" + boxContent)

	return box
}
