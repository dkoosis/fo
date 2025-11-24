// Package config provides configuration resolution with explicit priority order.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/dkoosis/fo/pkg/design"
)

// ConfigResolutionPriority defines the explicit priority order for configuration resolution.
// Higher priority sources override lower priority sources.
//
// Priority Order (highest to lowest):
//   1. CLI Flags (--theme-file, --theme, --no-color, --ci, --no-timer)
//   2. Environment Variables (FO_THEME, FO_NO_COLOR, FO_CI, CI, NO_COLOR)
//   3. .fo.yaml Configuration File (active_theme, no_color, ci, no_timer)
//   4. Design Package Defaults (UnicodeVibrantTheme, ASCIIMinimalTheme)
//
// This ensures predictable behavior: user intent (CLI) > environment > file > defaults.
const (
	// PriorityCLI is the highest priority - explicit user intent via command line
	PriorityCLI = 1

	// PriorityEnv is second - environment variables for automation/CI
	PriorityEnv = 2

	// PriorityFile is third - project-specific configuration
	PriorityFile = 3

	// PriorityDefault is lowest - sensible defaults
	PriorityDefault = 4
)

// ResolvedConfig holds the final resolved configuration after applying all priority rules.
type ResolvedConfig struct {
	// Theme configuration (visual presentation)
	Theme *design.Config

	// Behavioral settings
	NoColor   bool
	CI        bool
	NoTimer   bool
	Debug     bool
	Stream    bool
	ShowOutput string

	// Resource limits
	MaxBufferSize int64
	MaxLineLength int

	// Resolution metadata (for debugging)
	ThemeSource      string // "cli-file", "cli-name", "env", "file", "default"
	NoColorSource    string // "cli", "env", "file", "default"
	CISource         string // "cli", "env", "file", "default"
	NoTimerSource    string // "cli", "file", "default"
}

// ResolveConfig resolves configuration from all sources with explicit priority order.
// This is the single source of truth for config resolution.
//
// Resolution order:
//   1. Load base config from .fo.yaml (or defaults)
//   2. Apply CLI flags (highest priority)
//   3. Apply environment variables (if not set by CLI)
//   4. Apply file config (if not set by CLI/env)
//   5. Apply defaults (if nothing else set)
func ResolveConfig(cliFlags CliFlags) (*ResolvedConfig, error) {
	// Load base configuration from file (or defaults)
	appCfg := LoadConfig()

	// Start with file-based defaults
	resolved := &ResolvedConfig{
		NoColor:      appCfg.NoColor,
		CI:           appCfg.CI,
		NoTimer:      appCfg.NoTimer,
		Debug:        appCfg.Debug,
		Stream:       appCfg.Stream,
		ShowOutput:   appCfg.ShowOutput,
		MaxBufferSize: appCfg.MaxBufferSize,
		MaxLineLength: appCfg.MaxLineLength,
		NoColorSource: "file",
		CISource:      "file",
		NoTimerSource: "file",
	}

	// Resolve theme with priority: CLI file > CLI name > ENV > file > default
	resolved.Theme, resolved.ThemeSource = resolveTheme(cliFlags, appCfg)

	// Resolve NoColor with priority: CLI > ENV > file > default
	if cliFlags.NoColorSet {
		resolved.NoColor = cliFlags.NoColor
		resolved.NoColorSource = "cli"
	} else {
		envNoColor := getEnvBool("FO_NO_COLOR", "NO_COLOR")
		if envNoColor != nil {
			resolved.NoColor = *envNoColor
			resolved.NoColorSource = "env"
		}
	}

	// Resolve CI with priority: CLI > ENV > file > default
	if cliFlags.CISet {
		resolved.CI = cliFlags.CI
		resolved.CISource = "cli"
	} else {
		envCI := getEnvBool("FO_CI", "CI")
		if envCI != nil {
			resolved.CI = *envCI
			resolved.CISource = "env"
		}
	}

	// Resolve NoTimer with priority: CLI > file > default
	if cliFlags.NoTimerSet {
		resolved.NoTimer = cliFlags.NoTimer
		resolved.NoTimerSource = "cli"
	}

	// Resolve Debug with priority: CLI > file > default
	if cliFlags.DebugSet {
		resolved.Debug = cliFlags.Debug
	} else {
		envDebug := os.Getenv("FO_DEBUG") != ""
		if envDebug {
			resolved.Debug = true
		}
	}

	// Resolve Stream with priority: CLI > file > default
	if cliFlags.StreamSet {
		resolved.Stream = cliFlags.Stream
	}

	// Resolve ShowOutput with priority: CLI > file > default
	if cliFlags.ShowOutputSet {
		resolved.ShowOutput = cliFlags.ShowOutput
	}

	// Apply CI mode overrides (CI mode implies NoColor and NoTimer)
	if resolved.CI {
		resolved.NoColor = true
		resolved.NoTimer = true
		design.ApplyMonochromeDefaults(resolved.Theme)
		resolved.Theme.Style.NoTimer = true
		resolved.Theme.Style.UseBoxes = false
		resolved.Theme.CI = true
	} else if resolved.NoColor {
		design.ApplyMonochromeDefaults(resolved.Theme)
	}

	if resolved.NoTimer {
		resolved.Theme.Style.NoTimer = true
	}

	// Validate resolved configuration
	if err := validateResolvedConfig(resolved); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return resolved, nil
}

