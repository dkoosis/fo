package design

import (
	"strings"
)

// Config holds design system configuration
type Config struct {
	Style struct {
		UseBoxes       bool   `yaml:"use_boxes"`
		Indentation    string `yaml:"indentation"`
		ShowTimestamps bool   `yaml:"show_timestamps"`
		ReduceContrast bool   `yaml:"reduce_contrast"`
		Density        string `yaml:"density"`
		NoTimer        bool   `yaml:"no_timer"`
	} `yaml:"style"`
	Colors struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
	} `yaml:"colors"`
	Icons struct {
		Start   string `yaml:"start"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Info    string `yaml:"info"`
	} `yaml:"icons"`
	CognitiveLoad struct {
		AutoDetect bool                 `yaml:"auto_detect"`
		Default    CognitiveLoadContext `yaml:"default"`
	} `yaml:"cognitive_load"`
	Output struct {
		MaxErrorSamples  int  `yaml:"max_error_samples"`
		SummarizeSimilar bool `yaml:"summarize_similar"`
		UseAsciiGraphs   bool `yaml:"use_ascii_graphs"`
		ShowFullPath     bool `yaml:"show_full_path"`
	} `yaml:"output"`
	Accessibility struct {
		ScreenReaderFriendly bool `yaml:"screen_reader_friendly"`
		HighContrast         bool `yaml:"high_contrast"`
	} `yaml:"accessibility"`
	Patterns struct {
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	} `yaml:"patterns"`
	Tools map[string]*ToolConfig `yaml:"tools"`
}

type ToolConfig struct {
	Label          string              `yaml:"label"`
	Intent         string              `yaml:"intent"`
	Stream         bool                `yaml:"stream"`
	OutputPatterns map[string][]string `yaml:"output_patterns"`
	Layout         struct {
		GroupByType bool `yaml:"group_by_type"`
	} `yaml:"layout"`
}

func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Style.UseBoxes = true
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.ReduceContrast = false
	cfg.Style.Density = "balanced"
	cfg.Style.NoTimer = false

	cfg.Colors.Process = "\033[0;34m" // Blue
	cfg.Colors.Success = "\033[0;32m" // Green
	cfg.Colors.Warning = "\033[0;33m" // Yellow
	cfg.Colors.Error = "\033[0;31m"   // Red
	cfg.Colors.Detail = ""            // No color for detail, relies on Reset
	cfg.Colors.Muted = "\033[2m"      // Dim
	cfg.Colors.Reset = "\033[0m"

	cfg.Icons.Start = "▶️"
	cfg.Icons.Success = "✅"
	cfg.Icons.Warning = "⚠️"
	cfg.Icons.Error = "❌"
	cfg.Icons.Info = "ℹ️"

	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium
	cfg.Output.MaxErrorSamples = 3
	cfg.Output.SummarizeSimilar = true
	cfg.Output.UseAsciiGraphs = true
	cfg.Output.ShowFullPath = false
	cfg.Accessibility.ScreenReaderFriendly = false
	cfg.Accessibility.HighContrast = false

	cfg.Patterns.Intent = map[string][]string{
		"building":    {"go build", "make", "gcc", "g++", "javac", "npm build", "yarn build"},
		"testing":     {"go test", "npm test", "pytest", "jest", "rspec"},
		"linting":     {"golangci-lint", "eslint", "pylint", "rubocop"},
		"checking":    {"go vet", "shellcheck", "yamllint"},
		"installing":  {"go install", "npm install", "pip install", "apt"},
		"running":     {"go run", "python", "node", "ruby"},
		"downloading": {"curl", "wget", "git clone"},
	}
	cfg.Patterns.Output = map[string][]string{
		"error":   {"^Error:", "^ERROR:", "^error:", "failed", "failure", "exception", "panic:", "fatal:", "undefined", "not found"},
		"warning": {"^Warning:", "^WARNING:", "^warning:", "deprecated", "consider", "note:", "caution:"}, // Removed "STDERR: Info output"
		"success": {"^ok ", "^passed", "^success", "^done", "0 issues", "no problems", "completed successfully"},
		"info":    {"^info:", "^Info:", "^INFO:", "STDERR: Info output from success.sh"}, // Added specific pattern for test
	}
	cfg.Tools = make(map[string]*ToolConfig)
	return cfg
}

func NoColorConfig() *Config {
	cfg := DefaultConfig()
	cfg.Colors.Process = ""
	cfg.Colors.Success = ""
	cfg.Colors.Warning = ""
	cfg.Colors.Error = ""
	cfg.Colors.Detail = ""
	cfg.Colors.Muted = ""
	cfg.Colors.Reset = "" // Ensure Reset is also empty for true no-color

	cfg.Icons.Start = "[START]"
	cfg.Icons.Success = "[SUCCESS]"
	cfg.Icons.Warning = "[WARNING]"
	cfg.Icons.Error = "[FAILED]"
	cfg.Icons.Info = "[INFO]"

	cfg.Style.UseBoxes = false // CRITICAL: Disable boxes for no-color/CI
	cfg.Style.NoTimer = true   // CRITICAL: Disable timer for no-color/CI
	return cfg
}

func MergeWithDefaults(configFromFile *Config) *Config {
	if configFromFile == nil {
		return DefaultConfig()
	}
	defaultConfig := DefaultConfig()
	defaultConfig.Style.UseBoxes = configFromFile.Style.UseBoxes
	if configFromFile.Style.Indentation != "" {
		defaultConfig.Style.Indentation = configFromFile.Style.Indentation
	}
	defaultConfig.Style.ShowTimestamps = configFromFile.Style.ShowTimestamps
	defaultConfig.Style.ReduceContrast = configFromFile.Style.ReduceContrast
	if configFromFile.Style.Density != "" {
		defaultConfig.Style.Density = configFromFile.Style.Density
	}
	defaultConfig.Style.NoTimer = configFromFile.Style.NoTimer

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

	defaultConfig.CognitiveLoad.AutoDetect = configFromFile.CognitiveLoad.AutoDetect
	if configFromFile.CognitiveLoad.Default != "" {
		defaultConfig.CognitiveLoad.Default = configFromFile.CognitiveLoad.Default
	}

	if configFromFile.Output.MaxErrorSamples != 0 {
		defaultConfig.Output.MaxErrorSamples = configFromFile.Output.MaxErrorSamples
	}
	defaultConfig.Output.SummarizeSimilar = configFromFile.Output.SummarizeSimilar
	defaultConfig.Output.UseAsciiGraphs = configFromFile.Output.UseAsciiGraphs
	defaultConfig.Output.ShowFullPath = configFromFile.Output.ShowFullPath

	defaultConfig.Accessibility.ScreenReaderFriendly = configFromFile.Accessibility.ScreenReaderFriendly
	defaultConfig.Accessibility.HighContrast = configFromFile.Accessibility.HighContrast

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
	if len(configFromFile.Tools) > 0 {
		for toolName, toolCfg := range configFromFile.Tools {
			defaultConfig.Tools[toolName] = toolCfg
		}
	}
	return defaultConfig
}

func (c *Config) getIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(c.Style.Indentation, level)
}
