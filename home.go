// MusicLe CLI - Terminal Music Player
package main

import (
	"fmt"
	"strings"
	"time"

	"musicle-cli/bridge"
	"musicle-cli/state"
	"musicle-cli/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HomePage is the main dashboard
type HomePage struct {
	app   *tview.Application
	pages *tview.Pages
	root  *tview.Flex

	// Header
	headerView *tview.TextView

	// Sidebar
	sidebarTitle     *tview.TextView
	spotifyInput     *tview.InputField
	youtubeInput     *tview.InputField
	localBtn         *tview.Button
	playlistDropdown *tview.DropDown
	sidebarError     *tview.TextView

	// Content — playlist panel
	contentPlaylistDrop *tview.DropDown
	playlistArtView     *tview.TextView
	playlistInfoView    *tview.TextView
	controlBar          *tview.TextView

	// Content — song table
	songTable *tview.Table

	// Player bar
	playerBar *tview.TextView

	// Focus rotation
	focusTargets []tview.Primitive
	focusIdx     int

	// Player polling
	stopPoll chan struct{}

	// Key repeat limiting
	lastKeyTime time.Time
}

// NewHomePage creates the main dashboard
func NewHomePage(app *tview.Application, pages *tview.Pages) *HomePage {
	h := &HomePage{
		app:      app,
		pages:    pages,
		stopPoll: make(chan struct{}),
	}
	h.build()
	h.startPlayerPoll()
	return h
}

// Root returns the tview.Primitive to embed
func (h *HomePage) Root() tview.Primitive { return h.root }

// ── Build ─────────────────────────────────────────────────────────────────────

func (h *HomePage) build() {
	// ── Header ──
	h.headerView = tview.NewTextView()
	h.headerView.SetDynamicColors(true)
	h.headerView.SetBackgroundColor(tcell.NewRGBColor(18, 18, 18))
	h.headerView.SetText(h.buildHeaderText(false))

	// ── Sidebar ──
	sidebar := h.buildSidebar()

	// ── Content ──
	content := h.buildContent()

	// ── Player Bar ──
	h.playerBar = tview.NewTextView()
	h.playerBar.SetDynamicColors(true)
	h.playerBar.SetBackgroundColor(tcell.NewRGBColor(18, 18, 18))
	h.playerBar.SetBorder(true)
	h.playerBar.SetBorderColor(ui.ColorBorder)
	h.refreshPlayerBar()

	// ── Body (sidebar + content) ──
	body := tview.NewFlex()
	body.SetBackgroundColor(ui.ColorBackground)
	body.AddItem(sidebar, 0, 1, true)
	body.AddItem(content, 0, 3, false)

	// ── Root ──
	h.root = tview.NewFlex().SetDirection(tview.FlexRow)
	h.root.SetBackgroundColor(ui.ColorBackground)
	h.root.AddItem(h.headerView, 1, 0, false)
	h.root.AddItem(body, 0, 1, true)
	h.root.AddItem(h.playerBar, 4, 0, false)

	// Focus order: sidebar inputs → playlist drop → song table → player bar
	h.focusTargets = []tview.Primitive{
		h.spotifyInput, h.youtubeInput, h.localBtn,
		h.playlistDropdown, h.contentPlaylistDrop, h.songTable,
	}

	// Global keyboard handler for home page
	h.root.SetInputCapture(h.handleKeys)
}

func (h *HomePage) buildHeaderText(settingsActive bool) string {
	homeStyle := "[#1DB954::r] Home [-::-]"
	settingsStyle := "[#B3B3B3] [Settings] [-]"
	if settingsActive {
		homeStyle = "[#B3B3B3] [Home] [-]"
		settingsStyle = "[#1DB954::r] Settings [-::-]"
	}
	return "[white::b]Music[#1DB954::b]Le[-::-]    " + homeStyle + "  " + settingsStyle + "   [#B3B3B3][Tab] Focus  [Space] Play  [←→] Seek  [↑↓] Vol[-]"
}

// ── Sidebar ───────────────────────────────────────────────────────────────────

func (h *HomePage) buildSidebar() *tview.Flex {
	langT := func(en, tr string) string { return state.T(state.Current.Language, en, tr) }

	h.sidebarTitle = tview.NewTextView()
	h.sidebarTitle.SetDynamicColors(true)
	h.sidebarTitle.SetBackgroundColor(ui.ColorBackground)
	h.sidebarTitle.SetText("  [#1DB954::b]" + langT("MUSIC DOWNLOAD", "MÜZİK İNDİR") + "[-::-]")

	h.spotifyInput = makeInput("  Spotify URL:  ", "https://open.spotify.com/...")
	h.youtubeInput = makeInput("  YouTube URL:  ", "https://youtube.com/...")

	h.localBtn = tview.NewButton(langT("  + Add Local Music  ", "  + Yerel Müzik Ekle  "))
	h.localBtn.SetBackgroundColor(tcell.NewRGBColor(40, 40, 40))
	h.localBtn.SetLabelColor(ui.ColorAccent)
	h.localBtn.SetActivatedStyle(tcell.StyleDefault.Background(ui.ColorAccent).Foreground(tcell.ColorBlack))
	h.localBtn.SetSelectedFunc(func() { h.openLocalFileDialog() })

	h.playlistDropdown = tview.NewDropDown()
	h.playlistDropdown.SetLabel("  " + langT("Playlist", "Playlist") + ":  ")
	h.playlistDropdown.SetLabelColor(ui.ColorAccent)
	h.playlistDropdown.SetFieldBackgroundColor(tcell.NewRGBColor(40, 40, 40))
	h.playlistDropdown.SetFieldTextColor(ui.ColorPrimary)
	h.playlistDropdown.SetPrefixTextColor(ui.ColorAccent)
	h.playlistDropdown.SetBackgroundColor(ui.ColorBackground)
	h.refreshPlaylistDropdown()

	h.sidebarError = tview.NewTextView()
	h.sidebarError.SetDynamicColors(true)
	h.sidebarError.SetBackgroundColor(ui.ColorBackground)

	dlBtn := tview.NewButton(langT("  ⬇ Download  ", "  ⬇ İndir  "))
	dlBtn.SetBackgroundColor(ui.ColorAccent)
	dlBtn.SetLabelColor(tcell.ColorBlack)
	dlBtn.SetActivatedStyle(tcell.StyleDefault.Background(tcell.NewRGBColor(29, 185, 84)).Foreground(tcell.ColorBlack).Bold(true))
	dlBtn.SetSelectedFunc(func() { h.startDownload() })

	sidebar := tview.NewFlex().SetDirection(tview.FlexRow)
	sidebar.SetBackgroundColor(ui.ColorBackground)
	sidebar.SetBorder(true)
	sidebar.SetBorderColor(ui.ColorBorder)
	sidebar.AddItem(h.sidebarTitle, 1, 0, false)
	sidebar.AddItem(h.spotifyInput, 1, 0, true)
	sidebar.AddItem(h.youtubeInput, 1, 0, false)
	sidebar.AddItem(h.localBtn, 1, 0, false)
	sidebar.AddItem(h.playlistDropdown, 1, 0, false)
	sidebar.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 1, 0, false)
	sidebar.AddItem(h.playlistDropdown, 1, 0, false)
	sidebar.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 1, 0, false)
	sidebar.AddItem(h.sidebarError, 1, 0, false)
	sidebar.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 0, 1, false)
	sidebar.AddItem(dlBtn, 1, 0, false)
	sidebar.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 1, 0, false)

	return sidebar
}