// resolveTheme resolves the theme with explicit priority order.
func resolveTheme(cliFlags CliFlags, appCfg *AppConfig) (*design.Config, string) {
	// Priority 1: CLI --theme-file (highest)
	if cliFlags.ThemeFile != "" {
		theme, err := LoadThemeFromFile(cliFlags.ThemeFile)
		if err != nil {
			// Fall back to default on error
			return design.UnicodeVibrantTheme(), "default"
		}
		return design.DeepCopyConfig(theme), "cli-file"
	}

	// Priority 2: CLI --theme flag
	if cliFlags.ThemeName != "" {
		// Check built-in themes first
		if theme, ok := design.DefaultThemes()[cliFlags.ThemeName]; ok {
			return design.DeepCopyConfig(theme), "cli-name"
		}
		// Check file-based themes
		if theme, ok := appCfg.Themes[cliFlags.ThemeName]; ok {
			return design.DeepCopyConfig(theme), "cli-name"
		}
		// Fall back to default
		return design.UnicodeVibrantTheme(), "default"
	}

	// Priority 3: Environment variable
	if envTheme := os.Getenv("FO_THEME"); envTheme != "" {
		if theme, ok := design.DefaultThemes()[envTheme]; ok {
			return design.DeepCopyConfig(theme), "env"
		}
		if theme, ok := appCfg.Themes[envTheme]; ok {
			return design.DeepCopyConfig(theme), "env"
		}
	}

	// Priority 4: File config active_theme
	if theme, ok := appCfg.Themes[appCfg.ActiveThemeName]; ok {
		return design.DeepCopyConfig(theme), "file"
	}

	// Priority 5: Default
	return design.UnicodeVibrantTheme(), "default"
}

// getEnvBool reads a boolean from environment variables, trying multiple keys.
// Returns nil if none are set, or a pointer to the boolean value.
func getEnvBool(keys ...string) *bool {
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			if b, err := strconv.ParseBool(val); err == nil {
				return &b
			}
		}
	}
	return nil
}

// validateResolvedConfig validates the resolved configuration and returns errors for invalid states.
func validateResolvedConfig(cfg *ResolvedConfig) error {
	if cfg.Theme == nil {
		return fmt.Errorf("theme cannot be nil")
	}

	validShowOutput := map[string]bool{
		"on-fail": true,
		"always":  true,
		"never":   true,
	}
	if !validShowOutput[cfg.ShowOutput] {
		return fmt.Errorf("invalid show_output value: %s (must be: on-fail, always, never)", cfg.ShowOutput)
	}

	if cfg.MaxBufferSize <= 0 {
		return fmt.Errorf("max_buffer_size must be positive, got: %d", cfg.MaxBufferSize)
	}

	if cfg.MaxLineLength <= 0 {
		return fmt.Errorf("max_line_length must be positive, got: %d", cfg.MaxLineLength)
	}

	return nil
}

