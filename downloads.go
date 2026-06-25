package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ncruces/zenity"

	"MusicLeCLI/bridge"
	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

type DownloadsModel struct {
	width  int
	height int

	focusIdx    int
	playlistIdx int

	spotifyInput textinput.Model
	youtubeInput textinput.Model

	sidebarError      string
	sidebarErrIsError bool
	isDownloading     bool
	downloadPercent   float64
	downloadStatus    string

	playlistOptions []string
}

func NewDownloadsModel() *DownloadsModel {
	cursorStyle := lipgloss.NewStyle().
		Background(ui.ColorAccent).
		Foreground(lipgloss.Color("#000000"))

	si := textinput.New()
	si.Placeholder = "https://open.spotify.com/..."
	si.Prompt = "  Spotify URL:  "
	si.PromptStyle = ui.AccentStyle
	si.TextStyle = ui.WhiteStyle
	si.PlaceholderStyle = ui.DimStyle
	si.Cursor.Style = cursorStyle
	si.Width = 60
	si.CharLimit = 300

	yi := textinput.New()
	yi.Placeholder = "https://youtube.com/..."
	yi.Prompt = "  YouTube URL:  "
	yi.PromptStyle = ui.AccentStyle
	yi.TextStyle = ui.WhiteStyle
	yi.PlaceholderStyle = ui.DimStyle
	yi.Cursor.Style = cursorStyle
	yi.Width = 60
	yi.CharLimit = 300

	return &DownloadsModel{
		spotifyInput: si,
		youtubeInput: yi,
		playlistIdx:  0,
	}
}

func (m *DownloadsModel) Init() tea.Cmd {
	m.refreshPlaylistOptions()
	return nil
}

func (m *DownloadsModel) refreshPlaylistOptions() {
	m.playlistOptions = nil
	if state.Current.CurrentProfile != nil {
		for _, pl := range state.Current.CurrentProfile.Playlists {
			m.playlistOptions = append(m.playlistOptions, pl.Name)
		}
	}
	if len(m.playlistOptions) == 0 {
		m.playlistOptions = []string{"(no playlists)"}
	}
	if m.playlistIdx >= len(m.playlistOptions) {
		m.playlistIdx = 0
	}
}

func (m *DownloadsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case StartDownloadMsg:
		return m, m.handleDownloadStart(msg)

	case DownloadResultMsg:
		m.handleDownloadResult(msg)

	}

	return m, nil
}

func (m *DownloadsModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.cycleFocusDir(1)
		return m, nil
	case "shift+tab":
		m.cycleFocusDir(-1)
		return m, nil
	case "enter":
		return m.handleEnter()
	case "up", "k":
		if m.focusIdx == 2 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx - 1 + len(m.playlistOptions)) % len(m.playlistOptions)
			return m, nil
		}
	case "down", "j":
		if m.focusIdx == 2 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx + 1) % len(m.playlistOptions)
			return m, nil
		}
	case "ctrl+v":
		if m.focusIdx == 0 || m.focusIdx == 1 {
			var cmd tea.Cmd
			if m.focusIdx == 0 {
				m.spotifyInput, cmd = m.spotifyInput.Update(textinput.Paste())
			} else {
				m.youtubeInput, cmd = m.youtubeInput.Update(textinput.Paste())
			}
			return m, cmd
		}
	case "ctrl+a":
		if m.focusIdx == 0 || m.focusIdx == 1 {
			m.currentInput().SetValue("")
			return m, nil
		}
	default:
		if m.focusIdx == 0 {
			var cmd tea.Cmd
			m.spotifyInput, cmd = m.spotifyInput.Update(msg)
			return m, cmd
		} else if m.focusIdx == 1 {
			var cmd tea.Cmd
			m.youtubeInput, cmd = m.youtubeInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *DownloadsModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusIdx {
	case 0, 1:
		return m, m.startDownload()
	case 2:
		return m, nil
	case 3:
		return m, m.openLocalPlaylistDialog()
	case 4:
		return m, m.openLocalMusicDialog()
	case 5:
		return m, m.startDownload()
	}
	return m, nil
}