// ── Content Area ──────────────────────────────────────────────────────────────

func (h *HomePage) buildContent() *tview.Flex {
	// Left: playlist info panel
	playlistPanel := h.buildPlaylistPanel()

	// Right: song table
	h.songTable = tview.NewTable()
	h.songTable.SetBackgroundColor(ui.ColorBackground)
	h.songTable.SetBorder(false)
	h.songTable.SetSelectable(true, false)
	h.songTable.SetSelectedStyle(tcell.StyleDefault.
		Background(tcell.NewRGBColor(30, 50, 35)).
		Foreground(tcell.ColorWhite).Bold(true))
	h.songTable.SetFixed(1, 0)
	h.buildSongTable()

	// Refresh playlist art after everything is built
	if state.Current.CurrentPlaylist != nil {
		h.refreshPlaylistArt()
	}

	h.songTable.SetSelectedFunc(func(row, _ int) {
		if row == 0 || state.Current.CurrentPlaylist == nil {
			return
		}
		idx := row - 1
		if idx < len(state.Current.CurrentPlaylist.Songs) {
			h.playSong(&state.Current.CurrentPlaylist.Songs[idx])
		}
	})

	songBox := tview.NewFlex().SetDirection(tview.FlexRow)
	songBox.SetBackgroundColor(ui.ColorBackground)
	songBox.SetBorder(true)
	songBox.SetBorderColor(ui.ColorBorder)
	songBox.AddItem(h.songTable, 0, 1, true)

	content := tview.NewFlex()
	content.SetBackgroundColor(ui.ColorBackground)
	content.AddItem(playlistPanel, 0, 1, false)
	content.AddItem(songBox, 0, 4, true)

	return content
}

