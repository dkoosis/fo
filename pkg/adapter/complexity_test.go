package adapter

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestComplexityAdapter_Name(t *testing.T) {
	a := &ComplexityAdapter{}
	if got := a.Name(); got != "complexity-snapshot" {
		t.Errorf("Name() = %q, want %q", got, "complexity-snapshot")
	}
}

func TestComplexityAdapter_Detect(t *testing.T) {
	a := &ComplexityAdapter{}

	tests := []struct {
		name       string
		firstLines []string
		want       bool
	}{
		{
			name:       "empty input",
			firstLines: []string{},
			want:       false,
		},
		{
			name: "valid complexity snapshot",
			firstLines: []string{
				`{"timestamp": "2025-11-29T00:00:00Z", "metrics": {"files_over_500": 49, "avg_complexity": 8.2}, "hotspots": []}`,
			},
			want: true,
		},
		{
			name: "valid snapshot with hotspots only",
			firstLines: []string{
				`{"metrics": {"files_over_500": 49}, "hotspots": [{"path": "foo.go", "max_cc": 15}]}`,
			},
			want: true,
		},
		{
			name: "pretty printed JSON",
			firstLines: []string{
				`{`,
				`  "metrics": {`,
				`    "avg_complexity": 8.2`,
				`  },`,
				`  "hotspots": []`,
				`}`,
			},
			want: true,
		},
		{
			name: "go test JSON - should not match",
			firstLines: []string{
				`{"Action":"run","Package":"pkg/example","Test":"TestFoo"}`,
			},
			want: false,
		},
		{
			name: "mcp interviewer - should not match",
			firstLines: []string{
				`{"initialize_result": {"protocolVersion": "1.0"}, "tools": []}`,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.Detect(tt.firstLines)
			if got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplexityAdapter_Parse(t *testing.T) {
	a := &ComplexityAdapter{}

	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, p design.Pattern)
	}{
		{
			name: "full snapshot with previous",
			input: `{
				"timestamp": "2025-11-29T00:00:00Z",
				"week": "2025-48",
				"trend_window": "4w",
				"metrics": {
					"files_over_500": 49,
					"files_over_1000": 7,
					"avg_complexity": 8.2,
					"functions_over_15": 12
				},
				"hotspots": [
					{"path": "internal/handler.go", "loc": 2562, "max_cc": 23, "score": 58926}
				],
				"previous": {
					"week": "2025-44",
					"metrics": {
						"files_over_500": 52,
						"files_over_1000": 7,
						"avg_complexity": 9.1,
						"functions_over_15": 14
					}
				}
			}`,
			wantErr: false,
			check: func(t *testing.T, p design.Pattern) {
				dashboard, ok := p.(*design.ComplexityDashboard)
				if !ok {
					t.Fatal("expected ComplexityDashboard")
				}

				if len(dashboard.Metrics) != 4 {
					t.Errorf("expected 4 metrics, got %d", len(dashboard.Metrics))
				}

				// Check first metric
				m := dashboard.Metrics[0]
				if m.Label != "Files >500 LOC" {
					t.Errorf("metric label = %q, want %q", m.Label, "Files >500 LOC")
				}
				if m.Current != 49 {
					t.Errorf("metric current = %v, want 49", m.Current)
				}
				if m.Previous != 52 {
					t.Errorf("metric previous = %v, want 52", m.Previous)
				}

				// Check hotspots
				if len(dashboard.Hotspots) != 1 {
					t.Errorf("expected 1 hotspot, got %d", len(dashboard.Hotspots))
				} else {
					h := dashboard.Hotspots[0]
					if h.LOC != 2562 {
						t.Errorf("hotspot LOC = %d, want 2562", h.LOC)
					}
					if h.MaxComplexity != 23 {
						t.Errorf("hotspot MaxComplexity = %d, want 23", h.MaxComplexity)
					}
				}

				if dashboard.TrendWindow != "4w" {
					t.Errorf("TrendWindow = %q, want %q", dashboard.TrendWindow, "4w")
				}
			},
		},
		{
			name: "minimal snapshot without previous",
			input: `{
				"metrics": {
					"files_over_500": 30,
					"files_over_1000": 5,
					"avg_complexity": 6.5,
					"functions_over_15": 8
				},
				"hotspots": []
			}`,
			wantErr: false,
			check: func(t *testing.T, p design.Pattern) {
				dashboard, ok := p.(*design.ComplexityDashboard)
				if !ok {
					t.Fatal("expected ComplexityDashboard")
				}

				// Should default trend window
				if dashboard.TrendWindow != "4w" {
					t.Errorf("TrendWindow = %q, want default %q", dashboard.TrendWindow, "4w")
				}

				// Previous should be 0
				if dashboard.Metrics[0].Previous != 0 {
					t.Errorf("metric previous = %v, want 0 (no previous data)", dashboard.Metrics[0].Previous)
				}
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			pattern, err := a.Parse(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pattern == nil {
				t.Fatal("expected pattern, got nil")
			}

			if tt.check != nil {
				tt.check(t, pattern)
			}
		})
	}
}
