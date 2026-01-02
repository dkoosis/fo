package design

import (
	"strings"
	"testing"
)

func TestComplexityDashboard_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Icons.Warning = IconCharWarning
	cfg.Icons.Error = IconCharError
	cfg.Style.Indentation = "  "

	tests := []struct {
		name        string
		dashboard   *ComplexityDashboard
		wantContain []string
	}{
		{
			name: "basic metrics without history",
			dashboard: &ComplexityDashboard{
				Title:       "COMPLEXITY",
				TrendWindow: "4w",
				Metrics: []ComplexityMetric{
					{Label: "Files >500 LOC", Current: 49, Previous: 52, LowerBetter: true},
					{Label: "Files >1000 LOC", Current: 7, Previous: 7, LowerBetter: true},
					{Label: "Avg cyclomatic complexity", Current: 8.2, Previous: 9.1, LowerBetter: true},
				},
			},
			wantContain: []string{
				"COMPLEXITY",
				"Files >500 LOC",
				"49",
				"52",
				"improving",
				"Files >1000 LOC",
				"7",
				"stable",
				"Avg cyclomatic complexity",
				"8.2",
				"9.1",
			},
		},
		{
			name: "with hotspots",
			dashboard: &ComplexityDashboard{
				Title: "COMPLEXITY",
				Metrics: []ComplexityMetric{
					{Label: "Functions >15 complexity", Current: 12, Previous: 14, LowerBetter: true},
				},
				Hotspots: []ComplexityHotspot{
					{Path: "internal/mcp/server/handler.go", LOC: 2562, MaxComplexity: 23, Score: 58926},
					{Path: "internal/mcp/server/api.go", LOC: 1954, MaxComplexity: 18, Score: 35172},
				},
			},
			wantContain: []string{
				"COMPLEXITY",
				"Hotspots",
				"2562 LOC",
				"cc:23",
				"internal/mcp/server/handler.go",
				"1954 LOC",
				"cc:18",
			},
		},
		{
			name: "degrading metrics",
			dashboard: &ComplexityDashboard{
				Title: "COMPLEXITY",
				Metrics: []ComplexityMetric{
					{Label: "Files >500 LOC", Current: 55, Previous: 49, LowerBetter: true},
				},
			},
			wantContain: []string{
				"COMPLEXITY",
				"55",
				"49",
				"degrading",
			},
		},
		{
			name: "empty dashboard",
			dashboard: &ComplexityDashboard{
				Title: "COMPLEXITY",
			},
			wantContain: []string{
				"COMPLEXITY",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dashboard.Render(cfg)

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("output should contain %q, got: %q", want, got)
				}
			}
		})
	}
}

func TestComplexityDashboard_PatternType(t *testing.T) {
	dashboard := &ComplexityDashboard{}
	got := dashboard.PatternType()
	if got != PatternTypeComplexityDashboard {
		t.Errorf("PatternType() = %v, want %v", got, PatternTypeComplexityDashboard)
	}
}

func TestCalculateTrend(t *testing.T) {
	tests := []struct {
		name        string
		current     float64
		previous    float64
		lowerBetter bool
		want        string
	}{
		{"improving - lower is better, value decreased", 45, 50, true, "improving"},
		{"degrading - lower is better, value increased", 55, 50, true, "degrading"},
		{"stable - within threshold", 51, 50, true, "stable"},
		{"improving - higher is better, value increased", 55, 50, false, "improving"},
		{"degrading - higher is better, value decreased", 45, 50, false, "degrading"},
		{"stable - no previous", 50, 0, true, "stable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateTrend(tt.current, tt.previous, tt.lowerBetter)
			if got != tt.want {
				t.Errorf("calculateTrend(%v, %v, %v) = %v, want %v",
					tt.current, tt.previous, tt.lowerBetter, got, tt.want)
			}
		})
	}
}

func TestRenderInlineSparkline(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   int // expected length in runes
	}{
		{"empty values", []float64{}, 0},
		{"single value", []float64{5.0}, 1},
		{"multiple values", []float64{1, 2, 3, 4, 5, 6, 7, 8}, 8},
		{"constant values", []float64{3, 3, 3, 3}, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderInlineSparkline(tt.values)
			gotLen := len([]rune(got))
			if gotLen != tt.want {
				t.Errorf("renderInlineSparkline() length = %d, want %d", gotLen, tt.want)
			}
		})
	}
}

func TestFormatMetricValue(t *testing.T) {
	tests := []struct {
		value float64
		unit  string
		want  string
	}{
		{49, "", "49"},
		{8.2, "", "8.2"},
		{7.0, "", "7"},
		{3.14159, "", "3.1"},
	}

	for _, tt := range tests {
		got := formatMetricValue(tt.value, tt.unit)
		if got != tt.want {
			t.Errorf("formatMetricValue(%v, %q) = %q, want %q", tt.value, tt.unit, got, tt.want)
		}
	}
}
