package render

import "github.com/charmbracelet/lipgloss"

// Theme defines colors and icons for terminal rendering.
type Theme struct {
	Name    string
	Primary lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style
	Muted   lipgloss.Style
	Bold    lipgloss.Style
	Icons   ThemeIcons
}

// ThemeIcons defines the icon set for a theme.
type ThemeIcons struct {
	Pass   string
	Fail   string
	Warn   string
	Info   string
	WIP    string
	Bullet string
}

// DefaultTheme returns a vibrant color theme.
func DefaultTheme() Theme {
	return Theme{
		Name:    "default",
		Primary: lipgloss.NewStyle().Foreground(lipgloss.Color("39")),  // blue
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("34")),  // green
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("214")), // orange
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("196")), // red
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("242")), // gray
		Bold:    lipgloss.NewStyle().Bold(true),
		Icons: ThemeIcons{
			Pass:   "✓",
			Fail:   "✗",
			Warn:   "⚠",
			Info:   "●",
			WIP:    "○",
			Bullet: "·",
		},
	}
}

// OrcaTheme returns a muted, professional theme.
func OrcaTheme() Theme {
	return Theme{
		Name:    "orca",
		Primary: lipgloss.NewStyle().Foreground(lipgloss.Color("75")),  // pale blue
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("108")), // sage green
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("179")), // muted gold
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("167")), // muted red
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")), // lighter gray
		Bold:    lipgloss.NewStyle().Bold(true),
		Icons: ThemeIcons{
			Pass:   "✓",
			Fail:   "✗",
			Warn:   "!",
			Info:   "·",
			WIP:    "○",
			Bullet: "·",
		},
	}
}

// MonoTheme returns a monochrome theme (no colors).
func MonoTheme() Theme {
	return Theme{
		Name:    "mono",
		Primary: lipgloss.NewStyle(),
		Success: lipgloss.NewStyle(),
		Warning: lipgloss.NewStyle(),
		Error:   lipgloss.NewStyle(),
		Muted:   lipgloss.NewStyle(),
		Bold:    lipgloss.NewStyle().Bold(true),
		Icons: ThemeIcons{
			Pass:   "+",
			Fail:   "x",
			Warn:   "!",
			Info:   "*",
			WIP:    "-",
			Bullet: "-",
		},
	}
}

// ThemeByName returns a theme by name, defaulting to DefaultTheme.
func ThemeByName(name string) Theme {
	switch name {
	case "orca":
		return OrcaTheme()
	case "mono":
		return MonoTheme()
	default:
		return DefaultTheme()
	}
}
