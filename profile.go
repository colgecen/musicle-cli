package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
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

	focus       int
	lastProfile string // tracks last loaded profile folder to avoid re-populating inputs
	selectAll   bool
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
			ti.Width = 60
			return ti
		}(),
		nameInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Display Name:  "
			ti.Placeholder = "MusicLeCLI User"
			ti.Width = 60
			return ti
		}(),
		bioInput: func() textinput.Model {
			ti := textinput.New()
			ti.Prompt = "  Bio:  "
			ti.Placeholder = "Music lover"
			ti.Width = 60
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
		cp := state.Current.CurrentProfile
		if cp != nil && cp.FolderName != m.lastProfile {
			m.lastProfile = cp.FolderName
			m.nameInput.SetValue(cp.DisplayName)
			m.nameInput.SetCursor(len(cp.DisplayName))
			m.bioInput.SetValue(cp.Bio)
			m.bioInput.SetCursor(len(cp.Bio))
		}
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
			if m.focus == 0 {
				if m.profileDropIdx > 0 {
					m.profileDropIdx--
					m.refreshOptions()
				}
			} else if m.focus >= 1 && m.focus <= 3 {
				m.selectAll = false
				inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		case "down", "j":
			if m.focus == 0 {
				if m.profileDropIdx < len(m.profileOptions)-1 {
					m.profileDropIdx++
					m.refreshOptions()
				}
			} else if m.focus >= 1 && m.focus <= 3 {
				m.selectAll = false
				inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
				var cmd tea.Cmd
				*inputs[m.focus-1], cmd = inputs[m.focus-1].Update(msg)
				return m, cmd
			}
		case "tab":
			m.selectAll = false
			m.setFocus((m.focus + 1) % 5)
		case "shift+tab":
			m.selectAll = false
			m.setFocus((m.focus - 1 + 5) % 5)
		case "enter":
			if m.focus == 4 {
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
				_ = state.Current.SaveConfig()
				_ = state.Current.ScanProfiles()
				for i, p := range state.Current.Profiles {
					if p.FolderName == cp.FolderName {
						state.Current.CurrentProfile = &state.Current.Profiles[i]
						break
					}
				}
				m.refreshOptions()
				m.profileStatus = ui.AccentStyle.Render("  v " + langT("Saved!", "Kaydedildi!"))
			}
		case "esc":
			m.focus = 0
		case "ctrl+v":
			if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
				*inputs[m.focus-1], _ = inputs[m.focus-1].Update(textinput.Paste())
				return m, nil
			}
		case "ctrl+a":
			if m.focus >= 1 && m.focus <= 3 {
				inputs := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}
				if inputs[m.focus-1].Value() != "" {
					m.selectAll = true
				}
			}
			return m, nil
		default:
			if m.focus >= 1 && m.focus <= 3 {
				if m.selectAll {
					inp := []*textinput.Model{&m.avatarInput, &m.nameInput, &m.bioInput}[m.focus-1]
					s := msg.String()
					if len(s) == 1 || s == "backspace" || s == "delete" {
						inp.SetValue("")
						inp.SetCursor(0)
						m.selectAll = false
					} else {
						m.selectAll = false
					}
				}
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
	if idx < 0 || idx >= 5 {
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
	m.setFocus((m.focus + 1) % 5)
	return m.focus == 0
}

func (m *ProfileModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	profileV := "-"
	if len(m.profileOptions) > 0 && m.profileDropIdx < len(m.profileOptions) {
		profileV = m.profileOptions[m.profileDropIdx]
	}
	if m.focus == 0 {
		profileV = ui.AccentStyle.Render("> " + profileV)
	} else {
		profileV = "  " + ui.WhiteStyle.Render(profileV)
	}

	avatarVal := m.avatarInput.Value()
	if avatarVal == "" {
		avatarVal = m.avatarInput.Placeholder
	}
	var avatarV string
	if m.focus == 1 {
		avatarV = m.avatarInput.View()
	} else {
		avatarV = "  Avatar Path:  " + ui.WhiteStyle.Render(avatarVal)
	}

	nameVal := m.nameInput.Value()
	if nameVal == "" {
		nameVal = m.nameInput.Placeholder
	}
	var nameV string
	if m.focus == 2 {
		nameV = m.nameInput.View()
	} else {
		nameV = "  Display Name:  " + ui.WhiteStyle.Render(nameVal)
	}

	bioVal := m.bioInput.Value()
	if bioVal == "" {
		bioVal = m.bioInput.Placeholder
	}
	var bioV string
	if m.focus == 3 {
		bioV = m.bioInput.View()
	} else {
		bioV = "  Bio:  " + ui.WhiteStyle.Render(bioVal)
	}

	langOpts := "English"
	if m.langIdx == 1 {
		langOpts = "Turkce"
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		ui.SectionTitleStyle.Render(" "+langT("Profile", "Profil")+": ")+profileV,
		"",
		ui.SectionTitleStyle.Render(" Avatar Image "),
		"",
		avatarV,
		"",
		ui.SectionTitleStyle.Render(" Display Name "),
		"",
		nameV,
		"",
		ui.SectionTitleStyle.Render(" Bio "),
		"",
		bioV,
		"",
		ui.SectionTitleStyle.Render(" Language: ")+ui.WhiteStyle.Render(langOpts),
		"",
		m.profileStatus,
		"",
		func() string {
			btn := ui.AccentButtonStyle.Render(langT("  Save Profile  ", "  Profili Kaydet  "))
			if m.focus == 4 {
				btn = ui.FocusedButtonStyle.Render(langT("  Save Profile  ", "  Profili Kaydet  "))
			}
			return btn
		}(),
	)

	title := ui.SectionTitleStyle.Render(langT(" Profile Settings", " Profil Ayarlari"))
	box := ui.AccentBorderStyle.
		Width(75).
		Render(title + "\n" + boxContent)

	return box
}
