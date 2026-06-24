# 🎵 MusicLe CLI

> A terminal-based music player with Spotify-inspired UI, audio visualization, and multi-platform support.

<div align="center">

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-Apache%202.0-blue)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20macOS%20|%20Linux-lightgrey)
[![Release](https://img.shields.io/github/v/release/alperencolgecen/musicle-cli)](https://github.com/alperencolgecen/musicle-cli/releases/latest)

**English** · [Türkçe](#türkçe)

</div>

---

## ✨ Features

- **🎨 Spotify-Inspired UI** — Clean, modern terminal interface with album art, song list, and player bar
- **📊 Audio Visualization** — Real-time volume bars using CP437 characters (░▒▓█) — works in every terminal
- **📋 Playlist Management** — Create, edit, delete playlists; reorder songs
- **🎵 Multiple Sources** — Add music from Spotify, YouTube Music, or local files
- **🖼️ Album Art** — ANSI half-block rendering of cover images
- **🌐 Bilingual** — Full English and Turkish interface support
- **🎨 Themes** — Light, Dark, and custom color themes
- **⚡ Blazing Fast** — Written in Go, launches in milliseconds
- **🔊 Audio Engine** — Powered by Python (mutagen + ffmpeg), auto-advances on completion
- **🔄 Auto-Advance** — Automatic next track with configurable delay
- **🔍 Search** — Quick filtering across your library
- **🎛️ Equalizer-like Bars** — Real-time audio spectrum visualization in the player bar

---

## 📦 Installation

### Windows

<table>
<tr><th>Archive (zip)</th><th>Install Steps</th></tr>
<tr><td>

| Architecture | File |
|--------------|------|
| x86_64 | `musicle-cli_Windows_x86_64.zip` |
| x86 (32-bit) | `musicle-cli_Windows_x86.zip` |
| arm64 | `musicle-cli_Windows_arm64.zip` |

</td>
<td>

1. Download the `.zip` for your architecture from [Releases](https://github.com/alperencolgecen/musicle-cli/releases/latest)
2. Extract the archive
3. Run `musicle-cli.exe`

**No dependencies required.**

</td>
</tr>
</table>

### macOS

<table>
<tr><th>Archive (tar.gz)</th><th>Install Steps</th></tr>
<tr><td>

| Architecture | File |
|--------------|------|
| Intel (x86_64) | `musicle-cli_macOS_x86_64.tar.gz` |
| Apple Silicon (arm64) | `musicle-cli_macOS_arm64.tar.gz` |

</td>
<td>

1. Download the `.tar.gz` for your architecture from [Releases](https://github.com/alperencolgecen/musicle-cli/releases/latest)
2. Extract: `tar xzf muscle-cli_macOS_*.tar.gz`
3. Run: `./musicle-cli`

**No dependencies required.**

</td>
</tr>
</table>

### Linux

<table>
<tr><th>Format</th><th>Install Steps</th></tr>
<tr><td>

**tar.gz**  
`musicle-cli_Linux_x86_64.tar.gz`

**deb (Debian/Ubuntu)**  
`musicle-cli_Linux_x86_64.deb`

**rpm (Fedora/RHEL)**  
`musicle-cli_Linux_x86_64.rpm`

</td>
<td>

**tar.gz:**
```bash
tar xzf muscle-cli_Linux_x86_64.tar.gz
cd muscle-cli_Linux_x86_64
./musicle-cli
```

**deb:**
```bash
sudo dpkg -i muscle-cli_Linux_x86_64.deb
musicle-cli
```

**rpm:**
```bash
sudo rpm -ivh muscle-cli_Linux_x86_64.rpm
musicle-cli
```

**No dependencies required.**

</td>
</tr>
</table>

### Build from Source

```bash
git clone https://github.com/alperencolgecen/musicle-cli.git
cd muscle-cli

# Build (pure Go, no CGO required)
go build -o muscle-cli .

# Cross-platform:
GOOS=windows GOARCH=amd64 go build -o muscle-cli.exe .   # Windows
GOOS=darwin GOARCH=amd64 go build -o muscle-cli .         # macOS Intel
GOOS=linux GOARCH=amd64 go build -o muscle-cli .          # Linux
```

---

## 🚀 Quick Start

1. **Launch:** `./musicle-cli` (or double-click the binary)
2. **First Run Wizard:** Choose music directory, language, create a profile and playlist
3. **Add Music:**
   - Paste a Spotify or YouTube Music URL in the sidebar
   - Or use `+ Add Local Music` to browse local files
4. **Play:** Select a song and press `Space` or click `▶ Play`

---

## 🎮 Keybindings

| Key | Action | Description |
|-----|--------|-------------|
| `Space` | ⏯ Play/Pause | Toggle playback |
| `→` / `←` | ⏩⏪ Seek | 5 seconds forward/back |
| `↑` / `↓` | 🔊🔉 Volume | Increase/decrease volume |
| `Tab` | 🔄 Cycle Focus | Switch between sidebar, songs, editor, console |
| `F1` | 🔁 Cycle Sections | Cycle focus through all sections including console |
| `Ctrl+U` | 📋 Update Playlist | Save the current playlist |
| `n` | ⏭ Next Song | Skip to next track |
| `Ctrl+C` / `Esc` | ❌ Quit | Exit the application |
| `Enter` | ✏️ Edit Song | Open edit modal for selected song |

---

## 🖥️ Interface

```
┌─────────────────────────────────────────────────────────────────┐
│  MusicLe      [Home]  [Settings]                                │  ← Header
├──────────────┬──────────────────────────────────────────────────┤
│              │  [Playlist ▼]                                    │
│  MUSIC       │  ┌──────┐  Playlist Name                         │
│  DOWNLOAD    │  │ Art  │  Description / Bio                     │
│              │  └──────┘                                        │
│  [Spotify…]  │  [🔒 Lock] [🔀 Shuffle] [▶ Play] [⬇ Download]  │
│  [YouTube…]  │  ────────────────────────────────────────────    │
│  [+Local]    │  #  Title               Artist     Album   Dur   │
│  [Playlist▼] │  ────────────────────────────────────────────    │
│              │  1  Bohemian Rhapsody   Queen      A Night 05:55 │
│  (~25%)      │  2  Stairway to Heaven  Led Zepp  IV      08:02 │
│              │  3  …                                           │
├──────────────┴──────────────────────────────────────────────────┤
│  ░▒▓███████  Bohemian Rhapsody — Queen    ░▒▓███░  01:23/05:55 │  ← Player Bar
└─────────────────────────────────────────────────────────────────┘
```

### Sections

| Section | Description |
|---------|-------------|
| **Sidebar** | Music download (Spotify/YouTube/local) + playlist selector |
| **Playlist Info** | Album art, name, description, action buttons |
| **Songs Table** | Song list with title, artist, album, duration columns |
| **Player Bar** | Volume visualization, now-playing info, progress, metadata |
| **Console** | Log output and debug information |
| **Edit Modal** | Inline editing of song title, artist, album, and date |

---

## ⚙️ Configuration

Config file: `%APPDATA%/musicle/config.json` (Windows) or `~/.config/musicle/config.json` (Linux/macOS)

```json
{
  "language": "en",
  "theme": "dark",
  "musicDir": "~/Music",
  "player": {
    "volume": 80,
    "autoAdvance": true,
    "autoAdvanceDelay": 2
  }
}
```

### Themes

- **dark** — Dark background with vibrant accents (default)
- **light** — Light background
- **custom** — User-defined color scheme

---

## 🧩 Project Structure

```
musicle-cli/
├── main.go                 # Application entry point
├── model.go                # Main TUI model
├── home.go                 # Home screen logic (player, songs, sidebar)
├── settings.go             # Settings screen
├── bridge/
│   └── bridge.go           # Go ↔ Python bridge (daemon communication)
├── state/
│   ├── state.go            # Global application state
│   ├── config.go           # Configuration management
│   └── profile.go          # Profile data structures
├── ui/
│   ├── styles.go           # Lipgloss styles, theme system
│   ├── keys.go             # Keybinding definitions
│   └── help.go             # Help view
├── bridge/
│   ├── bridge.go           # Main dispatcher (player, playlist, metadata, download)
│   ├── player.go           # Audio playback engine (oto + beep + gonum FFT)
│   ├── metadata.go         # Metadata extraction (dhowden/tag)
│   ├── playlist.go         # Playlist CRUD + local file import
│   ├── download.go         # YouTube/Spotify download via yt-dlp/spotdl
├── maximize_windows.go     # Terminal maximize (Windows)
├── maximize_unix.go        # Terminal maximize (Linux/macOS)
├── .goreleaser.yaml        # Release build config
```

---

## 🔧 Technical Details

### Audio Engine
- **Go** powers everything: TUI (Bubble Tea + Lipgloss), audio playback (oto + beep), metadata (dhowden/tag), and FFT spectrum (gonum)
- **No Python required** — single binary, zero runtime dependencies
- Audio decoding supports MP3, FLAC, and WAV

### Visualization
- Real-time 16-band FFT spectrum computed with gonum/dsp/fourier
- Rendered as CP437 block characters (` ░▒▓█`) — guaranteed in every terminal
- 40-character bar width for consistent rendering in the player bar

### Release Artifacts
| Platform | Arch | Format |
|----------|------|--------|
| Windows | x86_64 | zip |
| Windows | x86 | zip |
| Windows | arm64 | zip |
| Linux | x86_64 | tar.gz, deb, rpm |
| Linux | arm64 | tar.gz, deb, rpm |
| Linux | x86 | tar.gz, deb, rpm |
| macOS | x86_64 | tar.gz |
| macOS | arm64 | tar.gz |

---

## 📄 Changelog

### v1.1.0
- ✨ **Pure Go rewrite** — no Python, no engine directory, single binary
- 📦 **Self-contained** — download, extract, run. Zero dependencies
- 🔊 **Audio engine** — oto + beep for playback, gonum FFT for spectrum
- 🏷️ **Native metadata** — dhowden/tag for ID3/FLAC/MP4/AAC
- 🚫 **CGO-free** — pure Go on all platforms (file dialogs via zenity)
- 🐧 **Linux arm64 + 386** — added alongside x86_64
- 📊 16-band real-time FFT spectrum visualization
- 🎨 Album art ANSI rendering

### v1.0.0
- 🎵 Initial release (Python engine)
- Spotify and YouTube Music integration
- Playlist management
- Modern terminal UI
- Windows, macOS, Linux support

---

## 🤝 Contributing

Contributions are welcome! Please see our [contributing guidelines](CONTRIBUTING.md).

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes
4. Push and open a Pull Request

---

## 📬 Contact

- **Developer:** Alperen Çölgeçen — alperencolgecen@gmail.com
- **GitHub:** [@alperencolgecen](https://github.com/alperencolgecen)
- **Issues:** [github.com/alperencolgecen/musicle-cli/issues](https://github.com/alperencolgecen/musicle-cli/issues)

---

## 📜 License

This project is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for details.

---

<div align="center">

**🎵 The most elegant way to enjoy music from your terminal.**

</div>
