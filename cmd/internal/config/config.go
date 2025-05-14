package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davidkoosis/fo/cmd/internal/design" // Assuming design package is correctly imported
	"gopkg.in/yaml.v3"
)

// AppConfig holds the application-level configuration, including theme management.
// This struct is what's populated from .fo.yaml, environment variables, and CLI flags.
type AppConfig struct {
	// Execution behavior settings (can be influenced by CLI flags and presets)
	Label         string `yaml:"-"` // Set by CLI flag or preset, not directly in top-level fo.yaml usually
	Stream        bool   `yaml:"stream"`
	ShowOutput    string `yaml:"show_output"` // "on-fail", "always", "never"
	NoTimer       bool   `yaml:"no_timer"`    // Global override for timer display
	NoColor       bool   `yaml:"no_color"`    // Global override for color display
	CI            bool   `yaml:"ci"`          // Global override for CI mode (implies no_color, no_timer, simpler theme)
	Debug         bool   `yaml:"debug"`
	MaxBufferSize int64  `yaml:"max_buffer_size"`
	MaxLineLength int    `yaml:"max_line_length"`

	// Theme management
	ActiveThemeName string                    `yaml:"active_theme"` // Name of the theme to use by default from the themes map
	Themes          map[string]*design.Config `yaml:"themes"`       // All themes defined in the config file
	EffectiveTheme  string                    `yaml:"-"`            // The actual theme name to be used after CLI/env override

	// Command-specific presets
	Presets map[string]Preset `yaml:"presets"`
}

// Preset represents command-specific configuration overrides.
type Preset struct {
	Label      string `yaml:"label,omitempty"`
	Stream     *bool  `yaml:"stream,omitempty"`
	ShowOutput string `yaml:"show_output,omitempty"`
	NoTimer    *bool  `yaml:"no_timer,omitempty"`
	// Presets could also suggest a theme_name if desired:
	// ThemeName  string `yaml:"theme_name,omitempty"`
}

// Default values for buffer sizes
const (
	DefaultMaxBufferSize int64 = 10 * 1024 * 1024 // 10MB
	DefaultMaxLineLength int   = 1 * 1024 * 1024  // 1MB
)

// defaultThemeName is used if no theme is specified or found.
const defaultThemeName = "unicode_vibrant" // Or "ascii_minimal" if you prefer that as absolute default

// NewDefaultAppConfig returns a new AppConfig with sensible defaults,
// including definitions for built-in themes.
func NewDefaultAppConfig() *AppConfig {
	cfg := &AppConfig{
		ShowOutput:      "on-fail",
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: defaultThemeName,
		Themes:          make(map[string]*design.Config),
		Presets:         make(map[string]Preset),
	}

	// Populate with built-in themes. These functions must exist in the design package.
	// These will be used if a config file isn't found or doesn't define them.
	// The keys here MUST match the theme names used in active_theme or by CLI/env.
	cfg.Themes["ascii_minimal"] = design.AsciiMinimalTheme()
	cfg.Themes["unicode_vibrant"] = design.UnicodeVibrantTheme()
	// Add a CI-specific theme if you have one, or CI mode will modify a base theme.
	// cfg.Themes["ci_theme"] = design.CITheme()

	return cfg
}

// LoadGlobalConfig loads configuration from standard file locations and environment variables.
// It starts with defaults, then layers file config, then environment overrides.
// CLI flag overrides are handled separately by the MergeCliWithAppConfig function.
func LoadGlobalConfig() *AppConfig {
	// Start with a configuration that includes built-in default themes
	appCfg := NewDefaultAppConfig()

	configLocations := []string{
		".fo.yaml",
		".fo.yml",
		filepath.Join(os.UserHomeDir(), ".config", "fo", "config.yaml"),
		filepath.Join(os.UserHomeDir(), ".config", "fo", ".fo.yaml"), // Common alternative
		filepath.Join(os.UserHomeDir(), ".fo.yaml"),
	}

	loadedFromFile := false
	for _, location := range configLocations {
		expandedPath := expandPath(location)
		if _, err := os.Stat(expandedPath); err == nil {
			data, errFile := os.ReadFile(expandedPath)
			if errFile == nil {
				// Create a temporary config to unmarshal into, so we don't overwrite defaults partially on error
				tempCfg := NewDefaultAppConfig() // Start with fresh defaults for this attempt
				if errYaml := yaml.Unmarshal(data, tempCfg); errYaml == nil {
					// Successfully unmarshalled, now merge.
					// File's top-level settings override initial defaults.
					appCfg.Stream = tempCfg.Stream
					appCfg.ShowOutput = tempCfg.ShowOutput
					appCfg.NoTimer = tempCfg.NoTimer
					appCfg.NoColor = tempCfg.NoColor
					appCfg.CI = tempCfg.CI
					appCfg.Debug = tempCfg.Debug
					if tempCfg.MaxBufferSize > 0 {
						appCfg.MaxBufferSize = tempCfg.MaxBufferSize
					}
					if tempCfg.MaxLineLength > 0 {
						appCfg.MaxLineLength = tempCfg.MaxLineLength
					}
					if tempCfg.ActiveThemeName != "" {
						appCfg.ActiveThemeName = tempCfg.ActiveThemeName
					}

					// Merge presets (file presets add to or override default presets)
					for k, v := range tempCfg.Presets {
						appCfg.Presets[k] = v
					}
					// Merge themes (file themes add to or override built-in themes)
					for k, v := range tempCfg.Themes {
						appCfg.Themes[k] = v
					}
					loadedFromFile = true
					break // Stop after first successful load
				} else {
					fmt.Fprintf(os.Stderr, "fo: warning: could not parse config file %s: %v\n", expandedPath, errYaml)
				}
			}
		}
	}

	if !loadedFromFile {
		// This message can be helpful for users wondering where config comes from
		// fmt.Fprintln(os.Stderr, "fo: notice: No .fo.yaml configuration file found or usable. Using internal default settings and themes.")
	}

	applyEnvironmentOverrides(appCfg) // Environment variables override file/defaults
	return appCfg
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[1:])
		}
		// If home dir can't be found, return path as is, os.Stat will fail later
	}
	return path
}

