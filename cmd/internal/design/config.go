package design

import (
	// "fmt" // Was used by formatDuration, no longer needed here
	"strings"
	// "time" // Was used by formatDuration, no longer needed here
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
		NoTimer        bool   `yaml:"no_timer"`        // ADDED: To control timer visibility
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
	Stream         bool                `yaml:"stream"` // This might be better at the top-level main.Config
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
	cfg.Style.NoTimer = false // Default is to show timer

	// Color defaults - research-informed (Zhou et al.)
	cfg.Colors.Process = "\033[0;34m" // Blue - use with caution for cognitive load
	cfg.Colors.Success = "\033[0;32m" // Green - universally positive
	cfg.Colors.Warning = "\033[0;33m" // Yellow - attention required
	cfg.Colors.Error = "\033[0;31m"   // Red - shown to reduce cognitive load for key info
	cfg.Colors.Detail = "\033[0m"     // Default (effectively reset)
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
	cfg.Output.UseAsciiGraphs = true // Assuming default is true based on previous context
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
		"error": { // This key is used by PatternMatcher.ClassifyOutputLine
			"^Error:", "^ERROR:", "^error:", // Case-insensitive error prefixes
			"failed", "failure", "exception", "panic:", "fatal:", // Common failure keywords
			"undefined", "not found", // Common specific errors
		},
		"warning": { // This key is used by PatternMatcher.ClassifyOutputLine
			"^Warning:", "^WARNING:", "^warning:", // Case-insensitive warning prefixes
			"deprecated", "consider", "note:", "caution:", // Common warning keywords
		},
		"success": { // This key is used by PatternMatcher.ClassifyOutputLine
			"^ok ", "^passed", "^success", "^done", // Common success prefixes/keywords
			"0 issues", "no problems", "completed successfully", // Common success phrases
		},
		"info": { // This key is used by PatternMatcher.ClassifyOutputLine
			"^info:", "^Info:", "^INFO:", // Case-insensitive info prefixes
		},
		// "progress" and "detail" types are often defaults or contextually determined
		// rather than matched by broad patterns here, but could be added if needed.
	}

	// Initialize tools map
	cfg.Tools = make(map[string]*ToolConfig)

	return cfg
}

// NoColorConfig returns a config with colors disabled and plain icons
func NoColorConfig() *Config {
	cfg := DefaultConfig() // Start with defaults to get all structure

	// Disable colors by setting them to empty strings (or keep reset if needed)
	cfg.Colors.Process = ""
	cfg.Colors.Success = ""
	cfg.Colors.Warning = ""
	cfg.Colors.Error = ""
	cfg.Colors.Detail = "" // Keep reset for plain text if Detail implies no color
	cfg.Colors.Muted = ""
	// cfg.Colors.Reset = "" // Optional: if all colors are empty, reset might not be needed or could be kept

	// Use plain text icons
	cfg.Icons.Start = "[START]"
	cfg.Icons.Success = "[SUCCESS]"
	cfg.Icons.Warning = "[WARNING]"
	cfg.Icons.Error = "[FAILED]"
	cfg.Icons.Info = "[INFO]"

	// Screen reader friendly often implies no complex ANSI/styling
	// cfg.Accessibility.ScreenReaderFriendly = true // Consider if --no-color implies this

	return cfg
}