func (h *HomePage) buildPlaylistPanel() *tview.Flex {
	h.contentPlaylistDrop = tview.NewDropDown()
	h.contentPlaylistDrop.SetLabel("  ")
	h.contentPlaylistDrop.SetLabelColor(ui.ColorAccent)
	h.contentPlaylistDrop.SetFieldBackgroundColor(tcell.NewRGBColor(40, 40, 40))
	h.contentPlaylistDrop.SetFieldTextColor(ui.ColorPrimary)
	h.contentPlaylistDrop.SetPrefixTextColor(ui.ColorAccent)
	h.contentPlaylistDrop.SetBackgroundColor(ui.ColorBackground)

	h.playlistArtView = tview.NewTextView()
	h.playlistArtView.SetDynamicColors(true)
	h.playlistArtView.SetBackgroundColor(ui.ColorBackground)
	h.playlistArtView.SetTextAlign(tview.AlignCenter)

	h.playlistInfoView = tview.NewTextView()
	h.playlistInfoView.SetDynamicColors(true)
	h.playlistInfoView.SetBackgroundColor(ui.ColorBackground)

	h.controlBar = tview.NewTextView()
	h.controlBar.SetDynamicColors(true)
	h.controlBar.SetBackgroundColor(ui.ColorBackground)

	// Initialize dropdown and refresh art after components are created
	h.refreshContentPlaylistDrop()
	if state.Current.CurrentPlaylist != nil {
		h.refreshPlaylistArt()
		h.refreshPlaylistInfo()
	}
	h.controlBar.SetText(
		"\n[#1DB954]🔒[-] [#B3B3B3]Encrypt[-]  [#1DB954]🔀[-] [#B3B3B3]Shuffle[-]\n[#1DB954]▶[-] [#B3B3B3]Play[-]    [#1DB954]⬇[-] [#B3B3B3]Download[-]",
	)

	panel := tview.NewFlex().SetDirection(tview.FlexRow)
	panel.SetBackgroundColor(ui.ColorBackground)
	panel.SetBorder(true)
	panel.SetBorderColor(ui.ColorBorder)
	panel.AddItem(h.contentPlaylistDrop, 1, 0, false)
	panel.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 1, 0, false)
	panel.AddItem(h.playlistArtView, 10, 0, false)
	panel.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 1, 0, false)
	panel.AddItem(h.playlistInfoView, 4, 0, false)
	panel.AddItem(h.controlBar, 4, 0, false)
	panel.AddItem(tview.NewBox().SetBackgroundColor(ui.ColorBackground), 0, 1, false)

	return panel
}

// ── Song Table ────────────────────────────────────────────────────────────────

