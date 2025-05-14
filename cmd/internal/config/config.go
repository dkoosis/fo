// cmd/internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	// Import the design package to use its Config type
	"github.com/davidkoosis/fo/cmd/internal/design" // Adjust import path if necessary
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
	// MaxBufferSizeSet and MaxLineLengthSet might also be useful
}

// ToolConfig defines specific settings for a command/tool preset
type ToolConfig struct {
	Label      string `yaml:"label,omitempty"`
	Stream     *bool  `yaml:"stream,omitempty"` // Pointer to distinguish not set vs. set to false
	ShowOutput string `yaml:"show_output,omitempty"`
	NoTimer    *bool  `yaml:"no_timer,omitempty"`
	// Add other overridable design.Config fields if needed for presets e.g.
	// NoColor    *bool            `yaml:"no_color,omitempty"`
	// Intent     string           `yaml:"intent,omitempty"` // Already in design.ToolConfig
	OutputPatterns map[string][]string `yaml:"output_patterns,omitempty"` // Already in design.ToolConfig
	Intent         string              `yaml:"intent,omitempty"`          // Already in design.ToolConfig
}

// AppConfig represents the application's overall configuration from .fo.yaml
type AppConfig struct {
	// Global settings from YAML - these act as defaults if not overridden by theme or flags
	Label           string                 `yaml:"label,omitempty"`
	Stream          bool                   `yaml:"stream"`
	ShowOutput      string                 `yaml:"show_output"`
	NoTimer         bool                   `yaml:"no_timer"`
	NoColor         bool                   `yaml:"no_color"`
	CI              bool                   `yaml:"ci"`
	Debug           bool                   `yaml:"debug"`
	MaxBufferSize   int64                  `yaml:"max_buffer_size"` // In bytes
	MaxLineLength   int                    `yaml:"max_line_length"` // In bytes
	ActiveThemeName string                 `yaml:"active_theme"`
	Presets         map[string]*ToolConfig `yaml:"presets"`
	// Themes are defined in the design package but loaded/referenced here
	Themes map[string]*design.Config `yaml:"themes"`
}

// Constants for default values (can be shared or specific to this package)
const (
	DefaultShowOutput      = "on-fail"
	DefaultMaxBufferSize   = 10 * 1024 * 1024  // 10MB
	DefaultMaxLineLength   = 1 * 1024 * 1024   // 1MB
	DefaultActiveThemeName = "unicode_vibrant" // Matches a key in design.Config themes
)

// LoadConfig loads the .fo.yaml configuration.
func LoadConfig() *AppConfig {
	appCfg := &AppConfig{
		ShowOutput:      DefaultShowOutput,
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: DefaultActiveThemeName,
		Themes: map[string]*design.Config{ // Initialize with known themes from design package
			"unicode_vibrant": design.UnicodeVibrantTheme(),
			"ascii_minimal":   design.AsciiMinimalTheme(),
			// Load other themes defined in design package if necessary
		},
		Presets: make(map[string]*ToolConfig),
	}

	configPath := getConfigPath()
	if configPath == "" {
		fmt.Fprintf(os.Stderr, "DEBUG: No .fo.yaml config file found, using defaults.\n")
		return appCfg // Return defaults if no config file found
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file %s: %v. Using defaults.\n", configPath, err)
		} else {
			fmt.Fprintf(os.Stderr, "DEBUG: Config file %s not found. Using defaults.\n", configPath)
		}
		return appCfg
	}

	// Unmarshal into a temporary structure that matches the YAML for themes,
	// as themes in YAML might be partial and need merging with design package defaults.
	// For simplicity here, we assume .fo.yaml can fully define themes if it wants to override.
	err = yaml.Unmarshal(yamlFile, appCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error unmarshalling config file %s: %v. Using defaults.\n", configPath, err)
		// Reset to coded defaults if unmarshalling fails to ensure a valid state
		return &AppConfig{
			ShowOutput:      DefaultShowOutput,
			MaxBufferSize:   DefaultMaxBufferSize,
			MaxLineLength:   DefaultMaxLineLength,
			ActiveThemeName: DefaultActiveThemeName,
			Themes: map[string]*design.Config{
				"unicode_vibrant": design.UnicodeVibrantTheme(),
				"ascii_minimal":   design.AsciiMinimalTheme(),
			},
			Presets: make(map[string]*ToolConfig),
		}
	}
	// Ensure default themes are present if not defined in YAML
	if _, ok := appCfg.Themes["unicode_vibrant"]; !ok {
		appCfg.Themes["unicode_vibrant"] = design.UnicodeVibrantTheme()
	}
	if _, ok := appCfg.Themes["ascii_minimal"]; !ok {
		appCfg.Themes["ascii_minimal"] = design.AsciiMinimalTheme()
	}

	if appCfg.MaxBufferSize == 0 {
		appCfg.MaxBufferSize = DefaultMaxBufferSize
	}
	if appCfg.MaxLineLength == 0 {
		appCfg.MaxLineLength = DefaultMaxLineLength
	}
	if appCfg.ShowOutput == "" {
		appCfg.ShowOutput = DefaultShowOutput
	}
	if appCfg.ActiveThemeName == "" {
		appCfg.ActiveThemeName = DefaultActiveThemeName
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Loaded config from %s. Active theme: %s\n", configPath, appCfg.ActiveThemeName)
	return appCfg
}

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
	return ""
}

