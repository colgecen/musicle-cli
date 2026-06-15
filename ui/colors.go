package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ThemeColors maps theme names to accent hex colors
var ThemeColors = map[string]string{
	"green":  "#1DB954",
	"red":    "#FF4444",
	"pink":   "#FF69B4",
	"purple": "#BB86FC",
	"blue":   "#4488FF",
	"orange": "#FFA500",
	"yellow": "#FFD700",
}

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

// Styles are mutable — call ApplyTheme() to rebuild
var (
	AppStyle         lipgloss.Style
	AccentStyle      lipgloss.Style
	WhiteStyle       lipgloss.Style
	DimStyle         lipgloss.Style
	ErrorStyle       lipgloss.Style
	OrangeStyle      lipgloss.Style
	BoldStyle        lipgloss.Style
	HeaderStyle      lipgloss.Style
	LogoStyle        lipgloss.Style
	LogoAccentStyle  lipgloss.Style
	BorderStyle      lipgloss.Style
	AccentBorderStyle  lipgloss.Style
	InputStyle       lipgloss.Style
	FocusedInputStyle  lipgloss.Style
	SelectedRowStyle lipgloss.Style
	NavActiveStyle   lipgloss.Style
	NavInactiveStyle lipgloss.Style
	ButtonStyle      lipgloss.Style
	AccentButtonStyle   lipgloss.Style
	ErrorButtonStyle    lipgloss.Style
	SectionTitleStyle   lipgloss.Style
	SeparatorStyle      string
	SurfaceStyle        lipgloss.Style
	FaintStyle          lipgloss.Style
	GreenDotStyle       string
	DimDotStyle         string
	SongNumStyle        lipgloss.Style
	SelectedBgStyle     lipgloss.Style
	FocusedButtonStyle  lipgloss.Style
	FocusedOutlineStyle lipgloss.Style
)

// InitStyles builds all styles with the current ColorAccent
func InitStyles() {
	AppStyle = lipgloss.NewStyle().
		Background(ColorBackground)

	AccentStyle = lipgloss.NewStyle().Foreground(ColorAccent)
	WhiteStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
	DimStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
	ErrorStyle = lipgloss.NewStyle().Foreground(ColorError)
	OrangeStyle = lipgloss.NewStyle().Foreground(ColorOrange)
	BoldStyle = lipgloss.NewStyle().Bold(true)

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
			Padding(0, 2)

	NavInactiveStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#282828")).
				Foreground(ColorPrimary).
				Padding(0, 2)

	ButtonStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#282828")).
			Foreground(ColorAccent).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#282828")).
			Padding(0, 2)

	AccentButtonStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(ColorBlack).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorAccent).
				Padding(0, 2)

	ErrorButtonStyle = lipgloss.NewStyle().
				Background(ColorError).
				Foreground(ColorPrimary).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorError).
				Padding(0, 2)

	FocusedButtonStyle = lipgloss.NewStyle().
				Background(ColorAccent).
				Foreground(ColorBlack).
				Bold(true).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 2)

	FocusedOutlineStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#282828")).
				Foreground(ColorAccent).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 2)

	SectionTitleStyle = lipgloss.NewStyle().
				Foreground(ColorAccent).
				Bold(true).
				Padding(0, 1)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Render(strings.Repeat("-", 40))

	SurfaceStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Padding(0, 1)

	FaintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	GreenDotStyle = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Render("o")

	DimDotStyle = lipgloss.NewStyle().
			Foreground(ColorBorder).
			Render("o")

	SongNumStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Width(3).
			Align(lipgloss.Right)

	SelectedBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3223"))
}

// ApplyTheme updates ColorAccent and rebuilds all styles
func ApplyTheme(name string) {
	hex, ok := ThemeColors[name]
	if !ok {
		return
	}
	ColorAccent = lipgloss.Color(hex)
	InitStyles()
}

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
			bar += "-"
		} else if i == n {
			bar += "o"
		} else {
			bar += "-"
		}
	}
	return bar
}

func FormatDuration(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

var spectrumColors = []lipgloss.Color{
	lipgloss.Color("#FF0000"),
	lipgloss.Color("#FF3300"),
	lipgloss.Color("#FF6600"),
	lipgloss.Color("#FF9900"),
	lipgloss.Color("#FFCC00"),
	lipgloss.Color("#FFFF00"),
	lipgloss.Color("#AAFF00"),
	lipgloss.Color("#55FF00"),
	lipgloss.Color("#00FF44"),
	lipgloss.Color("#00FFAA"),
	lipgloss.Color("#00CCFF"),
	lipgloss.Color("#0099FF"),
	lipgloss.Color("#4466FF"),
	lipgloss.Color("#8833FF"),
	lipgloss.Color("#BB00FF"),
	lipgloss.Color("#FF00CC"),
}

// ░▒▓█ are in CP437 — guaranteed to exist in every Windows console font since 1985.
// We also use a "brightness" variant of each color so low values look dim, high values glow.
var specShades = []string{" ", "░", "▒", "▓", "█"}

// smoothing buffers
var (
	prevSpecVal   [16]float64
	prevSpecPeak  [16]float64
	prevSpecTime  time.Time
)

const (
	specSmoothAlpha = 0.15       // heavy smoothing (85% old → no flicker)
	specPeakDecay   = 1.8        // peak dot decay speed
)

func SpectrumAnalyzer(spec [16]float64, bands int) string {
	if bands < 2 || bands > 16 {
		bands = 16
	}
	maxShade := len(specShades) - 1 // 4

	now := time.Now()
	dt := 0.1
	if !prevSpecTime.IsZero() {
		dt = now.Sub(prevSpecTime).Seconds()
	}
	prevSpecTime = now

	// Pick evenly-spaced band indices
	idxs := make([]int, bands)
	for i := range idxs {
		idxs[i] = i * 15 / (bands - 1)
	}

	var out strings.Builder
	for _, bi := range idxs {
		v := spec[bi]
		if math.IsNaN(v) || v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}

		// Heavy smoothing — 85% old, 15% new
		prevSpecVal[bi] = prevSpecVal[bi]*(1-specSmoothAlpha) + v*specSmoothAlpha
		cur := prevSpecVal[bi]

		// Peak hold & decay
		if cur >= prevSpecPeak[bi] {
			prevSpecPeak[bi] = cur
		} else {
			prevSpecPeak[bi] *= math.Exp(-dt * specPeakDecay)
			if prevSpecPeak[bi] < cur {
				prevSpecPeak[bi] = cur
			}
		}

		// Shade index
		si := int(cur * float64(maxShade))
		if si > maxShade {
			si = maxShade
		}
		if si < 0 || cur < 0.01 {
			si = 0
		}

		// Peak index
		pi := int(prevSpecPeak[bi] * float64(maxShade))
		if pi > maxShade {
			pi = maxShade
		}
		if pi < 0 {
			pi = 0
		}

		// Color: dim for low, band color for mid, white for peak
		var styled string
		if pi > si && prevSpecPeak[bi] > 0.1 {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(specShades[pi])
		} else if cur < 0.15 {
			styled = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(specShades[si])
		} else {
			c := spectrumColors[bi]
			// Brighten color for higher values
			if cur > 0.6 {
				c = lipgloss.Color("#FFFFFF")
			}
			styled = lipgloss.NewStyle().Foreground(c).Render(specShades[si])
		}
		out.WriteString(styled)
	}
	return out.String()
}