func (m *DownloadsModel) cycleFocusDir(dir int) {
	inputs := m.focusedInputs()
	for _, inp := range inputs {
		if inp != nil {
			inp.Blur()
		}
	}
	order := []int{0, 1, 2, 3, 4, 5}
	cur := -1
	for i, v := range order {
		if v == m.focusIdx {
			cur = i
			break
		}
	}
	if cur == -1 {
		m.focusIdx = order[0]
	} else {
		next := (cur + dir + len(order)) % len(order)
		m.focusIdx = order[next]
	}
	switch m.focusIdx {
	case 0:
		m.spotifyInput.Focus()
	case 1:
		m.youtubeInput.Focus()
	}
}

func (m *DownloadsModel) cycleFocus() bool {
	m.cycleFocusDir(1)
	return m.focusIdx == 0
}

func (m *DownloadsModel) focusedInputs() []*textinput.Model {
	return []*textinput.Model{&m.spotifyInput, &m.youtubeInput}
}

func (m *DownloadsModel) currentInput() *textinput.Model {
	if m.focusIdx == 0 {
		return &m.spotifyInput
	}
	return &m.youtubeInput
}

// RefreshTheme updates input styles to match the current theme accent.
func (m *DownloadsModel) RefreshTheme() {
	cursorStyle := lipgloss.NewStyle().
		Background(ui.ColorAccent).
		Foreground(lipgloss.Color("#000000"))
	m.spotifyInput.Cursor.Style = cursorStyle
	m.spotifyInput.PromptStyle = ui.AccentStyle
	m.youtubeInput.Cursor.Style = cursorStyle
	m.youtubeInput.PromptStyle = ui.AccentStyle
}

func (m *DownloadsModel) startDownload() tea.Cmd {
	if m.isDownloading {
		m.sidebarError = "Already downloading!"
		m.sidebarErrIsError = true
		return nil
	}
	spotifyURL := strings.TrimSpace(m.spotifyInput.Value())
	youtubeURL := strings.TrimSpace(m.youtubeInput.Value())

	outDir := ""
	if state.Current.CurrentProfile != nil && m.playlistIdx >= 0 && m.playlistIdx < len(state.Current.CurrentProfile.Playlists) {
		pl := state.Current.CurrentProfile.Playlists[m.playlistIdx]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	} else {
		if state.Current.CurrentProfile != nil && len(state.Current.CurrentProfile.Playlists) > 0 {
			pl := state.Current.CurrentProfile.Playlists[0]
			outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
		}
	}

	url := spotifyURL
	action := "download_spotify"
	if url == "" {
		url = youtubeURL
		action = "download_youtube"
	}
	if url == "" {
		m.sidebarError = "Enter a URL first"
		m.sidebarErrIsError = true
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.sidebarError = "Invalid URL"
		m.sidebarErrIsError = true
		return nil
	}
	m.isDownloading = true
	m.downloadPercent = 0
	m.downloadStatus = "0%"
	m.sidebarError = "Downloading..."
	m.sidebarErrIsError = false
	return func() tea.Msg {
		return StartDownloadMsg{Action: action, URL: url, Output: outDir}
	}
}

func (m *DownloadsModel) handleDownloadStart(msg StartDownloadMsg) tea.Cmd {
	return func() tea.Msg {
		result, err := bridge.RunScriptDownload(bridge.Action{
			Action: msg.Action,
			URL:    msg.URL,
			Output: msg.Output,
		})
		return DownloadResultMsg{Result: result, Error: err, Action: msg.Action}
	}
}

func (m *DownloadsModel) handleDownloadResult(msg DownloadResultMsg) {
	m.isDownloading = false
	if msg.Error != nil {
		m.sidebarError = "x " + msg.Error.Error()
		m.sidebarErrIsError = true
		return
	}
	if msg.Result == nil {
		m.sidebarError = "x No result"
		m.sidebarErrIsError = true
		return
	}
	if msg.Result.Status == "error" {
		m.sidebarError = "x " + msg.Result.Error
		m.sidebarErrIsError = true
		return
	}
	m.sidebarError = msg.Result.Message
	if m.sidebarError == "" {
		m.sidebarError = "v Download complete"
	}
	m.sidebarErrIsError = false
}

