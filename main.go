package main

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"musicle-cli/bridge"
	"musicle-cli/state"
)

func main() {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.TempDir()
	}
	state.Current.ConfigDir = filepath.Join(cfgDir, "musicle")

	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	projectDir := filepath.Dir(exe)
	bridge.Init(projectDir)

	if err := state.Current.LoadConfig(); err != nil {
		state.Current.IsFirstLaunch = true
		state.Current.Language = state.LangEnglish
	} else {
		if scanErr := state.Current.ScanProfiles(); scanErr != nil || len(state.Current.Profiles) == 0 {
			state.Current.IsFirstLaunch = true
		} else {
			state.Current.CurrentProfile = &state.Current.Profiles[0]
			if len(state.Current.CurrentProfile.Playlists) > 0 {
				state.Current.CurrentPlaylist = &state.Current.CurrentProfile.Playlists[0]
			}
		}
	}

	p := tea.NewProgram(NewMainModel(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		panic(err)
	}

	bridge.StopAll()
}
