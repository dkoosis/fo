// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davidkoosis/fo/cmd/internal/design" // Adjusted import path
	"gopkg.in/yaml.v3"
)

// CliFlags holds the values of command-line flags
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
	// MaxBufferSizeSet and MaxLineLengthSet might also be useful if 0 is a valid user input
}

// AppConfig represents the application's overall configuration from .fo.yaml
type AppConfig struct {
	// Global settings from YAML - these act as defaults if not overridden by theme or flags
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
	// Themes are defined in the design package but can be overridden or extended via YAML
	Themes map[string]*design.Config `yaml:"themes"` // Holds fully resolved design.Config objects
}

// Constants for default values
const (
	DefaultShowOutput      = "on-fail"
	DefaultMaxBufferSize   = 10 * 1024 * 1024 // 10MB
	DefaultMaxLineLength   = 1 * 1024 * 1024  // 1MB
	DefaultActiveThemeName = "unicode_vibrant"
)

// LoadConfig loads the .fo.yaml configuration.
func LoadConfig() *AppConfig {
	// Initialize with hardcoded defaults that themes build upon
	appCfg := &AppConfig{
		Stream:          false, // Default behavior is capture mode
		ShowOutput:      DefaultShowOutput,
		NoTimer:         false, // Timer is on by default
		NoColor:         false, // Color is on by default
		CI:              false,
		Debug:           false,
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: DefaultActiveThemeName,
		Themes: map[string]*design.Config{ // Initialize with known themes from design package
			"unicode_vibrant": design.UnicodeVibrantTheme(),
			"ascii_minimal":   design.AsciiMinimalTheme(),
		},
		Presets: make(map[string]*design.ToolConfig),
	}

	configPath := getConfigPath()
	if configPath == "" {
		if appCfg.Debug { // Only print if debug is on (e.g. via env var before this point)
			fmt.Fprintln(os.Stderr, "DEBUG: No .fo.yaml config file found, using defaults.")
		}
		return appCfg
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) { // Don't warn if file just doesn't exist, only on actual read errors
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file %s: %v. Using defaults.\n", configPath, err)
		} else if appCfg.Debug {
			fmt.Fprintf(os.Stderr, "DEBUG: Config file %s not found. Using defaults.\n", configPath)
		}
		return appCfg
	}

	// Unmarshal into a temporary structure.
	// This is because `appCfg.Themes` might contain partial theme definitions in YAML
	// that need to be merged onto the base themes from the `design` package.
	var yamlAppCfg AppConfig
	err = yaml.Unmarshal(yamlFile, &yamlAppCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error unmarshalling config file %s: %v. Using defaults.\n", configPath, err)
		return appCfg // Return coded defaults if unmarshalling fails
	}

	// Merge YAML settings onto the base appCfg
	// Global settings
	if yamlAppCfg.Label != "" {
		appCfg.Label = yamlAppCfg.Label
	}
	// For booleans, YAML `false` is a valid override.
	// Check if the key was present in YAML if distinguishing "not set" vs "set to false" is critical.
	// For simplicity, direct assignment works if YAML always provides the field.
	appCfg.Stream = yamlAppCfg.Stream
	if yamlAppCfg.ShowOutput != "" {
		appCfg.ShowOutput = yamlAppCfg.ShowOutput
	}
	appCfg.NoTimer = yamlAppCfg.NoTimer
	appCfg.NoColor = yamlAppCfg.NoColor
	appCfg.CI = yamlAppCfg.CI
	appCfg.Debug = yamlAppCfg.Debug // Debug can be enabled via YAML

	if yamlAppCfg.MaxBufferSize > 0 { // Allow YAML to set 0 if that's meaningful, otherwise only positive
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

	// Merge YAML theme definitions.
	// If a theme from YAML has the same name as a coded theme, YAML takes precedence.
	if yamlAppCfg.Themes != nil {
		for name, themeFromFile := range yamlAppCfg.Themes {
			if baseTheme, ok := appCfg.Themes[name]; ok {
				// Merge themeFromFile onto baseTheme (This requires a more sophisticated merge)
				// For now, let's assume YAML theme fully overrides if present
				appCfg.Themes[name] = design.DeepCopyConfig(themeFromFile) // Store a copy
			} else {
				appCfg.Themes[name] = design.DeepCopyConfig(themeFromFile) // Add new theme from file
			}
			// Ensure the theme name is set internally if loaded from YAML
			if appCfg.Themes[name] != nil {
				appCfg.Themes[name].ThemeName = name
			}
		}
	}

	// Ensure the active theme actually exists, fallback if not
	if _, ok := appCfg.Themes[appCfg.ActiveThemeName]; !ok {
		fmt.Fprintf(os.Stderr, "Warning: Active theme '%s' from config file not found in defined themes. Falling back to '%s'.\n", appCfg.ActiveThemeName, DefaultActiveThemeName)
		appCfg.ActiveThemeName = DefaultActiveThemeName
	}

	if appCfg.Debug {
		fmt.Fprintf(os.Stderr, "DEBUG: Loaded config from %s. Active theme: %s. Presets loaded: %d\n", configPath, appCfg.ActiveThemeName, len(appCfg.Presets))
	}
	return appCfg
}

