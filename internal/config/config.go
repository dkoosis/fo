// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/dkoosis/fo/pkg/design"
	"gopkg.in/yaml.v3"
)

// CliFlags holds the values of command-line flags.
type CliFlags struct {
	Label            string
	LiveStreamOutput bool
	ShowOutput       string
	PatternHint      string // Manual pattern selection (e.g., "test-table", "sparkline", "leaderboard")
	Format           string // Output format: "text" (default) or "json"
	Profile          bool   // Enable performance profiling
	ProfileOutput    string // Profile output destination: "stderr" or file path
	NoTimer          bool
	NoColor          bool
	CI               bool
	Debug            bool
	MaxBufferSize    int64 // In bytes, passed from main after parsing
	MaxLineLength    int   // In bytes, passed from main after parsing
	ThemeName        string
	ThemeFile        string // Path to custom theme YAML file

	// Flags to track if they were explicitly set by the user
	LiveStreamOutputSet bool
	ShowOutputSet       bool
	PatternHintSet      bool
	NoTimerSet          bool
	NoColorSet          bool
	CISet               bool
	DebugSet            bool
}

// AppConfig represents the application's overall configuration from .fo.yaml.
type AppConfig struct {
	Label            string                        `yaml:"label,omitempty"`
	LiveStreamOutput bool                          `yaml:"live_stream_output"`
	ShowOutput       string                        `yaml:"show_output"`
	NoTimer          bool                          `yaml:"no_timer"`
	NoColor          bool                          `yaml:"no_color"`
	CI               bool                          `yaml:"ci"`
	Debug            bool                          `yaml:"debug"`
	MaxBufferSize    int64                         `yaml:"max_buffer_size"` // In bytes
	MaxLineLength    int                           `yaml:"max_line_length"` // In bytes
	ActiveThemeName  string                        `yaml:"active_theme"`
	Presets          map[string]*design.ToolConfig `yaml:"presets"` // Uses design.ToolConfig
	Themes           map[string]*design.Config     `yaml:"themes"`  // Holds fully resolved design.Config objects
}

// Constants for default values.
const (
	DefaultShowOutput      = "on-fail"
	DefaultMaxBufferSize   = 10 * 1024 * 1024 // 10MB
	DefaultMaxLineLength   = 1 * 1024 * 1024  // 1MB
	DefaultActiveThemeName = "unicode_vibrant"
)

