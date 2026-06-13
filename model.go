package main

import (
	"net"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/bridge"
	"musicle-cli/components"
	"musicle-cli/state"
)

type StartDownloadMsg struct {
	Action string
	URL    string
	Output string
}

type DownloadResultMsg struct {
	Result *bridge.Result
	Error  error
	Action string
}

type LocalFileImportMsg struct {
	FilePath string
	Output   string
}

type ImportResultMsg struct {
	Result *bridge.Result
	Error  error
}

type PlaySongMsg struct {
	FilePath string
}

type PlayerCmdMsg struct {
	Action string
	Value  float64
}

type setupDoneMsg struct{}
type errorMsg string

type ViewType int

const (
	ViewHome ViewType = iota
	ViewProfile
	ViewPlaylist
	ViewSettings
)

type MainModel struct {
	view     ViewType
	width    int
	height   int
	ready    bool

	home     *HomeModel
	profile  *ProfileModel
	playlist *PlaylistModel
	settings *SettingsModel

	activeNav       string
	playerBarFocused bool
	showLangModal   bool
	lang          state.Language
	lastNetCheck  time.Time
}

func NewMainModel() *MainModel {
	m := &MainModel{
		view:          ViewHome,
		activeNav:     "home",
		home:          NewHomeModel(),
		profile:       NewProfileModel(),
		playlist:      NewPlaylistModel(),
		settings:      NewSettingsModel(),
		ready:         false,
		showLangModal: state.Current.IsFirstLaunch,
	}
	return m
}

func (m *MainModel) Init() tea.Cmd {
	return tea.Batch(
		tea.HideCursor,
		m.home.Init(),
		m.settings.Init(),
		m.pollTicker(),
	)
}

func (m *MainModel) pollTicker() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		return PollTickMsg(t)
	})
}

