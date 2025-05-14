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
	appCfg := &AppConfig{
		Stream:          false,
		ShowOutput:      DefaultShowOutput,
		NoTimer:         false,
		NoColor:         false,
		CI:              false,
		Debug:           false,
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: DefaultActiveThemeName,
		Themes: map[string]*design.Config{
			"unicode_vibrant": design.UnicodeVibrantTheme(),
			"ascii_minimal":   design.AsciiMinimalTheme(),
		},
		Presets: make(map[string]*design.ToolConfig),
	}

	configPath := getConfigPath()
	if configPath == "" {
		// Debug logging for config loading path can be helpful.
		// Consider adding a check for an environment variable like FO_DEBUG to enable this.
		// if os.Getenv("FO_DEBUG") != "" {
		// 	 fmt.Fprintln(os.Stderr, "DEBUG: No .fo.yaml config file found, using defaults.")
		// }
		return appCfg
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Error reading config file %s: %v. Using defaults.\n", configPath, err)
		}
		// else if os.Getenv("FO_DEBUG") != "" { // File not found is not a warning if optional
		// 	fmt.Fprintf(os.Stderr, "DEBUG: Config file %s not found. Using defaults.\n", configPath)
		// }
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
		for name, themeFromFile := range yamlAppCfg.Themes {
			// ***** THIS IS THE CORRECTED LINE *****
			if _, ok := appCfg.Themes[name]; ok { // Use blank identifier "_" if baseTheme value is not used for merging
				// YAML theme overrides coded theme with the same name
				appCfg.Themes[name] = design.DeepCopyConfig(themeFromFile)
			} else {
				// New theme defined entirely in YAML
				appCfg.Themes[name] = design.DeepCopyConfig(themeFromFile)
			}
			if themeCfg := appCfg.Themes[name]; themeCfg != nil { // Check for nil before accessing
				themeCfg.ThemeName = name // Ensure the theme knows its own name
			}
		}
	}

	if _, ok := appCfg.Themes[appCfg.ActiveThemeName]; !ok {
		// if os.Getenv("FO_DEBUG") != "" || appCfg.Debug { // Control debug output
		// fmt.Fprintf(os.Stderr, "DEBUG: Active theme '%s' from config file not found. Falling back to '%s'.\n", appCfg.ActiveThemeName, DefaultActiveThemeName)
		// }
		appCfg.ActiveThemeName = DefaultActiveThemeName
	}

	// if os.Getenv("FO_DEBUG") != "" || appCfg.Debug {
	// 	fmt.Fprintf(os.Stderr, "DEBUG: Loaded config from %s. Active theme: %s. Presets loaded: %d\n", configPath, appCfg.ActiveThemeName, len(appCfg.Presets))
	// }
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

func MergeWithFlags(appCfg *AppConfig, cliFlags CliFlags) *design.Config {
	effectiveThemeName := appCfg.ActiveThemeName
	if cliFlags.ThemeName != "" {
		effectiveThemeName = cliFlags.ThemeName
	}

	finalDesignConfig, themeExists := appCfg.Themes[effectiveThemeName]
	if !themeExists {
		// if os.Getenv("FO_DEBUG") != "" || appCfg.Debug {
		// 	fmt.Fprintf(os.Stderr, "DEBUG: Theme '%s' not found. Falling back to default theme '%s'.\n", effectiveThemeName, DefaultActiveThemeName)
		// }
		finalDesignConfig = appCfg.Themes[DefaultActiveThemeName]
		if finalDesignConfig == nil {
			// if os.Getenv("FO_DEBUG") != "" || appCfg.Debug {
			// 	fmt.Fprintln(os.Stderr, "DEBUG: Default theme definition also missing. Using coded UnicodeVibrant theme as absolute fallback.")
			// }
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
	if envCIStr := os.Getenv("FO_CI"); envCIStr != "" {
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

	// Stream flag logic is primarily handled in main.go for execution flow.
	// If stream mode inherently changes design (e.g., always non-boxed), that can be set here.
	if cliFlags.StreamSet && cliFlags.Stream {
		// Example: Force non-boxed if stream mode is on and current theme uses boxes.
		// This is the block that was causing SA9003.
		// Assuming the intention was to implement this logic:
		if finalDesignConfig.Style.UseBoxes {
			finalDesignConfig.Style.UseBoxes = false
			// Potentially apply other simplifications from ascii_minimal if stream implies simple output.
			// For example, one might want to switch to a simpler border style or density.
			// This is a placeholder for more specific logic if needed.
			// If no specific design changes are needed for stream mode beyond what main.go handles,
			// this if block could be removed if it remains empty after review.
		}
	}

	return finalDesignConfig
}

func ApplyCommandPreset(appCfg *AppConfig, commandName string) {
	baseCommand := filepath.Base(commandName)
	keysToTry := []string{commandName, baseCommand}

	for _, cmdKey := range keysToTry {
		if preset, ok := appCfg.Presets[cmdKey]; ok {
			// if os.Getenv("FO_DEBUG") != "" || appCfg.Debug {
			// 	fmt.Fprintf(os.Stderr, "DEBUG: Applying preset for command '%s'\n", cmdKey)
			// }
			if preset.Label != "" {
				appCfg.Label = preset.Label // This will be used if CLI -l is not set
			}
			// If design.ToolConfig had Stream, ShowOutput, NoTimer fields:
			// if preset.Stream != nil { appCfg.Stream = *preset.Stream }
			// if preset.ShowOutput != "" { appCfg.ShowOutput = preset.ShowOutput }
			// if preset.NoTimer != nil { appCfg.NoTimer = *preset.NoTimer }
			return
		}
	}
}
