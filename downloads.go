package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ncruces/zenity"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

type logEntry struct {
	timestamp string // ANSI-styled "HH:MM:SS"
	msgStyle  string // "error", "ok", "info", or "" for raw style
	message   string // plain text message (no timestamp, no level prefix)
}

type downloadHistoryItem struct {
	title  string
	status string // ok, error
	time   time.Time
}

// Section identifiers for DownloadsModel
const (
	dlSectionConsole  = 0
	dlSectionMusic    = 1
	dlSectionPlaylist = 2
)

type DownloadsModel struct {
	width  int
	height int

	sectionIdx int // 0=console, 1=music, 2=playlist
	focusIdx   int // element within current section
	playlistIdx int

	musicInput    textinput.Model // single URL input for music download
	plURLInput    textinput.Model // playlist download URL

	logLines          []logEntry
	consoleScroll     int
	consoleCursorPos  int
	consoleCursorCol  int
	consoleSelStart   int // -1 = no selection
	consoleSelCol     int // anchor column for selection

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

	mi := textinput.New()
	mi.Placeholder = "https://open.spotify.com/... veya https://youtube.com/..."
	mi.Prompt = "  Müzik URL:  "
	mi.PromptStyle = ui.AccentStyle
	mi.TextStyle = ui.WhiteStyle
	mi.PlaceholderStyle = ui.DimStyle
	mi.Cursor.Style = cursorStyle
	mi.Width = 60
	mi.CharLimit = 300

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
		musicInput:      mi,
		plURLInput:      pi,
		playlistIdx:     0,
		consoleSelStart: -1,
		consoleSelCol:   -1,
	}
}