type PollTickMsg time.Time
type PlayerStatusResult struct {
	Result *bridge.Result
	Error  error
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		if m.showLangModal {
			switch msg.String() {
			case "up", "k":
				if m.lang == state.LangEnglish {
					m.lang = state.LangTurkish
				} else {
					m.lang = state.LangEnglish
				}
			case "down", "j":
				if m.lang == state.LangEnglish {
					m.lang = state.LangTurkish
				} else {
					m.lang = state.LangEnglish
				}
			case "enter":
				return m, initializeDefaults(m.lang)
			}
			return m, nil
		}

		switch {
		case msg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case msg.Type == tea.KeyF1:
			if m.playerBarFocused {
				m.playerBarFocused = false
				switch m.view {
				case ViewHome:
					if m.home != nil { m.home.sectionFocus = 0 }
				case ViewProfile:
					if m.profile != nil { m.profile.focus = 0; m.profile.setFocus(0) }
				case ViewPlaylist:
					if m.playlist != nil { m.playlist.focus = 0; m.playlist.setFocus(0) }
				case ViewSettings:
					if m.settings != nil { m.settings.focus = 0 }
				}
				return m, nil
			}
			var cmd tea.Cmd
			wrapped := false
			switch m.view {
			case ViewHome:
				if m.home != nil {
					wrapped, cmd = m.home.CycleSection()
				}
			case ViewProfile:
				if m.profile != nil {
					wrapped = m.profile.cycleFocus()
				}
			case ViewPlaylist:
				if m.playlist != nil {
					wrapped = m.playlist.cycleFocus()
				}
			case ViewSettings:
				if m.settings != nil {
					wrapped = m.settings.cycleFocus()
				}
			}
			if wrapped {
				m.playerBarFocused = true
			}
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		case msg.Type == tea.KeyF2:
			m.view = (m.view + 1) % 4
			switch m.view {
			case ViewHome:
				m.activeNav = "home"
			case ViewProfile:
				m.activeNav = "profile"
			case ViewPlaylist:
				m.activeNav = "playlist"
			case ViewSettings:
				m.activeNav = "settings"
			}
			return m, nil
		case msg.Type == tea.KeyEscape:
			if m.view != ViewHome {
				m.view = ViewHome
				m.activeNav = "home"
				return m, nil
			}
		}

	case PollTickMsg:
		cmd := m.handlePlayerPoll()
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.pollTicker())
		m.maybeCheckNetwork()

	case StartDownloadMsg:
		cmds = append(cmds, m.handleDownload(msg))

	case DownloadResultMsg:
		if m.home != nil {
			cmds = append(cmds, m.home.OnDownloadResult(msg))
		}

	case LocalFileImportMsg:
		cmds = append(cmds, m.handleLocalImport(msg))

	case ImportResultMsg:
		if m.home != nil {
			cmds = append(cmds, m.home.OnImportResult(msg))
		}

	case setupDoneMsg:
		m.showLangModal = false

	case errorMsg:
		m.showLangModal = false
	}

	switch m.view {
	case ViewHome:
		if m.home != nil {
			newHome, cmd := m.home.Update(msg)
			m.home = newHome.(*HomeModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case ViewProfile:
		if m.profile != nil {
			newP, cmd := m.profile.Update(msg)
			m.profile = newP.(*ProfileModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case ViewPlaylist:
		if m.playlist != nil {
			newPl, cmd := m.playlist.Update(msg)
			m.playlist = newPl.(*PlaylistModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case ViewSettings:
		if m.settings != nil {
			newS, cmd := m.settings.Update(msg)
			m.settings = newS.(*SettingsModel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *MainModel) handlePlayerPoll() tea.Cmd {
	return func() tea.Msg {
		result, err := bridge.PlayerCall(bridge.Action{Action: "status"})
		return PlayerStatusResult{Result: result, Error: err}
	}
}

func (m *MainModel) handleDownload(msg StartDownloadMsg) tea.Cmd {
	return func() tea.Msg {
		result, err := bridge.RunScript(bridge.Action{
			Action: msg.Action,
			URL:    msg.URL,
			Output: msg.Output,
		})
		return DownloadResultMsg{Result: result, Error: err, Action: msg.Action}
	}
}

func (m *MainModel) handleLocalImport(msg LocalFileImportMsg) tea.Cmd {
	return func() tea.Msg {
		result, err := bridge.RunScript(bridge.Action{
			Action: "add_local",
			File:   msg.FilePath,
			Output: msg.Output,
		})
		return ImportResultMsg{Result: result, Error: err}
	}
}

func (m *MainModel) maybeCheckNetwork() {
	if time.Since(m.lastNetCheck) < 30*time.Second {
		return
	}
	m.lastNetCheck = time.Now()
	conn, err := net.DialTimeout("tcp", "google.com:80", 2*time.Second)
	if err == nil {
		conn.Close()
		state.Current.NetworkOnline = true
	} else {
		state.Current.NetworkOnline = false
	}
}

func (m *MainModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	header := components.RenderHeader(m.width, m.activeNav)
	playerBar := components.RenderPlayerBar(m.width, m.playerBarFocused)

	headerH := lipgloss.Height(header)
	barH := lipgloss.Height(playerBar)
	bodyH := m.height - headerH - barH
	if bodyH < 5 {
		bodyH = 5
	}

	body := ""
	switch m.view {
	case ViewHome:
		if m.home != nil {
			body = m.home.View()
		}
	case ViewProfile:
		if m.profile != nil {
			body = m.profile.View()
		}
	case ViewPlaylist:
		if m.playlist != nil {
			body = m.playlist.View()
		}
	case ViewSettings:
		if m.settings != nil {
			body = m.settings.View()
		}
	}
	bodyHActual := lipgloss.Height(body)
	if bodyHActual < bodyH {
		body += strings.Repeat("\n", bodyH-bodyHActual)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, header, body, playerBar)

	if m.showLangModal {
		modal := renderLangModal(m.lang)
		full = placeOverlay(full, modal, m.width)
	}

	return full
}
