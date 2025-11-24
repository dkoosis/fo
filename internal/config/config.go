// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dkoosis/fo/pkg/design"
	"gopkg.in/yaml.v3"
)

// CliFlags holds the values of command-line flags.
type CliFlags struct {
	Label         string
	Stream        bool
	ShowOutput    string
	Pattern       string // Manual pattern selection (e.g., "test-table", "sparkline", "leaderboard")
	NoTimer       bool
	NoColor       bool
	CI            bool
	Debug         bool
	MaxBufferSize int64  // In bytes, passed from main after parsing
	MaxLineLength int    // In bytes, passed from main after parsing
	ThemeName     string
	ThemeFile     string // Path to custom theme YAML file

	// Flags to track if they were explicitly set by the user
	StreamSet     bool
	ShowOutputSet bool
	PatternSet    bool
	NoTimerSet    bool
	NoColorSet    bool
	CISet         bool
	DebugSet      bool
}

// AppConfig represents the application's overall configuration from .fo.yaml.
type AppConfig struct {
	Label           string                        `yaml:"label,omitempty"`
	Stream          bool                          `yaml:"stream"`
	ShowOutput      string                        `yaml:"show_output"`
	NoTimer         bool                          `yaml:"no_timer"`
	NoColor         bool                          `yaml:"no_color"`
	CI              bool                          `yaml:"ci"`
	Debug           bool                          `yaml:"debug"`
	MaxBufferSize   int64                         `yaml:"max_buffer_size"` // In bytes
	MaxLineLength   int                           `yaml:"max_line_length"` // In bytes
	ActiveThemeName string                        `yaml:"active_theme"`
	Presets         map[string]*design.ToolConfig `yaml:"presets"` // Uses design.ToolConfig
	Themes          map[string]*design.Config     `yaml:"themes"`  // Holds fully resolved design.Config objects
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
		Stream:          false,
		ShowOutput:      DefaultShowOutput,
		NoTimer:         false,
		NoColor:         false,
		CI:              false,
		Debug:           false, // Debug will be determined by CLI flags or YAML later
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: DefaultActiveThemeName,
		Themes:          design.DefaultThemes(), // Single source of truth for default themes
		Presets:         make(map[string]*design.ToolConfig),
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
	appCfg.Stream = yamlAppCfg.Stream
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
				// Normalize color strings after YAML unmarshal to ensure proper ANSI escape sequences
				normalizeThemeColors(copiedTheme)
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

// normalizeThemeColors normalizes all ANSI escape sequences in a theme's color settings
// after YAML unmarshal. This ensures colors are properly formatted regardless of how
// they were specified in the YAML file.
func normalizeThemeColors(cfg *design.Config) {
	if cfg == nil || cfg.IsMonochrome {
		return
	}

	// Use the canonical ANSI normalization from the design package
	cfg.Colors.Process = design.NormalizeANSIEscape(cfg.Colors.Process)
	cfg.Colors.Success = design.NormalizeANSIEscape(cfg.Colors.Success)
	cfg.Colors.Warning = design.NormalizeANSIEscape(cfg.Colors.Warning)
	cfg.Colors.Error = design.NormalizeANSIEscape(cfg.Colors.Error)
	cfg.Colors.Detail = design.NormalizeANSIEscape(cfg.Colors.Detail)
	cfg.Colors.Muted = design.NormalizeANSIEscape(cfg.Colors.Muted)
	cfg.Colors.Reset = design.NormalizeANSIEscape(cfg.Colors.Reset)
	cfg.Colors.White = design.NormalizeANSIEscape(cfg.Colors.White)
	cfg.Colors.BlueFg = design.NormalizeANSIEscape(cfg.Colors.BlueFg)
	cfg.Colors.BlueBg = design.NormalizeANSIEscape(cfg.Colors.BlueBg)
	cfg.Colors.Bold = design.NormalizeANSIEscape(cfg.Colors.Bold)
	cfg.Colors.Italic = design.NormalizeANSIEscape(cfg.Colors.Italic)
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

	// Normalize color strings after YAML unmarshal
	normalizeThemeColors(&themeConfig)

	return &themeConfig, nil
}

// MergeWithFlags takes the application config (post-YAML and presets) and CLI flags,
// and returns the final design.Config to be used for rendering.
func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	envDebug := os.Getenv("FO_DEBUG") != ""

	// Priority: --theme-file > --theme > config active_theme > default
	var finalDesignConfig *design.Config
	effectiveThemeName := appCfg.ActiveThemeName

	// 1. Check for --theme-file flag (highest priority)
	if cliFlags.ThemeFile != "" {
		loadedTheme, err := LoadThemeFromFile(cliFlags.ThemeFile)
		if err != nil {
			if appCfg.Debug || envDebug {
				fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Error loading theme file: %v. Falling back to default.\n", err)
			}
			finalDesignConfig = design.UnicodeVibrantTheme()
		} else {
			if appCfg.Debug || envDebug {
				fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Loaded theme from file: %s\n", cliFlags.ThemeFile)
			}
			finalDesignConfig = loadedTheme
			effectiveThemeName = loadedTheme.ThemeName
		}
	}

	// 2. If no theme file, check --theme flag or config
	if finalDesignConfig == nil {
		if cliFlags.ThemeName != "" {
			effectiveThemeName = cliFlags.ThemeName
		}

		var themeExists bool
		finalDesignConfig, themeExists = appCfg.Themes[effectiveThemeName]
		if !themeExists {
			if appCfg.Debug || envDebug {
				fmt.Fprintf(os.Stderr,
					"[DEBUG MergeWithFlags] Theme '%s' not found. Falling back to '%s'.\n",
					effectiveThemeName, DefaultActiveThemeName)
			}
			finalDesignConfig = appCfg.Themes[DefaultActiveThemeName]
			if finalDesignConfig == nil {
				if appCfg.Debug || envDebug {
					fmt.Fprintln(os.Stderr, "[DEBUG MergeWithFlags] Default theme also missing. Using coded UnicodeVibrant as fallback.")
				}
				finalDesignConfig = design.UnicodeVibrantTheme()
			}
		}
	}

	finalDesignConfig = design.DeepCopyConfig(finalDesignConfig)
	if finalDesignConfig == nil {
		if appCfg.Debug || envDebug {
			fmt.Fprintln(os.Stderr, "[DEBUG MergeWithFlags] Failed to copy design config. Falling back to default theme.")
		}
		finalDesignConfig = design.DeepCopyConfig(appCfg.Themes[DefaultActiveThemeName])
		if finalDesignConfig == nil {
			finalDesignConfig = design.UnicodeVibrantTheme()
		}
	}
	finalDesignConfig.ThemeName = effectiveThemeName

	effectiveNoColor := appCfg.NoColor
	effectiveCI := appCfg.CI
	effectiveNoTimer := appCfg.NoTimer

	envNoColorStr := os.Getenv("FO_NO_COLOR")
	if envNoColorStr == "" {
		envNoColorStr = os.Getenv("NO_COLOR")
	}
	if envNoColorStr != "" {
		if bVal, err := strconv.ParseBool(envNoColorStr); err == nil {
			effectiveNoColor = bVal
		}
	}
	envCIStr := os.Getenv("FO_CI")
	if envCIStr == "" {
		envCIStr = os.Getenv("CI")
	}
	if envCIStr != "" {
		if bVal, err := strconv.ParseBool(envCIStr); err == nil {
			effectiveCI = bVal
		}
	}

	if cliFlags.NoColorSet {
		effectiveNoColor = cliFlags.NoColor
	}
	if cliFlags.CISet {
		effectiveCI = cliFlags.CI
	}
	if cliFlags.NoTimerSet {
		effectiveNoTimer = cliFlags.NoTimer
	}

	if effectiveCI {
		design.ApplyMonochromeDefaults(finalDesignConfig)
		finalDesignConfig.Style.NoTimer = true
		finalDesignConfig.Style.UseBoxes = false
		finalDesignConfig.CI = true // Set explicit CI flag
	} else if effectiveNoColor {
		design.ApplyMonochromeDefaults(finalDesignConfig)
	}

	if effectiveNoTimer {
		finalDesignConfig.Style.NoTimer = true
	}

	if appCfg.Debug || envDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Final Design for theme '%s': IsMonochrome=%t, NoTimer=%t\n",
			finalDesignConfig.ThemeName, finalDesignConfig.IsMonochrome, finalDesignConfig.Style.NoTimer)
	}

	return finalDesignConfig
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