// LoadConfig loads the .fo.yaml configuration.
func LoadConfig() *AppConfig {
	// Initialize with default themes from pkg/design (single source of truth).
	appCfg := &AppConfig{
		LiveStreamOutput: false,
		ShowOutput:       DefaultShowOutput,
		NoTimer:          false,
		NoColor:          false,
		CI:               false,
		Debug:            false, // Debug will be determined by CLI flags or YAML later
		MaxBufferSize:    DefaultMaxBufferSize,
		MaxLineLength:    DefaultMaxLineLength,
		ActiveThemeName:  DefaultActiveThemeName,
		Themes:           design.DefaultThemes(), // Single source of truth for default themes
		Presets:          make(map[string]*design.ToolConfig),
	}

	initialDebug := os.Getenv("FO_DEBUG") != ""
	debugEnabled := initialDebug

	if debugEnabled {
		fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Initial Hardcoded Themes ---")
		for themeName, themeCfg := range appCfg.Themes {
			if themeCfg != nil {
				fmt.Fprintf(os.Stderr, "Theme: %s (IsMonochrome: %t)\n", themeName, themeCfg.IsMonochrome)
			}
		}
		fmt.Fprintln(os.Stderr, "-------------------------------------------------")
	}

	configPath := getConfigPath() // Uses the corrected getConfigPath
	if configPath == "" {
		if debugEnabled {
			fmt.Fprintln(os.Stderr, "[DEBUG LoadConfig] No .fo.yaml config file found or user config dir problematic, using defaults only.")
		}
		return appCfg
	}

	// #nosec G304 -- configPath is from getConfigPath() which returns only local or XDG config paths
	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file %s: %v. Using defaults.\n", configPath, err)
		} else if debugEnabled {
			fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Config file %s not found. Using defaults.\n", configPath)
		}
		return appCfg
	}

	var yamlAppCfg AppConfig
	err = yaml.Unmarshal(yamlFile, &yamlAppCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error unmarshalling config file %s: %v. Using defaults.\n", configPath, err)
		return appCfg
	}

	// Merge YAML settings onto the base appCfg
	if yamlAppCfg.Label != "" {
		appCfg.Label = yamlAppCfg.Label
	}
	appCfg.LiveStreamOutput = yamlAppCfg.LiveStreamOutput
	if yamlAppCfg.ShowOutput != "" {
		appCfg.ShowOutput = yamlAppCfg.ShowOutput
	}
	appCfg.NoTimer = yamlAppCfg.NoTimer
	appCfg.NoColor = yamlAppCfg.NoColor
	appCfg.CI = yamlAppCfg.CI
	appCfg.Debug = yamlAppCfg.Debug

	if yamlAppCfg.MaxBufferSize > 0 {
		appCfg.MaxBufferSize = yamlAppCfg.MaxBufferSize
	}
	if yamlAppCfg.MaxLineLength > 0 {
		appCfg.MaxLineLength = yamlAppCfg.MaxLineLength
	}
	if yamlAppCfg.ActiveThemeName != "" {
		appCfg.ActiveThemeName = yamlAppCfg.ActiveThemeName
	}
	if yamlAppCfg.Presets != nil {
		appCfg.Presets = yamlAppCfg.Presets
	}

	if yamlAppCfg.Themes != nil {
		if appCfg.Debug || debugEnabled {
			fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Processing Themes from YAML ---")
		}
		for name, themeFromFile := range yamlAppCfg.Themes {
			copiedTheme := design.DeepCopyConfig(themeFromFile)
			if copiedTheme != nil {
				copiedTheme.ThemeName = name
				appCfg.Themes[name] = copiedTheme
				if appCfg.Debug || debugEnabled {
					fmt.Fprintf(os.Stderr, "Loaded/Overwrote Theme from YAML: %s (IsMonochrome: %t)\n", name, copiedTheme.IsMonochrome)
				}
			}
		}
		if appCfg.Debug || debugEnabled {
			fmt.Fprintln(os.Stderr, "---------------------------------------------------")
		}
	}

	if _, ok := appCfg.Themes[appCfg.ActiveThemeName]; !ok {
		if appCfg.Debug || debugEnabled {
			fmt.Fprintf(os.Stderr,
				"[DEBUG LoadConfig] Active theme '%s' not found. Falling back to '%s'.\n",
				appCfg.ActiveThemeName, DefaultActiveThemeName)
		}
		appCfg.ActiveThemeName = DefaultActiveThemeName
	}

	if appCfg.Debug || debugEnabled {
		fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Loaded config from %s. Final Active theme: %s.\n", configPath, appCfg.ActiveThemeName)
	}
	return appCfg
}

// getConfigPath tries to find the .fo.yaml configuration file.
// It checks local directory first, then XDG UserConfigDir (if valid).
func getConfigPath() string {
	debugEnabled := os.Getenv("FO_DEBUG") != ""

	// Try local path first
	localPath := ".fo.yaml"
	if _, err := os.Stat(localPath); err == nil {
		// If FO_DEBUG is set, print that we're using the local path
		if debugEnabled {
			absLocalPath, _ := filepath.Abs(localPath)
			fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] Using local config file: %s\n", absLocalPath)
		}
		return localPath
	}

	configHome, err := os.UserConfigDir()
	// If UserConfigDir fails OR returns an empty path or "/", it's not suitable for XDG path construction here.
	if err == nil && configHome != "" && configHome != "/" {
		// Construct path like /home/user/.config/fo/.fo.yaml
		// Ensure "fo" subdirectory is part of the path.
		xdgPath := filepath.Join(configHome, "fo", ".fo.yaml")
		if _, errStat := os.Stat(xdgPath); errStat == nil {
			// If FO_DEBUG is set, print that we're using the XDG path
			if debugEnabled {
				fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] Using XDG config file: %s\n", xdgPath)
			}
			return xdgPath
		}
		// If FO_DEBUG is set and XDG path not found, print that.
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] XDG config file not found at: %s\n", xdgPath)
		}
	} else if debugEnabled {
		// If FO_DEBUG is set and UserConfigDir was problematic, print that.
		fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] UserConfigDir error or unsuitable path. Error: %v, Path: '%s'\n", err, configHome)
	}

	// Fallback or if XDG path is not viable/found
	if debugEnabled {
		fmt.Fprintln(os.Stderr, "[DEBUG getConfigPath] No config file found. Will use default settings.")
	}
	return "" // No config file found or UserConfigDir was problematic
}

// LoadThemeFromFile loads a custom theme from a YAML file.
func LoadThemeFromFile(filePath string) (*design.Config, error) {
	// #nosec G304 -- filePath is from user-provided --theme-file flag
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading theme file %s: %w", filePath, err)
	}

	var themeConfig design.Config
	err = yaml.Unmarshal(yamlFile, &themeConfig)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling theme file %s: %w", filePath, err)
	}

	// Set theme name based on file name
	baseName := filepath.Base(filePath)
	themeConfig.ThemeName = baseName

	return &themeConfig, nil
}

