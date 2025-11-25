// fo/projectconfig.go - Project-specific configuration via .fo.yaml
//
// This file provides support for loading project-specific configuration
// from a .fo.yaml file in the project root. This allows projects to
// customize themes, thresholds, and behavior without code changes.
package fo

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig represents project-specific fo configuration.
// This is loaded from .fo.yaml in the project root.
type ProjectConfig struct {
	// Theme settings
	Theme string `yaml:"theme"` // Theme name: "orca", "unicode_vibrant", "ascii_minimal"

	// Project metadata (for display in headers)
	Project struct {
		Name string `yaml:"name"` // Project name for headers
	} `yaml:"project"`

	// File size thresholds
	FileSizes struct {
		WarnLines      int `yaml:"warn_lines"`       // Warn when files exceed (default: 500)
		ErrorLines     int `yaml:"error_lines"`      // Error when files exceed (default: 1000)
		WarnTestLines  int `yaml:"warn_test_lines"`  // Warn for test files (default: 800)
		ErrorTestLines int `yaml:"error_test_lines"` // Error for test files (default: 1400)
		TopCount       int `yaml:"top_count"`        // Show top N files (default: 5)
		WarnMarkdown   int `yaml:"warn_markdown"`    // Warn on markdown count (default: 50)
	} `yaml:"file_sizes"`

	// Directories to skip during analysis
	SkipDirs []string `yaml:"skip_dirs"` // Additional dirs to skip (vendor, node_modules always skipped)

	// Snapshot storage
	SnapshotDir string `yaml:"snapshot_dir"` // Directory for storing snapshots (default: ".fo")

	// Section configurations
	Sections struct {
		Dependencies struct {
			Enabled bool `yaml:"enabled"` // Run dependency section (default: true)
		} `yaml:"dependencies"`
		FileSizes struct {
			Enabled bool `yaml:"enabled"` // Run file sizes section (default: true)
		} `yaml:"file_sizes"`
		Build struct {
			Enabled   bool   `yaml:"enabled"`    // Run build section (default: true)
			OutputDir string `yaml:"output_dir"` // Binary output directory (default: ".")
		} `yaml:"build"`
		Test struct {
			Enabled      bool   `yaml:"enabled"`       // Run test section (default: true)
			CoverageFile string `yaml:"coverage_file"` // Coverage output file
		} `yaml:"test"`
		Lint struct {
			Enabled bool `yaml:"enabled"` // Run lint section (default: true)
		} `yaml:"lint"`
	} `yaml:"sections"`
}

// DefaultProjectConfig returns a ProjectConfig with sensible defaults.
func DefaultProjectConfig() *ProjectConfig {
	cfg := &ProjectConfig{
		Theme:       "orca",
		SnapshotDir: ".fo",
	}

	cfg.FileSizes.WarnLines = 500
	cfg.FileSizes.ErrorLines = 1000
	cfg.FileSizes.WarnTestLines = 800
	cfg.FileSizes.ErrorTestLines = 1400
	cfg.FileSizes.TopCount = 5
	cfg.FileSizes.WarnMarkdown = 50

	cfg.Sections.Dependencies.Enabled = true
	cfg.Sections.FileSizes.Enabled = true
	cfg.Sections.Build.Enabled = true
	cfg.Sections.Build.OutputDir = "."
	cfg.Sections.Test.Enabled = true
	cfg.Sections.Lint.Enabled = true

	return cfg
}

// LoadProjectConfig loads configuration from .fo.yaml, falling back to defaults.
func LoadProjectConfig() *ProjectConfig {
	cfg := DefaultProjectConfig()

	// Try to find .fo.yaml in current directory or parent directories
	configPath := findConfigFile()
	if configPath == "" {
		return cfg
	}

	data, err := os.ReadFile(configPath) // #nosec G304 - config file path is controlled
	if err != nil {
		return cfg
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg
	}

	return cfg
}

// findConfigFile looks for .fo.yaml in current and parent directories.
func findConfigFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, ".fo.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// ToFileSizeConfig converts project config to FileSizeConfig.
func (p *ProjectConfig) ToFileSizeConfig() FileSizeConfig {
	return FileSizeConfig{
		WarnLineCount:      p.FileSizes.WarnLines,
		ErrorLineCount:     p.FileSizes.ErrorLines,
		WarnLineCountTest:  p.FileSizes.WarnTestLines,
		ErrorLineCountTest: p.FileSizes.ErrorTestLines,
		TopFilesCount:      p.FileSizes.TopCount,
		WarnMarkdownCount:  p.FileSizes.WarnMarkdown,
		SnapshotDir:        p.SnapshotDir,
	}
}

// NewConsoleFromProject creates a Console configured from project settings.
func NewConsoleFromProject() *Console {
	cfg := LoadProjectConfig()
	return NewConsole(ConsoleConfig{
		ThemeName: cfg.Theme,
	})
}

