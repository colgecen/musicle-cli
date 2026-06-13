package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sqweek/dialog"

	"musicle-cli/bridge"
	"musicle-cli/components"
	"musicle-cli/state"
	"musicle-cli/ui"
)

type HomeModel struct {
	width  int
	height int
	ready  bool

	focusIdx    int
	sectionFocus int // 0=sidebar, 1=playlist, 2=songs, 3=console

	spotifyInput textinput.Model
	youtubeInput textinput.Model

	songFocusIdx    int
	songActionFocus int // 0=play, 1=edit, 2=delete, -1=none

	playlistOptions []string
	playlistIdx     int
	playlistExpanded bool

	sidebarError      string
	sidebarErrIsError bool

	logLines []string

	editModalOpen bool
	editSongIdx   int
	editTitle     textinput.Model
	editArtist    textinput.Model
	editDuration  textinput.Model
	editFocus     int

	deleteConfirm bool
	deleteSongIdx int
	deleteYes     bool
}

func NewHomeModel() *HomeModel {
	cursorStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1DB954")).
		Foreground(lipgloss.Color("#000000"))

	si := textinput.New()
	si.Placeholder = "https://open.spotify.com/..."
	si.Prompt = "  Spotify URL:  "
	si.Cursor.Style = cursorStyle
	si.Width = 50
	si.CharLimit = 300

	yi := textinput.New()
	yi.Placeholder = "https://youtube.com/..."
	yi.Prompt = "  YouTube URL:  "
	yi.Cursor.Style = cursorStyle
	yi.Width = 50
	yi.CharLimit = 300

	return &HomeModel{
		spotifyInput:  si,
		youtubeInput:  yi,
		playlistIdx:   0,
		sectionFocus:  -1,
		songFocusIdx:  -1,
		songActionFocus: -1,
		editTitle:     editInput(langT("Title", "Başlık")),
		editArtist:    editInput(langT("Artist", "Sanatçı")),
		editDuration:  editInput(langT("Duration", "Süre")),
	}
}

func editInput(prompt string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "  " + prompt + ":  "
	ti.Cursor.Style = lipgloss.NewStyle().
		Background(lipgloss.Color("#1DB954")).
		Foreground(lipgloss.Color("#000000"))
	ti.Width = 40
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
					m.playSong(&pl.Songs[i])
					break
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
	switch msg.String() {
	case "tab":
		if m.focusIdx == 5 {
			songs := m.songs()
			if len(songs) > 0 {
				m.songFocusIdx = (m.songFocusIdx + 1) % len(songs)
				m.songActionFocus = 0
			}
		} else {
			m.cycleFocus(1)
		}
		return m, nil
	case "shift+tab":
		if m.focusIdx == 5 {
			songs := m.songs()
			if len(songs) > 0 {
				m.songFocusIdx = (m.songFocusIdx - 1 + len(songs)) % len(songs)
				m.songActionFocus = 0
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
		if m.focusIdx == 5 && m.songFocusIdx >= 0 {
			m.songActionFocus = (m.songActionFocus + 1) % 3
			return m, tea.HideCursor
		}
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: 5})
		return m, nil
	case "left":
		if m.focusIdx == 5 && m.songFocusIdx >= 0 {
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
		if m.focusIdx != 5 {
			if m.focusIdx >= 0 && m.focusIdx <= 4 {
				inputs := m.focusedInputs()
				for _, inp := range inputs {
					if inp != nil {
						inp.Blur()
					}
				}
			}
			m.focusIdx = 5
			songs := m.songs()
			if len(songs) > 0 && m.songFocusIdx < 0 {
				m.songFocusIdx = 0
				m.songActionFocus = 0
			}
		}
		return m, tea.HideCursor
	case "e":
		if m.focusIdx == 5 && m.songFocusIdx >= 0 {
			m.openEditModal()
			return m, nil
		}
	case "d":
		if m.focusIdx == 5 && m.songFocusIdx >= 0 {
			m.openDeleteConfirm()
			return m, nil
		}
	case "up":
		if m.focusIdx == 5 {
			songs := m.songs()
			if len(songs) > 0 {
				m.songFocusIdx = (m.songFocusIdx - 1 + len(songs)) % len(songs)
				m.songActionFocus = 0
			}
			return m, nil
		}
		m.adjustVolume(0.05)
		return m, nil
	case "down":
		if m.focusIdx == 5 {
			songs := m.songs()
			if len(songs) > 0 {
				m.songFocusIdx = (m.songFocusIdx + 1) % len(songs)
				m.songActionFocus = 0
			}
			return m, nil
		}
		m.adjustVolume(-0.05)
		return m, nil
	}

	if m.focusIdx == 0 {
		var cmd tea.Cmd
		m.spotifyInput, cmd = m.spotifyInput.Update(msg)
		return m, cmd
	}
	if m.focusIdx == 1 {
		var cmd tea.Cmd
		m.youtubeInput, cmd = m.youtubeInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *HomeModel) handlePlaylistKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.playlistIdx = (m.playlistIdx - 1 + len(m.playlistOptions)) % len(m.playlistOptions)
	case "down", "j":
		m.playlistIdx = (m.playlistIdx + 1) % len(m.playlistOptions)
	case "enter":
		m.selectPlaylist(m.playlistIdx)
		m.playlistExpanded = false
	case "esc":
		m.playlistExpanded = false
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
	} else {
		m.focusIdx = (m.focusIdx + dir + 5) % 5
	}
	switch m.focusIdx {
	case 0:
		m.spotifyInput.Focus()
	case 1:
		m.youtubeInput.Focus()
	}
}

