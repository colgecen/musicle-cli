package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type SettingsModel struct {
	width  int
	height int

	activeTab string

	profileDropIdx  int
	profileOptions  []string
	avatarInput     textinput.Model
	nameInput       textinput.Model
	bioInput        textinput.Model
	langIdx         int
	profileStatus   string

	playlistDropIdx int
	playlistOptions []string
	artInput        textinput.Model
	plNameInput     textinput.Model
	plBioInput      textinput.Model
	playlistStatus  string

	focus int
}

func NewSettingsModel() *SettingsModel {
	return &SettingsModel{
		activeTab: "profile",
		avatarInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Avatar Path:  "
			ti.Placeholder = "optional"
			ti.Width = 50
			return ti
		}(),
		nameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Display Name:  "
			ti.Width = 50
			return ti
		}(),
		bioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Bio:           "
			ti.Width = 50
			return ti
		}(),
		artInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Art Path:        "
			ti.Placeholder = "optional"
			ti.Width = 50
			return ti
		}(),
		plNameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Playlist Name:   "
			ti.Width = 50
			return ti
		}(),
		plBioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Description:       "
			ti.Width = 50
			return ti
		}(),
	}
}

func (m *SettingsModel) Init() tea.Cmd {
	m.refreshProfileOptions()
	m.refreshPlaylistOptions()
	m.fillProfileFields()
	m.fillPlaylistFields()
	m.avatarInput.Focus()
	return nil
}

func (m *SettingsModel) refreshProfileOptions() {
	m.profileOptions = nil
	for _, p := range state.Current.Profiles {
		m.profileOptions = append(m.profileOptions, p.DisplayName+" ("+p.FolderName+")")
	}
	if len(m.profileOptions) == 0 {
		m.profileOptions = []string{"(no profiles)"}
	}
}

func (m *SettingsModel) refreshPlaylistOptions() {
	m.playlistOptions = nil
	if state.Current.CurrentProfile != nil {
		for _, pl := range state.Current.CurrentProfile.Playlists {
			m.playlistOptions = append(m.playlistOptions, pl.Name+" ("+pl.FolderName+")")
		}
	}
	if len(m.playlistOptions) == 0 {
		m.playlistOptions = []string{"(no playlists)"}
	}
}

func (m *SettingsModel) fillProfileFields() {
	if state.Current.CurrentProfile == nil {
		return
	}
	p := state.Current.CurrentProfile
	m.avatarInput.SetValue(p.AvatarPath)
	m.nameInput.SetValue(p.DisplayName)
	m.bioInput.SetValue(p.Bio)
	if p.Language == state.LangTurkish {
		m.langIdx = 1
	} else {
		m.langIdx = 0
	}
}

func (m *SettingsModel) fillPlaylistFields() {
	if state.Current.CurrentPlaylist == nil {
		return
	}
	pl := state.Current.CurrentPlaylist
	m.artInput.SetValue(pl.ArtPath)
	m.plNameInput.SetValue(pl.Name)
	m.plBioInput.SetValue(pl.Bio)
}

func (m *SettingsModel) setFocus(idx int) {
	inputs := m.activeInputs()
	if len(inputs) == 0 {
		return
	}
	if idx >= 0 && idx < len(inputs) {
		if m.focus >= 0 && m.focus < len(inputs) {
			inputs[m.focus].Blur()
		}
		m.focus = idx
		inputs[m.focus].Focus()
	}
}

func (m *SettingsModel) activeInputs() []*textinput.Model {
	if m.activeTab == "profile" {
		return []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
	}
	return []*textinput.Model{&m.artInput, &m.plNameInput, &m.plBioInput}
}