func (h *HomePage) buildSongTable() {
	if h.songTable == nil {
		return
	}
	t := h.songTable
	t.Clear()

	// Header row
	headers := []string{"#", "Art", "Title / Artist", "Date Added", "Duration"}
	aligns := []int{tview.AlignRight, tview.AlignCenter, tview.AlignLeft, tview.AlignCenter, tview.AlignRight}
	for col, hdr := range headers {
		cell := tview.NewTableCell(" " + hdr + " ").
			SetTextColor(ui.ColorSecondary).
			SetAlign(aligns[col]).
			SetSelectable(false).
			SetAttributes(tcell.AttrBold)
		cell.SetBackgroundColor(tcell.NewRGBColor(24, 24, 24))
		t.SetCell(0, col, cell)
	}

	pl := state.Current.CurrentPlaylist
	if pl == nil {
		t.SetCell(1, 2, tview.NewTableCell("  No playlist selected").SetTextColor(ui.ColorSecondary))
		return
	}
	if len(pl.Songs) == 0 {
		langT := func(en, tr string) string { return state.T(state.Current.Language, en, tr) }
		t.SetCell(1, 2, tview.NewTableCell("  "+langT("No songs yet — download something!", "Henüz şarkı yok — bir şey indirin!")).SetTextColor(ui.ColorSecondary))
		return
	}

	for i, song := range pl.Songs {
		row := i + 1
		isPlaying := state.Current.Player.CurrentSong != nil &&
			state.Current.Player.CurrentSong.Filename == song.Filename

		numColor := ui.ColorSecondary
		titleColor := ui.ColorPrimary
		if isPlaying {
			numColor = ui.ColorAccent
			titleColor = ui.ColorAccent
		}

		t.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf(" %d ", row)).
			SetTextColor(numColor).SetAlign(tview.AlignRight))
		t.SetCell(row, 1, tview.NewTableCell(" 🎵 ").
			SetTextColor(ui.ColorAccent).SetAlign(tview.AlignCenter))
		title := song.Title
		if len(title) > 30 {
			title = title[:28] + "…"
		}
		artist := song.Artist
		if len(artist) > 30 {
			artist = artist[:28] + "…"
		}
		t.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf(" %s\n [#B3B3B3]%s[-]", title, artist)).
			SetTextColor(titleColor).SetAlign(tview.AlignLeft))
		t.SetCell(row, 3, tview.NewTableCell(" "+song.DateAdded+" ").
			SetTextColor(ui.ColorSecondary).SetAlign(tview.AlignCenter))
		t.SetCell(row, 4, tview.NewTableCell(" "+song.Duration+" ").
			SetTextColor(ui.ColorSecondary).SetAlign(tview.AlignRight))
	}
}

// ── Player Bar ────────────────────────────────────────────────────────────────