func (m *DownloadsModel) Init() tea.Cmd {
	m.refreshPlaylistOptions()
	m.sectionIdx = dlSectionMusic
	m.focusIdx = 0
	m.musicInput.Focus()
	m.addLog("info", "Downloads ready — type a URL and press Enter")
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
	case "tab", "shift+tab":
		if m.sectionIdx == dlSectionConsole {
			break
		}
		dir := 1
		if msg.String() == "shift+tab" {
			dir = -1
		}
		m.cycleSectionFocus(dir)
		return m, nil
	case "enter":
		return m.handleEnter()
	case "up", "k":
		if m.sectionIdx == dlSectionConsole {
			if m.consoleCursorPos > 0 {
				m.consoleCursorPos--
				m.consoleCursorCol = 0
			}
			return m, nil
		}
		if m.sectionIdx == dlSectionMusic && m.focusIdx == 1 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx - 1 + len(m.playlistOptions)) % len(m.playlistOptions)
			return m, nil
		}
	case "down", "j":
		if m.sectionIdx == dlSectionConsole {
			if m.consoleCursorPos < len(m.logLines)-1 {
				m.consoleCursorPos++
				m.consoleCursorCol = 0
			}
			return m, nil
		}
		if m.sectionIdx == dlSectionMusic && m.focusIdx == 1 && len(m.playlistOptions) > 1 && m.playlistOptions[0] != "(no playlists)" {
			m.playlistIdx = (m.playlistIdx + 1) % len(m.playlistOptions)
			return m, nil
		}
	case "left", "h":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			if m.consoleCursorCol > 0 {
				m.consoleCursorCol--
			}
			m.consoleSelStart = -1
			m.consoleSelCol = -1
			return m, nil
		}
	case "right", "l":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			msg := m.logLines[m.consoleCursorPos].message
			if m.consoleCursorCol < len([]rune(msg)) {
				m.consoleCursorCol++
			}
			m.consoleSelStart = -1
			m.consoleSelCol = -1
			return m, nil
		}
	case "shift+left":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			if m.consoleSelStart < 0 {
				m.consoleSelStart = m.consoleCursorPos
				m.consoleSelCol = m.consoleCursorCol
			}
			if m.consoleCursorCol > 0 {
				m.consoleCursorCol--
			}
			return m, nil
		}
	case "shift+right":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			if m.consoleSelStart < 0 {
				m.consoleSelStart = m.consoleCursorPos
				m.consoleSelCol = m.consoleCursorCol
			}
			msg := m.logLines[m.consoleCursorPos].message
			if m.consoleCursorCol < len([]rune(msg)) {
				m.consoleCursorCol++
			}
			return m, nil
		}
	case "shift+ctrl+left", "ctrl+shift+left":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			if m.consoleSelStart < 0 {
				m.consoleSelStart = m.consoleCursorPos
				m.consoleSelCol = m.consoleCursorCol
			}
			msg := m.logLines[m.consoleCursorPos].message
			m.consoleCursorCol = wordJumpLeft([]rune(msg), m.consoleCursorCol)
			return m, nil
		}
	case "shift+ctrl+right", "ctrl+shift+right":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			if m.consoleSelStart < 0 {
				m.consoleSelStart = m.consoleCursorPos
				m.consoleSelCol = m.consoleCursorCol
			}
			msg := m.logLines[m.consoleCursorPos].message
			m.consoleCursorCol = wordJumpRight([]rune(msg), m.consoleCursorCol)
			return m, nil
		}
	case "ctrl+left":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			msg := m.logLines[m.consoleCursorPos].message
			m.consoleCursorCol = wordJumpLeft([]rune(msg), m.consoleCursorCol)
			m.consoleSelStart = -1
			m.consoleSelCol = -1
			return m, nil
		}
	case "ctrl+right":
		if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
			msg := m.logLines[m.consoleCursorPos].message
			m.consoleCursorCol = wordJumpRight([]rune(msg), m.consoleCursorCol)
			m.consoleSelStart = -1
			m.consoleSelCol = -1
			return m, nil
		}
	case "esc":
		if m.sectionIdx == dlSectionConsole {
			m.consoleSelStart = -1
			m.consoleSelCol = -1
			return m, nil
		}
	case "pgup":
		if m.sectionIdx == dlSectionConsole {
			m.consoleScroll -= 10
			return m, nil
		}
	case "pgdown":
		if m.sectionIdx == dlSectionConsole {
			m.consoleScroll += 10
			return m, nil
		}
	case "home":
		if m.sectionIdx == dlSectionConsole {
			m.consoleScroll = 0
			return m, nil
		}
	case "end":
		if m.sectionIdx == dlSectionConsole {
			m.consoleScroll = -1
			return m, nil
		}
	case "ctrl+v":
		inp := m.currentInput()
		if inp != nil {
			var cmd tea.Cmd
			*inp, cmd = inp.Update(textinput.Paste())
			return m, cmd
		}
	case "ctrl+a":
		inp := m.currentInput()
		if inp != nil {
			inp.SetValue("")
			return m, nil
		}
	default:
		inp := m.currentInput()
		if inp != nil {
			var cmd tea.Cmd
			*inp, cmd = inp.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *DownloadsModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.sectionIdx {
	case dlSectionMusic:
		switch m.focusIdx {
		case 0: // music input
			return m, m.startDownload()
		case 1: // playlist dropdown
			return m, nil
		case 2: // +Playlist
			return m, m.openLocalPlaylistDialog()
		case 3: // +Music
			return m, m.openLocalMusicDialog()
		case 4: // Download
			return m, m.startDownload()
		}
	case dlSectionPlaylist:
		switch m.focusIdx {
		case 0: // playlist URL input
			return m, m.startPlaylistDownload()
		case 1: // Download Playlist
			return m, m.startPlaylistDownload()
		}
	}
	return m, nil
}

// cycleSectionFocus moves focusIdx within the current section.
func (m *DownloadsModel) cycleSectionFocus(dir int) {
	inp := m.currentInput()
	if inp != nil {
		inp.Blur()
	}
	maxIdx := m.sectionMaxFocus()
	if maxIdx < 0 {
		return
	}
	m.focusIdx = (m.focusIdx + dir + maxIdx + 1) % (maxIdx + 1)
	inp2 := m.currentInput()
	if inp2 != nil {
		inp2.Focus()
	}
}

func (m *DownloadsModel) sectionMaxFocus() int {
	switch m.sectionIdx {
	case dlSectionMusic:
		return 4 // 0=input, 1=playlist, 2=+Playlist, 3=+Music, 4=Download
	case dlSectionPlaylist:
		return 1 // 0=input, 1=Download
	}
	return -1
}

// cycleSection switches between console/music/playlist sections (called by F1).
func (m *DownloadsModel) cycleSection() bool {
	inp := m.currentInput()
	if inp != nil {
		inp.Blur()
	}
	m.sectionIdx = (m.sectionIdx + 1) % 3
	m.focusIdx = 0
	if m.sectionIdx == dlSectionConsole {
		m.consoleScroll = -1
		m.consoleCursorPos = len(m.logLines) - 1
		m.consoleCursorCol = 0
		if m.consoleCursorPos < 0 {
			m.consoleCursorPos = 0
		}
	}
	inp2 := m.currentInput()
	if inp2 != nil {
		inp2.Focus()
	}
	return false
}

