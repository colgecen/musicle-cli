package main

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sqweek/dialog"

	"MusicLeCLI/bridge"
	"MusicLeCLI/state"
	"MusicLeCLI/ui"

	_ "image/jpeg"
	_ "image/png"
)

type HomeModel struct {
	width  int
	height int
	ready  bool

	focusIdx     int
	sectionFocus int // 0=sidebar, 1=playlist, 2=songs, 3=console

	spotifyInput textinput.Model
	youtubeInput textinput.Model

	songFocusIdx    int
	songActionFocus int // 0=play, 1=edit, 2=delete, -1=none
	songOffset      int
	bodyHeight      int // available body height for content

	songEndedAt   time.Time // when current song ended naturally, for auto-advance delay
	manualStop    bool      // true when user manually stopped/switched songs

	playlistOptions  []string
	playlistIdx      int
	playlistExpanded bool

	sidebarError      string
	sidebarErrIsError bool

	logLines        []string
	consoleScroll   int
	isDownloading   bool
	downloadPercent float64
	downloadStatus  string

	editModalOpen bool
	editSongIdx   int
	editTitle     textinput.Model
	editArtist    textinput.Model
	editDuration  textinput.Model
	editFocus     int

	deleteConfirm bool
	deleteSongIdx int
	deleteYes     bool

	renameMode  bool
	renameInput textinput.Model

	selectAll       bool // Ctrl+A select-all state for spotify/youtube inputs
	editSelectAll   bool // Ctrl+A select-all state for edit modal inputs
	renameSelectAll bool // Ctrl+A select-all state for rename input
}

func NewHomeModel() *HomeModel {
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

	return &HomeModel{
		spotifyInput:    si,
		youtubeInput:    yi,
		playlistIdx:     0,
		sectionFocus:    -1,
		songFocusIdx:    -1,
		songActionFocus: -1,
		consoleScroll:   -1,
		editTitle:       editInput(langT("Title", "Baslik")),
		editArtist:      editInput(langT("Artist", "Sanatci")),
		editDuration:    editInput(langT("Duration", "Sure")),
		renameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = ""
			ti.Width = 30
			ti.CharLimit = 100
			ti.Cursor.Style = lipgloss.NewStyle().
				Background(ui.ColorAccent).
				Foreground(lipgloss.Color("#000000"))
			return ti
		}(),
	}
}

func editInput(prompt string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "  " + prompt + ":  "
	ti.Cursor.Style = lipgloss.NewStyle().
		Background(lipgloss.Color("#1DB954")).
		Foreground(lipgloss.Color("#000000"))
	ti.Width = 50
	ti.CharLimit = 100
	return ti
}

func (m *HomeModel) Init() tea.Cmd {
	m.refreshPlaylistOptions()
	m.focusIdx = -1
	return nil
}

func (m *HomeModel) refreshPlaylistOptions() {
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

func (m *HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		if m.editModalOpen {
			return m.handleEditModalKey(msg)
		}
		if m.deleteConfirm {
			return m.handleDeleteKey(msg)
		}
		if m.renameMode {
			return m.handleRenameKey(msg)
		}
		return m.handleKeyMsg(msg)

	case PlayerStatusResult:
		if msg.Result != nil {
			m.processPlayerStatus(msg.Result)
		}

	case PlayResultMsg:
		if msg.Error != nil {
			m.addLog("error", fmt.Sprintf("Play error: %s - %v", msg.Title, msg.Error))
		} else {
			m.addLog("ok", fmt.Sprintf("Playing: %s", msg.Title))
		}

	case ClearSidebarMsg:
		m.sidebarError = ""
		m.sidebarErrIsError = false

	case DownloadResultMsg:
		return m, m.handleDownloadResult(msg)

	case ImportResultMsg:
		return m, m.handleImportResult(msg)

	case PlaySongMsg:
		pl := state.Current.CurrentPlaylist
		if pl != nil {
			for i := range pl.Songs {
				if pl.Songs[i].FilePath == msg.FilePath {
					m.songFocusIdx = i
					// Adjust songOffset so selected song is visible
					rows := m.bodyHeight - 10
					if rows < 1 {
						rows = 1
					}
					if m.songFocusIdx < m.songOffset {
						m.songOffset = m.songFocusIdx
					} else if m.songFocusIdx >= m.songOffset+rows {
						m.songOffset = m.songFocusIdx - rows + 1
					}
					return m, m.playSong(&pl.Songs[i])
				}
			}
		}
	}

	return m, nil
}

func (m *HomeModel) songs() []state.Song {
	if state.Current.CurrentPlaylist != nil {
		return state.Current.CurrentPlaylist.Songs
	}
	return nil
}

