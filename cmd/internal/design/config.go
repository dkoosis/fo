package design

import (
	"fmt"
	"strings"
	"time"
)

// Config holds design system configuration
type Config struct {
	// Visual style configuration
	Style struct {
		UseBoxes       bool   `yaml:"use_boxes"`
		Indentation    string `yaml:"indentation"`
		ShowTimestamps bool   `yaml:"show_timestamps"`
		ReduceContrast bool   `yaml:"reduce_contrast"` // Based on Zhou research
		Density        string `yaml:"density"`         // compact, balanced, relaxed
	} `yaml:"style"`

	// Color configuration (ANSI color codes)
	Colors struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
	} `yaml:"colors"`

	// Icons for status indicators
	Icons struct {
		Start   string `yaml:"start"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Info    string `yaml:"info"`
	} `yaml:"icons"`

	// Cognitive load settings
	CognitiveLoad struct {
		AutoDetect bool                 `yaml:"auto_detect"`
		Default    CognitiveLoadContext `yaml:"default"`
	} `yaml:"cognitive_load"`

	// Output control
	Output struct {
		MaxErrorSamples  int  `yaml:"max_error_samples"`
		SummarizeSimilar bool `yaml:"summarize_similar"`
		UseAsciiGraphs   bool `yaml:"use_ascii_graphs"`
		ShowFullPath     bool `yaml:"show_full_path"`
	} `yaml:"output"`

	// Accessibility options
	Accessibility struct {
		ScreenReaderFriendly bool `yaml:"screen_reader_friendly"`
		HighContrast         bool `yaml:"high_contrast"`
	} `yaml:"accessibility"`

	// Pattern matching configuration
	Patterns struct {
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	} `yaml:"patterns"`

	// Tool-specific configurations
	Tools map[string]*ToolConfig `yaml:"tools"`
}

// ToolConfig holds configuration for a specific tool
type ToolConfig struct {
	Label          string              `yaml:"label"`
	Intent         string              `yaml:"intent"`
	Stream         bool                `yaml:"stream"`
	OutputPatterns map[string][]string `yaml:"output_patterns"`
	Layout         struct {
		GroupByType bool `yaml:"group_by_type"`
	} `yaml:"layout"`
}

// DefaultConfig returns a design system config with research-backed defaults
func DefaultConfig() *Config {
	cfg := &Config{}

	// Style defaults
	cfg.Style.UseBoxes = true
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.ReduceContrast = false
	cfg.Style.Density = "balanced"

	// Color defaults - research-informed (Zhou et al.)
	cfg.Colors.Process = "\033[0;34m" // Blue - use with caution for cognitive load
	cfg.Colors.Success = "\033[0;32m" // Green - universally positive
	cfg.Colors.Warning = "\033[0;33m" // Yellow - attention required
	cfg.Colors.Error = "\033[0;31m"   // Red - shown to reduce cognitive load for key info
	cfg.Colors.Detail = "\033[0m"     // Default
	cfg.Colors.Muted = "\033[2m"      // Dim
	cfg.Colors.Reset = "\033[0m"      // Reset

	// Icon defaults
	cfg.Icons.Start = "▶️"
	cfg.Icons.Success = "✅"
	cfg.Icons.Warning = "⚠️"
	cfg.Icons.Error = "❌"
	cfg.Icons.Info = "ℹ️"

	// Cognitive load defaults
	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium

	// Output defaults
	cfg.Output.MaxErrorSamples = 3
	cfg.Output.SummarizeSimilar = true
	cfg.Output.UseAsciiGraphs = true
	cfg.Output.ShowFullPath = false

	// Accessibility defaults
	cfg.Accessibility.ScreenReaderFriendly = false
	cfg.Accessibility.HighContrast = false

	// Initialize pattern maps
	cfg.Patterns.Intent = make(map[string][]string)
	cfg.Patterns.Output = make(map[string][]string)

	// Add default intent patterns
	cfg.Patterns.Intent = map[string][]string{
		"building":    {"go build", "make", "gcc", "g++", "javac", "npm build", "yarn build"},
		"testing":     {"go test", "npm test", "pytest", "jest", "rspec"},
		"linting":     {"golangci-lint", "eslint", "pylint", "rubocop"},
		"checking":    {"go vet", "shellcheck", "yamllint"},
		"installing":  {"go install", "npm install", "pip install", "apt"},
		"running":     {"go run", "python", "node", "ruby"},
		"downloading": {"curl", "wget", "git clone"},
	}

	// Add default output classification patterns
	cfg.Patterns.Output = map[string][]string{
		"error": {
			"^Error:",
			"^ERROR:",
			"^error:",
			"failed",
			"failure",
			"exception",
			"panic:",
			"fatal:",
			"undefined",
			"not found",
		},
		"warning": {
			"^Warning:",
			"^WARNING:",
			"^warning:",
			"deprecated",
			"consider",
			"note:",
			"caution:",
		},
		"success": {
			"^ok ",
			"^passed",
			"^success",
			"^done",
			"0 issues",
			"no problems",
			"completed successfully",
		},
		"info": {
			"^info:",
			"^Info:",
			"^INFO:",
		},
	}

	// Initialize tools map
	cfg.Tools = make(map[string]*ToolConfig)

	return cfg
}

// NoColorConfig returns a config with colors disabled
func NoColorConfig() *Config {
	cfg := DefaultConfig()

	// Disable colors
	cfg.Colors.Process = ""
	cfg.Colors.Success = ""
	cfg.Colors.Warning = ""
	cfg.Colors.Error = ""
	cfg.Colors.Detail = ""
	cfg.Colors.Muted = ""
	cfg.Colors.Reset = ""

	// Use plain text icons
	cfg.Icons.Start = "[START]"
	cfg.Icons.Success = "[SUCCESS]"
	cfg.Icons.Warning = "[WARNING]"
	cfg.Icons.Error = "[FAILED]"
	cfg.Icons.Info = "[INFO]"

	return cfg
}

// formatDuration converts a duration to a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		secondsFraction := d.Seconds() - float64(minutes*60)
		return fmt.Sprintf("%dm%.1fs", minutes, secondsFraction)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

// MergeWithFlags merges the config with command-line flags
func MergeWithDefaults(configFromFile *Config) *Config {
	// If nil config provided, return default
	if configFromFile == nil {
		return DefaultConfig()
	}

	// Start with default config
	defaultConfig := DefaultConfig()

	// Merge pattern maps if they exist in configFromFile
	if len(configFromFile.Patterns.Intent) > 0 {
		for intent, patterns := range configFromFile.Patterns.Intent {
			defaultConfig.Patterns.Intent[intent] = patterns
		}
	}

	if len(configFromFile.Patterns.Output) > 0 {
		for outputType, patterns := range configFromFile.Patterns.Output {
			defaultConfig.Patterns.Output[outputType] = patterns
		}
	}

	// TODO: Add more merging logic for other fields

	return defaultConfig
}

// getIndentation returns the appropriate indentation string
func (c *Config) getIndentation(level int) string {
	return strings.Repeat(c.Style.Indentation, level)
}