// MergeWithFlags creates the final design.Config to be used for rendering.
// It takes the loaded AppConfig, applies CLI flag overrides, and resolves the theme.
func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	effectiveThemeName := appCfg.ActiveThemeName
	if cliFlags.ThemeName != "" {
		effectiveThemeName = cliFlags.ThemeName
	}

	// Get the base theme configuration from the design package (or AppConfig if it stores full themes)
	finalDesignConfig, themeExists := appCfg.Themes[effectiveThemeName]
	if !themeExists {
		fmt.Fprintf(os.Stderr, "Warning: Theme '%s' not found in AppConfig, falling back to default '%s'.\n", effectiveThemeName, DefaultActiveThemeName)
		finalDesignConfig = appCfg.Themes[DefaultActiveThemeName]
		if finalDesignConfig == nil { // Absolute fallback
			fmt.Fprintf(os.Stderr, "Warning: Default theme '%s' also not found, using coded UnicodeVibrant theme.\n", DefaultActiveThemeName)
			finalDesignConfig = design.UnicodeVibrantTheme()
		}
	}

	// Create a deep copy to avoid modifying the original theme map instance
	copiedConfig := design.DeepCopyConfig(finalDesignConfig) // You'll need to implement DeepCopyConfig in design package
	finalDesignConfig = copiedConfig

	// Apply overrides: CLI flags > Environment Variables > .fo.yaml global settings > Theme defaults

	// Environment variable overrides (apply before CLI flags so CLI can override env)
	// FO_NO_COLOR or NO_COLOR
	envNoColor := os.Getenv("FO_NO_COLOR")
	if envNoColor == "" {
		envNoColor = os.Getenv("NO_COLOR")
	}
	if envNoColor != "" {
		if bVal, err := strconv.ParseBool(envNoColor); err == nil && bVal {
			appCfg.NoColor = true // Update the appCfg field that CLI flags will check
		}
	}
	// FO_CI
	if envCI := os.Getenv("FO_CI"); envCI != "" {
		if bVal, err := strconv.ParseBool(envCI); err == nil && bVal {
			appCfg.CI = true // Update the appCfg field
		}
	}
	// FO_DEBUG
	if envDebug := os.Getenv("FO_DEBUG"); envDebug != "" {
		if bVal, err := strconv.ParseBool(envDebug); err == nil && bVal {
			appCfg.Debug = true
		}
	}

	// Apply NoColor from AppConfig (which might have been updated by Env vars)
	if appCfg.NoColor {
		finalDesignConfig.IsMonochrome = true
		design.ApplyMonochromeDefaults(finalDesignConfig) // Helper in design pkg
	}
	// Apply CI from AppConfig
	if appCfg.CI {
		finalDesignConfig.IsMonochrome = true
		finalDesignConfig.Style.NoTimer = true
		design.ApplyMonochromeDefaults(finalDesignConfig) // Ensure CI is fully monochrome + simple
		finalDesignConfig.Style.UseBoxes = false          // Typically CI mode is simpler
	}

	// Apply CLI flag overrides (these have highest precedence)
	if cliFlags.NoColorSet && cliFlags.NoColor {
		finalDesignConfig.IsMonochrome = true
		design.ApplyMonochromeDefaults(finalDesignConfig)
	}
	if cliFlags.CISet && cliFlags.CI {
		finalDesignConfig.IsMonochrome = true
		finalDesignConfig.Style.NoTimer = true
		design.ApplyMonochromeDefaults(finalDesignConfig)
		finalDesignConfig.Style.UseBoxes = false
	}
	if cliFlags.NoTimerSet && cliFlags.NoTimer {
		finalDesignConfig.Style.NoTimer = true
	}
	// Note: cliFlags.Stream, cliFlags.ShowOutput, cliFlags.Label, cliFlags.MaxBufferSize, cliFlags.MaxLineLength, cliFlags.Debug
	// are typically used directly in main.go to control execution flow or passed separately,
	// rather than modifying the finalDesignConfig directly, unless a specific theme property needs to change.
	// For example, Stream mode might change finalDesignConfig.Style.UseBoxes.
	if cliFlags.StreamSet && cliFlags.Stream {
		finalDesignConfig.Style.UseBoxes = false // Example: stream implies simpler output
	}

	return finalDesignConfig
}

