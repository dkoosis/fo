package check

// RendererConfig defines how check reports should be rendered.
type RendererConfig struct {
	// ShowTrend enables sparkline trend visualization when trend data is present.
	ShowTrend bool `yaml:"show_trend"`

	// ShowMetrics enables metric display.
	ShowMetrics bool `yaml:"show_metrics"`

	// ShowItems enables item list display.
	ShowItems bool `yaml:"show_items"`

	// ItemLimit caps the number of items shown. 0 = unlimited.
	ItemLimit int `yaml:"item_limit,omitempty"`

	// TrendWidth is the width of the sparkline in characters.
	TrendWidth int `yaml:"trend_width,omitempty"`

	// SeverityMapping maps check severities to fo semantic types.
	SeverityMapping map[string]string `yaml:"severity_mapping,omitempty"`
}

// DefaultRendererConfig returns sensible defaults for check rendering.
func DefaultRendererConfig() RendererConfig {
	return RendererConfig{
		ShowTrend:   true,
		ShowMetrics: true,
		ShowItems:   true,
		ItemLimit:   20,
		TrendWidth:  20,
		SeverityMapping: map[string]string{
			"error":   "error",
			"warning": "warning",
			"info":    "info",
		},
	}
}
