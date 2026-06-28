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

type downloadHistoryItem struct {
	title  string
	status string // ok, error
	time   time.Time
}

type DownloadsModel struct {
	width  int
	height int

	focusIdx    int
	playlistIdx int

	spotifyInput textinput.Model
	youtubeInput textinput.Model
	plURLInput   textinput.Model // playlist download URL

	logLines      []string
	consoleScroll int

	isDownloading     bool
	downloadStart     time.Time
	downloadedTracks  int
	failedTracks      int
	downloadHistory   []downloadHistoryItem
	downloadPercent   float64
	downloadStatus    string
	lastLoggedStatus  string // dedup progress logs

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

	pi := textinput.New()
	pi.Placeholder = "https://open.spotify.com/playlist/... veya https://youtube.com/playlist?..."
	pi.Prompt = "  Playlist URL:  "
	pi.PromptStyle = ui.AccentStyle
	pi.TextStyle = ui.WhiteStyle
	pi.PlaceholderStyle = ui.DimStyle
	pi.Cursor.Style = cursorStyle
	pi.Width = 60
	pi.CharLimit = 500

	return &DownloadsModel{
		spotifyInput: si,
		youtubeInput: yi,
		plURLInput:   pi,
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
		if m.focusIdx == 1 || m.focusIdx == 2 || m.focusIdx == 7 {
			var cmd tea.Cmd
			switch m.focusIdx {
			case 1:
				m.spotifyInput, cmd = m.spotifyInput.Update(textinput.Paste())
			case 2:
				m.youtubeInput, cmd = m.youtubeInput.Update(textinput.Paste())
			case 7:
				m.plURLInput, cmd = m.plURLInput.Update(textinput.Paste())
			}
			return m, cmd
		}
	case "ctrl+a":
		if m.focusIdx == 1 || m.focusIdx == 2 || m.focusIdx == 7 {
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
		} else if m.focusIdx == 7 {
			var cmd tea.Cmd
			m.plURLInput, cmd = m.plURLInput.Update(msg)
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
	case 7:
		return m, m.startPlaylistDownload()
	case 8:
		return m, m.startPlaylistDownload()
	}
	return m, nil
}

func (m *DownloadsModel) cycleFocusDir(dir int) {
	for _, inp := range m.focusedInputs() {
		if inp != nil {
			inp.Blur()
		}
	}
	order := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
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
	case 7:
		m.plURLInput.Focus()
	}
}

func (m *DownloadsModel) cycleFocus() bool {
	m.cycleFocusDir(1)
	return m.focusIdx == 0
}

func (m *DownloadsModel) focusedInputs() []*textinput.Model {
	return []*textinput.Model{&m.spotifyInput, &m.youtubeInput, &m.plURLInput}
}

func (m *DownloadsModel) currentInput() *textinput.Model {
	if m.focusIdx == 1 {
		return &m.spotifyInput
	} else if m.focusIdx == 2 {
		return &m.youtubeInput
	} else if m.focusIdx == 7 {
		return &m.plURLInput
	}
	return &m.spotifyInput
}

// TrackProgress logs download progress to console (deduplicated).
func (m *DownloadsModel) TrackProgress(pct float64, status string) {
	if status == "" || status == m.lastLoggedStatus {
		return
	}
	m.lastLoggedStatus = status
	level := "info"
	if pct < 100 && (strings.Contains(status, "Error") || strings.Contains(status, "error") || strings.Contains(status, "fail")) {
		level = "error"
	}
	m.addLog(level, status)
}

