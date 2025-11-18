// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davidkoosis/fo/internal/design"
	"gopkg.in/yaml.v3"
)

// CliFlags holds the values of command-line flags.
type CliFlags struct {
	Label         string
	Stream        bool
	ShowOutput    string
	NoTimer       bool
	NoColor       bool
	CI            bool
	Debug         bool
	MaxBufferSize int64 // In bytes, passed from main after parsing
	MaxLineLength int   // In bytes, passed from main after parsing
	ThemeName     string

	// Flags to track if they were explicitly set by the user
	StreamSet     bool
	ShowOutputSet bool
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
	// Initialize with hardcoded default themes first.
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
		Themes: map[string]*design.Config{
			"unicode_vibrant": design.UnicodeVibrantTheme(), // Hardcoded default
			"ascii_minimal":   design.ASCIIMinimalTheme(),   // Hardcoded default
		},
		Presets: make(map[string]*design.ToolConfig),
	}

	initialDebug := os.Getenv("FO_DEBUG") != ""

	if initialDebug {
		fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Initial Hardcoded Themes ---")
		for themeName, themeCfg := range appCfg.Themes {
			if themeCfg != nil {
				fmt.Fprintf(os.Stderr, "Theme: %s (IsMonochrome: %t)\n", themeName, themeCfg.IsMonochrome)
				// Add more detailed color logging here if needed for debugging themes
			}
		}
		fmt.Fprintln(os.Stderr, "-------------------------------------------------")
	}

	configPath := getConfigPath() // Uses the corrected getConfigPath
	if configPath == "" {
		if initialDebug {
			fmt.Fprintln(os.Stderr, "[DEBUG LoadConfig] No .fo.yaml config file found or user config dir problematic, using defaults only.")
		}
		return appCfg
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file %s: %v. Using defaults.\n", configPath, err)
		} else if initialDebug {
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
		if appCfg.Debug || initialDebug {
			fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Processing Themes from YAML ---")
		}
		for name, themeFromFile := range yamlAppCfg.Themes {
			copiedTheme := design.DeepCopyConfig(themeFromFile)
			if copiedTheme != nil {
				copiedTheme.ThemeName = name
				// Normalize color strings after YAML unmarshal to ensure proper ANSI escape sequences
				normalizeThemeColors(copiedTheme)
				appCfg.Themes[name] = copiedTheme
				if appCfg.Debug || initialDebug {
					fmt.Fprintf(os.Stderr, "Loaded/Overwrote Theme from YAML: %s (IsMonochrome: %t)\n", name, copiedTheme.IsMonochrome)
				}
			}
		}
		if appCfg.Debug || initialDebug {
			fmt.Fprintln(os.Stderr, "---------------------------------------------------")
		}
	}

	if _, ok := appCfg.Themes[appCfg.ActiveThemeName]; !ok {
		if appCfg.Debug || initialDebug {
			fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Active theme '%s' not found. Falling back to '%s'.\n", appCfg.ActiveThemeName, DefaultActiveThemeName)
		}
		appCfg.ActiveThemeName = DefaultActiveThemeName
	}

	if appCfg.Debug || initialDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Loaded config from %s. Final Active theme: %s.\n", configPath, appCfg.ActiveThemeName)
	}
	return appCfg
}

// getConfigPath tries to find the .fo.yaml configuration file.
// It checks local directory first, then XDG UserConfigDir (if valid).
func getConfigPath() string {
	// Try local path first
	localPath := ".fo.yaml"
	if _, err := os.Stat(localPath); err == nil {
		// If FO_DEBUG is set, print that we're using the local path
		if os.Getenv("FO_DEBUG") != "" {
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
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] Using XDG config file: %s\n", xdgPath)
			}
			return xdgPath
		}
		// If FO_DEBUG is set and XDG path not found, print that.
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] XDG config file not found at: %s\n", xdgPath)
		}
	} else if os.Getenv("FO_DEBUG") != "" {
		// If FO_DEBUG is set and UserConfigDir was problematic, print that.
		fmt.Fprintf(os.Stderr, "[DEBUG getConfigPath] UserConfigDir error or unsuitable path. Error: %v, Path: '%s'\n", err, configHome)
	}

	// Fallback or if XDG path is not viable/found
	if os.Getenv("FO_DEBUG") != "" {
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

	// Normalize all color fields
	escChar := string([]byte{27})
	normalizeColor := func(s string) string {
		if s == "" {
			return ""
		}
		// If already has ESC, it's fine
		if strings.HasPrefix(s, escChar) || strings.HasPrefix(s, "\033") {
			return s
		}
		// Handle YAML escape sequences that might come through as literals
		if strings.HasPrefix(s, `\x1b`) {
			return escChar + strings.TrimPrefix(s, `\x1b`)
		}
		if strings.HasPrefix(s, `\033`) {
			return escChar + strings.TrimPrefix(s, `\033`)
		}
		return s
	}

	cfg.Colors.Process = normalizeColor(cfg.Colors.Process)
	cfg.Colors.Success = normalizeColor(cfg.Colors.Success)
	cfg.Colors.Warning = normalizeColor(cfg.Colors.Warning)
	cfg.Colors.Error = normalizeColor(cfg.Colors.Error)
	cfg.Colors.Detail = normalizeColor(cfg.Colors.Detail)
	cfg.Colors.Muted = normalizeColor(cfg.Colors.Muted)
	cfg.Colors.Reset = normalizeColor(cfg.Colors.Reset)
	cfg.Colors.White = normalizeColor(cfg.Colors.White)
	cfg.Colors.BlueFg = normalizeColor(cfg.Colors.BlueFg)
	cfg.Colors.BlueBg = normalizeColor(cfg.Colors.BlueBg)
	cfg.Colors.Bold = normalizeColor(cfg.Colors.Bold)
	cfg.Colors.Italic = normalizeColor(cfg.Colors.Italic)
}

// MergeWithFlags takes the application config (post-YAML and presets) and CLI flags,
// and returns the final design.Config to be used for rendering.
func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	effectiveThemeName := appCfg.ActiveThemeName
	if cliFlags.ThemeName != "" {
		effectiveThemeName = cliFlags.ThemeName
	}

	finalDesignConfig, themeExists := appCfg.Themes[effectiveThemeName]
	if !themeExists {
		if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Theme '%s' not found. Falling back to '%s'.\n", effectiveThemeName, DefaultActiveThemeName)
		}
		finalDesignConfig = appCfg.Themes[DefaultActiveThemeName]
		if finalDesignConfig == nil {
			if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintln(os.Stderr, "[DEBUG MergeWithFlags] Default theme also missing. Using coded UnicodeVibrant as fallback.")
			}
			finalDesignConfig = design.UnicodeVibrantTheme()
		}
	}

	finalDesignConfig = design.DeepCopyConfig(finalDesignConfig)
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
	} else if effectiveNoColor {
		design.ApplyMonochromeDefaults(finalDesignConfig)
	}

	if effectiveNoTimer {
		finalDesignConfig.Style.NoTimer = true
	}

	if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Final Design for theme '%s': IsMonochrome=%t, NoTimer=%t\n",
			finalDesignConfig.ThemeName, finalDesignConfig.IsMonochrome, finalDesignConfig.Style.NoTimer)
	}

	return finalDesignConfig
}

// ApplyCommandPreset modifies the AppConfig based on a preset matching the commandName.
func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	baseCommand := filepath.Base(commandName)
	keysToTry := []string{commandName, baseCommand}

	for _, cmdKey := range keysToTry {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
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
