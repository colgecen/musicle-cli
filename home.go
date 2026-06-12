package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sqweek/dialog"

	"musicle-cli/bridge"
	"musicle-cli/state"
	"musicle-cli/ui"
)

type HomeModel struct {
	width  int
	height int
	ready  bool

	focusIdx int

	spotifyInput textinput.Model
	youtubeInput textinput.Model
	songTable    table.Model

	playlistOptions []string
	playlistIdx     int
	playlistExpanded bool

	sidebarError      string
	sidebarErrIsError bool
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
		spotifyInput: si,
		youtubeInput: yi,
		playlistIdx:  0,
	}
}

func (m *HomeModel) Init() tea.Cmd {
	m.refreshPlaylistOptions()
	m.initSongTable()
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

func (m *HomeModel) initSongTable() {
	columns := []table.Column{
		{Title: "#", Width: 5},
		{Title: "", Width: 4},
		{Title: "Title / Artist", Width: 40},
		{Title: "Date Added", Width: 14},
		{Title: "Duration", Width: 10},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(false),
		table.WithHeight(20),
	)
	t.SetStyles(table.Styles{
		Header:   ui.DimStyle.Bold(true).Padding(0, 1),
		Cell:     ui.WhiteStyle.Padding(0, 1),
		Selected: ui.SelectedRowStyle,
	})
	m.songTable = t
	m.buildSongRows()
}

func (m *HomeModel) buildSongRows() {
	var rows []table.Row
	pl := state.Current.CurrentPlaylist
	if pl == nil || len(pl.Songs) == 0 {
		rows = append(rows, table.Row{"", "", "No songs yet", "", ""})
		m.songTable.SetRows(rows)
		return
	}
	for i, song := range pl.Songs {
		row := i + 1
		title := song.Title
		if len(title) > 30 {
			title = title[:28] + "…"
		}
		artist := song.Artist
		if len(artist) > 30 {
			artist = artist[:28] + "…"
		}
		rows = append(rows, table.Row{
			fmt.Sprintf("%d", row),
			"♪",
			title + "\n" + artist,
			song.DateAdded,
			song.Duration,
		})
	}
	m.songTable.SetRows(rows)
}

func (m *HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.songTable.SetWidth(msg.Width - 50)
		h := msg.Height - 10
		if h < 5 {
			h = 5
		}
		m.songTable.SetHeight(h)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case PlayerStatusResult:
		if msg.Result != nil {
			m.processPlayerStatus(msg.Result)
		}

	case DownloadResultMsg:
		m.handleDownloadResult(msg)

	case ImportResultMsg:
		m.handleImportResult(msg)

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

	if m.focusIdx == 4 {
		var cmd tea.Cmd
		m.songTable, cmd = m.songTable.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *HomeModel) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.playlistExpanded {
		return m.handlePlaylistKey(msg)
	}
	switch msg.String() {
	case "tab":
		m.cycleFocus(1)
		return m, nil
	case "shift+tab":
		m.cycleFocus(-1)
		return m, nil
	case "enter":
		return m.handleEnter()
	case " ":
		m.togglePlayPause()
		return m, nil
	case "right":
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: 5})
		return m, nil
	case "left":
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: -5})
		return m, nil
	case "up":
		m.adjustVolume(0.05)
		return m, nil
	case "down":
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
		if m.focusIdx == 4 {
			m.songTable.Blur()
		}
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
	case 4:
		m.songTable.Focus()
	}
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
		return m, m.openLocalFileDialog()
	case 4:
		row := m.songTable.Cursor()
		songs := m.currentSongs()
		if row > 0 && row-1 < len(songs) {
			m.playSong(&songs[row-1])
		}
	}
	return m, nil
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

func (m *HomeModel) handleDownloadResult(msg DownloadResultMsg) {
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
		return
	}
	m.sidebarError = langT("✓ Downloaded: ", "✓ İndirildi: ") + msg.Result.Filename
	m.sidebarErrIsError = false
	m.refreshAllContent()
}

func (m *HomeModel) handleImportResult(msg ImportResultMsg) {
	if msg.Error != nil || msg.Result.Status == "error" {
		m.sidebarError = "⚠ File not found"
		m.sidebarErrIsError = true
		return
	}
	m.refreshAllContent()
}

