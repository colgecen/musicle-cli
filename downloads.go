package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ncruces/zenity"

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

	logLines      []string
	consoleScroll int

	isDownloading     bool
	downloadStart     time.Time
	downloadedTracks  int
	failedTracks      int
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
		if m.focusIdx == 3 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx - 1 + len(m.playlistOptions)) % len(m.playlistOptions)
			return m, nil
		}
	case "down", "j":
		if m.focusIdx == 3 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx + 1) % len(m.playlistOptions)
			return m, nil
		}
	case "pgup":
		if m.focusIdx == 0 {
			m.consoleScroll -= 10
			return m, nil
		}
	case "pgdown":
		if m.focusIdx == 0 {
			m.consoleScroll += 10
			return m, nil
		}
	case "home":
		if m.focusIdx == 0 {
			m.consoleScroll = 0
			return m, nil
		}
	case "end":
		if m.focusIdx == 0 {
			m.consoleScroll = -1
			return m, nil
		}
	case "ctrl+v":
		if m.focusIdx == 1 || m.focusIdx == 2 {
			var cmd tea.Cmd
			if m.focusIdx == 1 {
				m.spotifyInput, cmd = m.spotifyInput.Update(textinput.Paste())
			} else {
				m.youtubeInput, cmd = m.youtubeInput.Update(textinput.Paste())
			}
			return m, cmd
		}
	case "ctrl+a":
		if m.focusIdx == 1 || m.focusIdx == 2 {
			m.currentInput().SetValue("")
			return m, nil
		}
	default:
		if m.focusIdx == 1 {
			var cmd tea.Cmd
			m.spotifyInput, cmd = m.spotifyInput.Update(msg)
			return m, cmd
		} else if m.focusIdx == 2 {
			var cmd tea.Cmd
			m.youtubeInput, cmd = m.youtubeInput.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *DownloadsModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusIdx {
	case 1, 2:
		return m, m.startDownload()
	case 3:
		return m, nil
	case 4:
		return m, m.openLocalPlaylistDialog()
	case 5:
		return m, m.openLocalMusicDialog()
	case 6:
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
	order := []int{0, 1, 2, 3, 4, 5, 6}
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
	case 1:
		m.spotifyInput.Focus()
	case 2:
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
	if m.focusIdx == 1 {
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
		m.addLog("error", "Already downloading")
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
		m.addLog("error", "Enter a URL first")
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.addLog("error", "Invalid URL")
		return nil
	}
	m.isDownloading = true
	m.downloadStart = time.Now()
	m.downloadPercent = 0
	m.downloadStatus = "0%"
	m.downloadedTracks = 0
	m.failedTracks = 0
	m.addLog("info", fmt.Sprintf("Starting: %s", url))
	return func() tea.Msg {
		return StartDownloadMsg{Action: action, URL: url, Output: outDir}
	}
}

func (m *DownloadsModel) handleDownloadResult(msg DownloadResultMsg) {
	m.isDownloading = false
	elapsed := time.Since(m.downloadStart).Truncate(time.Second)

	if msg.Error != nil {
		m.failedTracks++
		m.addLog("error", fmt.Sprintf("%s (%v)", msg.Error.Error(), elapsed))
		return
	}
	if msg.Result == nil {
		m.failedTracks++
		m.addLog("error", fmt.Sprintf("No result from download (%v)", elapsed))
		return
	}
	if msg.Result.Status == "error" {
		m.failedTracks++
		m.addLog("error", fmt.Sprintf("%s (%v)", msg.Result.Error, elapsed))
		return
	}

	msgText := msg.Result.Message
	if msgText == "" {
		msgText = "Download complete"
	}
	m.downloadedTracks++
	m.addLog("ok", fmt.Sprintf("%s (%v)", msgText, elapsed))
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

var (
	logTimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	logErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
	logOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))  // green
	logInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))  // cyan
)

func (m *DownloadsModel) addLog(level, msg string) {
	now := time.Now().Format("15:04:05")
	ts := logTimeStyle.Render(now)
	var line string
	switch level {
	case "error":
		line = fmt.Sprintf("%s %s", ts, logErrStyle.Render("ERR "+msg))
	case "ok":
		line = fmt.Sprintf("%s %s", ts, logOKStyle.Render("OK  "+msg))
	case "info":
		line = fmt.Sprintf("%s %s", ts, logInfoStyle.Render("... "+msg))
	default:
		line = fmt.Sprintf("%s  %s", ts, msg)
	}
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
}

