package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the command-line options.
type Config struct {
	Label         string            `yaml:"label"`
	Stream        bool              `yaml:"stream"`
	ShowOutput    string            `yaml:"show_output"`
	NoTimer       bool              `yaml:"no_timer"`
	NoColor       bool              `yaml:"no_color"`
	CI            bool              `yaml:"ci"`
	MaxBufferSize int64             `yaml:"max_buffer_size"`
	MaxLineLength int               `yaml:"max_line_length"`
	Visual        VisualConfig      `yaml:"visual"`
	Presets       map[string]Preset `yaml:"presets"`
}

// VisualConfig contains visual appearance settings
type VisualConfig struct {
	StartIcon    string `yaml:"start_icon"`
	SuccessIcon  string `yaml:"success_icon"`
	FailureIcon  string `yaml:"failure_icon"`
	ColorStart   string `yaml:"color_start"`
	ColorSuccess string `yaml:"color_success"`
	ColorFailure string `yaml:"color_failure"`
}

// Preset represents command-specific configuration
type Preset struct {
	Label      string `yaml:"label"`
	Stream     *bool  `yaml:"stream,omitempty"`
	ShowOutput string `yaml:"show_output,omitempty"`
	NoTimer    *bool  `yaml:"no_timer,omitempty"`
}

// Default values
const (
	DefaultMaxBufferSize int64 = 10 * 1024 * 1024
	DefaultMaxLineLength int   = 1 * 1024 * 1024
)

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	return &Config{
		ShowOutput:    "on-fail",
		MaxBufferSize: DefaultMaxBufferSize,
		MaxLineLength: DefaultMaxLineLength,
		Visual: VisualConfig{
			StartIcon:    "▶️",
			SuccessIcon:  "✅",
			FailureIcon:  "❌",
			ColorStart:   "\033[34m", // Blue
			ColorSuccess: "\033[32m", // Green
			ColorFailure: "\033[31m", // Red
		},
		Presets: make(map[string]Preset),
	}
}

// LoadConfig loads configuration from standard locations
func LoadConfig() *Config {
	config := DefaultConfig()

	// Possible config locations (in order of precedence)
	configLocations := []string{
		".fo.yaml",
		".fo.yml",
		"~/.config/fo/config.yaml",
		"~/.fo.yaml",
	}

	// Try each location
	for _, location := range configLocations {
		expandedPath := expandPath(location)
		if _, err := os.Stat(expandedPath); err == nil {
			if cfg, err := LoadFromFile(expandedPath); err == nil {
				config = cfg
				break
			}
		}
	}

	// Apply environment overrides
	applyEnvironmentOverrides(config)

	return config
}

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(path string) (*Config, error) {
	// Expand ~ character in path
	expandedPath := expandPath(path)

	// Read the configuration file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, err
	}

	// Start with default configuration
	config := DefaultConfig()

	// Parse the YAML data
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

// applyEnvironmentOverrides applies configuration from environment variables
func applyEnvironmentOverrides(config *Config) {
	// Output settings
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

	// Visual settings
	if val := os.Getenv("FO_NO_COLOR"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.NoColor = b
		}
	}

	// Buffer settings
	if val := os.Getenv("FO_MAX_BUFFER_SIZE"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil && i > 0 {
			config.MaxBufferSize = i
		}
	}

	// Check CI environment
	if val := os.Getenv("CI"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil && b {
			config.CI = true
		}
	}
}

// ApplyCommandPreset applies command-specific configuration
func ApplyCommandPreset(config *Config, cmdName string) {
	if len(cmdName) == 0 {
		return
	}

	// Get command basename
	baseName := filepath.Base(cmdName)

	// Apply preset if found
	preset, ok := config.Presets[baseName]
	if ok {
		if preset.Label != "" {
			config.Label = preset.Label
		}

		if preset.Stream != nil {
			config.Stream = *preset.Stream
		}

		if preset.ShowOutput != "" {
			config.ShowOutput = preset.ShowOutput
		}

		if preset.NoTimer != nil {
			config.NoTimer = *preset.NoTimer
		}
	}
}

// MergeWithFlags merges file config with command-line flags
func MergeWithFlags(fileConfig *Config, flagConfig *Config) *Config {
	result := *fileConfig

	// Command-line flags take precedence over file config
	if flagConfig.Label != "" {
		result.Label = flagConfig.Label
	}

	// Only override Stream if explicitly set via flag
	if flagConfig.Stream {
		result.Stream = true
	}

	if flagConfig.ShowOutput != "on-fail" {
		result.ShowOutput = flagConfig.ShowOutput
	}

	if flagConfig.NoTimer {
		result.NoTimer = true
	}

	if flagConfig.NoColor {
		result.NoColor = true
	}

	if flagConfig.CI {
		result.CI = true
	}

	return &result
}