func (m *HomeModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.playlistExpanded {
		return m.handlePlaylistKey(msg)
	}

	if m.focusIdx == 0 || m.focusIdx == 1 {
		switch msg.String() {
		case "tab":
			m.cycleFocus(1)
			return m, nil
		case "shift+tab":
			m.cycleFocus(-1)
			return m, nil
		case "enter":
			return m.handleEnter()
		case "f5", "f6", "f7":
		case "ctrl+v":
			var cmd tea.Cmd
			if m.focusIdx == 0 {
				m.spotifyInput, cmd = m.spotifyInput.Update(textinput.Paste())
			} else {
				m.youtubeInput, cmd = m.youtubeInput.Update(textinput.Paste())
			}
			return m, cmd
		case "ctrl+a":
			inp := m.currentInput()
			if inp.Value() != "" {
				m.selectAll = true
			}
			return m, nil
		default:
			if m.selectAll {
				inp := m.currentInput()
				s := msg.String()
				// Replacement keys clear input first
				if len(s) == 1 || s == "backspace" || s == "delete" {
					inp.SetValue("")
					inp.SetCursor(0)
					m.selectAll = false
				} else {
					// Navigation keys just cancel selection
					m.selectAll = false
				}
			}
			var cmd tea.Cmd
			if m.focusIdx == 0 {
				m.spotifyInput, cmd = m.spotifyInput.Update(msg)
			} else {
				m.youtubeInput, cmd = m.youtubeInput.Update(msg)
			}
			return m, cmd
		}
	}

	switch msg.String() {
	case "tab":
		if m.sectionFocus == 1 {
			plen := len(m.playlistOptions)
			if plen > 0 {
				m.playlistIdx = (m.playlistIdx + 1) % plen
			}
		} else if m.sectionFocus == 3 {
			// Tab in console does nothing
		} else if m.focusIdx == 6 {
			songs := m.songs()
			if len(songs) > 0 {
				oldFocus := m.songFocusIdx
				m.songFocusIdx = (m.songFocusIdx + 1) % len(songs)
				m.songActionFocus = 0
				if m.songFocusIdx == 0 && oldFocus == len(songs)-1 {
					m.songOffset = 0
				} else {
					maxVis := m.maxVisibleSongs()
					if m.songFocusIdx >= m.songOffset+maxVis {
						m.songOffset = m.songFocusIdx - maxVis + 1
					}
				}
			}
		} else {
			m.cycleFocus(1)
		}
		return m, nil
	case "shift+tab":
		if m.sectionFocus == 1 {
			plen := len(m.playlistOptions)
			if plen > 0 {
				m.playlistIdx = (m.playlistIdx - 1 + plen) % plen
			}
		} else if m.sectionFocus == 3 {
			// Shift+Tab in console does nothing
		} else if m.focusIdx == 6 {
			songs := m.songs()
			if len(songs) > 0 {
				oldFocus := m.songFocusIdx
				m.songFocusIdx = (m.songFocusIdx - 1 + len(songs)) % len(songs)
				m.songActionFocus = 0
				if m.songFocusIdx == len(songs)-1 && oldFocus == 0 {
					m.songOffset = len(songs) - m.maxVisibleSongs()
					if m.songOffset < 0 {
						m.songOffset = 0
					}
				} else if m.songFocusIdx < m.songOffset {
					m.songOffset = m.songFocusIdx
				}
			}
		} else {
			m.cycleFocus(-1)
		}
		return m, nil
	case "enter":
		return m.handleEnter()
	case " ":
		m.togglePlayPause()
		return m, nil
	case "right":
		if m.focusIdx == 6 && m.songFocusIdx >= 0 {
			m.songActionFocus = (m.songActionFocus + 1) % 3
			return m, tea.HideCursor
		}
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: 5})
		return m, nil
	case "left":
		if m.focusIdx == 6 && m.songFocusIdx >= 0 {
			m.songActionFocus = (m.songActionFocus - 1 + 3) % 3
			return m, tea.HideCursor
		}
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: -5})
		return m, nil
	case "f7":
		return m, m.playSelectedSong()
	case "f5":
		m.sectionFocus = 0
		m.focusIdx = -1
		m.songFocusIdx = -1
		m.songActionFocus = -1
		return m, tea.HideCursor
	case "f6":
		m.sectionFocus = 2
		if m.focusIdx != 6 {
			if m.focusIdx >= 0 && m.focusIdx <= 4 {
				inputs := m.focusedInputs()
				for _, inp := range inputs {
					if inp != nil {
						inp.Blur()
					}
				}
			}
			m.focusIdx = 6
			songs := m.songs()
			if len(songs) > 0 && m.songFocusIdx < 0 {
				m.songFocusIdx = 0
				m.songActionFocus = 0
			}
		}
		return m, tea.HideCursor
	case "e":
		if m.focusIdx == 6 && m.songFocusIdx >= 0 {
			m.openEditModal()
			return m, nil
		}
	case "d", "delete":
		if m.focusIdx == 6 && m.songFocusIdx >= 0 {
			m.openDeleteConfirm()
			return m, nil
		}
	case "r":
		if m.focusIdx == 2 || m.sectionFocus == 1 {
			m.startRename()
			return m, nil
		}
	case "up":
		if m.sectionFocus == 1 {
			plen := len(m.playlistOptions)
			if plen > 0 {
				m.playlistIdx = (m.playlistIdx - 1 + plen) % plen
			}
			return m, nil
		}
		if m.sectionFocus == 3 {
			if m.consoleScroll < 0 {
				m.consoleScroll = len(m.logLines)
			}
			if m.consoleScroll > 0 {
				m.consoleScroll--
			}
			return m, nil
		}
		if m.focusIdx == 6 {
			songs := m.songs()
			if len(songs) > 0 {
				oldFocus := m.songFocusIdx
				m.songFocusIdx = (m.songFocusIdx - 1 + len(songs)) % len(songs)
				m.songActionFocus = 0
				if m.songFocusIdx == len(songs)-1 && oldFocus == 0 {
					m.songOffset = len(songs) - m.maxVisibleSongs()
					if m.songOffset < 0 {
						m.songOffset = 0
					}
				} else if m.songFocusIdx < m.songOffset {
					m.songOffset = m.songFocusIdx
				}
			}
			return m, nil
		}
		m.adjustVolume(0.05)
		return m, nil
	case "down":
		if m.sectionFocus == 1 {
			plen := len(m.playlistOptions)
			if plen > 0 {
				m.playlistIdx = (m.playlistIdx + 1) % plen
			}
			return m, nil
		}
		if m.sectionFocus == 3 {
			if m.consoleScroll < 0 {
				return m, nil
			}
			m.consoleScroll++
			return m, nil
		}
		if m.focusIdx == 6 {
			songs := m.songs()
			if len(songs) > 0 {
				oldFocus := m.songFocusIdx
				m.songFocusIdx = (m.songFocusIdx + 1) % len(songs)
				m.songActionFocus = 0
				// Wrapped from last to first
				if m.songFocusIdx == 0 && oldFocus == len(songs)-1 {
					m.songOffset = 0
				} else {
					maxVis := m.maxVisibleSongs()
					if m.songFocusIdx >= m.songOffset+maxVis {
						m.songOffset = m.songFocusIdx - maxVis + 1
					}
				}
			}
			return m, nil
		}
		m.adjustVolume(-0.05)
		return m, nil
	case "pgup":
		if m.sectionFocus == 3 {
			if m.consoleScroll < 0 {
				m.consoleScroll = len(m.logLines)
			}
			if m.consoleScroll > 0 {
				m.consoleScroll -= 10
				if m.consoleScroll < 0 {
					m.consoleScroll = 0
				}
			}
			return m, nil
		}
		return m, nil
	case "pgdown":
		if m.sectionFocus == 3 {
			if m.consoleScroll < 0 {
				return m, nil
			}
			m.consoleScroll += 10
			return m, nil
		}
		return m, nil
	case "end":
		if m.sectionFocus == 3 {
			m.consoleScroll = -1
			return m, nil
		}
		return m, nil
	case "home":
		if m.sectionFocus == 3 {
			m.consoleScroll = 0
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m *HomeModel) handlePlaylistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		plen := len(m.playlistOptions)
		if plen > 0 {
			m.playlistIdx = (m.playlistIdx - 1 + plen) % plen
		}
	case "down", "j":
		plen := len(m.playlistOptions)
		if plen > 0 {
			m.playlistIdx = (m.playlistIdx + 1) % plen
		}
	case "enter":
		m.selectPlaylist(m.playlistIdx)
		m.playlistExpanded = false
	case "esc":
		m.playlistExpanded = false
	case "tab":
		m.playlistExpanded = false
		m.cycleFocus(1)
	case "shift+tab":
		m.playlistExpanded = false
		m.cycleFocus(-1)
	}
	return m, nil
}

func (m *HomeModel) cycleFocus(dir int) {
	if m.focusIdx >= 0 {
		prevInputs := m.focusedInputs()
		for _, inp := range prevInputs {
			if inp != nil {
				inp.Blur()
			}
		}
	}
	m.playlistExpanded = false
	if m.focusIdx < 0 {
		m.focusIdx = 0
		m.spotifyInput.Focus()
		return
	}
	// Sidebar tab order: 0 (spotify), 1 (youtube), 3 (+Playlist), 4 (+Music), 5 (Download)
	sidebarOrder := []int{0, 1, 3, 4, 5}
	cur := -1
	for i, v := range sidebarOrder {
		if v == m.focusIdx {
			cur = i
			break
		}
	}
	if cur == -1 {
		m.focusIdx = sidebarOrder[0]
	} else {
		next := (cur + dir + len(sidebarOrder)) % len(sidebarOrder)
		m.focusIdx = sidebarOrder[next]
	}
	m.selectAll = false
	switch m.focusIdx {
	case 0:
		m.spotifyInput.Focus()
	case 1:
		m.youtubeInput.Focus()
	}
}