func (m *DownloadsModel) renderConsole(bodyH int) string {
	w := 38
	if m.width > 0 {
		w = m.width / 3
		if w < 40 {
			w = 40
		}
		if w > 55 {
			w = 55
		}
	}
	title := ui.SectionTitleStyle.Render(langT("CONSOLE", "KONSOL"))
	visible := bodyH - 4
	if visible < 8 {
		visible = 8
	}
	contentW := w - 4

	logCount := len(m.logLines)
	totalLines := logCount

	maxScroll := totalLines - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.consoleScroll < 0 || m.consoleScroll > maxScroll {
		m.consoleScroll = maxScroll
	}

	start := m.consoleScroll
	end := start + visible
	if end > totalLines {
		end = totalLines
	}

	haveScrollbar := totalLines > visible

	var inner string
	if totalLines == 0 {
		inner = title + "\n" + ui.FaintStyle.Render("  No logs")
	} else {
		var contentParts []string
		for i := start; i < end; i++ {
			raw := m.logLines[i]
			if len(raw) > contentW-1 {
				raw = raw[:contentW-1]
			}
			if strings.HasPrefix(raw, "x ") {
				contentParts = append(contentParts, ui.ErrorStyle.Render(raw))
			} else if strings.HasPrefix(raw, "v ") {
				contentParts = append(contentParts, lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render(raw))
			} else {
				contentParts = append(contentParts, ui.FaintStyle.Render(raw))
			}
		}
		contentStr := strings.Join(contentParts, "\n")

		if haveScrollbar {
			thumbPos := 0
			if maxScroll > 0 {
				thumbPos = int(float64(m.consoleScroll) / float64(maxScroll) * float64(visible-1))
			}
			var scrollParts []string
			for i := 0; i < visible; i++ {
				if i == thumbPos {
					scrollParts = append(scrollParts, ui.AccentStyle.Render("█"))
				} else {
					scrollParts = append(scrollParts, ui.FaintStyle.Render("│"))
				}
			}
			scrollStr := strings.Join(scrollParts, "\n")
			contentStr = lipgloss.JoinHorizontal(lipgloss.Top, contentStr, " ", scrollStr)
		}

		inner = title + "\n" + contentStr
	}

	innerH := lipgloss.Height(inner)
	targetH := bodyH - 2
	if innerH < targetH {
		inner += strings.Repeat("\n", targetH-innerH)
	}

	consoleStyle := ui.BorderStyle
	if m.focusIdx == 0 {
		consoleStyle = ui.AccentBorderStyle
	}
	box := consoleStyle.Width(w).Render(inner)
	boxH := lipgloss.Height(box)
	if boxH < bodyH {
		box += strings.Repeat("\n", bodyH-boxH)
	}
	return box
}

func (m *DownloadsModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	console := m.renderConsole(m.height)

	spotifyV := m.spotifyInput.View()
	if m.focusIdx != 1 {
		val := m.spotifyInput.Value()
		if val == "" {
			val = m.spotifyInput.Placeholder
		}
		spotifyV = "  Spotify URL:  " + ui.WhiteStyle.Render(val)
	}
	youtubeV := m.youtubeInput.View()
	if m.focusIdx != 2 {
		val := m.youtubeInput.Value()
		if val == "" {
			val = m.youtubeInput.Placeholder
		}
		youtubeV = "  YouTube URL:  " + ui.WhiteStyle.Render(val)
	}

	playlistBtn := ui.ButtonStyle.Render("  + Playlist  ")
	musicBtn := ui.ButtonStyle.Render("  + Music  ")
	if m.focusIdx == 4 {
		playlistBtn = ui.FocusedOutlineStyle.Render("  + Playlist  ")
	}
	if m.focusIdx == 5 {
		musicBtn = ui.FocusedOutlineStyle.Render("  + Music  ")
	}
	localBtn := lipgloss.JoinHorizontal(lipgloss.Left, playlistBtn, "  ", musicBtn)

	playlistV := m.viewPlaylistDropdown()

	dlBtn := ui.AccentButtonStyle.Render("  v Download  ")
	if m.focusIdx == 6 {
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

	title := ui.SectionTitleStyle.Render(" " + langT("Music Download", "Müzik İndirme") + " ")
	downloadBox := ui.AccentBorderStyle.Width(75).Render(title + "\n" + boxContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, console, downloadBox)
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
	if m.focusIdx == 3 {
		return label + ui.AccentStyle.Render(current)
	}
	return label + ui.WhiteStyle.Render(current)
}