// MergeWithDefaults merges the config with command-line flags
// This function might be more accurately named MergeWithLoadedConfig or applyDefaultsToLoaded
// as it seems to take a loaded config and ensure defaults are applied.
// For now, keeping the name as is from previous context.
func MergeWithDefaults(configFromFile *Config) *Config {
	if configFromFile == nil {
		return DefaultConfig()
	}

	defaultConfig := DefaultConfig()

	// Simple field overrides (if present in file, use it, else default)
	if configFromFile.Style.Indentation != "" { // Check for empty string, not just nil for struct
		defaultConfig.Style.Indentation = configFromFile.Style.Indentation
	}
	// Example for boolean: (booleans default to false, so explicit check might be needed if file can set to false)
	// if configFromFile.Style.UseBoxes was explicitly set in YAML (e.g. using a pointer or checking a 'isSet' map)
	defaultConfig.Style.UseBoxes = configFromFile.Style.UseBoxes // Direct assignment for bools
	defaultConfig.Style.ShowTimestamps = configFromFile.Style.ShowTimestamps
	defaultConfig.Style.ReduceContrast = configFromFile.Style.ReduceContrast
	if configFromFile.Style.Density != "" {
		defaultConfig.Style.Density = configFromFile.Style.Density
	}
	defaultConfig.Style.NoTimer = configFromFile.Style.NoTimer

	// Colors: if a color is defined in the file, use it.
	if configFromFile.Colors.Process != "" {
		defaultConfig.Colors.Process = configFromFile.Colors.Process
	}
	if configFromFile.Colors.Success != "" {
		defaultConfig.Colors.Success = configFromFile.Colors.Success
	}
	if configFromFile.Colors.Warning != "" {
		defaultConfig.Colors.Warning = configFromFile.Colors.Warning
	}
	if configFromFile.Colors.Error != "" {
		defaultConfig.Colors.Error = configFromFile.Colors.Error
	}
	if configFromFile.Colors.Detail != "" {
		defaultConfig.Colors.Detail = configFromFile.Colors.Detail
	}
	if configFromFile.Colors.Muted != "" {
		defaultConfig.Colors.Muted = configFromFile.Colors.Muted
	}
	if configFromFile.Colors.Reset != "" {
		defaultConfig.Colors.Reset = configFromFile.Colors.Reset
	}

	// Icons
	if configFromFile.Icons.Start != "" {
		defaultConfig.Icons.Start = configFromFile.Icons.Start
	}
	if configFromFile.Icons.Success != "" {
		defaultConfig.Icons.Success = configFromFile.Icons.Success
	}
	if configFromFile.Icons.Warning != "" {
		defaultConfig.Icons.Warning = configFromFile.Icons.Warning
	}
	if configFromFile.Icons.Error != "" {
		defaultConfig.Icons.Error = configFromFile.Icons.Error
	}
	if configFromFile.Icons.Info != "" {
		defaultConfig.Icons.Info = configFromFile.Icons.Info
	}

	// CognitiveLoad
	defaultConfig.CognitiveLoad.AutoDetect = configFromFile.CognitiveLoad.AutoDetect
	if configFromFile.CognitiveLoad.Default != "" {
		defaultConfig.CognitiveLoad.Default = configFromFile.CognitiveLoad.Default
	}

	// Output
	if configFromFile.Output.MaxErrorSamples != 0 { // Check for non-default int
		defaultConfig.Output.MaxErrorSamples = configFromFile.Output.MaxErrorSamples
	}
	defaultConfig.Output.SummarizeSimilar = configFromFile.Output.SummarizeSimilar
	defaultConfig.Output.UseAsciiGraphs = configFromFile.Output.UseAsciiGraphs
	defaultConfig.Output.ShowFullPath = configFromFile.Output.ShowFullPath

	// Accessibility
	defaultConfig.Accessibility.ScreenReaderFriendly = configFromFile.Accessibility.ScreenReaderFriendly
	defaultConfig.Accessibility.HighContrast = configFromFile.Accessibility.HighContrast

	// Merge pattern maps: Add/overwrite from file to defaults
	if len(configFromFile.Patterns.Intent) > 0 {
		for intent, patterns := range configFromFile.Patterns.Intent {
			defaultConfig.Patterns.Intent[intent] = patterns // This overwrites or adds
		}
	}
	if len(configFromFile.Patterns.Output) > 0 {
		for outputType, patterns := range configFromFile.Patterns.Output {
			defaultConfig.Patterns.Output[outputType] = patterns // This overwrites or adds
		}
	}

	// Merge Tools: Add/overwrite from file to defaults
	if len(configFromFile.Tools) > 0 {
		for toolName, toolCfg := range configFromFile.Tools {
			defaultConfig.Tools[toolName] = toolCfg // This overwrites or adds
		}
	}

	return defaultConfig
}

// getIndentation returns the appropriate indentation string based on level.
func (c *Config) getIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(c.Style.Indentation, level)
}

// REMOVED formatDuration from here as it's redeclared. It should live in render.go