// CycleSection cycles between sections via F1: sidebar (0) → playlist info (1) → songs (2) → console (3) → wrap (player bar)
func (m *HomeModel) CycleSection() (bool, tea.Cmd) {
	if m.focusIdx >= 0 && m.focusIdx <= 5 {
		inputs := m.focusedInputs()
		for _, inp := range inputs {
			if inp != nil {
				inp.Blur()
			}
		}
	}
	m.songFocusIdx = -1
	m.songActionFocus = -1
	m.focusIdx = -1
	m.selectAll = false
	m.editSelectAll = false
	wrapped := false
	switch m.sectionFocus {
	case 0:
		m.sectionFocus = 1
	case 1:
		m.sectionFocus = 2
		m.focusIdx = 6
		m.songFocusIdx = 0
		m.songActionFocus = 0
		m.songOffset = 0
	case 2:
		m.sectionFocus = 3
		m.consoleScroll = -1
	case 3:
		m.sectionFocus = 0
		wrapped = true
	default:
		m.sectionFocus = 0
		wrapped = true
	}
	return wrapped, tea.HideCursor
}

func (m *HomeModel) FocusConsole() tea.Cmd {
	if m.focusIdx >= 0 && m.focusIdx <= 4 {
		inputs := m.focusedInputs()
		for _, inp := range inputs {
			if inp != nil {
				inp.Blur()
			}
		}
	}
	m.sectionFocus = 3
	m.focusIdx = -1
	m.songFocusIdx = -1
	m.songActionFocus = -1
	return tea.HideCursor
}

func (m *HomeModel) focusedInputs() []*textinput.Model {
	return []*textinput.Model{&m.spotifyInput, &m.youtubeInput}
}

func (m *HomeModel) currentInput() *textinput.Model {
	if m.focusIdx == 0 {
		return &m.spotifyInput
	}
	return &m.youtubeInput
}

func (m *HomeModel) maxVisibleSongs() int {
	h := m.bodyHeight
	if h < 10 {
		h = m.height
	}
	// tableBox = 2(border) + 1(title) + 1(sep) + 3(header) + 3*N(songs) + 1(sep) + 1(hint)
	// total = 9 + 3*N. Fit within h: N = (h - 9) / 3
	n := (h - 9) / 3
	if n < 1 {
		n = 1
	}
	return n
}

func (m *HomeModel) editCurrentInput() *textinput.Model {
	switch m.editFocus {
	case 0:
		return &m.editTitle
	case 1:
		return &m.editArtist
	case 2:
		return &m.editDuration
	}
	return nil
}

func (m *HomeModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.sectionFocus == 1 {
		if len(m.playlistOptions) > 0 {
			m.selectPlaylist(m.playlistIdx)
		}
		return m, tea.HideCursor
	}
	switch m.focusIdx {
	case 0:
		cmd := m.startDownload()
		m.spotifyInput.SetValue("")
		return m, cmd
	case 1:
		cmd := m.startDownload()
		m.youtubeInput.SetValue("")
		return m, cmd
	case 5:
		return m, m.startDownload()
	case 2:
		m.playlistExpanded = true
		return m, nil
	case 3:
		return m, m.openLocalPlaylistDialog()
	case 4:
		return m, m.openLocalMusicDialog()
	case 6:
		songs := m.songs()
		if m.songFocusIdx >= 0 && m.songFocusIdx < len(songs) {
			switch m.songActionFocus {
			case 0:
				return m, m.playSong(&songs[m.songFocusIdx])
			case 1:
				m.openEditModal()
			case 2:
				m.openDeleteConfirm()
			}
		}
	}
	return m, tea.HideCursor
}

func (m *HomeModel) startDownload() tea.Cmd {
	if m.isDownloading {
		m.addLog("error", langT("Already downloading!", "Zaten indiriliyor!"))
		return nil
	}
	spotifyURL := strings.TrimSpace(m.spotifyInput.Value())
	youtubeURL := strings.TrimSpace(m.youtubeInput.Value())
	url := spotifyURL
	action := "download_spotify"
	if url == "" {
		url = youtubeURL
		action = "download_youtube"
	}
	if url == "" {
		m.sidebarError = langT("Enter a URL first", "Once bir URL girin")
		m.sidebarErrIsError = true
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.sidebarError = langT("Invalid URL", "Hatali Link")
		m.sidebarErrIsError = true
		return nil
	}
	outDir := ""
	if state.Current.CurrentProfile != nil && m.playlistIdx >= 0 && m.playlistIdx < len(state.Current.CurrentProfile.Playlists) {
		pl := state.Current.CurrentProfile.Playlists[m.playlistIdx]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	}
	m.isDownloading = true
	m.downloadPercent = 0
	m.downloadStatus = "0%"
	m.sidebarError = langT("Downloading...", "Indiriliyor...")
	m.sidebarErrIsError = false
	m.addLog("", langT("Downloading: ", "Indiriliyor: ")+url)
	return func() tea.Msg {
		return StartDownloadMsg{Action: action, URL: url, Output: outDir}
	}
}

// ClearSidebarMsg is sent after a timeout to clear the sidebar error/success message
type ClearSidebarMsg struct{}

func clearSidebarAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return ClearSidebarMsg{}
	})
}

func (m *HomeModel) openLocalPlaylistDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := dialog.Directory().Title(langT("Select Music Directory", "Muzik Klasoru Sec")).Browse()
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

func (m *HomeModel) openLocalMusicDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := dialog.File().
			Filter(langT("Audio Files", "Ses Dosyalari"), "mp3").
			Title(langT("Select Audio Files", "Ses Dosyasi Sec")).
			Load()
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

func (m *HomeModel) handleDownloadResult(msg DownloadResultMsg) tea.Cmd {
	m.isDownloading = false
	if msg.Error != nil || msg.Result.Status == "error" {
		errMsg := ""
		if msg.Result != nil {
			errMsg = msg.Result.Error
		}
		if errMsg == "" && msg.Error != nil {
			errMsg = msg.Error.Error()
		}
		m.sidebarError = "x " + errMsg
		m.sidebarErrIsError = true
		m.addLog("error", langT("Download failed: ", "Indirme basarisiz: ")+errMsg)
		return clearSidebarAfter(4 * time.Second)
	}
	if len(msg.Result.Songs) > 0 {
		n := len(msg.Result.Songs)
		msgText := langT(fmt.Sprintf("v Downloaded %d songs", n), fmt.Sprintf("v %d sarki indirildi", n))
		m.sidebarError = msgText
		m.sidebarErrIsError = false
		m.addLog("ok", fmt.Sprintf(langT("Downloaded %d songs", "%d sarki indirildi"), n))
	} else {
		msgText := langT("v Downloaded: ", "v Indirildi: ") + msg.Result.Filename
		m.sidebarError = msgText
		m.sidebarErrIsError = false
		m.addLog("ok", langT("Downloaded: ", "Indirildi: ")+msg.Result.Filename)
	}
	m.refreshAllContent()
	return clearSidebarAfter(4 * time.Second)
}

