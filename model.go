package main

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/bridge"
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
	ViewSettings
)

type MainModel struct {
	view     ViewType
	width    int
	height   int
	ready    bool

	home     *HomeModel
	settings *SettingsModel

	activeNav     string
	showLangModal bool
	lang          state.Language
}

func NewMainModel() *MainModel {
	m := &MainModel{
		view:          ViewHome,
		activeNav:     "home",
		home:          NewHomeModel(),
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
			if m.view == ViewHome && m.home != nil {
				return m, m.home.CycleSection()
			}
		case msg.Type == tea.KeyF2:
			switch m.view {
			case ViewHome:
				m.view = ViewSettings
				m.activeNav = "settings"
			case ViewSettings:
				m.view = ViewHome
				m.activeNav = "home"
			}
			return m, nil
		case msg.Type == tea.KeyF12:
			if m.view == ViewHome && m.home != nil {
				return m, m.home.FocusConsole()
			}
		case msg.Type == tea.KeyEscape:
			if m.view == ViewSettings {
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
	case ViewSettings:
		if m.settings != nil {
			newSettings, cmd := m.settings.Update(msg)
			m.settings = newSettings.(*SettingsModel)
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

func (m *MainModel) View() string {
	if !m.ready {
		return "Loading..."
	}
	content := ""
	switch m.view {
	case ViewHome:
		if m.home != nil {
			content = m.home.View()
		}
	case ViewSettings:
		if m.settings != nil {
			content = m.settings.View()
		}
	}
	if m.height > 0 {
		h := lipgloss.Height(content)
		if h < m.height {
			content += strings.Repeat("\n", m.height-h)
		}
	}

	if m.showLangModal {
		modal := renderLangModal(m.lang)
		content = placeOverlay(content, modal, m.width)
	}

	return content
}
