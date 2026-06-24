package main

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"MusicLeCLI/bridge"
	"MusicLeCLI/state"
	"MusicLeCLI/ui"
)

var version = "dev"
var commit = ""
var date = ""

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		v := version
		if commit != "" {
			v += " (" + commit[:7] + ")"
		}
		if date != "" {
			v += " " + date
		}
		println("musicle", v)
		return
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.TempDir()
	}
	state.Current.ConfigDir = filepath.Join(cfgDir, "musicle")

	bridge.Init("")

	ui.InitStyles()

	if err := state.Current.LoadConfig(); err != nil {
		state.Current.IsFirstLaunch = true
		state.Current.Language = state.LangEnglish
	} else {
		ui.ApplyTheme(state.Current.Theme)
		if scanErr := state.Current.ScanProfiles(); scanErr != nil || len(state.Current.Profiles) == 0 {
			state.Current.IsFirstLaunch = true
		} else {
			state.Current.CurrentProfile = &state.Current.Profiles[0]
			if len(state.Current.CurrentProfile.Playlists) > 0 {
				state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[0]
			}
		}
	}

	maximizeTerminal()

	p := tea.NewProgram(NewMainModel(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		panic(err)
	}

	bridge.StopAll()
}