// ApplyCommandPreset modifies the AppConfig based on a command preset.
// This should be called *before* MergeWithFlags if presets can affect theme choice or global bools.
// Or, it can modify the `finalDesignConfig` if presets only change design aspects.
// For simplicity, let's assume it modifies AppConfig for now.
func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	// Extract just the command name from a path if necessary
	baseCommand := filepath.Base(commandName)

	keyToCheck := []string{commandName, baseCommand} // Check full path then base name

	for _, cmdKey := range keyToCheck {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			if preset.Label != "" {
				appCfg.Label = preset.Label // Overrides global label if preset has one
			}
			if preset.Stream != nil {
				appCfg.Stream = *preset.Stream
			}
			if preset.ShowOutput != "" {
				appCfg.ShowOutput = preset.ShowOutput
			}
			if preset.NoTimer != nil {
				appCfg.NoTimer = *preset.NoTimer
			}
			// If presets could define tool-specific intents or output patterns for the design.Config
			// that logic would also go here, potentially by merging into a temporary design.Config
			// or by having MergeWithFlags be aware of the active command.
			return // Applied first found preset
		}
	}
}

// You will also need to add DeepCopyConfig and ApplyMonochromeDefaults to your design package:
// In cmd/internal/design/config.go (package design):
/*
func DeepCopyConfig(original *Config) *Config {
    copied := *original // Shallow copy for top-level fields

    // Deep copy maps and slices
    copied.Colors = original.Colors // Assuming Colors struct is simple values
    copied.Icons = original.Icons   // Assuming Icons struct is simple values
    copied.Border = original.Border // Assuming Border struct is simple values
    copied.Style = original.Style   // Assuming Style struct is simple values
    copied.CognitiveLoad = original.CognitiveLoad

    if original.Elements != nil {
        copied.Elements = make(map[string]ElementStyleDef)
        for k, v := range original.Elements {
            copied.Elements[k] = v // ElementStyleDef contains slices, but let's assume they are not modified post-copy for now
        }
    }
    if original.Patterns.Intent != nil {
        copied.Patterns.Intent = make(map[string][]string)
        for k, v := range original.Patterns.Intent {
            // Perform deep copy for slice 'v' if necessary
            s := make([]string, len(v))
            copy(s,v)
            copied.Patterns.Intent[k] = s
        }
    }
    if original.Patterns.Output != nil {
        copied.Patterns.Output = make(map[string][]string)
        for k, v := range original.Patterns.Output {
            s := make([]string, len(v))
            copy(s,v)
            copied.Patterns.Output[k] = s
        }
    }
    if original.Tools != nil {
        copied.Tools = make(map[string]*ToolConfig) // design.ToolConfig might be different from config.ToolConfig
                                                    // This suggests ToolConfig specific to design might be needed,
                                                    // or that ToolConfig from package config is passed around.
                                                    // For now, let's assume this is design.ToolConfig if it exists
        for k, v := range original.Tools {
            toolCfgCopy := *v // shallow copy of the ToolConfig itself
            // deep copy slices/maps within v if any
            copied.Tools[k] = &toolCfgCopy
        }
    }
    return &copied
}

func ApplyMonochromeDefaults(cfg *Config) {
    cfg.IsMonochrome = true
    asciiMinimal := AsciiMinimalTheme() // Get the full minimal theme
    cfg.Icons = asciiMinimal.Icons
    cfg.Colors = asciiMinimal.Colors // All colors become empty strings
    cfg.Border = asciiMinimal.Border
    cfg.Style.UseBoxes = asciiMinimal.Style.UseBoxes // Usually simpler for monochrome

    // Ensure all element styles are updated for monochrome
    if cfg.Elements == nil {
        cfg.Elements = make(map[string]ElementStyleDef)
    }
    for elKey, minimalElDef := range asciiMinimal.Elements {
        currentElDef, ok := cfg.Elements[elKey]
        if !ok {
            currentElDef = ElementStyleDef{} // Start fresh if element didn't exist
        }
        currentElDef.ColorFG = "" // Remove color
        currentElDef.ColorBG = ""

        // If minimal theme defines specific text or icon, prefer it if current is empty or different
        if minimalElDef.IconKey != "" {
             currentElDef.IconKey = minimalElDef.IconKey
        }
         if minimalElDef.Text != "" && currentElDef.Text == "" { // If minimal theme defines text and current doesn't
             currentElDef.Text = minimalElDef.Text
        }
        // Preserve other non-color aspects from the original theme unless minimal explicitly changes them
        // (e.g., TextCase, TextStyle if not color dependent)
        cfg.Elements[elKey] = currentElDef
    }
}
*/