func getConfigPath() string {
	// Check local directory first
	localPath := ".fo.yaml"
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	// Check XDG config directory
	configHome, err := os.UserConfigDir()
	if err == nil {
		xdgPath := filepath.Join(configHome, "fo", ".fo.yaml")
		if _, err := os.Stat(xdgPath); err == nil {
			return xdgPath
		}
	}
	// Could add other paths here, like ~/.fo.yaml as a fallback
	return ""
}

// MergeWithFlags creates the final design.Config to be used for rendering.
// It takes the loaded AppConfig, applies CLI flag overrides, and resolves the theme.
func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	// Determine the effective theme name (CLI flag > AppConfig active_theme)
	effectiveThemeName := appCfg.ActiveThemeName
	if cliFlags.ThemeName != "" {
		effectiveThemeName = cliFlags.ThemeName
	}

	// Get the base theme configuration. Fallback if not found.
	finalDesignConfig, themeExists := appCfg.Themes[effectiveThemeName]
	if !themeExists {
		fmt.Fprintf(os.Stderr, "Warning: Theme '%s' not found. Falling back to default theme '%s'.\n", effectiveThemeName, DefaultActiveThemeName)
		finalDesignConfig = appCfg.Themes[DefaultActiveThemeName]
		if finalDesignConfig == nil { // Absolute fallback if default also somehow missing
			fmt.Fprintln(os.Stderr, "Fatal: Default theme definition is missing. Using coded UnicodeVibrant theme.")
			finalDesignConfig = design.UnicodeVibrantTheme() // Hardcoded fallback
		}
	}

	// Create a deep copy to avoid modifying the original theme map instance
	finalDesignConfig = design.DeepCopyConfig(finalDesignConfig)
	finalDesignConfig.ThemeName = effectiveThemeName // Ensure the final config knows its name

	// Apply overrides: CLI flags > Environment Variables > .fo.yaml global settings > Theme defaults

	// --- Start with AppConfig global settings as a base for behavioral flags ---
	// These will be potentially overridden by Env and CLI flags.
	// The `finalDesignConfig` (which is a theme) might have its own defaults for NoTimer, etc.
	// We need to decide the precedence. Typically: CLI > Env > AppConfig > Theme

	effectiveNoColor := appCfg.NoColor
	effectiveCI := appCfg.CI
	effectiveNoTimer := appCfg.NoTimer
	// effectiveStream := appCfg.Stream // Stream is trickier, often a direct CLI control

	// --- Environment variable overrides ---
	// FO_NO_COLOR or NO_COLOR
	envNoColorStr := os.Getenv("FO_NO_COLOR")
	if envNoColorStr == "" {
		envNoColorStr = os.Getenv("NO_COLOR") // Standard NO_COLOR
	}
	if envNoColorStr != "" {
		if bVal, err := strconv.ParseBool(envNoColorStr); err == nil {
			effectiveNoColor = bVal
		}
	}
	// FO_CI
	if envCIStr := os.Getenv("FO_CI"); envCIStr != "" {
		if bVal, err := strconv.ParseBool(envCIStr); err == nil {
			effectiveCI = bVal
		}
	}
	// FO_DEBUG (Handled by main.go usually for direct use)

	// --- Apply CLI flag overrides (highest precedence for these behavioral/display flags) ---
	if cliFlags.NoColorSet { // Check if the flag was actually used
		effectiveNoColor = cliFlags.NoColor
	}
	if cliFlags.CISet {
		effectiveCI = cliFlags.CI
	}
	if cliFlags.NoTimerSet {
		effectiveNoTimer = cliFlags.NoTimer
	}

	// --- Apply the determined effective flags to the design config ---
	if effectiveCI { // CI implies NoColor and NoTimer
		design.ApplyMonochromeDefaults(finalDesignConfig)
		finalDesignConfig.Style.NoTimer = true
		finalDesignConfig.Style.UseBoxes = false // CI usually means simpler line output
	} else if effectiveNoColor {
		design.ApplyMonochromeDefaults(finalDesignConfig)
	}

	if effectiveNoTimer {
		finalDesignConfig.Style.NoTimer = true
	}

	// Stream flag from CLI usually affects execution mode in main.go directly.
	// If stream mode should always force a line-oriented theme:
	if cliFlags.StreamSet && cliFlags.Stream {
		if finalDesignConfig.Style.UseBoxes { // Only simplify if it was a boxed theme
			// finalDesignConfig.Style.UseBoxes = false // Or switch to ascii_minimal elements
			// design.ApplyMonochromeDefaults(finalDesignConfig) // Could also imply monochrome
			// For now, let main.go handle stream mode's display implications primarily.
		}
	}

	// Debug setting for main.go to use.
	// finalDesignConfig does not have a Debug field. Debug is an app-level concern.
	// If cliFlags.DebugSet { appCfg.Debug = cliFlags.Debug } // main.go will use appCfg.Debug

	return finalDesignConfig
}

