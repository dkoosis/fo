// Package theme provides the v2 Tufte-Swiss theme system: structure
// (bold, dim, alignment) lives in the mono preset; color layers on top.
//
// Two presets, no interface. Mono is the base — Color calls Mono first
// and overlays chroma on the severity and outcome styles. NO_COLOR forces
// Mono regardless of TTY (this is checked by Default).
package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Theme bundles every style and glyph the renderer needs. One value per
// run, threaded through the views.
type Theme struct {
	Name string

	// Severity styles for static-analysis findings.
	Error   lipgloss.Style
	Warning lipgloss.Style
	Note    lipgloss.Style

	// Outcome styles for go test results.
	Pass       lipgloss.Style
	Fail       lipgloss.Style
	Skip       lipgloss.Style
	Panic      lipgloss.Style
	BuildError lipgloss.Style

	// Structural styles.
	Bold    lipgloss.Style
	Muted   lipgloss.Style
	Heading lipgloss.Style

	Icons Icons
}

// Icons are the Tufte-Swiss glyph set: minimal, no box-drawing.
// Bar / BarEmpty are the segments used by the paint package's bar
// primitive; Up / Down / Same drive the Delta view.
type Icons struct {
	Pass       string
	Fail       string
	Warn       string
	Note       string
	Panic      string
	BuildError string
	Bullet     string
	Bar        string
	BarEmpty   string
	Up         string
	Down       string
	Same       string
}

// Mono is the structure-only preset. Bold and dim do all the hierarchy
// work; color is absent. Safe in any environment, including NO_COLOR
// terminals, log files, and CI capture.
func Mono() Theme {
	bold := lipgloss.NewStyle().Bold(true)
	dim := lipgloss.NewStyle().Faint(true)

	return Theme{
		Name: "mono",

		Error:   bold,
		Warning: lipgloss.NewStyle(),
		Note:    dim,

		Pass:       lipgloss.NewStyle(),
		Fail:       bold,
		Skip:       dim,
		Panic:      bold,
		BuildError: bold,

		Bold:    bold,
		Muted:   dim,
		Heading: bold,

		Icons: Icons{
			Pass:       "+",
			Fail:       "x",
			Warn:       "!",
			Note:       ".",
			Panic:      "!!",
			BuildError: "X",
			Bullet:     "-",
			Bar:        "#",
			BarEmpty:   "-",
			Up:         "^",
			Down:       "v",
			Same:       "=",
		},
	}
}

// Color overlays chroma on Mono. The structural styles (Bold, Muted,
// Heading) are unchanged; severity and outcome get foreground colors.
// Glyphs upgrade from ASCII to Unicode where it adds clarity.
func Color() Theme {
	t := Mono()
	t.Name = "color"

	red := lipgloss.Color("196")
	orange := lipgloss.Color("214")
	yellow := lipgloss.Color("220")
	green := lipgloss.Color("34")
	gray := lipgloss.Color("242")
	magenta := lipgloss.Color("201")

	t.Error = t.Error.Foreground(red)
	t.Warning = lipgloss.NewStyle().Foreground(orange)
	t.Note = lipgloss.NewStyle().Foreground(gray)

	t.Pass = lipgloss.NewStyle().Foreground(green)
	t.Fail = t.Fail.Foreground(red)
	t.Skip = lipgloss.NewStyle().Foreground(yellow)
	t.Panic = t.Panic.Foreground(magenta)
	t.BuildError = t.BuildError.Foreground(red)

	t.Icons = Icons{
		Pass:       "✓",
		Fail:       "✗",
		Warn:       "⚠",
		Note:       "·",
		Panic:      "⚡",
		BuildError: "⛔",
		Bullet:     "•",
		Bar:        "█",
		BarEmpty:   "░",
		Up:         "▲",
		Down:       "▼",
		Same:       "·",
	}
	return t
}

// Default returns the right theme for the environment: Mono when
// NO_COLOR is set or when stdout is not a TTY; Color otherwise.
func Default(stdoutIsTTY bool) Theme {
	if os.Getenv("NO_COLOR") != "" || !stdoutIsTTY {
		return Mono()
	}
	return Color()
}