func (m *DownloadsModel) cycleFocus() bool {
	return m.cycleSection()
}

func (m *DownloadsModel) focusedInputs() []*textinput.Model {
	return []*textinput.Model{&m.musicInput, &m.plURLInput}
}

func (m *DownloadsModel) currentInput() *textinput.Model {
	if m.sectionIdx == dlSectionMusic && m.focusIdx == 0 {
		return &m.musicInput
	}
	if m.sectionIdx == dlSectionPlaylist && m.focusIdx == 0 {
		return &m.plURLInput
	}
	return nil
}

// TrackProgress logs download progress to console.
func (m *DownloadsModel) TrackProgress(active bool, pct float64, status string) {
	if status == "" || status == m.lastLoggedStatus {
		return
	}
	m.lastLoggedStatus = status
	level := "info"
	if strings.Contains(strings.ToLower(status), "error") || strings.Contains(strings.ToLower(status), "fail") {
		level = "error"
	} else if !active {
		level = "ok"
	}
	m.addLog(level, fmt.Sprintf("[%d%%] %s", int(pct), status))
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
	m.musicInput.Cursor.Style = cursorStyle
	m.musicInput.PromptStyle = ui.AccentStyle
	m.plURLInput.Cursor.Style = cursorStyle
	m.plURLInput.PromptStyle = ui.AccentStyle
}

func (m *DownloadsModel) startDownload() tea.Cmd {
	if m.isDownloading {
		m.addLog("error", Tr("dl.error")+": already downloading")
		return nil
	}
	url := strings.TrimSpace(m.musicInput.Value())
	m.addLog("info", fmt.Sprintf("Raw input value: %q", url))
	if url == "" {
		m.addLog("error", Tr("dl.enter_url"))
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.addLog("error", Tr("dl.invalid_url"))
		return nil
	}

	// Determine output directory
	outDir := ""
	if state.Current.CurrentProfile != nil && m.playlistIdx >= 0 && m.playlistIdx < len(state.Current.CurrentProfile.Playlists) {
		pl := state.Current.CurrentProfile.Playlists[m.playlistIdx]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	} else if state.Current.CurrentProfile != nil && len(state.Current.CurrentProfile.Playlists) > 0 {
		pl := state.Current.CurrentProfile.Playlists[0]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	}
	if outDir == "" {
		var err error
		outDir, err = os.Getwd()
		if err != nil {
			outDir = "."
		}
		m.addLog("info", "No playlist selected, using current directory")
	}

	action := "download_spotify"
	if strings.Contains(strings.ToLower(url), "youtube") || strings.Contains(strings.ToLower(url), "youtu.be") {
		action = "download_youtube"
	}

	m.isDownloading = true
	m.downloadStart = time.Now()
	m.downloadPercent = 0
	m.downloadStatus = "0%"
	m.downloadedTracks = 0
	m.failedTracks = 0
	m.lastLoggedStatus = ""
	m.addLog("info", fmt.Sprintf("Sending: action=%s url=%s output=%s", action, url, outDir))
	return func() tea.Msg {
		return StartDownloadMsg{Action: action, URL: url, Output: outDir}
	}
}

func (m *DownloadsModel) handleDownloadResult(msg DownloadResultMsg) {
	m.isDownloading = false
	elapsed := time.Since(m.downloadStart).Truncate(time.Second)

	m.addLog("info", fmt.Sprintf("Download result received (action=%s, elapsed=%v)", msg.Action, elapsed))

	if msg.Error != nil {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: "error", status: "error", time: time.Now()})
		m.addLog("error", fmt.Sprintf("Bridge error: %+v", msg.Error))
		return
	}
	if msg.Result == nil {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: "error", status: "error", time: time.Now()})
		m.addLog("error", "Result is nil (no response from bridge)")
		return
	}
	m.addLog("info", fmt.Sprintf("Result: status=%s message=%q error=%q", msg.Result.Status, msg.Result.Message, msg.Result.Error))

	if msg.Result.Status == "error" {
		m.failedTracks++
		m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: msg.Result.Error, status: "error", time: time.Now()})
		m.addLog("error", msg.Result.Error)
		return
	}

	msgText := msg.Result.Message
	if msgText == "" {
		msgText = "Download complete"
	}
	m.downloadedTracks++
	m.downloadHistory = append(m.downloadHistory, downloadHistoryItem{title: msgText, status: "ok", time: time.Now()})
	m.addLog("ok", msgText)
}