func (h *HomePage) refreshPlayerBar() {
	ps := state.Current.Player

	title := "[#B3B3B3]No track playing[-]"
	artist := ""
	posStr := "00:00"
	durStr := "00:00"
	progress := ui.ProgressBar(0, 1, 28)

	if ps.CurrentSong != nil {
		t := ps.CurrentSong.Title
		if len(t) > 28 {
			t = t[:26] + "…"
		}
		title = "[white::b]" + t + "[-::-]"
		a := ps.CurrentSong.Artist
		if len(a) > 28 {
			a = a[:26] + "…"
		}
		artist = "[#B3B3B3]" + a + "[-]"
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

	volColor := ui.TagAccent
	if ps.Volume > 0.66 {
		volColor = ui.TagError
	} else if ps.Volume > 0.33 {
		volColor = ui.TagOrange
	}
	volBar := ui.VolumeBar(ps.Volume, 8)

	// Compose player bar text
	line1 := fmt.Sprintf("  🎵  %s  %s", title, artist)
	line2 := fmt.Sprintf(
		"  [#B3B3B3]%s[-]  [#1DB954]%s[-]  [#B3B3B3]%s[-] / [#B3B3B3]%s[-]   🔊 %s%s[-]",
		statusIcon, progress, posStr, durStr, volColor, volBar,
	)

	if ps.StatusMsg != "" {
		color := ui.TagAccent
		if ps.IsError {
			color = ui.TagError
		}
		line1 = "  " + color + ps.StatusMsg + ui.TagReset
		line2 = ""
	}

	h.playerBar.SetText(line1 + "\n" + line2)
}

// ── Player polling goroutine ─────────────────────────────────────────────────

func (h *HomePage) startPlayerPoll() {
	go func() {
		ticker := time.NewTicker(2 * time.Second) // Reduced from 1s to 2s
		defer ticker.Stop()
		for {
			select {
			case <-h.stopPoll:
				return
			case <-ticker.C:
				result, err := bridge.PlayerCall(bridge.Action{Action: "status"})
				if err != nil {
					continue
				}
				wasPlaying := state.Current.Player.IsPlaying
				oldPosition := state.Current.Player.Position
				state.Current.Player.Position = result.Position
				state.Current.Player.Duration = result.Duration
				if result.Status == "playing" {
					state.Current.Player.IsPlaying = true
					state.Current.Player.IsPaused = false
				} else if result.Status == "paused" {
					state.Current.Player.IsPlaying = false
					state.Current.Player.IsPaused = true
				} else if result.Status == "stopped" || result.Status == "idle" {
					if wasPlaying {
						// Song ended — advance to next
						h.playNextSong()
					}
					state.Current.Player.IsPlaying = false
					state.Current.Player.IsPaused = false
				}

				// Only update UI if something meaningful changed
				needsUIUpdate := wasPlaying != state.Current.Player.IsPlaying ||
					oldPosition != state.Current.Player.Position ||
					result.Status == "stopped"

				if needsUIUpdate {
					h.app.QueueUpdateDraw(func() {
						h.refreshPlayerBar()
						// Only rebuild song table when play state changes, not position
						if wasPlaying != state.Current.Player.IsPlaying {
							h.buildSongTable()
						}
					})
				}
			}
		}
	}()
}

// ── Playback controls ─────────────────────────────────────────────────────────

func (h *HomePage) playSong(song *state.Song) {
	state.Current.Player.CurrentSong = song
	state.Current.Player.IsPlaying = true
	state.Current.Player.IsPaused = false
	state.Current.Player.StatusMsg = ""

	go func() {
		result, err := bridge.PlayerCall(bridge.Action{Action: "play", File: song.FilePath})
		h.app.QueueUpdateDraw(func() {
			if err != nil || result.Status == "error" {
				msg := "Engine error"
				if result != nil {
					msg = result.Error
				}
				state.Current.Player.StatusMsg = "⚠ " + msg
				state.Current.Player.IsError = true
			} else {
				state.Current.Player.Duration = result.Duration
			}
			h.refreshPlayerBar()
			h.buildSongTable()
		})
	}()
}

func (h *HomePage) playNextSong() {
	pl := state.Current.CurrentPlaylist
	if pl == nil || len(pl.Songs) == 0 {
		return
	}
	cur := state.Current.Player.CurrentSong
	if cur == nil {
		h.playSong(&pl.Songs[0])
		return
	}
	if state.Current.Player.IsShuffled {
		// Simple shuffle: pick any song except current
		for i, s := range pl.Songs {
			if s.Filename != cur.Filename {
				h.playSong(&pl.Songs[i])
				return
			}
		}
		return
	}
	for i, s := range pl.Songs {
		if s.Filename == cur.Filename && i+1 < len(pl.Songs) {
			h.playSong(&pl.Songs[i+1])
			return
		}
	}
}

// ── Key handler ───────────────────────────────────────────────────────────────

var lastKeyTime time.Time
var lastKey rune = 0

func (h *HomePage) handleKeys(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyCtrlC {
		return nil
	}

	now := time.Now()

	if event.Rune() == lastKey {
		var timeout time.Duration
		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyDelete || event.Key() == tcell.KeyBackspace2 {
			timeout = 100 * time.Millisecond
		} else {
			timeout = 760 * time.Millisecond
		}

		if now.Sub(lastKeyTime) < timeout {
			return nil
		}
	}

	if event.Rune() == 0 && now.Sub(lastKeyTime) < 50*time.Millisecond {
		return nil
	}

	lastKeyTime = now
	lastKey = event.Rune()

	switch event.Key() {
	case tcell.KeyTab:
		h.cycleFocus(1)
		return nil
	case tcell.KeyBacktab:
		h.cycleFocus(-1)
		return nil
	case tcell.KeyEnter:
		// Same as Tab in home context
		h.cycleFocus(1)
		return nil
	case tcell.KeyRight:
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: 5})
		return nil
	case tcell.KeyLeft:
		go bridge.PlayerCall(bridge.Action{Action: "seek", Value: -5})
		return nil
	case tcell.KeyUp:
		v := state.Current.Player.Volume + 0.05
		if v > 1 {
			v = 1
		}
		state.Current.Player.Volume = v
		go bridge.PlayerCall(bridge.Action{Action: "volume", Value: v})
		return nil
	case tcell.KeyDown:
		v := state.Current.Player.Volume - 0.05
		if v < 0 {
			v = 0
		}
		state.Current.Player.Volume = v
		go bridge.PlayerCall(bridge.Action{Action: "volume", Value: v})
		return nil
	case tcell.KeyCtrlV:
		// Ctrl+V - Paste from clipboard (geçici olarak devre dışı)
		// TODO: Windows clipboard API entegrasyonu
		return nil
	case tcell.KeyCtrlC:
		// Ctrl+C - Copy from current input (geçici olarak devre dışı)
		// TODO: Windows clipboard API entegrasyonu
		return nil
	}

	// 3. Sadece özel tuşları yakala, diğerlerini input'a bırak
	switch event.Rune() {
	case ' ':
		h.togglePlayPause()
		return nil
		// 's' tuşu kaldırıldı - settings F2 ile açılacak
	}

	// 4. Diğer tüm tuşları input alanlarına bırak
	return event
}