// ApplyCommandPreset modifies the AppConfig based on a command preset.
// This function applies preset values to the *AppConfig* structure.
// These preset values will then be considered when MergeWithFlags is called.
func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	baseCommand := filepath.Base(commandName)
	keysToTry := []string{commandName, baseCommand} // Try full command path/name, then base name

	for _, cmdKey := range keysToTry {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			if appCfg.Debug {
				fmt.Fprintf(os.Stderr, "DEBUG: Applying preset for command '%s'\n", cmdKey)
			}
			// Note: design.ToolConfig from YAML preset contains Label, Intent, OutputPatterns.
			// It does NOT contain Stream, ShowOutput, NoTimer flags directly based on current struct.
			// If presets need to control these, the design.ToolConfig in YAML (and struct) would need them,
			// or you'd have a separate struct for presets in AppConfig that includes these behavioral fields.

			// For now, assuming design.ToolConfig primarily sets Label and Intent that influence design:
			if preset.Label != "" {
				// This label from preset will be used by main.go if CLI flag -l is not set.
				// We can set it on appCfg so it's available.
				appCfg.Label = preset.Label
			}

			// If design.ToolConfig *did* have fields like Stream, ShowOutput, NoTimer (as *bool or string):
			// if preset.Stream != nil { appCfg.Stream = *preset.Stream }
			// if preset.ShowOutput != "" { appCfg.ShowOutput = preset.ShowOutput }
			// if preset.NoTimer != nil { appCfg.NoTimer = *preset.NoTimer }
			// This approach keeps preset application focused on `AppConfig` which `MergeWithFlags` then consumes.

			// The design-specific parts of design.ToolConfig (Intent, OutputPatterns)
			// are part of the design.Config.Tools map.
			// `MergeWithFlags` gets a design.Config which includes a `Tools` map.
			// The pattern matcher in `main.go` then uses this `Tools` map.
			// So, `appCfg.Presets` effectively feeds into `finalDesignConfig.Tools`.
			// We need to ensure this link. `design.Config` already has `Tools map[string]*design.ToolConfig`.
			// The `LoadConfig` should populate this if themes are defined in YAML with tool-specific settings.
			// `ApplyCommandPreset` here is more about App-level behavior overrides from a preset name.

			return // Applied first found preset
		}
	}
}
