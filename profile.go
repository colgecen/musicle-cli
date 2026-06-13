package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

type ProfileModel struct {
	width  int
	height int

	profileDropIdx int
	profileOptions []string
	avatarInput    textinput.Model
	nameInput      textinput.Model
	bioInput       textinput.Model
	langIdx        int
	profileStatus  string

	focus int
}

func NewProfileModel() *ProfileModel {
	return &ProfileModel{
		langIdx: func() int {
			if state.Current.Language == state.LangTurkish {
				return 1
			}
			return 0
		}(),
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
			ti.Placeholder = "MusicLe User"
			ti.Width = 50
			return ti
		}(),
		bioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Bio:  "
			ti.Placeholder = "Music lover"
			ti.Width = 50
			return ti
		}(),
	}
}

func (m *ProfileModel) Init() tea.Cmd { return nil }

func (m *ProfileModel) refreshOptions() {
	state.Current.ScanProfiles()
	opts := make([]string, len(state.Current.Profiles))
	for i, p := range state.Current.Profiles {
		opts[i] = p.DisplayName
	}
	m.profileOptions = opts
	if m.profileDropIdx >= len(opts) {
		m.profileDropIdx = 0
	}
	if len(opts) > 0 {
		state.Current.CurrentProfile = &state.Current.Profiles[m.profileDropIdx]
	}
}

func (m *ProfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.refreshOptions()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.focus == 0 && m.profileDropIdx > 0 {
				m.profileDropIdx--
				m.refreshOptions()
			}
			m.setFocus((m.focus - 1 + 4) % 4)
		case "down", "j":
			if m.focus == 0 && m.profileDropIdx < len(m.profileOptions)-1 {
				m.profileDropIdx++
				m.refreshOptions()
			}
			m.setFocus((m.focus + 1) % 4)
		case "tab":
			m.setFocus((m.focus + 1) % 4)
		case "shift+tab":
			m.setFocus((m.focus - 1 + 4) % 4)
		case "enter":
			cp := state.Current.CurrentProfile
			if cp == nil {
				return m, nil
			}
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				name = cp.FolderName
			}
			bio := strings.TrimSpace(m.bioInput.Value())
			avatarSrc := strings.TrimSpace(m.avatarInput.Value())
			if err := state.Current.SaveProfileMeta(cp.FolderName, name, bio); err != nil {
				m.profileStatus = ui.ErrorStyle.Render("  x " + err.Error())
				return m, nil
			}
			if avatarSrc != "" {
				profileDir := state.Current.ProfilesDir()
				ext := ".jpg"
				if strings.HasSuffix(strings.ToLower(avatarSrc), ".png") {
					ext = ".png"
				}
				_ = state.CopyFile(avatarSrc, profileDir+"/"+cp.FolderName+"/avatar/avatar"+ext)
			}
			lang := state.LangEnglish
			if m.langIdx == 1 {
				lang = state.LangTurkish
			}
			state.Current.Language = lang
			state.Current.CurrentProfile.DisplayName = name
			state.Current.CurrentProfile.Bio = bio
			state.Current.CurrentProfile.Language = lang
			_ = state.Current.SaveConfig()
			_ = state.Current.ScanProfiles()
			m.refreshOptions()
			m.profileStatus = ui.AccentStyle.Render("  v " + langT("Saved!", "Kaydedildi!"))
		case "esc":
			m.focus = 0
		default:
			if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *ProfileModel) setFocus(idx int) {
	if idx < 0 || idx >= 4 {
		return
	}
	m.focus = idx
	inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
	for i, inp := range inputs {
		if i+1 == idx {
			inp.Focus()
		} else {
			inp.Blur()
		}
	}
}

func (m *ProfileModel) cycleFocus() bool {
	m.setFocus((m.focus + 1) % 4)
	return m.focus == 0
}

func (m *ProfileModel) View() string {
	if m.width <= 0 {
		m.width = 120
		m.height = 40
	}

	profileV := "—"
	if len(m.profileOptions) > 0 && m.profileDropIdx < len(m.profileOptions) {
		profileV = m.profileOptions[m.profileDropIdx]
	}
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

	return box
}
