package dashboard

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DashboardConfig is the top-level structure for dashboard configuration in .fo.yaml
type DashboardConfig struct {
	Dashboard *DashboardTheme `yaml:"dashboard"`
}

// LoadThemeFromConfig loads the dashboard theme from .fo.yaml if present.
// Falls back to default theme if not found or on error.
func LoadThemeFromConfig() *DashboardTheme {
	configPath := findConfigPath()
	if configPath == "" {
		return DefaultDashboardTheme()
	}

	// #nosec G304 -- configPath is from findConfigPath which only returns local or XDG paths
	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultDashboardTheme()
	}

	var cfg DashboardConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultDashboardTheme()
	}

	if cfg.Dashboard == nil {
		return DefaultDashboardTheme()
	}

	// Merge with defaults for any unset fields
	theme := mergeWithDefaults(cfg.Dashboard)
	return theme
}

// findConfigPath looks for .fo.yaml in current dir or XDG config.
func findConfigPath() string {
	// Try local path first
	if _, err := os.Stat(".fo.yaml"); err == nil {
		return ".fo.yaml"
	}

	// Try XDG config dir
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	xdgPath := filepath.Join(configDir, "fo", ".fo.yaml")
	if _, err := os.Stat(xdgPath); err == nil {
		return xdgPath
	}

	return ""
}

// mergeWithDefaults fills in missing values from the default theme.
func mergeWithDefaults(theme *DashboardTheme) *DashboardTheme {
	def := DefaultDashboardTheme()

	// Colors
	if theme.Colors.Primary == "" {
		theme.Colors.Primary = def.Colors.Primary
	}
	if theme.Colors.Success == "" {
		theme.Colors.Success = def.Colors.Success
	}
	if theme.Colors.Error == "" {
		theme.Colors.Error = def.Colors.Error
	}
	if theme.Colors.Warning == "" {
		theme.Colors.Warning = def.Colors.Warning
	}
	if theme.Colors.Muted == "" {
		theme.Colors.Muted = def.Colors.Muted
	}
	if theme.Colors.Text == "" {
		theme.Colors.Text = def.Colors.Text
	}
	if theme.Colors.Border == "" {
		theme.Colors.Border = def.Colors.Border
	}
	if theme.Colors.Highlight == "" {
		theme.Colors.Highlight = def.Colors.Highlight
	}

	// Icons
	if theme.Icons.Pending == "" {
		theme.Icons.Pending = def.Icons.Pending
	}
	if theme.Icons.Running == "" {
		theme.Icons.Running = def.Icons.Running
	}
	if theme.Icons.Success == "" {
		theme.Icons.Success = def.Icons.Success
	}
	if theme.Icons.Error == "" {
		theme.Icons.Error = def.Icons.Error
	}
	if theme.Icons.Group == "" {
		theme.Icons.Group = def.Icons.Group
	}
	if theme.Icons.Select == "" {
		theme.Icons.Select = def.Icons.Select
	}

	// Title
	if theme.Title.Text == "" {
		theme.Title.Text = def.Title.Text
	}
	if theme.Title.Icon == "" {
		theme.Title.Icon = def.Title.Icon
	}

	// Spinner
	if theme.Spinner.Frames == "" {
		theme.Spinner.Frames = def.Spinner.Frames
	}
	if theme.Spinner.Interval == 0 {
		theme.Spinner.Interval = def.Spinner.Interval
	}

	// Subsystems - use defaults if none specified
	if len(theme.Subsystems) == 0 {
		theme.Subsystems = def.Subsystems
	}

	return theme
}

// InitThemeFromConfig loads and sets the dashboard theme from config.
// Call this early in main() before running the dashboard.
func InitThemeFromConfig() {
	theme := LoadThemeFromConfig()
	SetTheme(theme)
}