// startPlaylistDownload starts downloading a playlist URL.
func (m *DownloadsModel) startPlaylistDownload() tea.Cmd {
	if m.isDownloading {
		m.addLog("error", Tr("dl.error")+": already downloading")
		return nil
	}
	url := strings.TrimSpace(m.plURLInput.Value())
	if url == "" {
		m.addLog("error", "Enter a playlist URL first")
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.addLog("error", "Invalid playlist URL")
		return nil
	}

	outDir := ""
	if state.Current.CurrentProfile != nil && len(state.Current.CurrentProfile.Playlists) > 0 {
		pl := state.Current.CurrentProfile.Playlists[0]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	}

	m.isDownloading = true
	m.downloadStart = time.Now()
	m.downloadPercent = 0
	m.downloadStatus = "0%"
	m.downloadedTracks = 0
	m.failedTracks = 0
	m.addLog("info", fmt.Sprintf("Starting playlist download: %s", url))

	// Detect if it's a Spotify or YouTube playlist URL
	action := "download_spotify"
	if strings.Contains(strings.ToLower(url), "youtube") || strings.Contains(strings.ToLower(url), "youtu.be") {
		action = "download_youtube"
	}

	return func() tea.Msg {
		return StartDownloadMsg{Action: action, URL: url, Output: outDir}
	}
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
	m.plURLInput.Cursor.Style = cursorStyle
	m.plURLInput.PromptStyle = ui.AccentStyle
}

func (m *DownloadsModel) startDownload() tea.Cmd {
	if m.isDownloading {
		m.addLog("error", Tr("dl.error")+": already downloading")
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
		m.addLog("error", Tr("dl.enter_url"))
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.addLog("error", Tr("dl.invalid_url"))
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

	title := extractTitleFromResult(msg)
	if msg.Error != nil {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: title, status: "error", time: time.Now()})
		m.addLog("error", fmt.Sprintf("%s (%v)", msg.Error.Error(), elapsed))
		return
	}
	if msg.Result == nil {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: title, status: "error", time: time.Now()})
		m.addLog("error", fmt.Sprintf("No result from download (%v)", elapsed))
		return
	}
	if msg.Result.Status == "error" {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: title, status: "error", time: time.Now()})
		m.addLog("error", fmt.Sprintf("%s (%v)", msg.Result.Error, elapsed))
		return
	}

	msgText := msg.Result.Message
	if msgText == "" {
		msgText = "Download complete"
	}
	m.downloadedTracks++
	m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: msgText, status: "ok", time: time.Now()})
	m.addLog("ok", fmt.Sprintf("%s (%v)", msgText, elapsed))
}

func extractTitleFromResult(msg DownloadResultMsg) string {
	if msg.Result != nil && msg.Result.Message != "" {
		parts := strings.SplitN(msg.Result.Message, " - ", 2)
		if len(parts) == 2 {
			return strings.TrimSuffix(parts[1], ".mp3")
		}
		return msg.Result.Message
	}
	return "unknown"
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
		if strings.Contains(raw, "ERR ") {
			contentParts = append(contentParts, ui.ErrorStyle.Render(raw))
		} else if strings.Contains(raw, "OK  ") {
			contentParts = append(contentParts, lipgloss.NewStyle().Foreground(ui.ColorSuccess).Render(raw))
		} else if strings.Contains(raw, "... ") {
			contentParts = append(contentParts, ui.FaintStyle.Render(raw))
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

	// ── Music Download section ──
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

	musicContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		spotifyV,
		"",
		youtubeV,
		"",
		localBtn, "",
		playlistV, "",
		dlBtn,
	)

	musicTitle := ui.SectionTitleStyle.Render(" " + Tr("dl.title") + " ")
	musicBox := ui.AccentBorderStyle.Width(75).Render(musicTitle + "\n" + musicContent)

	// ── Playlist Download section ──
	plURLV := m.plURLInput.View()
	if m.focusIdx != 7 {
		val := m.plURLInput.Value()
		if val == "" {
			val = m.plURLInput.Placeholder
		}
		plURLV = "  Playlist URL:  " + ui.WhiteStyle.Render(val)
	}

	plDlBtn := ui.AccentButtonStyle.Render("  v Download Playlist  ")
	if m.focusIdx == 8 {
		plDlBtn = ui.FocusedButtonStyle.Render("  v Download Playlist  ")
	}

	plContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		plURLV,
		"",
		plDlBtn,
	)

	plTitle := ui.SectionTitleStyle.Render(" " + langT("Playlist Download", "Playlist İndirme") + " ")
	plBox := ui.BorderStyle.Width(75).Render(plTitle + "\n" + plContent)

	// Join sections vertically
	rightSide := lipgloss.JoinVertical(lipgloss.Left, musicBox, "", plBox)

	return lipgloss.JoinHorizontal(lipgloss.Top, console, rightSide)
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