func (m *SettingsModel) focusedInput() *textinput.Model {
	inputs := m.activeInputs()
	if m.focus >= 0 && m.focus < len(inputs) {
		return inputs[m.focus]
	}
	return nil
}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	inputs := m.activeInputs()
	var cmds []tea.Cmd
	for i := range inputs {
		var cmd tea.Cmd
		*inputs[i], cmd = inputs[i].Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *SettingsModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m, nil
	case "f3":
		if m.activeTab == "profile" {
			m.activeTab = "playlist"
		} else {
			m.activeTab = "profile"
		}
		m.focus = 0
		inputs := m.activeInputs()
		if len(inputs) > 0 {
			inputs[0].Focus()
		}
		return m, nil
	case "tab":
		inputs := m.activeInputs()
		m.setFocus((m.focus + 1) % len(inputs))
		return m, nil
	case "shift+tab":
		inputs := m.activeInputs()
		m.setFocus((m.focus - 1 + len(inputs)) % len(inputs))
		return m, nil
	case "enter":
		if m.activeTab == "profile" {
			m.saveProfile()
		} else {
			m.savePlaylist()
		}
		return m, nil
	case "up", "k":
		if m.activeTab == "profile" {
			m.profileDropIdx = (m.profileDropIdx - 1 + len(m.profileOptions)) % len(m.profileOptions)
			m.selectProfile(m.profileDropIdx)
		} else {
			m.playlistDropIdx = (m.playlistDropIdx - 1 + len(m.playlistOptions)) % len(m.playlistOptions)
			m.selectPlaylist(m.playlistDropIdx)
		}
	case "down", "j":
		if m.activeTab == "profile" {
			m.profileDropIdx = (m.profileDropIdx + 1) % len(m.profileOptions)
			m.selectProfile(m.profileDropIdx)
		} else {
			m.playlistDropIdx = (m.playlistDropIdx + 1) % len(m.playlistOptions)
			m.selectPlaylist(m.playlistDropIdx)
		}
	default:
		inp := m.focusedInput()
		if inp != nil {
			var cmd tea.Cmd
			*inp, cmd = inp.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *SettingsModel) selectProfile(idx int) {
	if idx < len(state.Current.Profiles) {
		state.Current.CurrentProfile = &state.Current.Profiles[idx]
		m.fillProfileFields()
	}
}

func (m *SettingsModel) selectPlaylist(idx int) {
	if state.Current.CurrentProfile != nil && idx < len(state.Current.CurrentProfile.Playlists) {
		state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[idx]
		m.fillPlaylistFields()
	}
}

func (m *SettingsModel) saveProfile() {
	if state.Current.CurrentProfile == nil {
		m.profileStatus = ui.ErrorStyle.Render("  No profile selected")
		return
	}
	name := strings.TrimSpace(m.nameInput.Value())
	bio := strings.TrimSpace(m.bioInput.Value())
	avatar := strings.TrimSpace(m.avatarInput.Value())
	newLang := state.LangEnglish
	if m.langIdx == 1 {
		newLang = state.LangTurkish
	}
	state.Current.Language = newLang
	state.Current.CurrentProfile.Language = newLang
	if err := state.Current.SaveProfileMeta(state.Current.CurrentProfile.FolderName, name, bio); err != nil {
		m.profileStatus = ui.ErrorStyle.Render("  ✗ " + err.Error())
		return
	}
	_ = os.WriteFile(filepath.Join(state.Current.ProfilesDir(), state.Current.CurrentProfile.FolderName, "lang.txt"), []byte(string(newLang)), 0644)
	if avatar != "" && avatar != state.Current.CurrentProfile.AvatarPath {
		_ = state.CopyFile(avatar, state.Current.PlaylistDir(state.Current.CurrentProfile.FolderName, "avatar"))
	}
	state.Current.CurrentProfile.DisplayName = name
	state.Current.CurrentProfile.Bio = bio
	_ = state.Current.SaveConfig()
	m.profileStatus = ui.AccentStyle.Render("  ✓ " + langT("Saved!", "Kaydedildi!"))
}

func (m *SettingsModel) savePlaylist() {
	if state.Current.CurrentProfile == nil || state.Current.CurrentPlaylist == nil {
		m.playlistStatus = ui.ErrorStyle.Render("  No playlist selected")
		return
	}
	name := strings.TrimSpace(m.plNameInput.Value())
	bio := strings.TrimSpace(m.plBioInput.Value())
	if err := state.Current.SavePlaylistMeta(
		state.Current.CurrentProfile.FolderName,
		state.Current.CurrentPlaylist.FolderName,
		name, bio,
	); err != nil {
		m.playlistStatus = ui.ErrorStyle.Render("  ✗ " + err.Error())
		return
	}
	state.Current.CurrentPlaylist.Name = name
	state.Current.CurrentPlaylist.Bio = bio
	m.playlistStatus = ui.AccentStyle.Render("  ✓ " + langT("Saved!", "Kaydedildi!"))
}

func (m *SettingsModel) View() string {
	header := m.viewHeader()
	tabBar := m.viewTabBar()
	headerH := lipgloss.Height(header)
	tabH := lipgloss.Height(tabBar)
	bodyH := m.height - headerH - tabH
	if bodyH < 5 {
		bodyH = 5
	}
	content := ""
	if m.activeTab == "profile" {
		content = m.viewProfileTab(bodyH)
	} else {
		content = m.viewPlaylistTab(bodyH)
	}
	contentH := lipgloss.Height(content)
	if contentH < bodyH {
		content += strings.Repeat("\n", bodyH-contentH)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, tabBar, content)
}

func (m *SettingsModel) viewHeader() string {
	logoSmall := ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le")
	logoBig := lipgloss.NewStyle().
		Padding(1, 6, 6, 6).
		Render(logoSmall)
	homeTab := lipgloss.NewStyle().
		Background(lipgloss.Color("#282828")).
		Foreground(ui.ColorPrimary).
		Padding(1, 2).
		Render(" Home ")
	settingsTab := lipgloss.NewStyle().
		Background(ui.ColorAccent).
		Foreground(ui.ColorBlack).
		Bold(true).
		Padding(1, 2).
		Render(" Settings ")
	hints := ui.DimStyle.Render("  [Esc] Back  [Tab] Fields  [F3] Tabs")
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, logoBig, "  ", homeTab, " ", settingsTab, "  ", hints)
	return ui.BorderStyle.Width(m.width - 2).Render(headerLine)
}

