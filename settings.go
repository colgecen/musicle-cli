package main

import (
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

var themeNames = func() []string {
	names := make([]string, 0, len(ui.ThemeColors))
	for n := range ui.ThemeColors {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}()

// settingsTab describes one of the buttons in the left panel.
type settingsTab struct {
	id string
	en string
	tr string
}

var settingsTabs = []settingsTab{
	{"tab.theme", "Theme", "Tema"},
	{"tab.language", "Language", "Dil"},
	{"tab.sound", "Sound", "Ses"},
	{"tab.extras", "Extras", "Ekstralar"},
	{"tab.policies", "Policies", "Politikalar"},
	{"tab.about", "About", "Hakkinda"},
}

type SettingsModel struct {
	width  int
	height int

	langIdx  int
	themeIdx int

	// activeTab is the index into settingsTabs that is currently shown.
	activeTab int
	// rightFocused is true while the right panel (selection list) has focus.
	// When false, keys flow through to the MainModel for F1/F2/F3 handling.
	rightFocused bool
	// focus is kept in sync with MainModel F1 player-bar cycling (-1 = bar).
	focus int
}

func NewSettingsModel() *SettingsModel {
	m := &SettingsModel{}
	for i, l := range state.AllLanguages() {
		if l == state.Current.Language {
			m.langIdx = i
			break
		}
	}
	for i, n := range themeNames {
		if n == state.Current.Theme {
			m.themeIdx = i
			break
		}
	}
	return m
}

func (m *SettingsModel) Init() tea.Cmd { return nil }

// tabLabel returns the localized label for a tab index.
func (m *SettingsModel) tabLabel(i int) string {
	t := settingsTabs[i]
	return Tr(t.id)
}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// When the right panel is focused, keys navigate the selection list
		// instead of bubbling up to the MainModel global handlers.
		if m.rightFocused {
			switch msg.String() {
			case "esc", "tab", "shift+tab":
				m.rightFocused = false
				return m, nil
			case "up", "k":
				m.moveSelection(-1)
				return m, nil
			case "down", "j":
				m.moveSelection(1)
				return m, nil
			case "left", "right":
				// Allow horizontal cycling too — handy for single-row lists.
				m.moveSelection(1)
				return m, nil
			case "enter":
				return m, m.applyActiveTab()
			}
			// Swallow other keys while focused so they don't leak out.
			return m, nil
		}

		switch msg.String() {
		case "enter", "tab", "shift+tab":
			// Enter or Tab enters the right-panel selection list.
			if m.hasSelectionList() {
				m.rightFocused = true
				return m, nil
			}
			return m, m.applyActiveTab()
		}
	}
	return m, nil
}

// hasSelectionList reports whether the active tab owns a navigable list.
func (m *SettingsModel) hasSelectionList() bool {
	switch settingsTabs[m.activeTab].id {
	case "tab.theme", "tab.language":
		return true
	}
	return false
}

// moveSelection shifts the highlighted item of the active tab's list.
func (m *SettingsModel) moveSelection(dir int) {
	switch settingsTabs[m.activeTab].id {
	case "tab.theme":
		n := len(themeNames)
		if n == 0 {
			return
		}
		m.themeIdx = (m.themeIdx + dir + n) % n
	case "tab.language":
		langs := state.AllLanguages()
		m.langIdx = (m.langIdx + dir + len(langs)) % len(langs)
	}
}

// applyActiveTab commits the change for the currently visible tab, if any.
func (m *SettingsModel) applyActiveTab() tea.Cmd {
	id := settingsTabs[m.activeTab].id
	switch id {
	case "tab.language":
		langs := state.AllLanguages()
		state.Current.Language = langs[m.langIdx]
		_ = state.Current.SaveConfig()
	case "tab.theme":
		theme := themeNames[m.themeIdx]
		state.Current.Theme = theme
		_ = state.Current.SaveConfig()
		ui.ApplyTheme(theme)
		return func() tea.Msg { return ThemeChangedMsg{} }
	}
	return nil
}

// cycleTab is invoked by the MainModel F3 handler: advance to the next tab,
// wrapping back to the first.
func (m *SettingsModel) cycleTab() {
	m.activeTab = (m.activeTab + 1) % len(settingsTabs)
}

// cycleFocus exists for MainModel F1 player-bar focus cycling compatibility.
func (m *SettingsModel) cycleFocus() bool {
	m.focus = -1
	return true
}

// Button padding. Vertical padding is left at 0 so each button is a tidy
// single-line label; horizontal padding widens every button equally.
const (
	settingsBtnPadV = 0
	settingsBtnPadH = 4
)

func (m *SettingsModel) View() string {
	if m.width <= 0 {
		m.width = 120
	}
	if m.height <= 0 {
		m.height = 40
	}

	// Left panel = 35% of available width, right panel = remaining 65%.
	// Both scale automatically with the terminal width.
	totalW := m.width - 2
	if totalW < 40 {
		totalW = 40
	}
	leftW := totalW * 35 / 100
	gap := 3
	rightW := totalW - leftW - gap

	// Content height below the title line.
	contentH := m.height - 4
	if contentH < 10 {
		contentH = 10
	}

	title := ui.SectionTitleStyle.Render(" " + Tr("settings.title") + " ")

	leftPanel := m.renderLeftPanel(leftW, contentH)
	rightPanel := m.renderRightPanel(rightW, contentH)

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(leftW).Align(lipgloss.Center).Render(leftPanel),
		strings.Repeat(" ", gap),
		rightPanel,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, "", body)
}