func (m *HomeModel) handleImportResult(msg ImportResultMsg) tea.Cmd {
	if msg.Error != nil || msg.Result.Status == "error" {
		errMsg := ""
		if msg.Result != nil {
			errMsg = msg.Result.Error
		}
		if errMsg == "" && msg.Error != nil {
			errMsg = msg.Error.Error()
		}
		m.sidebarError = "x " + errMsg
		m.sidebarErrIsError = true
		m.addLog("error", langT("Import failed: ", "Ice aktarma basarisiz: ")+errMsg)
		return clearSidebarAfter(4 * time.Second)
	}
	msgText := langT("v Imported: ", "v Ice Aktarildi: ") + msg.Result.Filename
	m.sidebarError = msgText
	m.sidebarErrIsError = false
	m.addLog("ok", langT("Imported: ", "Ice Aktarildi: ")+msg.Result.Filename)
	m.refreshAllContent()
	return clearSidebarAfter(4 * time.Second)
}

func (m *HomeModel) openEditModal() {
	songs := m.songs()
	idx := m.songFocusIdx
	if idx < 0 || idx >= len(songs) {
		return
	}
	m.editSongIdx = idx
	song := songs[idx]
	m.editTitle.SetValue(song.Title)
	m.editArtist.SetValue(song.Artist)
	m.editDuration.SetValue(song.Duration)
	m.editFocus = 0
	m.editTitle.Focus()
	m.editModalOpen = true
}

func (m *HomeModel) handleEditModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeEditModal()
		return m, nil
	case "tab":
		m.editFocus = (m.editFocus + 1) % 3
		m.setEditFocus()
		return m, nil
	case "shift+tab":
		m.editFocus = (m.editFocus - 1 + 3) % 3
		m.setEditFocus()
		return m, nil
	case "enter":
		return m.saveEditModal()
	case "ctrl+v":
		var cmd tea.Cmd
		switch m.editFocus {
		case 0:
			m.editTitle, cmd = m.editTitle.Update(textinput.Paste())
		case 1:
			m.editArtist, cmd = m.editArtist.Update(textinput.Paste())
		case 2:
			m.editDuration, cmd = m.editDuration.Update(textinput.Paste())
		}
		return m, cmd
	case "ctrl+a":
		inp := m.editCurrentInput()
		if inp != nil && inp.Value() != "" {
			m.editSelectAll = true
		}
		return m, nil
	}
	if m.editSelectAll {
		inp := m.editCurrentInput()
		if inp != nil {
			s := msg.String()
			if len(s) == 1 || s == "backspace" || s == "delete" {
				inp.SetValue("")
				inp.SetCursor(0)
				m.editSelectAll = false
			} else {
				m.editSelectAll = false
			}
		}
	}
	switch m.editFocus {
	case 0:
		var cmd tea.Cmd
		m.editTitle, cmd = m.editTitle.Update(msg)
		return m, cmd
	case 1:
		var cmd tea.Cmd
		m.editArtist, cmd = m.editArtist.Update(msg)
		return m, cmd
	case 2:
		var cmd tea.Cmd
		m.editDuration, cmd = m.editDuration.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *HomeModel) setEditFocus() {
	m.editTitle.Blur()
	m.editArtist.Blur()
	m.editDuration.Blur()
	switch m.editFocus {
	case 0:
		m.editTitle.Focus()
	case 1:
		m.editArtist.Focus()
	case 2:
		m.editDuration.Focus()
	}
}

func (m *HomeModel) closeEditModal() {
	m.editModalOpen = false
	m.editTitle.Blur()
	m.editArtist.Blur()
	m.editDuration.Blur()
}

func (m *HomeModel) saveEditModal() (tea.Model, tea.Cmd) {
	songs := m.songs()
	if m.editSongIdx < 0 || m.editSongIdx >= len(songs) {
		m.closeEditModal()
		return m, nil
	}
	song := songs[m.editSongIdx]
	title := strings.TrimSpace(m.editTitle.Value())
	artist := strings.TrimSpace(m.editArtist.Value())
	duration := strings.TrimSpace(m.editDuration.Value())
	if title == "" {
		title = song.Title
	}
	if artist == "" {
		artist = song.Artist
	}
	if duration == "" {
		duration = song.Duration
	}
	pl := state.Current.CurrentPlaylist
	if pl == nil {
		m.closeEditModal()
		return m, nil
	}
	profile := state.Current.CurrentProfile
	if profile == nil {
		m.closeEditModal()
		return m, nil
	}
	listPath := state.Current.SongListPath(profile.FolderName, pl.FolderName)

	m.closeEditModal()

	return m, func() tea.Msg {
		result, err := bridge.RunScript(bridge.Action{
			Action: "update_song",
			File:   listPath,
			Path:   song.FilePath,
			Value:  map[string]string{"title": title, "artist": artist, "duration": duration},
		})
		if err == nil && result.Status == "ok" {
			_ = state.Current.ScanProfiles()
			m.refreshAllContent()
			m.addLog("ok", langT("Updated: ", "Guncellendi: ")+song.Title)
		} else {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else if result != nil {
				errMsg = result.Error
			}
			m.sidebarError = errMsg
			m.sidebarErrIsError = true
			m.addLog("error", langT("Update failed: ", "Guncelleme basarisiz: ")+errMsg)
		}
		return nil
	}
}

func (m *HomeModel) renderEditOverlay(full string) string {
	titleLbl := ui.AccentStyle.Render(" Title ") + "\n" + m.editTitle.View()
	artistLbl := ui.AccentStyle.Render(" Artist ") + "\n" + m.editArtist.View()
	durLbl := ui.AccentStyle.Render(" Duration ") + "\n" + m.editDuration.View()
	content := lipgloss.JoinVertical(lipgloss.Left, titleLbl, "", artistLbl, "", durLbl, "", ui.DimStyle.Render("  [Tab] Next  [Enter] Save  [Esc] Cancel"))
	content = ui.BorderStyle.Width(50).Render(ui.WhiteStyle.Bold(true).Render(" EDIT SONG ") + "\n" + content)
	return m.placeOverlay(full, content)
}

