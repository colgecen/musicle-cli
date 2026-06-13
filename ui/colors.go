package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ColorBackground = lipgloss.Color("#121212")
	ColorSurface    = lipgloss.Color("#181818")
	ColorBorder     = lipgloss.Color("#282828")
	ColorAccent     = lipgloss.Color("#1DB954")
	ColorPrimary    = lipgloss.Color("#FFFFFF")
	ColorSecondary  = lipgloss.Color("#B3B3B3")
	ColorError      = lipgloss.Color("#FF4444")
	ColorOrange     = lipgloss.Color("#FFA500")
	ColorRowHover   = lipgloss.Color("#1ED760")
	ColorBlack      = lipgloss.Color("#000000")
)

var (
	AppStyle = lipgloss.NewStyle().
		Background(ColorBackground)

	AccentStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	WhiteStyle  = lipgloss.NewStyle().Foreground(ColorPrimary)
	DimStyle    = lipgloss.NewStyle().Foreground(ColorSecondary)
	ErrorStyle  = lipgloss.NewStyle().Foreground(ColorError)
	OrangeStyle = lipgloss.NewStyle().Foreground(ColorOrange)
	BoldStyle   = lipgloss.NewStyle().Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Background(ColorBackground).
			Foreground(ColorPrimary).
			Bold(true)

	LogoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	LogoAccentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAccent)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	AccentBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent)

	InputStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorPrimary).
			Padding(0, 1)

	FocusedInputStyle = lipgloss.NewStyle().
				Background(ColorSurface).
				Foreground(ColorPrimary).
				Border(lipgloss.NormalBorder()).
				BorderForeground(ColorAccent).
				Padding(0, 1)

	SelectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E3223")).
				Foreground(ColorPrimary).
				Bold(true)

	NavActiveStyle = lipgloss.NewStyle().
			Background(ColorAccent).
			Foreground(ColorBlack).
			Bold(true).
			Padding(0, 1)

	NavInactiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#282828")).
				Foreground(ColorPrimary).
				Padding(0, 1)

	ButtonStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#282828")).
			Foreground(ColorAccent).
			Padding(0, 2)

	AccentButtonStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(ColorBlack).
				Bold(true).
				Padding(0, 2)

	ErrorButtonStyle = lipgloss.NewStyle().
				Background(ColorError).
				Foreground(ColorPrimary).
				Bold(true).
				Padding(0, 2)

	SectionTitleStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true).
				Padding(0, 1)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Render(strings.Repeat("─", 40))

	SurfaceStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Padding(0, 1)

	FaintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	GreenDotStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Render("●")

	DimDotStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Render("●")

	SongNumStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Width(3).
			Align(lipgloss.Right)

	SelectedBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3223"))
)

func VolumeColor(vol float64) lipgloss.Color {
	switch {
	case vol <= 0.33:
		return ColorAccent
	case vol <= 0.66:
		return ColorOrange
	default:
		return ColorError
	}
}

func VolumeBar(filled float64, width int) string {
	if width <= 0 {
		width = 10
	}
	n := int(filled * float64(width))
	if n < 0 {
		n = 0
	}
	if n > width {
		n = width
	}
	bar := ""
	for i := range width {
		if i < n {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}

func ProgressBar(pos, dur float64, width int) string {
	if width <= 0 {
		width = 20
	}
	ratio := 0.0
	if dur > 0 {
		ratio = pos / dur
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	n := int(ratio * float64(width))
	bar := ""
	for i := 0; i < width; i++ {
		if i < n {
			bar += "─"
		} else if i == n {
			bar += "●"
		} else {
			bar += "─"
		}
	}
	return bar
}

func FormatDuration(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