func (m *HomeModel) openLocalFileDialog() tea.Cmd {
	return func() tea.Msg {
		selectedPath, err := dialog.Directory().Title(langT("Select Music Directory", "Müzik Dosyası Seç")).Browse()
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

func (m *HomeModel) currentSongs() []state.Song {
	if state.Current.CurrentPlaylist != nil {
		return state.Current.CurrentPlaylist.Songs
	}
	return nil
}

func (m *HomeModel) playSong(song *state.Song) {
	if song == nil {
		return
	}
	state.Current.Player.CurrentSong = song
	state.Current.Player.IsPlaying = true
	state.Current.Player.IsPaused = false
	state.Current.Player.StatusMsg = ""
	go bridge.PlayerCall(bridge.Action{Action: "play", File: song.FilePath})
	m.buildSongRows()
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
		m.buildSongRows()
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
	m.buildSongRows()
}

func (m *HomeModel) View() string {
	header := m.viewHeader()
	playerBar := m.viewPlayerBar(m.width)

	headerH := lipgloss.Height(header)
	barH := lipgloss.Height(playerBar)
	bodyH := m.height - headerH - barH
	if bodyH < 5 {
		bodyH = 5
	}

	sidebar := m.viewSidebar(bodyH)
	content := m.viewContent(bodyH)
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
	return full
}

func (m *HomeModel) viewHeader() string {
	homeTab := ui.NavActiveStyle.Render(" Home ")
	settingsTab := ui.NavInactiveStyle.Render(" Settings ")
	hints := ui.DimStyle.Render("  [Tab] Focus  [Space] Play  [←→] Seek  [↑↓] Vol")
	logo := ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le")
	return lipgloss.JoinHorizontal(lipgloss.Left, logo, "  ", homeTab, " ", settingsTab, "  ", hints)
}

func (m *HomeModel) viewSidebar(bodyH int) string {
	title := ui.AccentStyle.Bold(true).Render("  " + langT("MUSIC DOWNLOAD", "MÜZİK İNDİR"))
	focusBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#1DB954")).
		Padding(0, 1)

	spotifyV := m.spotifyInput.View()
	if m.focusIdx == 0 {
		spotifyV = focusBorder.Render(m.spotifyInput.View())
	} else {
		if m.spotifyInput.Value() == "" {
			spotifyV = ui.DimStyle.Render(m.spotifyInput.Placeholder)
		} else {
			spotifyV = ui.DimStyle.Render(m.spotifyInput.Value())
		}
	}
	youtubeV := m.youtubeInput.View()
	if m.focusIdx == 1 {
		youtubeV = focusBorder.Render(m.youtubeInput.View())
	} else {
		if m.youtubeInput.Value() == "" {
			youtubeV = ui.DimStyle.Render(m.youtubeInput.Placeholder)
		} else {
			youtubeV = ui.DimStyle.Render(m.youtubeInput.Value())
		}
	}
	localBtn := ui.ButtonStyle.Render(langT("  + Add Local Music  ", "  + Yerel Müzik Ekle  "))
	if m.focusIdx == 3 {
		localBtn = ui.AccentBorderStyle.Render(langT("  + Add Local Music  ", "  + Yerel Müzik Ekle  "))
	}
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
	content := lipgloss.JoinVertical(lipgloss.Left, title, "", spotifyV, "", youtubeV, "", localBtn, "", playlistV, "", errText, "", dlBtn, "", "", "", "", "", "", "")
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
	return ui.BorderStyle.Width(w).Render(content)
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

func (m *HomeModel) viewContent(h int) string {
	plInfo := m.viewPlaylistInfo()
	plH := lipgloss.Height(plInfo)
	tableH := h - plH - 1
	if tableH < 3 {
		tableH = 3
	}
	m.songTable.SetHeight(tableH)
	m.songTable.SetWidth(m.width - 68)
	return lipgloss.JoinHorizontal(lipgloss.Top, plInfo, m.songTable.View())
}

func (m *HomeModel) viewPlaylistInfo() string {
	pl := state.Current.CurrentPlaylist
	if pl == nil {
		return ui.BorderStyle.Width(30).Render(ui.DimStyle.Render("\n  No playlist selected"))
	}
	name := ui.WhiteStyle.Bold(true).Render("  " + pl.Name)
	bio := ui.DimStyle.Render("  " + pl.Bio)
	count := ui.AccentStyle.Render(fmt.Sprintf("  %d songs", len(pl.Songs)))
	content := lipgloss.JoinVertical(lipgloss.Left, "", name, "", bio, "", count, "", "", ui.DimStyle.Render("  ♪ Play    ⬇ Download"), "", "", "")
	return ui.BorderStyle.Width(30).Render(content)
}

func (m *HomeModel) viewPlayerBar(w int) string {
	ps := state.Current.Player
	title := ui.DimStyle.Render("No track playing")
	artist := ""
	posStr := "00:00"
	durStr := "00:00"
	progress := ui.ProgressBar(0, 1, 28)
	if ps.CurrentSong != nil {
		t := ps.CurrentSong.Title
		if len(t) > 28 {
			t = t[:26] + "…"
		}
		title = ui.WhiteStyle.Bold(true).Render(t)
		a := ps.CurrentSong.Artist
		if len(a) > 28 {
			a = a[:26] + "…"
		}
		artist = "  " + ui.DimStyle.Render(a)
		posStr = ui.FormatDuration(ps.Position)
		durStr = ui.FormatDuration(ps.Duration)
		progress = ui.ProgressBar(ps.Position, ps.Duration, 28)
	}
	statusIcon := "▶"
	if ps.IsPaused {
		statusIcon = "⏸"
	} else if !ps.IsPlaying {
		statusIcon = "⏹"
	}
	volColor := ui.ColorAccent
	if ps.Volume > 0.66 {
		volColor = ui.ColorError
	} else if ps.Volume > 0.33 {
		volColor = ui.ColorOrange
	}
	volStr := lipgloss.NewStyle().Foreground(volColor).Render(ui.VolumeBar(ps.Volume, 8))
	line1 := fmt.Sprintf("  %s  %s%s", statusIcon, title, artist)
	line2 := fmt.Sprintf("  %s  %s  %s / %s   🔊 %s", ui.DimStyle.Render(posStr), ui.AccentStyle.Render(progress), ui.DimStyle.Render(posStr), ui.DimStyle.Render(durStr), volStr)
	if ps.StatusMsg != "" {
		c := ui.AccentStyle
		if ps.IsError {
			c = ui.ErrorStyle
		}
		line1 = "  " + c.Render(ps.StatusMsg)
		line2 = ""
	}
	bar := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
	return ui.BorderStyle.Width(w).Padding(0, 1).Render(bar)
}