// applyEnvironmentOverrides applies configuration from environment variables.
// These override settings from the config file or defaults.
func applyEnvironmentOverrides(config *AppConfig) {
	if val := os.Getenv("FO_STREAM"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.Stream = b
		}
	}
	if val := os.Getenv("FO_SHOW_OUTPUT"); val != "" {
		if val == "on-fail" || val == "always" || val == "never" {
			config.ShowOutput = val
		}
	}
	if val := os.Getenv("FO_NO_TIMER"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.NoTimer = b
		}
	}
	if val := os.Getenv("FO_NO_COLOR"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.NoColor = b
		}
	}
	if val := os.Getenv("FO_DEBUG"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.Debug = b
		}
	}
	if val := os.Getenv("FO_MAX_BUFFER_SIZE"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil && i > 0 {
			config.MaxBufferSize = i
		}
	}
	if val := os.Getenv("FO_MAX_LINE_LENGTH"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 32); err == nil && i > 0 {
			config.MaxLineLength = int(i)
		}
	}
	if val := os.Getenv("FO_THEME"); val != "" {
		config.ActiveThemeName = val // Env var overrides active_theme from file
	}

	// CI environment variable implies --ci behavior
	if val := os.Getenv("CI"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil && b {
			config.CI = true
		}
	}
	// If CI is true (from env or flag later), it will also imply NoColor and NoTimer
}

// CliFlags represents the values passed via command-line flags.
// It also tracks if a flag was explicitly set by the user.
type CliFlags struct {
	Label         string
	Stream        bool
	StreamSet     bool
	ShowOutput    string
	ShowOutputSet bool
	NoTimer       bool
	NoTimerSet    bool
	NoColor       bool
	NoColorSet    bool
	CI            bool
	CISet         bool
	Debug         bool
	DebugSet      bool
	ThemeName     string // From --theme
	MaxBufferSize int64  // Value from flag (MB, converted to bytes)
	MaxLineLength int    // Value from flag (KB, converted to bytes)
}

// MergeCliWithAppConfig merges CLI flags into the AppConfig.
// CLI flags generally take the highest precedence.
func MergeCliWithAppConfig(appCfg *AppConfig, cli CliFlags) {
	// Label is handled directly in main.go from CLI if provided, or preset, or inferred.
	// appCfg.Label is not directly set here; it's more about effective settings.

	if cli.StreamSet {
		appCfg.Stream = cli.Stream
	}
	if cli.ShowOutputSet {
		appCfg.ShowOutput = cli.ShowOutput
	}
	if cli.NoTimerSet {
		appCfg.NoTimer = cli.NoTimer
	}
	if cli.NoColorSet {
		appCfg.NoColor = cli.NoColor
	}
	if cli.CISet {
		appCfg.CI = cli.CI
	}
	if cli.DebugSet {
		appCfg.Debug = cli.Debug
	}
	if cli.ThemeName != "" {
		appCfg.ActiveThemeName = cli.ThemeName // CLI flag overrides theme from env/file
	}
	if cli.MaxBufferSize > 0 { // Assume flag parsing already converted MB to bytes
		appCfg.MaxBufferSize = cli.MaxBufferSize
	}
	if cli.MaxLineLength > 0 { // Assume flag parsing already converted KB to bytes
		appCfg.MaxLineLength = cli.MaxLineLength
	}

	// CI mode implies NoColor and NoTimer, overriding other settings for these
	if appCfg.CI {
		appCfg.NoColor = true
		appCfg.NoTimer = true
	}
}