// CycleSection cycles between sidebar (0) and songs (2) sections
func (m *HomeModel) CycleSection() tea.Cmd {
	if m.focusIdx >= 0 && m.focusIdx <= 4 {
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
	switch m.sectionFocus {
	case 0:
		m.sectionFocus = 1
	case 1:
		m.sectionFocus = 2
		m.focusIdx = 5
		m.songFocusIdx = 0
		m.songActionFocus = 0
	case 2:
		m.sectionFocus = 4
	default:
		m.sectionFocus = 0
	}
	return tea.HideCursor
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

func (m *HomeModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focusIdx {
	case 0, 1:
		return m, m.startDownload()
	case 2:
		m.playlistExpanded = true
		return m, nil
	case 3:
		return m, m.openLocalPlaylistDialog()
	case 4:
		return m, m.openLocalMusicDialog()
	case 5:
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
	spotifyURL := strings.TrimSpace(m.spotifyInput.Value())
	youtubeURL := strings.TrimSpace(m.youtubeInput.Value())
	url := spotifyURL
	action := "download_spotify"
	if url == "" {
		url = youtubeURL
		action = "download_youtube"
	}
	if url == "" {
		m.sidebarError = langT("Enter a URL first", "Önce bir URL girin")
		m.sidebarErrIsError = true
		return nil
	}
	if !strings.HasPrefix(url, "http") {
		m.sidebarError = langT("Invalid URL", "Hatalı Link")
		m.sidebarErrIsError = true
		return nil
	}
	outDir := ""
	if state.Current.CurrentProfile != nil && m.playlistIdx >= 0 && m.playlistIdx < len(state.Current.CurrentProfile.Playlists) {
		pl := state.Current.CurrentProfile.Playlists[m.playlistIdx]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	}
	m.sidebarError = langT("Downloading…", "İndiriliyor…")
	m.sidebarErrIsError = false
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
		selectedPath, err := dialog.Directory().Title(langT("Select Music Directory", "Müzik Klasörü Seç")).Browse()
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
			Filter(langT("Audio Files", "Ses Dosyaları"), "mp3").
			Title(langT("Select Audio Files", "Ses Dosyası Seç")).
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
	if msg.Error != nil || msg.Result.Status == "error" {
		errMsg := ""
		if msg.Result != nil {
			errMsg = msg.Result.Error
		}
		if errMsg == "" && msg.Error != nil {
			errMsg = msg.Error.Error()
		}
		m.sidebarError = "✗ " + errMsg
		m.sidebarErrIsError = true
		m.addLog("error", langT("Download failed: ", "İndirme başarısız: ")+errMsg)
		return clearSidebarAfter(4 * time.Second)
	}
	msgText := langT("✓ Downloaded: ", "✓ İndirildi: ") + msg.Result.Filename
	m.sidebarError = msgText
	m.sidebarErrIsError = false
	m.addLog("ok", langT("Downloaded: ", "İndirildi: ")+msg.Result.Filename)
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
		m.sidebarError = "✗ " + errMsg
		m.sidebarErrIsError = true
		m.addLog("error", langT("Import failed: ", "İçe aktarma başarısız: ")+errMsg)
		return clearSidebarAfter(4 * time.Second)
	}
	msgText := langT("✓ Imported: ", "✓ İçe Aktarıldı: ") + msg.Result.Filename
	m.sidebarError = msgText
	m.sidebarErrIsError = false
	m.addLog("ok", langT("Imported: ", "İçe Aktarıldı: ")+msg.Result.Filename)
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
			m.addLog("ok", langT("Updated: ", "Güncellendi: ")+song.Title)
		} else {
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			} else if result != nil {
				errMsg = result.Error
			}
			m.sidebarError = errMsg
			m.sidebarErrIsError = true
			m.addLog("error", langT("Update failed: ", "Güncelleme başarısız: ")+errMsg)
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
			Path:   song.FilePath,
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
			m.addLog("error", langT("Delete failed: ", "Silme başarısız: ")+errMsg)
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
	noBtn := ui.ButtonStyle.Render("  No  ")
	yesBtn := ui.ErrorButtonStyle.Render("  Yes  ")
	if m.deleteYes {
		yesBtn = ui.AccentButtonStyle.Render("  Yes  ")
	} else {
		noBtn = ui.AccentButtonStyle.Render("  No  ")
	}
	btns := lipgloss.JoinHorizontal(lipgloss.Left, yesBtn, "  ", noBtn)
	content := lipgloss.JoinVertical(lipgloss.Center, "", msg, "", btns, "")
	content = ui.BorderStyle.Width(40).Render(ui.WhiteStyle.Bold(true).Render(" CONFIRM DELETE ") + "\n" + content)
	return m.placeOverlay(full, content)
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
	state.Current.Player.CurrentSong = song
	state.Current.Player.IsPlaying = true
	state.Current.Player.IsPaused = false
	state.Current.Player.StatusMsg = ""
	return func() tea.Msg {
		_, err := bridge.PlayerCall(bridge.Action{Action: "play", File: song.FilePath})
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
		for i, s := range pl.Songs {
			if s.Filename == cur.Filename && i+1 < len(pl.Songs) {
				return PlaySongMsg{FilePath: pl.Songs[i+1].FilePath}
			}
		}
		return nil
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
	switch r.Status {
	case "playing":
		state.Current.Player.IsPlaying = true
		state.Current.Player.IsPaused = false
	case "paused":
		state.Current.Player.IsPlaying = false
		state.Current.Player.IsPaused = true
	case "stopped", "idle":
		if wasPlaying {
			state.Current.Player.IsPlaying = false
			state.Current.Player.IsPaused = false
		}
	}
}

func (m *HomeModel) selectPlaylist(idx int) {
	if state.Current.CurrentProfile != nil && idx < len(state.Current.CurrentProfile.Playlists) {
		state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[idx]
		m.playlistIdx = idx
	}
}

func (m *HomeModel) refreshAllContent() {
	_ = state.Current.ScanProfiles()
	if len(state.Current.Profiles) > 0 {
		state.Current.CurrentProfile = &state.Current.Profiles[0]
		if state.Current.CurrentPlaylist != nil {
			for i, pl := range state.Current.CurrentProfile.Playlists {
				if pl.FolderName == state.Current.CurrentPlaylist.FolderName {
					state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[i]
					break
				}
			}
		} else if len(state.Current.CurrentProfile.Playlists) > 0 {
			state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[0]
		}
	}
	m.refreshPlaylistOptions()
}

func (m *HomeModel) View() string {
	if m.width <= 0 {
		m.width = 120
		m.height = 40
	}

	header := m.viewHeader()
	playerBar := m.viewPlayerBar(0)

	headerH := lipgloss.Height(header)
	barH := lipgloss.Height(playerBar)
	bodyH := m.height - headerH - barH
	if bodyH < 5 {
		bodyH = 5
	}

	sidebar := m.viewSidebar(bodyH)
	sidebarW := lipgloss.Width(sidebar)
	bodyW := m.width
	contentW := bodyW - sidebarW
	if contentW < 40 {
		contentW = 40
	}

	content := m.viewContent(bodyH, contentW)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	bodyHActual := lipgloss.Height(body)
	if bodyHActual < bodyH {
		body += strings.Repeat("\n", bodyH-bodyHActual)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, header, body, playerBar)
	fullH := lipgloss.Height(full)
	if fullH < m.height {
		full += strings.Repeat("\n", m.height-fullH)
	}
	if m.editModalOpen {
		return m.renderEditOverlay(full)
	}
	if m.deleteConfirm {
		return m.renderDeleteOverlay(full)
	}
	return full
}

func (m *HomeModel) viewHeader() string {
	return components.RenderHeader(m.width, "home")
}

func (m *HomeModel) viewPlayerBar(_ int) string {
	return components.RenderPlayerBar(m.width, m.sectionFocus == 4)
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
		line = ui.ErrorStyle.Render(fmt.Sprintf("✗ %s %s", now, msg))
	case "ok":
		line = ui.AccentStyle.Render(fmt.Sprintf("✓ %s %s", now, msg))
	default:
		line = ui.DimStyle.Render(fmt.Sprintf("• %s %s", now, msg))
	}
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 100 {
		m.logLines = m.logLines[len(m.logLines)-100:]
	}
}

func (m *HomeModel) viewSidebarTop(bodyH int) string {
	title := ui.SectionTitleStyle.Render(langT("♫ MUSIC DOWNLOAD", "♫ MÜZİK İNDİR"))
	focusBorder := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#1DB954")).
		Padding(0, 1)

	spotifyV := m.spotifyInput.View()
	if m.focusIdx == 0 {
		spotifyV = focusBorder.Render(m.spotifyInput.View())
	} else {
		if m.spotifyInput.Value() == "" {
			spotifyV = ui.FaintStyle.Render(m.spotifyInput.Prompt + m.spotifyInput.Placeholder)
		} else {
			spotifyV = ui.DimStyle.Render(m.spotifyInput.Prompt + m.spotifyInput.Value())
		}
	}
	youtubeV := m.youtubeInput.View()
	if m.focusIdx == 1 {
		youtubeV = focusBorder.Render(m.youtubeInput.View())
	} else {
		if m.youtubeInput.Value() == "" {
			youtubeV = ui.FaintStyle.Render(m.youtubeInput.Prompt + m.youtubeInput.Placeholder)
		} else {
			youtubeV = ui.DimStyle.Render(m.youtubeInput.Prompt + m.youtubeInput.Value())
		}
	}
	playlistBtn := ui.ButtonStyle.Render(langT("  + Playlist  ", "  + Playlist  "))
	musicBtn := ui.ButtonStyle.Render(langT("  + Music  ", "  + Müzik  "))
	if m.focusIdx == 3 {
		playlistBtn = ui.AccentBorderStyle.Render(langT("  + Playlist  ", "  + Playlist  "))
	}
	if m.focusIdx == 4 {
		musicBtn = ui.AccentBorderStyle.Render(langT("  + Music  ", "  + Müzik  "))
	}
	localBtn := lipgloss.JoinHorizontal(lipgloss.Left, playlistBtn, "  ", musicBtn)
	playlistV := m.viewPlaylistDropdown()
	if m.focusIdx == 2 {
		playlistV = ui.AccentBorderStyle.Render(m.playlistOptions[m.playlistIdx])
	}
	errText := ""
	if m.sidebarError != "" {
		if m.sidebarErrIsError {
			errText = ui.ErrorStyle.Render("  " + m.sidebarError)
		} else {
			errText = ui.AccentStyle.Render("  " + m.sidebarError)
		}
	}
	dlBtn := ui.AccentButtonStyle.Render(langT("  Download  ", "  İndir  "))
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", spotifyV, "", youtubeV, "", localBtn, "", playlistV, "", errText, "", dlBtn)
	contentH := lipgloss.Height(content)
	targetH := bodyH - 2
	if contentH < targetH {
		content += strings.Repeat("\n", targetH-contentH)
	}
	w := 38
	if m.width > 0 {
		w = m.width / 4
		if w < 30 {
			w = 30
		}
		if w > 50 {
			w = 50
		}
	}
	sectionStyle := ui.BorderStyle
	if m.sectionFocus == 0 || (m.focusIdx >= 0 && m.focusIdx <= 4) {
		sectionStyle = ui.AccentBorderStyle
	}
	return sectionStyle.Width(w).Render(content)
}

func (m *HomeModel) viewSidebarBottom(bodyH int) string {
	w := 38
	if m.width > 0 {
		w = m.width / 4
		if w < 30 {
			w = 30
		}
		if w > 50 {
			w = 50
		}
	}
	title := ui.SectionTitleStyle.Render(langT("CONSOLE", "KONSOL"))
	var logText string
	if len(m.logLines) == 0 {
		logText = ui.FaintStyle.Render("  No messages")
	} else {
		start := len(m.logLines) - (bodyH - 5)
		if start < 0 {
			start = 0
		}
		logText = strings.Join(m.logLines[start:], "\n")
	}
	inner := title + "\n" + logText
	innerH := lipgloss.Height(inner)
	targetH := bodyH - 2
	if innerH < targetH {
		inner += strings.Repeat("\n", targetH-innerH)
	}
	consoleStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444"))
	if m.sectionFocus == 3 {
		consoleStyle = ui.AccentBorderStyle
	}
	return consoleStyle.Width(w).Render(inner)
}

func (m *HomeModel) viewPlaylistDropdown() string {
	label := ui.AccentStyle.Render("  " + langT("Playlist", "Playlist") + ":  ")
	current := m.playlistOptions[m.playlistIdx]
	if m.playlistExpanded {
		var items []string
		for i, opt := range m.playlistOptions {
			if i == m.playlistIdx {
				items = append(items, ui.AccentStyle.Render("▸ "+opt))
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
	songsW := contentW - 32
	if songsW < 30 {
		songsW = 30
	}
	tableW := songsW
	tableTitle := ui.SectionTitleStyle.Render(langT("SONGS", "ŞARKILAR"))
	hint := ui.DimStyle.Render("  ← → actions  Enter: exec")
	borderStyle := ui.BorderStyle
	if m.sectionFocus == 2 || m.focusIdx == 5 {
		borderStyle = ui.AccentBorderStyle
	}
	songsHTML := m.renderSongs(tableW - 4)

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

func (m *HomeModel) renderSongs(w int) string {
	songs := m.songs()
	if len(songs) == 0 {
		return ui.DimStyle.Render("  No songs yet")
	}

	selectedBg := lipgloss.Color("#1E3223")
	titleStyle := ui.WhiteStyle.Bold(true)
	artistStyle := ui.DimStyle
	durStyle := ui.DimStyle
	btnActiveText := ui.WhiteStyle.Bold(true)
	btnInactiveText := ui.DimStyle

	isFocused := m.focusIdx == 5

	var items []string
	for i, song := range songs {
		title := song.Title
		artist := song.Artist
		maxTitle := 24

		if len(title) > maxTitle {
			title = title[:maxTitle-1] + "…"
		}
		if len(artist) > maxTitle {
			artist = artist[:maxTitle-1] + "…"
		}

		numStr := ui.SongNumStyle.Render(fmt.Sprintf("%d.", i+1))
		titleArtist := titleStyle.Render(title) + " " + artistStyle.Render(artist)
		dur := durStyle.Render(song.Duration)

		isThisFocused := isFocused && m.songFocusIdx == i
		af := m.songActionFocus

		playBtn := btnInactiveText.Render(" Play")
		editBtn := btnInactiveText.Render(" Edit")
		delBtn := btnInactiveText.Render(" Del")

		if isThisFocused {
			switch af {
			case 0:
				playBtn = btnActiveText.Render(" Play")
			case 1:
				editBtn = btnActiveText.Render(" Edit")
			case 2:
				delBtn = btnActiveText.Render(" Del")
			}
		}

		line := fmt.Sprintf(" %s %s %s %s  %s %s %s", numStr, ui.GreenDotStyle, titleArtist, dur, playBtn, editBtn, delBtn)

		if m.focusIdx == 5 && m.songFocusIdx == i {
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
	border := ui.BorderStyle
	if m.sectionFocus == 1 {
		border = ui.AccentBorderStyle
	}
	if pl == nil {
		title := ui.WhiteStyle.Bold(true).Render(" " + langT("PLAYLIST", "PLAYLIST") + " ")
		pad := bodyH - 6
		if pad < 0 {
			pad = 0
		}
		inner := title + "\n" + ui.DimStyle.Render("\n  No playlist selected") + strings.Repeat("\n", pad)
		return border.Width(28).Render(inner)
	}
	name := ui.WhiteStyle.Bold(true).Render("  " + pl.Name)
	bio := ui.DimStyle.Render("  " + pl.Bio)
	count := ui.AccentStyle.Render(fmt.Sprintf("  %d songs", len(pl.Songs)))
	inner := lipgloss.JoinVertical(lipgloss.Left, "", name, "", bio, "", count, "", "", ui.DimStyle.Render("  ♪ Play    ⬇ Download"))
	innerH := lipgloss.Height(inner)
	targetH := bodyH - 3
	if innerH < targetH {
		inner += strings.Repeat("\n", targetH-innerH)
	}
	title := ui.WhiteStyle.Bold(true).Render(" " + langT("PLAYLIST", "PLAYLIST") + " ")
	return border.Width(28).Render(title + "\n" + inner)
}