func (m *SettingsModel) viewTabBar() string {
	profileStyle := "[#1DB954::r] Profile [-::-]"
	playlistStyle := "[#B3B3B3]  Playlist [-]"
	if m.activeTab == "playlist" {
		profileStyle = "[#B3B3B3]  Profile [-]"
		playlistStyle = "[#1DB954::r] Playlist [-::-]"
	}
	profileV := ui.NavActiveStyle.Render(" Profile ")
	playlistV := ui.NavInactiveStyle.Render(" Playlist ")
	if m.activeTab == "playlist" {
		profileV = ui.NavInactiveStyle.Render(" Profile ")
		playlistV = ui.NavActiveStyle.Render(" Playlist ")
	}
	_ = profileStyle
	_ = playlistStyle
	return "  " + profileV + "  " + playlistV
}

func (m *SettingsModel) viewProfileTab(bodyH int) string {
	profileV := m.profileOptions[m.profileDropIdx]
	inputV1 := ui.FaintStyle.Render(m.avatarInput.Value())
	inputV2 := ui.FaintStyle.Render(m.nameInput.Value())
	inputV3 := ui.FaintStyle.Render(m.bioInput.Value())
	if m.focus == 1 { inputV1 = m.avatarInput.View() }
	if m.focus == 2 { inputV2 = m.nameInput.View() }
	if m.focus == 3 { inputV3 = m.bioInput.View() }

	langOpts := "English"
	if m.langIdx == 1 {
		langOpts = "Türkçe"
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" Profile: ") + ui.WhiteStyle.Render(profileV),
		"",
		inputV1,
		"",
		inputV2,
		"",
		inputV3,
		"",
		ui.SectionTitleStyle.Render(" Language: ") + ui.WhiteStyle.Render(langOpts),
		"",
		m.profileStatus,
		"",
		ui.AccentButtonStyle.Render(langT("  Save Profile  ", "  Profili Kaydet  ")),
	)

	title := ui.SectionTitleStyle.Render(langT(" Profile Settings", " Profil Ayarları"))
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + boxContent)

	boxH := lipgloss.Height(box)
	if boxH < bodyH {
		return lipgloss.JoinVertical(lipgloss.Left, box, strings.Repeat("\n", bodyH-boxH))
	}
	return box
}

func (m *SettingsModel) viewPlaylistTab(bodyH int) string {
	plV := m.playlistOptions[m.playlistDropIdx]
	inputV1 := ui.FaintStyle.Render(m.artInput.Value())
	inputV2 := ui.FaintStyle.Render(m.plNameInput.Value())
	inputV3 := ui.FaintStyle.Render(m.plBioInput.Value())
	if m.focus == 1 { inputV1 = m.artInput.View() }
	if m.focus == 2 { inputV2 = m.plNameInput.View() }
	if m.focus == 3 { inputV3 = m.plBioInput.View() }

	saveBtn := ui.AccentButtonStyle.Render(langT("  Save  ", "  Kaydet  "))
	deleteBtn := ui.ErrorButtonStyle.Render(langT("  Delete  ", "  Sil  "))

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" Playlist: ") + ui.WhiteStyle.Render(plV),
		"",
		inputV1,
		"",
		inputV2,
		"",
		inputV3,
		"",
		m.playlistStatus,
		"",
		lipgloss.JoinHorizontal(lipgloss.Left, saveBtn, "  ", deleteBtn),
	)

	title := ui.SectionTitleStyle.Render(langT(" Playlist Settings", " Playlist Ayarları"))
	box := ui.BorderStyle.
		Width(60).
		Render(title + "\n" + boxContent)

	boxH := lipgloss.Height(box)
	if boxH < bodyH {
		return lipgloss.JoinVertical(lipgloss.Left, box, strings.Repeat("\n", bodyH-boxH))
	}
	return box
}
