package main

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"musicle-cli/state"
	"musicle-cli/ui"
)

func renderLangModal(lang state.Language) string {
	langOpts := ""
	if lang == state.LangEnglish {
		langOpts = ui.AccentStyle.Render("> English") + "\n  Turkce"
	} else {
		langOpts = "  English\n" + ui.AccentStyle.Render("> Turkce")
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		renderLogo(),
		"",
		ui.WhiteStyle.Bold(true).Render("Language / Dil:"),
		"",
		"  "+langOpts,
		"",
		ui.DimStyle.Render("[^v] Change  [Enter] Confirm"),
	)

	title := ui.WhiteStyle.Render("  " + ui.LogoStyle.Render("Music") + ui.LogoAccentStyle.Render("Le") + "  " + langT("Welcome", "Hos Geldiniz"))
	box := ui.AccentBorderStyle.
		Width(46).
		Render(title + "\n" + content)

	return lipgloss.Place(50, 16, lipgloss.Center, lipgloss.Center, box)
}

func placeOverlay(full, overlay string, width int) string {
	lines := strings.Split(full, "\n")
	totalH := len(lines)
	overlayH := lipgloss.Height(overlay)
	overlayW := lipgloss.Width(overlay)
	topPad := (totalH - overlayH) / 2
	leftPad := (width - overlayW) / 2
	if topPad < 0 {
		topPad = 0
	}
	if leftPad < 0 {
		leftPad = 0
	}
	overlayLines := strings.Split(overlay, "\n")
	var result []string
	for i, line := range lines {
		if i >= topPad && i < topPad+overlayH {
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

func initializeDefaults(lang state.Language) tea.Cmd {
	return func() tea.Msg {
		state.Current.Language = lang

		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		rootDir := filepath.Join(homeDir, "Music", "MusicLe")
		if err := state.Current.InitializeBaseDirs(rootDir); err != nil {
			return errorMsg(err.Error())
		}
		if err := state.Current.CreateProfileStructure("default", "Default", "", "", lang); err != nil {
			return errorMsg(err.Error())
		}
		if err := state.Current.CreatePlaylistStructure("default", "my-playlist", "My Playlist", "", ""); err != nil {
			return errorMsg(err.Error())
		}
		if err := state.Current.SaveConfig(); err != nil {
			return errorMsg(err.Error())
		}
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
