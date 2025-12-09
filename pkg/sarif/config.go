package sarif

// RendererConfig defines how SARIF results should be rendered.
// Added to .fo.yaml under the `sarif` key.
type RendererConfig struct {
	// Tools maps tool names to their rendering configuration.
	// Tool name matches runs[].tool.driver.name from SARIF.
	Tools map[string]ToolRenderConfig `yaml:"tools"`

	// Defaults apply when no tool-specific config exists.
	Defaults ToolRenderConfig `yaml:"defaults"`
}

// ToolRenderConfig defines rendering rules for a specific tool.
type ToolRenderConfig struct {
	// Patterns defines what visual patterns to render and in what order.
	Patterns []PatternConfig `yaml:"patterns"`

	// SeverityMapping maps SARIF levels to fo semantic types.
	// Keys: "error", "warning", "note", "none"
	// Values: "error", "warning", "info", "muted"
	SeverityMapping map[string]string `yaml:"severity_mapping,omitempty"`
}

// PatternConfig defines a single pattern to render from SARIF data.
type PatternConfig struct {
	// Type is the pattern type: "summary", "leaderboard", "issue_list"
	Type string `yaml:"type"`

	// GroupBy defines how to group results.
	// Options: "severity", "rule_id", "file", "none"
	GroupBy string `yaml:"group_by,omitempty"`

	// Limit caps the number of items shown. 0 = unlimited.
	Limit int `yaml:"limit,omitempty"`

	// Direction for leaderboard: "highest" or "lowest"
	Direction string `yaml:"direction,omitempty"`

	// Detail level: "compact", "normal", "detailed"
	Detail string `yaml:"detail,omitempty"`

	// ShowCounts includes count summaries (for summary pattern)
	ShowCounts bool `yaml:"show_counts,omitempty"`

	// Metric for leaderboard: "issue_count", "error_count", "warning_count"
	Metric string `yaml:"metric,omitempty"`
}

// DefaultRendererConfig returns sensible defaults for SARIF rendering.
func DefaultRendererConfig() RendererConfig {
	return RendererConfig{
		Tools: make(map[string]ToolRenderConfig),
		Defaults: ToolRenderConfig{
			Patterns: []PatternConfig{
				{
					Type:       "summary",
					GroupBy:    "severity",
					ShowCounts: true,
				},
				{
					Type:      "leaderboard",
					GroupBy:   "file",
					Metric:    "issue_count",
					Direction: "highest",
					Limit:     5,
				},
				{
					Type:    "issue_list",
					GroupBy: "file",
					Limit:   20,
					Detail:  "compact",
				},
			},
			SeverityMapping: map[string]string{
				"error":   "error",
				"warning": "warning",
				"note":    "info",
				"none":    "muted",
			},
		},
	}
}

// GolangciLintConfig returns optimized config for golangci-lint.
func GolangciLintConfig() ToolRenderConfig {
	return ToolRenderConfig{
		Patterns: []PatternConfig{
			{
				Type:       "summary",
				GroupBy:    "rule_id",
				ShowCounts: true,
			},
			{
				Type:      "leaderboard",
				GroupBy:   "file",
				Metric:    "issue_count",
				Direction: "highest",
				Limit:     10,
			},
			{
				Type:    "issue_list",
				GroupBy: "rule_id",
				Limit:   0, // Show all, grouped by rule
				Detail:  "compact",
			},
		},
		SeverityMapping: map[string]string{
			"error":   "error",
			"warning": "warning",
			"note":    "info",
			"none":    "muted",
		},
	}
}
