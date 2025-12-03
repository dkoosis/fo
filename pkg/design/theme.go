// Package design implements pattern-based CLI output visualization.
//
// This file defines the new lipgloss-idiomatic theming system.
// Colors use lipgloss.Color format (color names, hex, or 256-color numbers).
// Styles are composed using lipgloss methods, not manual ANSI escapes.
package design

import "github.com/charmbracelet/lipgloss"

// Theme defines all visual styling for fo output.
// Colors use lipgloss format: color names ("red"), hex ("#ff0000"), or 256-color ("120").
type Theme struct {
	Name string

	// Semantic colors for status and UI elements
	Colors ThemeColors

	// Pre-built styles for common elements
	Styles ThemeStyles

	// Icons for status indicators
	Icons ThemeIcons

	// Border configuration
	Border ThemeBorder
}

// ThemeColors defines semantic color values.
// All colors use lipgloss format, NOT raw ANSI escapes.
type ThemeColors struct {
	// Status colors
	Primary lipgloss.Color // Main accent color (headers, labels)
	Success lipgloss.Color // Success state
	Warning lipgloss.Color // Warning state
	Error   lipgloss.Color // Error state

	// Text colors
	Text    lipgloss.Color // Normal text
	Muted   lipgloss.Color // De-emphasized text
	Subtle  lipgloss.Color // Very subtle (borders, separators)
	Inverse lipgloss.Color // Inverse text (on colored backgrounds)
}

// ThemeStyles provides pre-built lipgloss styles.
// These are computed from colors and cached for reuse.
type ThemeStyles struct {
	// Box style for task output containers
	Box lipgloss.Style

	// Header style for task labels
	Header lipgloss.Style

	// Status line styles
	StatusSuccess lipgloss.Style
	StatusWarning lipgloss.Style
	StatusError   lipgloss.Style

	// Text styles
	TextNormal lipgloss.Style
	TextMuted  lipgloss.Style
	TextBold   lipgloss.Style
}

// ThemeIcons defines icon characters for status indicators.
type ThemeIcons struct {
	Running string
	Success string
	Warning string
	Error   string
	Info    string
	Bullet  string
}

// ThemeBorder defines border characters and style.
type ThemeBorder struct {
	Style lipgloss.Border // lipgloss border definition
}

// NewTheme creates a theme with computed styles from colors.
func NewTheme(name string, colors ThemeColors, icons ThemeIcons) *Theme {
	t := &Theme{
		Name:   name,
		Colors: colors,
		Icons:  icons,
	}

	// Build styles from colors
	t.Styles = ThemeStyles{
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colors.Subtle).
			Padding(0, 1),

		Header: lipgloss.NewStyle().
			Foreground(colors.Primary).
			Bold(true),

		StatusSuccess: lipgloss.NewStyle().
			Foreground(colors.Success),

		StatusWarning: lipgloss.NewStyle().
			Foreground(colors.Warning),

		StatusError: lipgloss.NewStyle().
			Foreground(colors.Error),

		TextNormal: lipgloss.NewStyle().
			Foreground(colors.Text),

		TextMuted: lipgloss.NewStyle().
			Foreground(colors.Muted),

		TextBold: lipgloss.NewStyle().
			Foreground(colors.Text).
			Bold(true),
	}

	t.Border = ThemeBorder{
		Style: lipgloss.RoundedBorder(),
	}

	return t
}

// DefaultTheme returns the default fo theme.
func DefaultTheme() *Theme {
	return NewTheme(
		"default",
		ThemeColors{
			Primary: lipgloss.Color("39"),  // Bright blue
			Success: lipgloss.Color("120"), // Light green
			Warning: lipgloss.Color("214"), // Orange
			Error:   lipgloss.Color("196"), // Red
			Text:    lipgloss.Color("252"), // Light gray
			Muted:   lipgloss.Color("242"), // Dark gray
			Subtle:  lipgloss.Color("238"), // Very dark gray
			Inverse: lipgloss.Color("231"), // White
		},
		ThemeIcons{
			Running: "▶",
			Success: "✓",
			Warning: "⚠",
			Error:   "✗",
			Info:    "ℹ",
			Bullet:  "•",
		},
	)
}

// OrcaTheme returns the Orca-inspired theme.
func OrcaTheme2() *Theme {
	return NewTheme(
		"orca",
		ThemeColors{
			Primary: lipgloss.Color("111"), // Pale blue
			Success: lipgloss.Color("120"), // Light green
			Warning: lipgloss.Color("214"), // Orange
			Error:   lipgloss.Color("196"), // Red
			Text:    lipgloss.Color("252"), // Light gray
			Muted:   lipgloss.Color("242"), // Dark gray
			Subtle:  lipgloss.Color("250"), // Pale gray (lighter borders)
			Inverse: lipgloss.Color("231"), // White
		},
		ThemeIcons{
			Running: "▶",
			Success: "✓",
			Warning: "⚠",
			Error:   "✗",
			Info:    "ℹ",
			Bullet:  "•",
		},
	)
}

// MonochromeTheme returns a theme with no colors.
func MonochromeTheme() *Theme {
	return NewTheme(
		"monochrome",
		ThemeColors{
			// Empty colors = no styling
			Primary: lipgloss.Color(""),
			Success: lipgloss.Color(""),
			Warning: lipgloss.Color(""),
			Error:   lipgloss.Color(""),
			Text:    lipgloss.Color(""),
			Muted:   lipgloss.Color(""),
			Subtle:  lipgloss.Color(""),
			Inverse: lipgloss.Color(""),
		},
		ThemeIcons{
			Running: "[RUN]",
			Success: "[OK]",
			Warning: "[WARN]",
			Error:   "[FAIL]",
			Info:    "[INFO]",
			Bullet:  "*",
		},
	)
}

// ThemeFromConfig creates a Theme from an existing Config.
// This bridges the old Config system to the new Theme system during migration.
func ThemeFromConfig(cfg *Config) *Theme {
	if cfg == nil {
		return DefaultTheme()
	}

	if cfg.IsMonochrome {
		return MonochromeTheme()
	}

	// Map old theme names to new themes
	switch cfg.ThemeName {
	case "orca":
		return OrcaTheme2()
	case "ascii_minimal":
		return MonochromeTheme()
	}

	// For other themes, build from config values
	// Use proper lipgloss color format (256-color numbers)
	colors := ThemeColors{
		Primary: lipgloss.Color("39"),  // Default bright blue
		Success: lipgloss.Color("120"), // Light green
		Warning: lipgloss.Color("214"), // Orange
		Error:   lipgloss.Color("196"), // Red
		Text:    lipgloss.Color("252"), // Light gray
		Muted:   lipgloss.Color("242"), // Dark gray
		Subtle:  lipgloss.Color("238"), // Very dark gray
		Inverse: lipgloss.Color("231"), // White
	}

	icons := ThemeIcons{
		Running: cfg.Icons.Start,
		Success: cfg.Icons.Success,
		Warning: cfg.Icons.Warning,
		Error:   cfg.Icons.Error,
		Info:    cfg.Icons.Info,
		Bullet:  cfg.Icons.Bullet,
	}

	// Use defaults for empty icons
	if icons.Running == "" {
		icons.Running = "▶"
	}
	if icons.Success == "" {
		icons.Success = "✓"
	}
	if icons.Warning == "" {
		icons.Warning = "⚠"
	}
	if icons.Error == "" {
		icons.Error = "✗"
	}
	if icons.Info == "" {
		icons.Info = "ℹ"
	}
	if icons.Bullet == "" {
		icons.Bullet = "•"
	}

	return NewTheme(cfg.ThemeName, colors, icons)
}