// ThemeOverrides represents partial theme configuration for composition.
// Only fields that are set will override the base theme.
type ThemeOverrides struct {
	Colors struct {
		Process *string `yaml:"process,omitempty"`
		Success *string `yaml:"success,omitempty"`
		Warning *string `yaml:"warning,omitempty"`
		Error   *string `yaml:"error,omitempty"`
		Detail  *string `yaml:"detail,omitempty"`
		Muted   *string `yaml:"muted,omitempty"`
		Spinner *string `yaml:"spinner,omitempty"`
		White   *string `yaml:"white,omitempty"`
		GreenFg *string `yaml:"green_fg,omitempty"`
		BlueFg  *string `yaml:"blue_fg,omitempty"`
		BlueBg  *string `yaml:"blue_bg,omitempty"`
		Bold    *string `yaml:"bold,omitempty"`
		Italic  *string `yaml:"italic,omitempty"`
	} `yaml:"colors,omitempty"`
	Style struct {
		UseBoxes          *bool   `yaml:"use_boxes,omitempty"`
		Indentation       *string `yaml:"indentation,omitempty"`
		ShowTimestamps    *bool   `yaml:"show_timestamps,omitempty"`
		NoTimer           *bool   `yaml:"no_timer,omitempty"`
		Density           *string `yaml:"density,omitempty"`
		HeaderWidth       *int    `yaml:"header_width,omitempty"`
	} `yaml:"style,omitempty"`
	Border struct {
		HeaderChar             *string `yaml:"header_char,omitempty"`
		VerticalChar           *string `yaml:"vertical_char,omitempty"`
		TopCornerChar          *string `yaml:"top_corner_char,omitempty"`
		TopRightChar           *string `yaml:"top_right_char,omitempty"`
		BottomCornerChar       *string `yaml:"bottom_corner_char,omitempty"`
		BottomRightChar        *string `yaml:"bottom_right_char,omitempty"`
		FooterContinuationChar *string `yaml:"footer_continuation_char,omitempty"`
	} `yaml:"border,omitempty"`
}

// LoadThemeOverrides loads theme overrides from a YAML file.
// Returns a ThemeOverrides struct that can be merged with a base theme.
func LoadThemeOverrides(filePath string) (*ThemeOverrides, error) {
	// #nosec G304 -- filePath is from user-provided path
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading theme override file %s: %w", filePath, err)
	}

	var overrides ThemeOverrides
	err = yaml.Unmarshal(yamlFile, &overrides)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling theme override file %s: %w", filePath, err)
	}

	return &overrides, nil
}