// renderLeftPanel builds the vertically centered list of tab buttons.
// Buttons are spread evenly: equal gaps above, below, and between every button.
func (m *SettingsModel) renderLeftPanel(width int, height int) string {
	// Widest localized label (e.g. "Politikalar") — every button pads to this.
	maxW := 0
	for i := range settingsTabs {
		w := lipgloss.Width(m.tabLabel(i))
		if w > maxW {
			maxW = w
		}
	}
	innerW := maxW + settingsBtnPadH*2

	btnStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary)

	activeBtnStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorAccent).
		Background(ui.ColorAccent).
		Foreground(ui.ColorBlack).
		Bold(true)

	var btns []string
	for i := range settingsTabs {
		label := centerPad(m.tabLabel(i), innerW)
		if i == m.activeTab {
			btns = append(btns, activeBtnStyle.Render(label))
		} else {
			btns = append(btns, btnStyle.Render(label))
		}
	}

	// Reserve the hint at the very bottom; spread buttons in the remaining space.
	hint := ui.DimStyle.Render(Tr("settings.f3_hint"))
	hintH := lipgloss.Height(hint) + 2 // blank line above + the hint itself
	btnRegionH := height - hintH
	if btnRegionH < 1 {
		btnRegionH = 1
	}

	totalBtnH := 0
	for _, b := range btns {
		totalBtnH += lipgloss.Height(b)
	}
	// Equal gaps: one above the first, one between each pair, one below the last.
	numGaps := len(btns) + 1
	gapLines := (btnRegionH - totalBtnH) / numGaps
	if gapLines < 0 {
		gapLines = 0
	}
	blank := strings.Repeat("\n", gapLines)

	var b strings.Builder
	b.WriteString(blank)
	for i, btn := range btns {
		b.WriteString(btn)
		if i < len(btns)-1 {
			b.WriteString(blank)
		}
	}
	b.WriteString(blank)
	b.WriteString("\n")
	b.WriteString(hint)

	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(b.String())
}

// renderRightPanel renders the content for the active tab inside its own border.
// The border uses the active theme's accent color so it reflects the selection.
func (m *SettingsModel) renderRightPanel(width int, height int) string {
	var content string
	switch settingsTabs[m.activeTab].id {
	case "tab.theme":
		content = m.renderThemeTab(width)
	case "tab.language":
		content = m.renderLangTab(width)
	default:
		content = m.renderPlaceholder(width)
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorAccent).
		Width(width).
		Height(height).
		Align(lipgloss.Left, lipgloss.Top).
		Padding(0, 1)

	return style.Render(content)
}

func (m *SettingsModel) renderThemeTab(width int) string {
	var lines []string
	lines = append(lines, ui.SectionTitleStyle.Render(" "+Tr("tab.theme")+" "))
	lines = append(lines, "")
	for i, n := range themeNames {
		colorHex := ui.ThemeColors[n]
		colorSample := lipgloss.NewStyle().Foreground(lipgloss.Color(colorHex)).Render("###")
		line := "  " + colorSample + "  " + n
		if m.rightFocused && i == m.themeIdx {
			line = ui.AccentStyle.Bold(true).Render("> ") + colorSample + "  " + ui.WhiteStyle.Bold(true).Render(n)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "")
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+Tr("settings.select_hint")))
	return strings.Join(lines, "\n")
}

func (m *SettingsModel) renderLangTab(width int) string {
	langs := state.AllLanguages()
	var items []string
	for i, l := range langs {
		endonym := state.LanguageEndonym(l)
		line := "  " + endonym
		if m.rightFocused && i == m.langIdx {
			line = ui.AccentStyle.Bold(true).Render("> ") + ui.WhiteStyle.Bold(true).Render(endonym)
		}
		items = append(items, line)
	}
	lines := []string{
		ui.SectionTitleStyle.Render(" " + Tr("tab.language") + " "),
		"",
	}
	lines = append(lines, items...)
	lines = append(lines, "")
	lines = append(lines, ui.DimStyle.Render("  "+Tr("settings.select_hint")))
	return strings.Join(lines, "\n")
}

// renderPlaceholder is shown for not-yet-implemented tabs.
func (m *SettingsModel) renderPlaceholder(width int) string {
	title := ui.SectionTitleStyle.Render(" " + m.tabLabel(m.activeTab) + " ")
	soon := ui.DimStyle.Render("  " + Tr("common.coming_soon"))
	return strings.Join([]string{title, "", soon}, "\n")
}

// centerPad pads s with spaces so its display width equals targetW, centering
// the text. Used to make button borders all line up at an identical width.
func centerPad(s string, targetW int) string {
	w := lipgloss.Width(s)
	if w >= targetW {
		return s
	}
	total := targetW - w
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}