func (m *DownloadsModel) openLocalPlaylistDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := zenity.SelectFile(
			zenity.Title("Select Music Directory"),
			zenity.Directory(),
		)
		if err != nil || selectedPath == "" {
			return nil
		}
		if state.Current.CurrentProfile == nil || state.Current.CurrentPlaylist == nil {
			return nil
		}
		outDir := state.Current.PlaylistDir(
			state.Current.CurrentProfile.FolderName,
			state.Current.CurrentPlaylist.FolderName,
		)
		return LocalFileImportMsg{FilePath: selectedPath, Output: outDir}
	}
}

func (m *DownloadsModel) openLocalMusicDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := zenity.SelectFile(
			zenity.Title("Select Audio Files"),
			zenity.FileFilter{
				Name:     "Audio Files",
				Patterns: []string{"*.mp3"},
			},
		)
		if err != nil || selectedPath == "" {
			return nil
		}
		if state.Current.CurrentProfile == nil || state.Current.CurrentPlaylist == nil {
			return nil
		}
		outDir := state.Current.PlaylistDir(
			state.Current.CurrentProfile.FolderName,
			state.Current.CurrentPlaylist.FolderName,
		)
		return LocalFileImportMsg{FilePath: selectedPath, Output: outDir}
	}
}

func (m *DownloadsModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	spotifyV := m.spotifyInput.View()
	if m.focusIdx != 0 {
		val := m.spotifyInput.Value()
		if val == "" {
			val = m.spotifyInput.Placeholder
		}
		spotifyV = "  Spotify URL:  " + ui.WhiteStyle.Render(val)
	}
	youtubeV := m.youtubeInput.View()
	if m.focusIdx != 1 {
		val := m.youtubeInput.Value()
		if val == "" {
			val = m.youtubeInput.Placeholder
		}
		youtubeV = "  YouTube URL:  " + ui.WhiteStyle.Render(val)
	}

	playlistBtn := ui.ButtonStyle.Render("  + Playlist  ")
	musicBtn := ui.ButtonStyle.Render("  + Music  ")
	if m.focusIdx == 3 {
		playlistBtn = ui.FocusedOutlineStyle.Render("  + Playlist  ")
	}
	if m.focusIdx == 4 {
		musicBtn = ui.FocusedOutlineStyle.Render("  + Music  ")
	}
	localBtn := lipgloss.JoinHorizontal(lipgloss.Left, playlistBtn, "  ", musicBtn)

	playlistV := m.viewPlaylistDropdown()
	if m.focusIdx == 2 && m.playlistIdx < len(m.playlistOptions) {
		playlistV = ui.AccentBorderStyle.Render(m.playlistOptions[m.playlistIdx])
	}

	dlBtn := ui.AccentButtonStyle.Render("  v Download  ")
	if m.focusIdx == 5 {
		dlBtn = ui.FocusedButtonStyle.Render("  v Download  ")
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		spotifyV,
		"",
		youtubeV,
		"",
		localBtn, "",
		playlistV, "",
		dlBtn,
	)

	if m.sidebarError != "" {
		errStyle := ui.ErrorStyle
		if !m.sidebarErrIsError {
			errStyle = lipgloss.NewStyle().Foreground(ui.ColorAccent)
		}
		boxContent += "\n\n" + errStyle.Render(m.sidebarError)
	}

	title := ui.SectionTitleStyle.Render(" " + langT("Music Download", "Müzik İndirme") + " ")
	return ui.AccentBorderStyle.Width(75).Render(title + "\n" + boxContent)
}

func (m *DownloadsModel) viewPlaylistDropdown() string {
	if len(m.playlistOptions) == 0 {
		return "  " + ui.FaintStyle.Render("(no playlists)")
	}
	if m.playlistIdx >= len(m.playlistOptions) {
		m.playlistIdx = 0
	}
	if m.playlistOptions[0] == "(no playlists)" {
		return "  " + ui.FaintStyle.Render("(no playlists)")
	}
	label := ui.AccentStyle.Render("  Playlist:  ")
	current := m.playlistOptions[m.playlistIdx]
	return label + ui.WhiteStyle.Render(current)
}