// MergeThemes merges a base theme with overrides using Lip Gloss inheritance.
// Base theme provides defaults, overrides selectively override specific values.
// Returns a new Config with merged values.
func MergeThemes(base *design.Config, overrides *ThemeOverrides) *design.Config {
	if overrides == nil {
		return design.DeepCopyConfig(base)
	}

	merged := design.DeepCopyConfig(base)
	if merged == nil {
		return base
	}

	// Merge colors (simple field override, convert to lipgloss.Color)
	if overrides.Colors.Process != nil {
		merged.Tokens.Colors.Process = lipgloss.Color(*overrides.Colors.Process)
	}
	if overrides.Colors.Success != nil {
		merged.Tokens.Colors.Success = lipgloss.Color(*overrides.Colors.Success)
	}
	if overrides.Colors.Warning != nil {
		merged.Tokens.Colors.Warning = lipgloss.Color(*overrides.Colors.Warning)
	}
	if overrides.Colors.Error != nil {
		merged.Tokens.Colors.Error = lipgloss.Color(*overrides.Colors.Error)
	}
	if overrides.Colors.Detail != nil {
		merged.Tokens.Colors.Detail = lipgloss.Color(*overrides.Colors.Detail)
	}
	if overrides.Colors.Muted != nil {
		merged.Tokens.Colors.Muted = lipgloss.Color(*overrides.Colors.Muted)
	}
	if overrides.Colors.Spinner != nil {
		merged.Tokens.Colors.Spinner = lipgloss.Color(*overrides.Colors.Spinner)
	}
	if overrides.Colors.White != nil {
		merged.Tokens.Colors.White = lipgloss.Color(*overrides.Colors.White)
	}
	if overrides.Colors.GreenFg != nil {
		merged.Tokens.Colors.GreenFg = lipgloss.Color(*overrides.Colors.GreenFg)
	}
	if overrides.Colors.BlueFg != nil {
		merged.Tokens.Colors.BlueFg = lipgloss.Color(*overrides.Colors.BlueFg)
	}
	if overrides.Colors.BlueBg != nil {
		merged.Tokens.Colors.BlueBg = lipgloss.Color(*overrides.Colors.BlueBg)
	}
	if overrides.Colors.Bold != nil {
		merged.Tokens.Colors.Bold = lipgloss.Color(*overrides.Colors.Bold)
	}
	if overrides.Colors.Italic != nil {
		merged.Tokens.Colors.Italic = lipgloss.Color(*overrides.Colors.Italic)
	}

	// Merge style settings
	if overrides.Style.UseBoxes != nil {
		merged.Style.UseBoxes = *overrides.Style.UseBoxes
	}
	if overrides.Style.Indentation != nil {
		merged.Style.Indentation = *overrides.Style.Indentation
	}
	if overrides.Style.ShowTimestamps != nil {
		merged.Style.ShowTimestamps = *overrides.Style.ShowTimestamps
	}
	if overrides.Style.NoTimer != nil {
		merged.Style.NoTimer = *overrides.Style.NoTimer
	}
	if overrides.Style.Density != nil {
		merged.Style.Density = *overrides.Style.Density
	}
	if overrides.Style.HeaderWidth != nil {
		merged.Style.HeaderWidth = *overrides.Style.HeaderWidth
	}

	// Merge border settings
	if overrides.Border.HeaderChar != nil {
		merged.Border.HeaderChar = *overrides.Border.HeaderChar
	}
	if overrides.Border.VerticalChar != nil {
		merged.Border.VerticalChar = *overrides.Border.VerticalChar
	}
	if overrides.Border.TopCornerChar != nil {
		merged.Border.TopCornerChar = *overrides.Border.TopCornerChar
	}
	// Note: TopRightChar and BottomRightChar will be available after #122 is merged
	// For now, skip these fields
	if overrides.Border.BottomCornerChar != nil {
		merged.Border.BottomCornerChar = *overrides.Border.BottomCornerChar
	}
	if overrides.Border.FooterContinuationChar != nil {
		merged.Border.FooterContinuationChar = *overrides.Border.FooterContinuationChar
	}

	// Sync tokens to colors for backwards compatibility
	// Note: syncTokensToColors is unexported, but it's called automatically
	// when Config is used. We'll rely on that behavior.

	// Use Lip Gloss style inheritance for Styles struct
	// Base styles inherit from base theme, overrides inherit from base
	if merged.Tokens != nil {
		// Box style: inherit border settings
		if merged.Border.TopCornerChar != "" {
			border := lipgloss.Border{
				Top:        merged.Border.HeaderChar,
				Bottom:     merged.Border.FooterContinuationChar,
				Left:       merged.Border.VerticalChar,
				Right:      merged.Border.VerticalChar,
				TopLeft:    merged.Border.TopCornerChar,
				BottomLeft: merged.Border.BottomCornerChar,
				// TopRight and BottomRight will be added after #122 is merged
			}
			baseBoxStyle := lipgloss.NewStyle().Border(border)
			merged.Tokens.Styles.Box = baseBoxStyle.Inherit(merged.Tokens.Styles.Box)
		}

		// Header style: inherit from Process color
		headerStyle := lipgloss.NewStyle().
			Foreground(merged.Tokens.Colors.Process).
			Bold(true)
		merged.Tokens.Styles.Header = headerStyle.Inherit(merged.Tokens.Styles.Header)

		// Content style: inherit from Detail color
		contentStyle := lipgloss.NewStyle().
			Foreground(merged.Tokens.Colors.Detail)
		merged.Tokens.Styles.Content = contentStyle.Inherit(merged.Tokens.Styles.Content)
	}

	return merged
}

// ApplyCommandPreset modifies the AppConfig based on a preset matching the commandName.
func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	baseCommand := filepath.Base(commandName)
	keysToTry := []string{commandName, baseCommand}
	debugEnabled := appCfg.Debug || os.Getenv("FO_DEBUG") != ""

	for _, cmdKey := range keysToTry {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			if debugEnabled {
				fmt.Fprintf(os.Stderr, "[DEBUG ApplyCommandPreset] Applying preset for '%s'\n", cmdKey)
			}
			if preset.Label != "" {
				appCfg.Label = preset.Label
			}
			// Apply other preset fields to appCfg if they exist in design.ToolConfig
			// e.g., if preset.StreamIsSet { appCfg.Stream = preset.StreamValue }
			return
		}
	}
}