func (m *HomeModel) placeOverlay(full, overlay string) string {
	lines := strings.Split(full, "\n")
	totalH := len(lines)
	contentH := lipgloss.Height(overlay)
	contentW := lipgloss.Width(overlay)
	topPad := (totalH - contentH) / 2
	leftPad := (m.width - contentW) / 2
	if topPad < 0 {
		topPad = 0
	}
	if leftPad < 0 {
		leftPad = 0
	}
	overlayLines := strings.Split(overlay, "\n")
	var result []string
	for i, line := range lines {
		if i >= topPad && i < topPad+contentH {
			ci := i - topPad
			if ci >= 0 && ci < len(overlayLines) {
				result = append(result, strings.Repeat(" ", leftPad)+overlayLines[ci])
			} else {
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func (m *HomeModel) openDeleteConfirm() {
	songs := m.songs()
	idx := m.songFocusIdx
	if idx < 0 || idx >= len(songs) {
		return
	}
	m.deleteSongIdx = idx
	m.deleteYes = false
	m.deleteConfirm = true
}

func (m *HomeModel) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.deleteConfirm = false
		return m, nil
	case "left", "right", "tab":
		m.deleteYes = !m.deleteYes
		return m, nil
	case "enter":
		return m.executeDelete()
	}
	return m, nil
}

func (m *HomeModel) executeDelete() (tea.Model, tea.Cmd) {
	if !m.deleteYes {
		m.deleteConfirm = false
		return m, nil
	}
	songs := m.songs()
	if m.deleteSongIdx < 0 || m.deleteSongIdx >= len(songs) {
		m.deleteConfirm = false
		return m, nil
	}
	song := songs[m.deleteSongIdx]
	pl := state.Current.CurrentPlaylist
	if pl == nil {
		m.deleteConfirm = false
		return m, nil
	}
	profile := state.Current.CurrentProfile
	if profile == nil {
		m.deleteConfirm = false
		return m, nil
	}
	listPath := state.Current.SongListPath(profile.FolderName, pl.FolderName)

	m.deleteConfirm = false

	return m, func() tea.Msg {
		result, err := bridge.RunScript(bridge.Action{
			Action: "remove_song",
			File:   listPath,
			Path:   song.Filename,
		})
		if err == nil && result.Status == "ok" {
			_ = state.Current.ScanProfiles()
			m.refreshAllContent()
			m.addLog("ok", langT("Deleted: ", "Silindi: ")+song.Title)
		} else {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else if result != nil {
				errMsg = result.Error
			}
			m.sidebarError = errMsg
			m.sidebarErrIsError = true
			m.addLog("error", langT("Delete failed: ", "Silme basarisiz: ")+errMsg)
		}
		return nil
	}
}

func (m *HomeModel) renderDeleteOverlay(full string) string {
	songs := m.songs()
	songName := ""
	if m.deleteSongIdx >= 0 && m.deleteSongIdx < len(songs) {
		songName = songs[m.deleteSongIdx].Title
	}
	msg := ui.WhiteStyle.Render(fmt.Sprintf("  Delete \"%s\"?", songName))
	purpleBtn := lipgloss.NewStyle().
		Background(lipgloss.Color("#BB86FC")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FFFFFF")).
		Padding(0, 2)
	noBtn := ui.ButtonStyle.Render("  No  ")
	yesBtn := ui.ErrorButtonStyle.Render("  Yes  ")
	if m.deleteYes {
		yesBtn = purpleBtn.Render("  Yes  ")
	} else {
		noBtn = purpleBtn.Render("  No  ")
	}
	btns := lipgloss.JoinHorizontal(lipgloss.Left, yesBtn, "  ", noBtn)
	content := lipgloss.JoinVertical(lipgloss.Center, "", msg, "", btns, "")
	content = ui.BorderStyle.Width(40).Render(ui.WhiteStyle.Bold(true).Render(" CONFIRM DELETE ") + "\n" + content)
	return m.placeOverlay(full, content)
}

func (m *HomeModel) startRename() {
	pl := state.Current.CurrentPlaylist
	if pl == nil {
		return
	}
	m.renameInput.SetValue(pl.Name)
	m.renameInput.SetCursor(len(pl.Name))
	m.renameInput.Focus()
	m.renameMode = true
}

func (m *HomeModel) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.renameMode = false
		m.renameInput.Blur()
		return m, nil
	case "enter":
		return m.saveRename()
	case "ctrl+a":
		if m.renameInput.Value() != "" {
			m.renameSelectAll = true
		}
		return m, nil
	default:
		if m.renameSelectAll {
			s := msg.String()
			if len(s) == 1 || s == "backspace" || s == "delete" {
				m.renameInput.SetValue("")
				m.renameInput.SetCursor(0)
				m.renameSelectAll = false
			} else {
				m.renameSelectAll = false
			}
		}
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

func (m *HomeModel) saveRename() (tea.Model, tea.Cmd) {
	m.renameMode = false
	m.renameInput.Blur()
	newName := strings.TrimSpace(m.renameInput.Value())
	if newName == "" {
		return m, nil
	}
	pl := state.Current.CurrentPlaylist
	cp := state.Current.CurrentProfile
	if pl == nil || cp == nil {
		return m, nil
	}
	if err := state.Current.SavePlaylistMeta(cp.FolderName, pl.FolderName, newName, pl.Bio); err != nil {
		m.sidebarError = err.Error()
		m.sidebarErrIsError = true
		return m, nil
	}
	_ = state.Current.ScanProfiles()
	m.refreshAllContent()
	m.addLog("ok", langT("Playlist renamed: ", "Playlist yeniden adlandirildi: ")+newName)
	return m, nil
}

func (m *HomeModel) playSelectedSong() tea.Cmd {
	songs := m.songs()
	if m.songFocusIdx >= 0 && m.songFocusIdx < len(songs) {
		return m.playSong(&songs[m.songFocusIdx])
	}
	return nil
}

// PlayResultMsg is returned after a play attempt
type PlayResultMsg struct {
	Title string
	Error error
}

func (m *HomeModel) playSong(song *state.Song) tea.Cmd {
	if song == nil {
		return nil
	}
	m.songEndedAt = time.Time{} // cancel any pending auto-advance
	m.manualStop = true         // mark as manual so processPlayerStatus won't re-arm
	// Stop any current playback before starting a new one
	if state.Current.Player.IsPlaying {
		bridge.PlayerCall(bridge.Action{Action: "stop"})
	}
	state.Current.Player.CurrentSong = song
	state.Current.Player.IsPlaying = true
	state.Current.Player.IsPaused = false
	state.Current.Player.StatusMsg = ""
	return func() tea.Msg {
		path := song.FilePath
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return PlayResultMsg{Title: song.Title, Error: fmt.Errorf("file not found: %s", path)}
		}
		pyPath := strings.ReplaceAll(path, "\\", "/")
		result, err := bridge.PlayerCall(bridge.Action{Action: "play", File: pyPath})
		if err == nil && result != nil {
			if result.Status == "error" {
				return PlayResultMsg{Title: song.Title, Error: fmt.Errorf("%s", result.Error)}
			}
			state.Current.Player.Duration = result.Duration
			state.Current.Player.Position = result.Position
			if result.Duration == 0 && song.Duration != "00:00" && song.Duration != "" {
				parts := strings.Split(song.Duration, ":")
				if len(parts) == 2 {
					m, s := 0, 0
					fmt.Sscanf(parts[0], "%d", &m)
					fmt.Sscanf(parts[1], "%d", &s)
					state.Current.Player.Duration = float64(m*60 + s)
				}
			}
		}
		return PlayResultMsg{Title: song.Title, Error: err}
	}
}

func (m *HomeModel) NextSong() tea.Cmd {
	return func() tea.Msg {
		pl := state.Current.CurrentPlaylist
		if pl == nil || len(pl.Songs) == 0 {
			return nil
		}
		cur := state.Current.Player.CurrentSong
		if cur == nil {
			return PlaySongMsg{FilePath: pl.Songs[0].FilePath}
		}
		if state.Current.Player.IsShuffled {
			for _, s := range pl.Songs {
				if s.Filename != cur.Filename {
					return PlaySongMsg{FilePath: s.FilePath}
				}
			}
			return nil
		}
		n := len(pl.Songs)
		for i, s := range pl.Songs {
			if s.Filename == cur.Filename {
				return PlaySongMsg{FilePath: pl.Songs[(i+1)%n].FilePath}
			}
		}
		// Fallback: play first song
		return PlaySongMsg{FilePath: pl.Songs[0].FilePath}
	}
}

func (m *HomeModel) OnDownloadResult(msg DownloadResultMsg) tea.Cmd {
	m.handleDownloadResult(msg)
	return nil
}

func (m *HomeModel) OnImportResult(msg ImportResultMsg) tea.Cmd {
	m.handleImportResult(msg)
	return nil
}

func (m *HomeModel) togglePlayPause() {
	ps := &state.Current.Player
	if ps.CurrentSong == nil {
		if state.Current.CurrentPlaylist != nil && len(state.Current.CurrentPlaylist.Songs) > 0 {
			m.playSong(&state.Current.CurrentPlaylist.Songs[0])
		}
		return
	}
	if ps.IsPlaying {
		ps.IsPaused = true
		ps.IsPlaying = false
		go bridge.PlayerCall(bridge.Action{Action: "pause"})
	} else {
		ps.IsPlaying = true
		ps.IsPaused = false
		go bridge.PlayerCall(bridge.Action{Action: "resume"})
	}
}

func (m *HomeModel) adjustVolume(delta float64) {
	v := state.Current.Player.Volume + delta
	if v > 1 {
		v = 1
	} else if v < 0 {
		v = 0
	}
	state.Current.Player.Volume = v
	go bridge.PlayerCall(bridge.Action{Action: "volume", Value: v})
}

func (m *HomeModel) processPlayerStatus(r *bridge.Result) {
	wasPlaying := state.Current.Player.IsPlaying
	state.Current.Player.Position = r.Position
	state.Current.Player.Duration = r.Duration
	state.Current.Player.AudioLevelL = r.AudioLevelL
	state.Current.Player.AudioLevelR = r.AudioLevelR
	if len(r.Spectrum) >= 16 {
		for i := 0; i < 16; i++ {
			state.Current.Player.Spectrum[i] = r.Spectrum[i]
		}
	}
	if r.Format != "" {
		state.Current.Player.Format = r.Format
		state.Current.Player.SampleRate = r.SampleRate
		state.Current.Player.Bitrate = r.Bitrate
	}
	switch r.Status {
	case "playing":
		state.Current.Player.IsPlaying = true
		state.Current.Player.IsPaused = false
		m.manualStop = false
	case "paused":
		state.Current.Player.IsPlaying = false
		state.Current.Player.IsPaused = true
	case "stopped", "idle":
		if wasPlaying {
			state.Current.Player.IsPlaying = false
			state.Current.Player.IsPaused = false
			if !m.manualStop {
				m.songEndedAt = time.Now()
			}
		}
		m.manualStop = false
	}
}

func (m *HomeModel) checkAutoAdvance() tea.Cmd {
	if m.songEndedAt.IsZero() {
		return nil
	}
	if time.Since(m.songEndedAt) >= 2*time.Second {
		m.songEndedAt = time.Time{}
		return m.NextSong()
	}
	return nil
}

func (m *HomeModel) selectPlaylist(idx int) {
	if state.Current.CurrentProfile != nil && idx < len(state.Current.CurrentProfile.Playlists) {
		state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[idx]
		m.playlistIdx = idx
	}
}

func (m *HomeModel) refreshAllContent() {
	savedProfile := state.Current.CurrentProfile
	savedPlaylist := state.Current.CurrentPlaylist
	_ = state.Current.ScanProfiles()
	if len(state.Current.Profiles) > 0 {
		if savedProfile != nil {
			found := false
			for i, p := range state.Current.Profiles {
				if p.FolderName == savedProfile.FolderName {
					state.Current.CurrentProfile = &state.Current.Profiles[i]
					found = true
					state.Current.CurrentPlaylist = nil
					if savedPlaylist != nil {
						for j, pl := range state.Current.CurrentProfile.Playlists {
							if pl.FolderName == savedPlaylist.FolderName {
								state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[j]
								break
							}
						}
					}
					break
				}
			}
			if !found {
				state.Current.CurrentProfile = &state.Current.Profiles[0]
			}
		} else {
			state.Current.CurrentProfile = &state.Current.Profiles[0]
		}
		if state.Current.CurrentPlaylist == nil && len(state.Current.CurrentProfile.Playlists) > 0 {
			state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[0]
		}
	}
	m.refreshPlaylistOptions()
	if state.Current.CurrentPlaylist != nil {
		for i, pl := range state.Current.CurrentProfile.Playlists {
			if pl.FolderName == state.Current.CurrentPlaylist.FolderName {
				m.playlistIdx = i
				break
			}
		}
	}
}

func (m *HomeModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}
	m.bodyHeight = m.height

	sidebar := m.viewSidebar(m.height)
	sidebarW := lipgloss.Width(sidebar)
	bodyW := m.width
	contentW := bodyW - sidebarW
	if contentW < 40 {
		contentW = 40
	}

	content := m.viewContent(m.height, contentW)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	body = strings.TrimRight(body, "\n")

	if m.editModalOpen {
		return m.renderEditOverlay(body)
	}
	if m.deleteConfirm {
		return m.renderDeleteOverlay(body)
	}
	return body
}

func (m *HomeModel) viewSidebar(bodyH int) string {
	topH := bodyH / 2
	if topH < 18 {
		topH = 18
	}
	bottomH := bodyH - topH
	if bottomH < 4 {
		bottomH = 4
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.viewSidebarTop(topH), m.viewSidebarBottom(bottomH))
}

func (m *HomeModel) addLog(level, msg string) {
	now := time.Now().Format("15:04:05")
	var line string
	switch level {
	case "error":
		line = fmt.Sprintf("x %s %s", now, msg)
	case "ok":
		line = fmt.Sprintf("v %s %s", now, msg)
	default:
		line = fmt.Sprintf("> %s %s", now, msg)
	}
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
}

func (m *HomeModel) viewSidebarTop(bodyH int) string {
	title := ui.SectionTitleStyle.Render(langT("> MUSIC DOWNLOAD", "> MUZIK INDIR"))

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
	playlistBtn := ui.ButtonStyle.Render(langT("  + Playlist  ", "  + Playlist  "))
	musicBtn := ui.ButtonStyle.Render(langT("  + Music  ", "  + Muzik  "))
	if m.focusIdx == 3 {
		playlistBtn = ui.FocusedOutlineStyle.Render(langT("  + Playlist  ", "  + Playlist  "))
	}
	if m.focusIdx == 4 {
		musicBtn = ui.FocusedOutlineStyle.Render(langT("  + Music  ", "  + Muzik  "))
	}
	localBtn := lipgloss.JoinHorizontal(lipgloss.Left, playlistBtn, "  ", musicBtn)
	var playlistV string
	if m.renameMode {
		playlistV = "  " + langT("Rename:", "Yeni isim:") + "  " + m.renameInput.View()
	} else {
		playlistV = m.viewPlaylistDropdown()
		if m.focusIdx == 2 {
			playlistV = ui.AccentBorderStyle.Render(m.playlistOptions[m.playlistIdx])
		}
	}
	dlBtn := ui.AccentButtonStyle.Render(langT("  v Download  ", "  v Indir  "))
	if m.focusIdx == 5 {
		dlBtn = ui.FocusedButtonStyle.Render(langT("  v Download  ", "  v Indir  "))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", spotifyV, "", youtubeV, "", localBtn, "", playlistV, "", dlBtn)
	contentH := lipgloss.Height(content)
	targetH := bodyH - 2
	if contentH < targetH {
		content += strings.Repeat("\n", targetH-contentH)
	}
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
	sectionStyle := ui.BorderStyle
	if m.sectionFocus == 0 || (m.focusIdx >= 0 && m.focusIdx <= 5) {
		sectionStyle = ui.AccentBorderStyle
	}
	return sectionStyle.Width(w).Render(content)
}

func (m *HomeModel) viewSidebarBottom(bodyH int) string {
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
	visible := 16
	contentW := w - 4

	logCount := len(m.logLines)
	progLine := ""
	if m.isDownloading {
		progLine = fmt.Sprintf("  > Download: %.0f%%", m.downloadPercent)
	}
	totalLines := logCount
	if progLine != "" {
		totalLines++
	}

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
			var raw string
			if i < logCount {
				raw = m.logLines[i]
			} else {
				raw = progLine
			}
			if len(raw) > contentW-1 {
				raw = raw[:contentW-1]
			}
			if strings.HasPrefix(raw, "x ") {
				contentParts = append(contentParts, ui.ErrorStyle.Render(raw))
			} else if strings.HasPrefix(raw, "v ") {
				contentParts = append(contentParts, lipgloss.NewStyle().Foreground(lipgloss.Color("#1DB954")).Render(raw))
			} else if i >= logCount {
				contentParts = append(contentParts, lipgloss.NewStyle().Foreground(lipgloss.Color("#1DB954")).Render(raw))
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
	targetInner := visible + 2
	if innerH < targetInner {
		inner += strings.Repeat("\n", targetInner-innerH)
	}

	consoleStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444"))
	if m.sectionFocus == 3 {
		consoleStyle = ui.AccentBorderStyle
	}
	box := consoleStyle.Width(w).Render(inner)
	boxH := lipgloss.Height(box)
	if boxH < bodyH {
		box += strings.Repeat("\n", bodyH-boxH)
	}
	return box
}

func (m *HomeModel) viewPlaylistDropdown() string {
	label := ui.AccentStyle.Render("  " + langT("Playlist", "Playlist") + ":  ")
	current := m.playlistOptions[m.playlistIdx]
	if m.playlistExpanded {
		var items []string
		for i, opt := range m.playlistOptions {
			if i == m.playlistIdx {
				items = append(items, ui.AccentStyle.Render("> "+opt))
			} else {
				items = append(items, "  "+opt)
			}
		}
		return label + "\n" + strings.Join(items, "\n")
	}
	return label + ui.WhiteStyle.Render(current)
}

func (m *HomeModel) viewContent(bodyH, contentW int) string {
	plInfo := m.viewPlaylistInfo(bodyH)
	songsW := contentW - 42
	if songsW < 20 {
		songsW = 20
	}
	tableW := songsW
	tableTitle := ui.SectionTitleStyle.Render(langT("SONGS", "SARKILAR"))
	hint := ui.DimStyle.Render("  < > actions  Enter: exec")
	borderStyle := ui.BorderStyle
	if m.sectionFocus == 2 || m.focusIdx == 6 {
		borderStyle = ui.AccentBorderStyle
	}
	maxVis := m.maxVisibleSongs()
	if maxVis < 1 {
		maxVis = 1
	}
	if m.songOffset < 0 {
		m.songOffset = 0
	}
	songs := m.songs()
	if m.songOffset > len(songs)-maxVis && m.songOffset > 0 {
		m.songOffset = len(songs) - maxVis
	}
	songsHTML := m.renderSongs(tableW-4, m.songOffset, maxVis)

	songsH := lipgloss.Height(songsHTML)
	availH := bodyH - 4
	padding := availH - songsH
	if padding < 0 {
		padding = 0
	}
	songsHTML += strings.Repeat("\n", padding)

	tableBox := borderStyle.Width(tableW).Render(tableTitle + "\n" + songsHTML + "\n" + hint)
	return lipgloss.JoinHorizontal(lipgloss.Top, plInfo, tableBox)
}

func (m *HomeModel) renderSongs(w, offset, max int) string {
	songs := m.songs()
	if len(songs) == 0 {
		return ui.DimStyle.Render("  No songs yet")
	}

	if offset >= len(songs) {
		offset = 0
	}
	end := offset + max
	if end > len(songs) {
		end = len(songs)
	}

	selectedBg := lipgloss.Color("#1E3223")
	titleStyle := ui.WhiteStyle.Bold(true)
	artistStyle := ui.DimStyle
	headerStyle := ui.DimStyle.Bold(true)
	btnActiveText := ui.WhiteStyle.Bold(true)
	btnInactiveText := ui.DimStyle

	isFocused := m.focusIdx == 6

	numW := 4
	durW := 9
	actionsW := 20
	songW := w - numW - durW - actionsW
	if songW < 10 {
		songW = 10
	}
	titleW := songW * 3 / 5
	artistW := songW - titleW

	numCol := lipgloss.NewStyle().Width(numW).Align(lipgloss.Center)
	titleCol := lipgloss.NewStyle().Width(titleW).Align(lipgloss.Left)
	artistCol := lipgloss.NewStyle().Width(artistW).Align(lipgloss.Center)
	durCol := lipgloss.NewStyle().Width(durW).Align(lipgloss.Center)
	actCol := lipgloss.NewStyle().Width(actionsW)
	actColCenter := lipgloss.NewStyle().Width(actionsW).Align(lipgloss.Center)

	hNum := numCol.Render("#")
	hTitle := lipgloss.NewStyle().Width(titleW).Align(lipgloss.Center).Render(langT("Title", "Isim"))
	hArtist := artistCol.Render(langT("Artist", "Sanatci"))
	hDur := durCol.Render(langT("Dur.", "Sre."))
	hAct := actColCenter.Render(langT("Operations", "Islemler"))
	h := headerStyle.Render(fmt.Sprintf(" %s %s%s%s %s", hNum, hTitle, hArtist, hDur, hAct))
	items := []string{ui.BorderStyle.Width(w).Render(h)}

	for i := offset; i < end; i++ {
		song := songs[i]
		title := song.Title
		artist := song.Artist

		if len(title) > titleW-1 {
			title = title[:titleW-4] + "..."
		}
		if len(artist) > artistW-1 {
			artist = artist[:artistW-4] + "..."
		}

		numStr := numCol.Render(fmt.Sprintf("%d.", i+1))
		titleR := titleCol.Render(titleStyle.Render(title))
		artistR := artistCol.Render(artistStyle.Render(artist))
		dur := durCol.Render(song.Duration)

		isThisFocused := isFocused && m.songFocusIdx == i
		af := m.songActionFocus

		playBtn := " Play"
		editBtn := " Edit"
		delBtn := " Del"

		if isThisFocused {
			switch af {
			case 0:
				playBtn = btnActiveText.Render(" Play")
			case 1:
				editBtn = btnActiveText.Render(" Edit")
			case 2:
				delBtn = btnActiveText.Render(" Del")
			}
		} else {
			playBtn = btnInactiveText.Render(" Play")
			editBtn = btnInactiveText.Render(" Edit")
			delBtn = btnInactiveText.Render(" Del")
		}

		line := fmt.Sprintf(" %s %s%s%s %s", numStr, titleR, artistR, dur, actCol.Render(fmt.Sprintf("%-5s %-5s %-4s", playBtn, editBtn, delBtn)))

		if m.focusIdx == 6 && m.songFocusIdx == i {
			songStyle := ui.AccentBorderStyle.
				Width(w).
				Background(selectedBg)
			items = append(items, songStyle.Render(line))
		} else {
			songStyle := ui.BorderStyle.
				Width(w)
			items = append(items, songStyle.Render(line))
		}
	}
	return strings.Join(items, "\n")
}

func (m *HomeModel) viewPlaylistInfo(bodyH int) string {
	pl := state.Current.CurrentPlaylist
	cp := state.Current.CurrentProfile
	border := ui.BorderStyle
	if m.sectionFocus == 1 {
		border = ui.AccentBorderStyle
	}
	if pl == nil || cp == nil {
		title := ui.WhiteStyle.Bold(true).Render(" " + langT("PLAYLIST", "PLAYLIST") + " ")
		pad := bodyH - 6
		if pad < 0 {
			pad = 0
		}
		inner := title + "\n" + ui.DimStyle.Render("\n  No playlist selected") + strings.Repeat("\n", pad)
		return border.Width(38).Render(inner)
	}
	title := ui.WhiteStyle.Bold(true).Render(" " + langT("PLAYLIST", "PLAYLIST") + " ")
	if m.sectionFocus == 1 {
		var items []string
		items = append(items, ui.AccentStyle.Render("  "+langT("Playlist", "Playlist")+":"))
		for i, opt := range m.playlistOptions {
			if i == m.playlistIdx {
				items = append(items, ui.AccentStyle.Render("> "+opt))
			} else {
				items = append(items, "  "+opt)
			}
		}
		items = append(items, "", ui.DimStyle.Render(langT("  \u2191/\u2193 navigate  Enter: select", "  \u2191/\u2193 gez  Enter: sec")))
		inner := strings.Join(items, "\n")
		innerH := lipgloss.Height(inner)
		targetH := bodyH - 3
		if innerH < targetH {
			inner += strings.Repeat("\n", targetH-innerH)
		}
		return border.Width(38).Render(title + "\n" + inner)
	}
	name := ui.WhiteStyle.Bold(true).Render("  " + pl.Name)
	profileName := ui.FaintStyle.Render("  " + langT("Profile:", "Profil:") + " " + cp.DisplayName)

	// Art section
	var artStr string
	artTitle := ui.SectionTitleStyle.Render(" " + langT("Playlist Image", "Playlist Resmi") + " ")
	baseH := 12 // empties + name + profile + artTitle + bio + count + hints
	targetH := bodyH - 3
	avail := targetH - baseH
	if avail >= 4 && pl.ArtPath != "" {
		maxRows := 18
		if avail < maxRows {
			maxRows = avail
		}
		artStr = renderPlaylistArt(pl, 36, maxRows)
	} else {
		artTitle = ""
	}

	bio := ui.DimStyle.Render("  " + pl.Bio)
	count := ui.AccentStyle.Render(fmt.Sprintf("  %d songs", len(pl.Songs)))
	inner := lipgloss.JoinVertical(lipgloss.Left, "", name, profileName, artTitle, artStr, "", bio, "", count, "", "", ui.DimStyle.Render("  > Play    v Download"))
	innerH := lipgloss.Height(inner)
	if innerH < targetH {
		inner += strings.Repeat("\n", targetH-innerH)
	}
	return border.Width(38).Render(title + "\n" + inner)
}

func renderPlaylistArt(pl *state.Playlist, cols, rows int) string {
	if pl == nil || pl.ArtPath == "" {
		return ""
	}
	f, err := os.Open(pl.ArtPath)
	if err != nil {
		return ""
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return ""
	}
	// Inner area (excluding border)
	inCols := cols - 2
	inRows := rows - 2
	if inCols < 4 || inRows < 2 {
		return ""
	}

	// Resize: each cell shows 2 vertical pixels via half-block (▄)
	pixelW := inCols
	pixelH := inRows * 2
	resized := scaleImage(img, pixelW, pixelH)
	applyRoundedCorners(resized, pixelW/12)

	// Extract dominant color from resized image
	dr, dg, db := averageColor(resized)

	var out strings.Builder
	domFg := fmt.Sprintf("\033[38;2;%d;%d;%dm", dr, dg, db)
	reset := "\033[0m"

	// Top border line
	out.WriteString(domFg + "┌" + strings.Repeat("─", inCols) + "┐" + reset + "\n")

	// Image rows with side borders
	for cy := 0; cy < inRows; cy++ {
		out.WriteString(domFg + "│" + reset)
		for cx := 0; cx < inCols; cx++ {
			r1, g1, b1, a1 := resized.At(cx, cy*2).RGBA()
			r2, g2, b2, a2 := resized.At(cx, cy*2+1).RGBA()

			if a1 < 128 && a2 < 128 {
				out.WriteByte(' ')
				continue
			}
			if a2 < 128 {
				out.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm▀\033[0m", r1>>8, g1>>8, b1>>8))
				continue
			}
			if a1 < 128 {
				out.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm▄\033[0m", r2>>8, g2>>8, b2>>8))
			} else {
				out.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm▄\033[0m", r2>>8, g2>>8, b2>>8, r1>>8, g1>>8, b1>>8))
			}
		}
		out.WriteString(domFg + "│" + reset + "\n")
	}

	// Bottom border line
	out.WriteString(domFg + "└" + strings.Repeat("─", inCols) + "┘" + reset)

	return out.String()
}

func scaleImage(img image.Image, dstW, dstH int) *image.RGBA {
	src := img.Bounds()
	srcW := src.Dx()
	srcH := src.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			sy := y * srcH / dstH
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

func applyRoundedCorners(img *image.RGBA, r int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Top-left
			if x < r && y < r {
				dx, dy := r-x, r-y
				if dx*dx+dy*dy > r*r {
					img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				}
			}
			// Top-right
			if x >= w-r && y < r {
				dx, dy := x-(w-r-1), r-y
				if dx*dx+dy*dy > r*r {
					img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				}
			}
			// Bottom-left
			if x < r && y >= h-r {
				dx, dy := r-x, y-(h-r-1)
				if dx*dx+dy*dy > r*r {
					img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				}
			}
			// Bottom-right
			if x >= w-r && y >= h-r {
				dx, dy := x-(w-r-1), y-(h-r-1)
				if dx*dx+dy*dy > r*r {
					img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				}
			}
		}
	}
}

func averageColor(img *image.RGBA) (uint8, uint8, uint8) {
	b := img.Bounds()
	var tr, tg, tb uint64
	var n uint64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a > 128 {
				tr += uint64(r >> 8)
				tg += uint64(g >> 8)
				tb += uint64(bl >> 8)
				n++
			}
		}
	}
	if n == 0 {
		return 100, 100, 100
	}
	return uint8(tr / n), uint8(tg / n), uint8(tb / n)
}
