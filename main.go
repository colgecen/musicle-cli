package main

import (
	"fmt"
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

const helpText = `MusicLeCLI — terminal music player with download engine
Usage:  MusicLeCLI.exe [--version] [--help]

Keys:
  F2          Cycle tabs (Home / Downloads / Profile / Playlist / Settings)
  F1          Cycle focus within current view
  Tab/Shift+Tab  Move between elements
  Arrow keys  Navigate lists
  Enter       Select / Confirm
  Escape      Back to Home
  F5          Focus console (Home)
  F6          Focus song list (Home)
  F7          Play selected song
  PageUp/Dn   Scroll console log

Downloads:
  Paste a YouTube or Spotify URL, select target playlist, press Download.
  All audio is converted to MP3 with full ID3 metadata.

Config:     %s
`

func versionString() string {
	v := version
	if commit != "" {
		v += " (" + commit[:7] + ")"
	}
	if date != "" {
		v += " " + date
	}
	return v
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version":
			fmt.Println("musicle", versionString())
			return
		case "--help", "-h":
			cfgDir, _ := os.UserConfigDir()
			fmt.Printf(helpText, filepath.Join(cfgDir, "musicle"))
			return
		}
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
