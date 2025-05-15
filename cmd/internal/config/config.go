// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davidkoosis/fo/cmd/internal/design"
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
}

// AppConfig represents the application's overall configuration from .fo.yaml
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

// Constants for default values
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
			"ascii_minimal":   design.AsciiMinimalTheme(),   // Hardcoded default
		},
		Presets: make(map[string]*design.ToolConfig),
	}

	// Check initial hardcoded themes for debug purposes if FO_DEBUG is set
	// This debug check must happen before YAML overrides debug settings.
	initialDebug := os.Getenv("FO_DEBUG") != ""

	if initialDebug {
		fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Initial Hardcoded Themes ---")
		for themeName, themeCfg := range appCfg.Themes {
			if themeCfg != nil {
				fmt.Fprintf(os.Stderr, "Theme: %s (IsMonochrome: %t)\n", themeName, themeCfg.IsMonochrome)
				fmt.Fprintf(os.Stderr, "  Process: '%s' (Hex: %x)\n", themeCfg.Colors.Process, themeCfg.Colors.Process)
				fmt.Fprintf(os.Stderr, "  Success: '%s' (Hex: %x)\n", themeCfg.Colors.Success, themeCfg.Colors.Success)
				fmt.Fprintf(os.Stderr, "  Warning: '%s' (Hex: %x)\n", themeCfg.Colors.Warning, themeCfg.Colors.Warning)
				fmt.Fprintf(os.Stderr, "  Error:   '%s' (Hex: %x)\n", themeCfg.Colors.Error, themeCfg.Colors.Error)
				fmt.Fprintf(os.Stderr, "  Detail:  '%s' (Hex: %x)\n", themeCfg.Colors.Detail, themeCfg.Colors.Detail)
				fmt.Fprintf(os.Stderr, "  Muted:   '%s' (Hex: %x)\n", themeCfg.Colors.Muted, themeCfg.Colors.Muted)
				fmt.Fprintf(os.Stderr, "  Reset:   '%s' (Hex: %x)\n", themeCfg.Colors.Reset, themeCfg.Colors.Reset)
			}
		}
		fmt.Fprintln(os.Stderr, "-------------------------------------------------")
	}

	configPath := getConfigPath()
	if configPath == "" {
		if initialDebug { // Use initialDebug as appCfg.Debug might not be set yet
			fmt.Fprintln(os.Stderr, "[DEBUG LoadConfig] No .fo.yaml config file found, using defaults only.")
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

	var yamlAppCfg AppConfig // Temporary struct to unmarshal YAML into
	err = yaml.Unmarshal(yamlFile, &yamlAppCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error unmarshalling config file %s: %v. Using defaults.\n", configPath, err)
		return appCfg
	}

	// Merge YAML settings onto the base appCfg
	// General settings
	if yamlAppCfg.Label != "" {
		appCfg.Label = yamlAppCfg.Label
	}
	appCfg.Stream = yamlAppCfg.Stream // Overwrites default
	if yamlAppCfg.ShowOutput != "" {
		appCfg.ShowOutput = yamlAppCfg.ShowOutput
	}
	appCfg.NoTimer = yamlAppCfg.NoTimer
	appCfg.NoColor = yamlAppCfg.NoColor
	appCfg.CI = yamlAppCfg.CI
	appCfg.Debug = yamlAppCfg.Debug // YAML can set debug status

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

	// Merge themes from YAML. This will override hardcoded themes if names match,
	// or add new themes defined only in YAML.
	if yamlAppCfg.Themes != nil {
		if appCfg.Debug || initialDebug { // Use current debug status
			fmt.Fprintln(os.Stderr, "--- [DEBUG LoadConfig] Processing Themes from YAML ---")
		}
		for name, themeFromFile := range yamlAppCfg.Themes {
			// DeepCopy is important here to avoid modifying the original themeFromFile
			// if it's used elsewhere or if multiple YAML configs point to the same theme object.
			copiedTheme := design.DeepCopyConfig(themeFromFile)
			if copiedTheme != nil {
				copiedTheme.ThemeName = name      // Ensure the theme knows its own name
				appCfg.Themes[name] = copiedTheme // Add or overwrite in appCfg.Themes
				if appCfg.Debug || initialDebug {
					fmt.Fprintf(os.Stderr, "Loaded/Overwrote Theme from YAML: %s (IsMonochrome: %t)\n", name, copiedTheme.IsMonochrome)
					fmt.Fprintf(os.Stderr, "  YAML Process: '%s' (Hex: %x)\n", copiedTheme.Colors.Process, copiedTheme.Colors.Process)
					fmt.Fprintf(os.Stderr, "  YAML Success: '%s' (Hex: %x)\n", copiedTheme.Colors.Success, copiedTheme.Colors.Success)
					fmt.Fprintf(os.Stderr, "  YAML Warning: '%s' (Hex: %x)\n", copiedTheme.Colors.Warning, copiedTheme.Colors.Warning)
					fmt.Fprintf(os.Stderr, "  YAML Error:   '%s' (Hex: %x)\n", copiedTheme.Colors.Error, copiedTheme.Colors.Error)
					fmt.Fprintf(os.Stderr, "  YAML Detail:  '%s' (Hex: %x)\n", copiedTheme.Colors.Detail, copiedTheme.Colors.Detail)
					fmt.Fprintf(os.Stderr, "  YAML Muted:   '%s' (Hex: %x)\n", copiedTheme.Colors.Muted, copiedTheme.Colors.Muted)
					fmt.Fprintf(os.Stderr, "  YAML Reset:   '%s' (Hex: %x)\n", copiedTheme.Colors.Reset, copiedTheme.Colors.Reset)
				}
			}
		}
		if appCfg.Debug || initialDebug {
			fmt.Fprintln(os.Stderr, "---------------------------------------------------")
		}
	}

	// Validate that the active theme (possibly set from YAML) actually exists.
	if _, ok := appCfg.Themes[appCfg.ActiveThemeName]; !ok {
		if appCfg.Debug || initialDebug {
			fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Active theme '%s' (from config file or default) not found in final themes map. Falling back to '%s'.\n", appCfg.ActiveThemeName, DefaultActiveThemeName)
		}
		appCfg.ActiveThemeName = DefaultActiveThemeName // Fallback to a known default.
	}

	if appCfg.Debug || initialDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Loaded config from %s. Final Active theme: %s. Presets loaded: %d\n", configPath, appCfg.ActiveThemeName, len(appCfg.Presets))
		// Use the two-value assignment directly in an if statement's initializer part
		if finalActiveThemeForDebug, ok := appCfg.Themes[appCfg.ActiveThemeName]; ok && finalActiveThemeForDebug != nil {
			// Key was found, and the theme pointer is not nil
			fmt.Fprintf(os.Stderr, "  Final Active Theme '%s' Colors (found and not nil):\n", appCfg.ActiveThemeName)
			fmt.Fprintf(os.Stderr, "    Process: '%s' (Hex: %x)\n", finalActiveThemeForDebug.Colors.Process, finalActiveThemeForDebug.Colors.Process)
			// ... print other colors if needed for deep debugging
		} else if ok && finalActiveThemeForDebug == nil {
			// Key was found, but the theme pointer itself was nil in the map
			fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Final Active Theme '%s' was found in the themes map, but its value is nil.\n", appCfg.ActiveThemeName)
		} else if !ok {
			// Key was not found in the map
			fmt.Fprintf(os.Stderr, "[DEBUG LoadConfig] Final Active Theme '%s' was NOT found in the themes map.\n", appCfg.ActiveThemeName)
		}
	}
	return appCfg
}