func (h *HomePage) cycleFocus(dir int) {
	n := len(h.focusTargets)
	if n == 0 {
		return
	}
	h.focusIdx = (h.focusIdx + dir + n) % n
	h.app.SetFocus(h.focusTargets[h.focusIdx])
}

func (h *HomePage) togglePlayPause() {
	ps := &state.Current.Player
	if ps.CurrentSong == nil {
		// Start first song if playlist has songs
		if state.Current.CurrentPlaylist != nil && len(state.Current.CurrentPlaylist.Songs) > 0 {
			h.playSong(&state.Current.CurrentPlaylist.Songs[0])
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
	h.app.QueueUpdateDraw(func() { h.refreshPlayerBar() })
}

// ── Download ──────────────────────────────────────────────────────────────────

func (h *HomePage) startDownload() {
	langT := func(en, tr string) string { return state.T(state.Current.Language, en, tr) }

	spotifyURL := strings.TrimSpace(h.spotifyInput.GetText())
	youtubeURL := strings.TrimSpace(h.youtubeInput.GetText())

	url := spotifyURL
	action := "download_spotify"
	if url == "" {
		url = youtubeURL
		action = "download_youtube"
	}
	if url == "" {
		h.setSidebarError(langT("Enter a URL first", "Önce bir URL girin"))
		return
	}
	if !strings.HasPrefix(url, "http") {
		h.setSidebarError(langT("Invalid URL — Hatalı Link", "Hatalı Link"))
		return
	}

	idx, _ := h.playlistDropdown.GetCurrentOption()
	outDir := ""
	if state.Current.CurrentProfile != nil && idx >= 0 && idx < len(state.Current.CurrentProfile.Playlists) {
		pl := state.Current.CurrentProfile.Playlists[idx]
		outDir = state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, pl.FolderName)
	}

	h.setSidebarError(langT("Downloading…", "İndiriliyor…"))
	h.sidebarError.SetTextColor(ui.ColorAccent)

	go func() {
		result, err := bridge.RunScript(bridge.Action{Action: action, URL: url, Output: outDir})
		h.app.QueueUpdateDraw(func() {
			if err != nil || result.Status == "error" {
				msg := result.Error
				if msg == "" {
					msg = err.Error()
				}
				h.setSidebarError("✗ " + msg)
				return
			}
			h.sidebarError.SetTextColor(ui.ColorAccent)
			h.sidebarError.SetText(langT("✓ Downloaded: ", "✓ İndirildi: ") + result.Filename)
			// Reload playlist
			h.refreshAllContent()
		})
	}()
}

func (h *HomePage) setSidebarError(msg string) {
	h.sidebarError.SetTextColor(ui.ColorError)
	h.sidebarError.SetText("  " + msg)
}

// ── Local file import ────────────────────────────────────────────────────────

func (h *HomePage) openLocalFileDialog() {
	langT := func(en, tr string) string { return state.T(state.Current.Language, en, tr) }
	const dialogName = "localFileDialog"

	input := makeInput("  File path:  ", "/path/to/song.mp3")
	errView := makeErrorView()
	hint := hintBar("  [Enter] Add  |  [Esc] Cancel")

	inner := tview.NewFlex().SetDirection(tview.FlexRow)
	inner.SetBackgroundColor(tcell.NewRGBColor(28, 28, 28))
	inner.SetBorder(true)
	inner.SetBorderColor(ui.ColorAccent)
	inner.SetTitle(" " + langT("Add Local Music", "Yerel Müzik Ekle") + " ")
	inner.SetTitleColor(ui.ColorPrimary)
	inner.AddItem(tview.NewBox().SetBackgroundColor(tcell.NewRGBColor(28, 28, 28)), 1, 0, false)
	inner.AddItem(input, 1, 0, true)
	inner.AddItem(errView, 1, 0, false)
	inner.AddItem(tview.NewBox().SetBackgroundColor(tcell.NewRGBColor(28, 28, 28)), 0, 1, false)
	inner.AddItem(hint, 1, 0, false)

	dialog := centeredFlex(inner, 70, 10)
	dialog.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			path := strings.TrimSpace(input.GetText())
			if path == "" {
				errView.SetText("[#FF4444]  Path required[-]")
				return nil
			}
			h.pages.RemovePage(dialogName)
			h.importLocalFile(path)
		case tcell.KeyEsc:
			h.pages.RemovePage(dialogName)
		}
		return event
	})

	h.pages.AddPage(dialogName, dialog, true, true)
	h.app.SetFocus(input)
}