// ApplyCommandPreset modifies the AppConfig based on presets for the given command.
// This should be called *after* CLI flags are merged if CLI flags for these
// specific fields (Label, Stream, ShowOutput, NoTimer) should override presets.
// Or, call before merging CLI flags if presets should be overridden by CLI.
// Current fo logic: CLI overrides presets. Presets override file/default.
// So, this function would typically be called on the config derived from file/default,
// and then CLI flags are merged on top.
// For simplicity here, we'll assume it modifies the passed config.
func ApplyCommandPreset(config *AppConfig, cmdName string, cliDidSetLabel bool) {
	if len(cmdName) == 0 {
		return
	}
	baseName := filepath.Base(cmdName)
	preset, ok := config.Presets[baseName]
	if !ok {
		// Try with ".sh" suffix if it's a script
		if strings.HasSuffix(cmdName, ".sh") {
			preset, ok = config.Presets[cmdName]
		}
		if !ok {
			return
		}
	}

	// Only apply preset label if CLI did not provide one AND config.Label is still empty
	if !cliDidSetLabel && config.Label == "" && preset.Label != "" {
		config.Label = preset.Label
	}
	// Apply other preset values if they exist
	// These will be overridden by CLI flags if MergeCliWithAppConfig is called later.
	if preset.Stream != nil {
		config.Stream = *preset.Stream
	}
	if preset.ShowOutput != "" {
		config.ShowOutput = preset.ShowOutput
	}
	if preset.NoTimer != nil {
		config.NoTimer = *preset.NoTimer
	}
	// if preset.ThemeName != "" { // If presets could suggest themes
	// 	config.ActiveThemeName = preset.ThemeName
	// }
}

// GetResolvedDesignConfig selects the active theme from the AppConfig,
// applies global overrides (like NoColor, NoTimer from AppConfig which reflect CLI flags),
// and returns the final *design.Config to be used for rendering.
func (ac *AppConfig) GetResolvedDesignConfig() *design.Config {
	themeToLoad := ac.ActiveThemeName
	if themeToLoad == "" { // Should have been set by LoadGlobalConfig or MergeCli
		themeToLoad = defaultThemeName
		fmt.Fprintf(os.Stderr, "fo: warning: no active theme specified, defaulting to '%s'.\n", themeToLoad)
	}

	// Attempt to get the selected theme; fallback to a known default if not found
	baseDesignConfig, themeFound := ac.Themes[themeToLoad]
	if !themeFound {
		fmt.Fprintf(os.Stderr, "fo: warning: theme '%s' not found in configuration. Falling back to internal default theme '%s'.\n", themeToLoad, defaultThemeName)
		baseDesignConfig, themeFound = ac.Themes[defaultThemeName]
		if !themeFound { // Should not happen if NewDefaultAppConfig populates defaults
			fmt.Fprintf(os.Stderr, "fo: critical error: default theme '%s' also not found. Using emergency minimal theme.\n", defaultThemeName)
			baseDesignConfig = design.AsciiMinimalTheme() // Absolute fallback
		}
	}

	// Create a mutable copy to apply global overrides
	// This needs to be a deep enough copy if design.Config has nested structs that will be modified
	finalDesignCfg := *baseDesignConfig // Start with a shallow copy

	// Apply global overrides (NoColor, NoTimer, CI) from AppConfig
	// These AppConfig fields (ac.NoColor, ac.NoTimer, ac.CI) should already
	// reflect the highest precedence settings (CLI > Env > File default).

	isMonochrome := ac.NoColor // This NoColor field in AppConfig is the final say after CLI/env/CI
	showTimer := !ac.NoTimer   // This NoTimer field in AppConfig is the final say

	if isMonochrome {
		// If a theme explicitly named (e.g.) "ascii_minimal_ci" or "selected_theme_monochrome" exists,
		// we could try to load that. For now, transform the loaded theme.

		// Create a truly monochrome config based on the structure of design.NoColorConfig()
		monoDesign := design.NoColorConfig() // This provides the color/icon/style settings for monochrome

		finalDesignCfg.Colors = monoDesign.Colors
		finalDesignCfg.Icons = monoDesign.Icons
		// Decide if monochrome should always force non-boxed style for tasks, or respect theme's box choice
		// For fo, --no-color typically also means simpler ASCII icons and often simpler structure.
		finalDesignCfg.Style.UseBoxes = monoDesign.Style.UseBoxes // Usually false for NoColorConfig
		// Border chars should also come from the monochrome/ASCII set
		finalDesignCfg.Border.HeaderChar = monoDesign.Border.HeaderChar
		finalDesignCfg.Border.VerticalChar = monoDesign.Border.VerticalChar
		finalDesignCfg.Border.TopCornerChar = monoDesign.Border.TopCornerChar
		finalDesignCfg.Border.BottomCornerChar = monoDesign.Border.BottomCornerChar
		// ... and table chars if they differ
	}

	finalDesignCfg.Style.NoTimer = !showTimer

	if ac.CI {
		// Apply any other CI-specific structural simplifications to finalDesignCfg
		// For example, many CI themes prefer no boxes for tasks.
		// design.CITheme() could return a *design.Config with these settings.
		// If Style.UseBoxes is part of the theme, a CI theme would set it to false.
		// Or, as a simpler override:
		// finalDesignCfg.Style.UseBoxes = false // Example direct override for CI
	}

	return &finalDesignCfg
}