// getConfigPath tries to find the .fo.yaml configuration file.
// It checks local directory first, then XDG UserConfigDir.
func getConfigPath() string {
	localPath := ".fo.yaml"
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	configHome, err := os.UserConfigDir()
	if err == nil {
		xdgPath := filepath.Join(configHome, "fo", ".fo.yaml")
		if _, err := os.Stat(xdgPath); err == nil {
			return xdgPath
		}
	}
	return "" // No config file found
}

// MergeWithFlags takes the application config (post-YAML and presets) and CLI flags,
// and returns the final design.Config to be used for rendering.
// It resolves theme selection, monochrome mode, and timer display.
func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	// Determine the effective theme name (CLI flag > app config).
	effectiveThemeName := appCfg.ActiveThemeName
	if cliFlags.ThemeName != "" {
		effectiveThemeName = cliFlags.ThemeName
	}

	// Get the design.Config for the effective theme.
	finalDesignConfig, themeExists := appCfg.Themes[effectiveThemeName]
	if !themeExists {
		// This fallback should ideally use the hardcoded default theme if the named one isn't found.
		if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Theme '%s' not found in appCfg.Themes. Falling back to default theme '%s'.\n", effectiveThemeName, DefaultActiveThemeName)
		}
		finalDesignConfig = appCfg.Themes[DefaultActiveThemeName] // Assumes DefaultActiveThemeName exists
		if finalDesignConfig == nil {                             // Absolute fallback if even default is missing (should not happen)
			if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintln(os.Stderr, "[DEBUG MergeWithFlags] Default theme definition also missing. Using coded UnicodeVibrant theme as absolute fallback.")
			}
			finalDesignConfig = design.UnicodeVibrantTheme()
		}
	}

	// It's crucial to work on a copy to avoid modifying the shared theme in appCfg.Themes.
	finalDesignConfig = design.DeepCopyConfig(finalDesignConfig)
	finalDesignConfig.ThemeName = effectiveThemeName // Ensure the copied config knows its name.

	// Determine effective NoColor, CI, and NoTimer states.
	// Priority: CLI flag > Environment Variable > AppConfig setting.
	effectiveNoColor := appCfg.NoColor // Start with AppConfig value
	effectiveCI := appCfg.CI
	effectiveNoTimer := appCfg.NoTimer

	// Environment variables override AppConfig.
	envNoColorStr := os.Getenv("FO_NO_COLOR")
	if envNoColorStr == "" {
		envNoColorStr = os.Getenv("NO_COLOR") // Standard NO_COLOR
	}
	if envNoColorStr != "" {
		if bVal, err := strconv.ParseBool(envNoColorStr); err == nil {
			effectiveNoColor = bVal
		}
	}
	if envCIStr := os.Getenv("FO_CI"); envCIStr != "" { // FO_CI specific env var
		if bVal, err := strconv.ParseBool(envCIStr); err == nil {
			effectiveCI = bVal
		}
	} else if envCIStr := os.Getenv("CI"); envCIStr != "" { // Standard CI env var
		if bVal, err := strconv.ParseBool(envCIStr); err == nil {
			effectiveCI = bVal
		}
	}

	// CLI flags override everything else if they were set.
	if cliFlags.NoColorSet {
		effectiveNoColor = cliFlags.NoColor
	}
	if cliFlags.CISet {
		effectiveCI = cliFlags.CI
	}
	if cliFlags.NoTimerSet {
		effectiveNoTimer = cliFlags.NoTimer
	}

	// Apply final decisions to the design config.
	// CI mode implies monochrome and no timer.
	if effectiveCI {
		design.ApplyMonochromeDefaults(finalDesignConfig) // This sets IsMonochrome = true
		finalDesignConfig.Style.NoTimer = true
		// Ensure UseBoxes is false for CI, as ApplyMonochromeDefaults should do.
		finalDesignConfig.Style.UseBoxes = false
	} else if effectiveNoColor {
		design.ApplyMonochromeDefaults(finalDesignConfig) // This sets IsMonochrome = true
	}

	// Explicit --no-timer overrides theme/CI settings for timer.
	if effectiveNoTimer {
		finalDesignConfig.Style.NoTimer = true
	}

	// Stream flag logic is primarily handled in main.go for execution flow.
	// If stream mode inherently changes design (e.g., always non-boxed), that can be set here.
	// Currently, stream mode doesn't force design changes beyond what main.go handles for output.
	// Example: if cliFlags.StreamSet && cliFlags.Stream && finalDesignConfig.Style.UseBoxes {
	//    finalDesignConfig.Style.UseBoxes = false
	// }

	if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG MergeWithFlags] Final Design Config for theme '%s': IsMonochrome=%t, NoTimer=%t, UseBoxes=%t\n",
			finalDesignConfig.ThemeName, finalDesignConfig.IsMonochrome, finalDesignConfig.Style.NoTimer, finalDesignConfig.Style.UseBoxes)
		fmt.Fprintf(os.Stderr, "  Final Colors - Process: '%s' (Hex: %x), Reset: '%s' (Hex: %x)\n",
			finalDesignConfig.Colors.Process, finalDesignConfig.Colors.Process,
			finalDesignConfig.Colors.Reset, finalDesignConfig.Colors.Reset)
	}

	return finalDesignConfig
}

// ApplyCommandPreset modifies the AppConfig based on a preset matching the commandName.
func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	baseCommand := filepath.Base(commandName)
	keysToTry := []string{commandName, baseCommand} // Try full path then base name.

	for _, cmdKey := range keysToTry {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			if appCfg.Debug || os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG ApplyCommandPreset] Applying preset for command '%s'\n", cmdKey)
			}
			if preset.Label != "" {
				appCfg.Label = preset.Label // This will be used if CLI -l is not set.
			}
			// If design.ToolConfig had fields like Stream, ShowOutput, NoTimer, etc.,
			// they would be applied to appCfg here.
			// Example:
			// if preset.StreamIsSet { appCfg.Stream = preset.StreamValue }
			return // Preset found and applied.
		}
	}
}