func (m *DownloadsModel) HandleCtrlC() bool {
	if m.sectionIdx != dlSectionConsole || len(m.logLines) == 0 {
		return false
	}
	if m.consoleSelStart < 0 {
		return false
	}
	msg := m.logLines[m.consoleCursorPos].message
	runes := []rune(msg)
	lo := m.consoleSelCol
	hi := m.consoleCursorCol
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo < 0 { lo = 0 }
	if hi > len(runes) { hi = len(runes) }
	if lo >= hi {
		return false
	}
	text := string(runes[lo:hi])
	m.musicInput.SetValue(text)
	m.musicInput.CursorEnd()
	if err := clipboard.WriteAll(text); err != nil {
		m.addLog("err", fmt.Sprintf("Clipboard copy failed: %v", err))
	} else {
		m.addLog("info", "Copied to clipboard")
	}
	m.consoleSelStart = -1
	m.consoleSelCol = -1
	return true
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
	m.logLines = append(m.logLines, logEntry{timestamp: ts, msgStyle: level, message: msg})
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
	// If console is focused and cursor was at last line, follow new logs
	if m.sectionIdx == dlSectionConsole && len(m.logLines) > 0 {
		if m.consoleCursorPos >= len(m.logLines)-2 {
			m.consoleCursorPos = len(m.logLines) - 1
		}
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
	logCount := len(m.logLines)
	totalLines := logCount

	maxScroll := totalLines - visible
	if maxScroll < 0 {
		maxScroll = 0
	}

	// Clamp cursor position
	if m.sectionIdx == dlSectionConsole && totalLines > 0 {
		if m.consoleCursorPos < 0 {
			m.consoleCursorPos = 0
		}
		if m.consoleCursorPos >= totalLines {
			m.consoleCursorPos = totalLines - 1
		}
		// Keep cursor visible by adjusting scroll
		if m.consoleCursorPos < m.consoleScroll {
			m.consoleScroll = m.consoleCursorPos
		}
		if m.consoleCursorPos >= m.consoleScroll+visible {
			m.consoleScroll = m.consoleCursorPos - visible + 1
		}
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

	cursorStyle := lipgloss.NewStyle().Background(ui.ColorAccent).Foreground(lipgloss.Color("#000000"))
	isConsoleFocused := m.sectionIdx == dlSectionConsole

	var inner string
	if totalLines == 0 {
		inner = title + "\n" + ui.FaintStyle.Render("  No logs")
	} else {
		var contentParts []string
		for i := start; i < end; i++ {
			entry := m.logLines[i]

			// Reconstruct full styled message with level prefix (like old behavior)
			var prefix string
			var levelStyle lipgloss.Style
			switch entry.msgStyle {
			case "error":
				prefix = "ERR"
				levelStyle = logErrStyle
			case "ok":
				prefix = "OK"
				levelStyle = logOKStyle
			case "info":
				prefix = "..."
				levelStyle = logInfoStyle
			default:
				prefix = ""
				levelStyle = logInfoStyle
			}
			levelTxt := ""
			if prefix != "" {
				levelTxt = levelStyle.Render(prefix+" ") + " "
			}
			msgStyled := levelTxt + levelStyle.Render(entry.message)

			selBg := lipgloss.NewStyle().Background(lipgloss.Color("#3B3B5C"))
			isCursor := isConsoleFocused && i == m.consoleCursorPos
			hasSel := isConsoleFocused && m.consoleSelStart >= 0 && i == m.consoleSelStart && i == m.consoleCursorPos

			if hasSel {
				msg := entry.message
				runes := []rune(msg)
				selLo := m.consoleSelCol
				selHi := m.consoleCursorCol
				if selLo > selHi {
					selLo, selHi = selHi, selLo
				}
				if selLo < 0 { selLo = 0 }
				if selHi > len(runes) { selHi = len(runes) }
				before := string(runes[:selLo])
				mid := string(runes[selLo:selHi])
				after := string(runes[selHi:])
				msgStyled = levelTxt + levelStyle.Render(before) + selBg.Render(levelStyle.Render(mid)) + levelStyle.Render(after)
			} else if isCursor {
				msg := entry.message
				runes := []rune(msg)
				col := m.consoleCursorCol
				if col < 0 { col = 0 }
				if col > len(runes) { col = len(runes) }
				before := string(runes[:col])
				after := string(runes[col:])
				msgStyled = levelTxt + levelStyle.Render(before) + cursorCell + levelStyle.Render(after)
			}

			var line string
			if isCursor || hasSel {
				line = entry.timestamp + "  " + msgStyled
			} else {
				line = entry.timestamp + "    " + msgStyled
			}
			contentParts = append(contentParts, line)
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

	// just fill remaining space and render border
	for lipgloss.Height(inner) < bodyH-2 {
		inner += "\n"
	}
	border := ui.BorderStyle
	if m.sectionIdx == dlSectionConsole {
		border = ui.AccentBorderStyle
	}
	box := border.Width(w).Render(inner)
	return box
}

func (m *DownloadsModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	boxW := 75
	inSection := m.sectionIdx // 0=console, 1=music, 2=playlist

	// ── Music Download section ──
	musicFocus := inSection == dlSectionMusic
	musicInputV := m.musicInput.View()
	if !(musicFocus && m.focusIdx == 0) {
		val := m.musicInput.Value()
		if val == "" {
			val = m.musicInput.Placeholder
		}
		musicInputV = "  Müzik URL:  " + ui.WhiteStyle.Render(val)
	}

	playlistBtn := ui.ButtonStyle.Render("  + Playlist  ")
	musicBtn := ui.ButtonStyle.Render("  + Music  ")
	if musicFocus && m.focusIdx == 2 {
		playlistBtn = ui.FocusedOutlineStyle.Render("  + Playlist  ")
	}
	if musicFocus && m.focusIdx == 3 {
		musicBtn = ui.FocusedOutlineStyle.Render("  + Music  ")
	}
	localBtn := lipgloss.JoinHorizontal(lipgloss.Left, playlistBtn, "  ", musicBtn)

	playlistV := m.viewPlaylistDropdown()

	dlBtn := ui.AccentButtonStyle.Render("  v Download  ")
	if musicFocus && m.focusIdx == 4 {
		dlBtn = ui.FocusedButtonStyle.Render("  v Download  ")
	}

	musicContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		musicInputV,
		"",
		localBtn, "",
		playlistV, "",
		dlBtn,
	)

	musicBorder := ui.BorderStyle
	if musicFocus {
		musicBorder = ui.AccentBorderStyle
	}
	musicTitle := ui.SectionTitleStyle.Render(" " + Tr("dl.title") + " ")
	musicBox := musicBorder.Width(boxW).Render(musicTitle + "\n" + musicContent)

	// ── Playlist Download section ──
	plFocus := inSection == dlSectionPlaylist
	plURLV := m.plURLInput.View()
	if !(plFocus && m.focusIdx == 0) {
		val := m.plURLInput.Value()
		if val == "" {
			val = m.plURLInput.Placeholder
		}
		plURLV = "  Playlist URL:  " + ui.WhiteStyle.Render(val)
	}

	plDlBtn := ui.AccentButtonStyle.Render("  v Download Playlist  ")
	if plFocus && m.focusIdx == 1 {
		plDlBtn = ui.FocusedButtonStyle.Render("  v Download Playlist  ")
	}

	plContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		plURLV,
		"",
		plDlBtn,
	)

	plBorder := ui.BorderStyle
	if plFocus {
		plBorder = ui.AccentBorderStyle
	}
	plTitle := ui.SectionTitleStyle.Render(" " + langT("Playlist Download", "Playlist İndirme") + " ")
	plBox := plBorder.Width(boxW).Render(plTitle + "\n" + plContent)

	// Join sections vertically with same height
	rightSide := lipgloss.JoinVertical(lipgloss.Left, musicBox, "", plBox)
	rightH := lipgloss.Height(rightSide)

	// Console height matches right side (like playlist page)
	console := m.renderConsole(rightH)

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
	if m.sectionIdx == dlSectionMusic && m.focusIdx == 1 {
		return label + ui.AccentStyle.Render(current)
	}
	return label + ui.WhiteStyle.Render(current)
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func wordJumpLeft(runes []rune, col int) int {
	if col <= 0 {
		return 0
	}
	// skip any non-word at cursor
	i := col - 1
	if !isWordRune(runes[i]) {
		for i > 0 && !isWordRune(runes[i]) {
			i--
		}
		return i + 1
	}
	// skip word
	for i >= 0 && isWordRune(runes[i]) {
		i--
	}
	return i + 1
}

func wordJumpRight(runes []rune, col int) int {
	n := len(runes)
	if col >= n {
		return n
	}
	// skip word at cursor
	i := col
	if isWordRune(runes[i]) {
		for i < n && isWordRune(runes[i]) {
			i++
		}
		return i
	}
	// skip non-word, then skip next word
	for i < n && !isWordRune(runes[i]) {
		i++
	}
	for i < n && isWordRune(runes[i]) {
		i++
	}
	return i
}
