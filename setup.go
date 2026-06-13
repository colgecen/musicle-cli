package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sqweek/dialog"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type SetupModel struct {
	step     int
	done     bool
	width    int
	height   int
	lang     state.Language
	rootDir  string
	profile  struct{ folder, avatar, name, bio string }
	playlist struct{ folder, art, name, bio string }
	err      string

	inputs []InputField
	focus  int
}

func NewSetupModel() *SetupModel {
	return &SetupModel{
		step: 1,
		lang: state.LangEnglish,
	}
}

func (m *SetupModel) Init() tea.Cmd { return nil }

func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.step {
		case 1:
			return m.updateStep1(msg)
		case 2:
			return m.updateStep2(msg)
		case 3:
			return m.updateStep3(msg)
		case 4:
			return m.updateStep4(msg)
		}
	}
	return m, nil
}

func (m *SetupModel) updateStep1(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.lang = state.LangTurkish
		case "down", "j":
			m.lang = state.LangEnglish
		case "enter":
			state.Current.Language = m.lang
			m.step = 2
			m.buildStep2Inputs()
		case "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *SetupModel) updateStep2(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down", "j":
		m.setFocus((m.focus + 1) % len(m.inputs))
	case "shift+tab", "up", "k":
		m.setFocus((m.focus - 1 + len(m.inputs)) % len(m.inputs))
	case "enter":
		val := strings.TrimSpace(m.inputs[0].Value())
		if val == "" {
			m.err = langT("Please select a directory", "Lütfen bir dizin seçin")
			return m, nil
		}
		_ = state.Current.InitializeBaseDirs(val)
		m.rootDir = val
		m.step = 3
		m.err = ""
		m.buildStep3Inputs()
	case "f2":
		m.dirBrowse()
	case "esc":
		m.step = 1
		m.err = ""
	default:
		if len(m.inputs) > 0 && m.focus < len(m.inputs) {
			var cmd tea.Cmd
			m.inputs[m.focus].Model, cmd = m.inputs[m.focus].Model.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *SetupModel) setFocus(idx int) {
	if len(m.inputs) == 0 {
		return
	}
	if idx >= 0 && idx < len(m.inputs) {
		if m.focus >= 0 && m.focus < len(m.inputs) {
			m.inputs[m.focus].Blur()
		}
		m.focus = idx
		m.inputs[m.focus].Focus()
	}
}

func (m *SetupModel) updateStep3(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down", "j":
		m.setFocus((m.focus + 1) % len(m.inputs))
	case "shift+tab", "up", "k":
		m.setFocus((m.focus - 1 + len(m.inputs)) % len(m.inputs))
	case "enter":
		folder := strings.TrimSpace(m.inputs[0].Value())
		if folder == "" {
			m.err = langT("Folder name required", "Klasör adı gerekli")
			return m, nil
		}
		m.profile.folder = folder
		m.profile.avatar = strings.TrimSpace(m.inputs[1].Value())
		m.profile.name = strings.TrimSpace(m.inputs[2].Value())
		if m.profile.name == "" {
			m.profile.name = folder
		}
		m.profile.bio = strings.TrimSpace(m.inputs[3].Value())
		m.step = 4
		m.err = ""
		m.buildStep4Inputs()
	case "esc":
		m.step = 2
		m.err = ""
	default:
		if len(m.inputs) > 0 && m.focus < len(m.inputs) {
			var cmd tea.Cmd
			m.inputs[m.focus].Model, cmd = m.inputs[m.focus].Model.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *SetupModel) updateStep4(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down", "j":
		m.setFocus((m.focus + 1) % len(m.inputs))
	case "shift+tab", "up", "k":
		m.setFocus((m.focus - 1 + len(m.inputs)) % len(m.inputs))
	case "enter":
		folder := strings.TrimSpace(m.inputs[0].Value())
		if folder == "" {
			m.err = langT("Folder name required", "Klasör adı gerekli")
			return m, nil
		}
		m.playlist.folder = folder
		m.playlist.art = strings.TrimSpace(m.inputs[1].Value())
		m.playlist.name = strings.TrimSpace(m.inputs[2].Value())
		if m.playlist.name == "" {
			m.playlist.name = folder
		}
		m.playlist.bio = strings.TrimSpace(m.inputs[3].Value())
		m.err = ""
		return m, m.finish()
	case "esc":
		m.step = 3
		m.err = ""
	default:
		if len(m.inputs) > 0 && m.focus < len(m.inputs) {
			var cmd tea.Cmd
			m.inputs[m.focus].Model, cmd = m.inputs[m.focus].Model.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *SetupModel) finish() tea.Cmd {
	return func() tea.Msg {
		state.Current.RootDir = m.rootDir
		state.Current.Language = m.lang

		if err := state.Current.CreateProfileStructure(
			m.profile.folder, m.profile.name, m.profile.bio, m.profile.avatar, m.lang,
		); err != nil {
			return errorMsg(err.Error())
		}

		if err := state.Current.CreatePlaylistStructure(
			m.profile.folder, m.playlist.folder, m.playlist.name, m.playlist.bio, m.playlist.art,
		); err != nil {
			return errorMsg(err.Error())
		}

		_ = state.Current.SaveConfig()
		_ = state.Current.ScanProfiles()
		if len(state.Current.Profiles) > 0 {
			state.Current.CurrentProfile = &state.Current.Profiles[0]
			if len(state.Current.CurrentProfile.Playlists) > 0 {
				state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[0]
			}
		}
		state.Current.IsFirstLaunch = false
		return setupDoneMsg{}
	}
}

func (m *SetupModel) dirBrowse() {
	go func() {
		selectedPath, err := dialog.Directory().Title(langT("Select Music Directory", "Müzik Dizini Seç")).Browse()
		if err == nil && selectedPath != "" {
			if len(m.inputs) > 0 {
				m.inputs[0].SetValue(selectedPath)
			}
		}
	}()
}

func (m *SetupModel) buildStep2Inputs() {
	placeholder := langT("e.g. C:\\Music", "Örn: C:\\Müzik")
	inp := NewInputField("  "+langT("Music Directory", "Müzik Dizini")+":  ", placeholder)
	inp.Width = 50
	m.inputs = []InputField{inp}
	m.focus = 0
	m.err = ""
	m.inputs[0].Focus()
}

func (m *SetupModel) buildStep3Inputs() {
	m.inputs = []InputField{
		NewInputField("  "+langT("Folder Name", "Klasör Adı")+":       ", "myprofile"),
		NewInputField("  "+langT("Profile Picture path", "Profil Fotoğrafı")+": ", langT("optional", "isteğe bağlı")),
		NewInputField("  "+langT("Display Name", "Görünen Ad")+":       ", "MusicLe User"),
		NewInputField("  "+langT("Bio", "Biyografi")+":              ", langT("Music lover", "Müzik sever")),
	}
	m.focus = 0
	m.err = ""
	m.inputs[0].Focus()
}

func (m *SetupModel) buildStep4Inputs() {
	m.inputs = []InputField{
		NewInputField("  "+langT("Folder Name", "Klasör Adı")+":        ", "my-playlist"),
		NewInputField("  "+langT("Playlist Art path", "Playlist Görseli")+":  ", langT("optional", "isteğe bağlı")),
		NewInputField("  "+langT("Playlist Name", "Playlist Adı")+":     ", langT("My Playlist", "Listem")),
		NewInputField("  "+langT("Description", "Açıklama")+":         ", langT("My favorite songs", "Favori şarkılarım")),
	}
	m.focus = 0
	m.err = ""
	m.inputs[0].Focus()
}

func (m *SetupModel) View() string {
	switch m.step {
	case 1:
		return m.viewStep1()
	case 2:
		return m.viewStep2()
	case 3:
		return m.viewStep3()
	case 4:
		return m.viewStep4()
	}
	return ""
}

func (m *SetupModel) viewStep1() string {
	langOpts := ""
	if m.lang == state.LangEnglish {
		langOpts = ui.AccentStyle.Render("▸ English") + "\n  Türkçe"
	} else {
		langOpts = "  English\n" + ui.AccentStyle.Render("▸ Türkçe")
	}

	steps := ui.DimStyle.Render("  Step 1/4 — Language")
	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		renderLogo(),
		"",
		ui.DimStyle.Render("  A Spotify-inspired CLI music player"),
		"",
		steps,
		"",
		"  " + ui.WhiteStyle.Bold(true).Render("Language / Dil:"),
		"  "+langOpts,
		"",
		ui.DimStyle.Render("  [↑↓] Change  [Enter] Next  [Esc] Quit"),
	)

	title := ui.WhiteStyle.Render("  " + ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le") + "  Setup")
	box := ui.AccentBorderStyle.
		Width(50).
		Render(title + "\n" + content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *SetupModel) viewStep2() string {
	inputView := ui.FaintStyle.Render(m.inputs[0].Value())
	if m.focus == 0 {
		inputView = m.inputs[0].View()
	}
	errView := ""
	if m.err != "" {
		errView = "\n" + ui.ErrorStyle.Render("  ✗ "+m.err)
	}

	steps := ui.DimStyle.Render("  Step 2/4 — " + langT("Directory", "Dizin"))
	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		"  "+renderLogo(),
		"",
		steps,
		"",
		ui.AccentStyle.Render("  "+langT("Where should MusicLe store your music?", "MusicLe müziği nereye kaydetsin?")),
		"",
		inputView,
		errView,
		"",
		ui.DimStyle.Render("  [Enter] Next  [F2] Browse  [Esc] Back"),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("Directory Setup", "Dizin Seçimi") + " ")
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *SetupModel) viewStep3() string {
	var inputViews []string
	for i, inp := range m.inputs {
		if i == m.focus {
			inputViews = append(inputViews, inp.View())
		} else {
			inputViews = append(inputViews, ui.FaintStyle.Render(inp.Value()))
		}
	}
	errView := ""
	if m.err != "" {
		errView = "\n" + ui.ErrorStyle.Render("  ✗ "+m.err)
	}

	steps := ui.DimStyle.Render("  Step 3/4 — " + langT("Profile", "Profil"))
	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		"  "+renderLogo(),
		"",
		steps,
		"",
		inputViews[0],
		"",
		inputViews[1],
		"",
		inputViews[2],
		"",
		inputViews[3],
		errView,
		"",
		ui.DimStyle.Render("  [Enter] Next  [Tab] Next Field  [Esc] Back"),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("Profile Setup", "Profil Oluştur") + " ")
	box := ui.BorderStyle.
		Width(62).
		Render(title + "\n" + content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *SetupModel) viewStep4() string {
	var inputViews []string
	for i, inp := range m.inputs {
		if i == m.focus {
			inputViews = append(inputViews, inp.View())
		} else {
			inputViews = append(inputViews, ui.FaintStyle.Render(inp.Value()))
		}
	}
	errView := ""
	if m.err != "" {
		errView = "\n" + ui.ErrorStyle.Render("  ✗ "+m.err)
	}

	steps := ui.DimStyle.Render("  Step 4/4 — " + langT("Playlist", "Playlist"))
	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		"  "+renderLogo(),
		"",
		steps,
		"",
		inputViews[0],
		"",
		inputViews[1],
		"",
		inputViews[2],
		"",
		inputViews[3],
		errView,
		"",
		ui.DimStyle.Render("  [Enter] Finish  [Tab] Next Field  [Esc] Back"),
	)

	title := ui.SectionTitleStyle.Render(" " + langT("Playlist Setup", "İlk Playlist") + " ")
	box := ui.BorderStyle.
		Width(62).
		Render(title + "\n" + content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