func (h *HomePage) importLocalFile(filePath string) {
	if state.Current.CurrentProfile == nil || state.Current.CurrentPlaylist == nil {
		return
	}
	go func() {
		result, err := bridge.RunScript(bridge.Action{
			Action: "add_local",
			File:   filePath,
			Output: state.Current.PlaylistDir(
				state.Current.CurrentProfile.FolderName,
				state.Current.CurrentPlaylist.FolderName,
			),
		})
		h.app.QueueUpdateDraw(func() {
			if err != nil || result.Status == "error" {
				state.Current.Player.StatusMsg = "⚠ File not found / Dosya bulunamadı"
				state.Current.Player.IsError = true
				h.refreshPlayerBar()
				return
			}
			h.refreshAllContent()
		})
	}()
}

// ── Refresh helpers ───────────────────────────────────────────────────────────

func (h *HomePage) refreshPlaylistDropdown() {
	var opts []string
	if state.Current.CurrentProfile != nil {
		for _, pl := range state.Current.CurrentProfile.Playlists {
			opts = append(opts, pl.Name)
		}
	}
	if len(opts) == 0 {
		opts = []string{"(no playlists)"}
	}
	h.playlistDropdown.SetOptions(opts, nil)
	h.playlistDropdown.SetCurrentOption(0)
}

func (h *HomePage) refreshContentPlaylistDrop() {
	var opts []string
	if state.Current.CurrentProfile != nil {
		for _, pl := range state.Current.CurrentProfile.Playlists {
			opts = append(opts, pl.Name)
		}
	}
	if len(opts) == 0 {
		opts = []string{"(no playlists)"}
	}
	h.contentPlaylistDrop.SetOptions(opts, func(text string, idx int) {
		if state.Current.CurrentProfile != nil && idx < len(state.Current.CurrentProfile.Playlists) {
			state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[idx]
			if state.Current.CurrentPlaylist != nil {
				h.refreshPlaylistArt()
				h.refreshPlaylistInfo()
				h.buildSongTable()
			}
		}
	})
	if len(opts) > 0 {
		h.contentPlaylistDrop.SetCurrentOption(0)
	}
}

func (h *HomePage) refreshPlaylistArt() {
	if state.Current.CurrentPlaylist == nil {
		h.playlistArtView.SetText("[#B3B3B3]\n\n    ╔════════════╗\n    ║            ║\n    ║   [#1DB954]♫♫♫[-]       [#B3B3B3]║\n    ║            ║\n    ╚════════════╝[-]")
		return
	}
	art := "[#B3B3B3]\n\n    ╔════════════╗\n    ║            ║\n    ║   [#1DB954]♫♫♫[-]       [#B3B3B3]║\n    ║            ║\n    ╚════════════╝[-]"
	if state.Current.CurrentPlaylist.ArtPath != "" {
		art = "[#B3B3B3]\n  (playlist art loaded)\n  " + state.Current.CurrentPlaylist.ArtPath + "[-]"
	}
	h.playlistArtView.SetText(art)
}

func (h *HomePage) refreshPlaylistInfo() {
	pl := state.Current.CurrentPlaylist
	if pl == nil {
		h.playlistInfoView.SetText("  [#B3B3B3]No playlist selected[-]")
		return
	}
	name := pl.Name
	bio := pl.Bio
	cnt := len(pl.Songs)
	h.playlistInfoView.SetText(fmt.Sprintf(
		"\n  [white::b]%s[-::-]\n  [#B3B3B3]%s[-]\n  [#1DB954]%d songs[-]",
		name, bio, cnt,
	))
}

func (h *HomePage) openSettings() {
	h.pages.SwitchToPage("settings")
}

func (h *HomePage) refreshAllContent() {
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
	h.refreshPlaylistDropdown()
	h.refreshContentPlaylistDrop()
	h.refreshPlaylistArt()
	h.refreshPlaylistInfo()
	h.buildSongTable()
}
