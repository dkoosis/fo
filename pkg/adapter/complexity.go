// Package adapter provides stream adapters for parsing structured command output
// into rich visualization patterns.
package adapter

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// ComplexityAdapter parses complexity snapshot JSON into ComplexityDashboard patterns.
//
// Expected JSON format (from orca complexity snapshots):
//
//	{
//	  "timestamp": "2025-11-29T00:00:00Z",
//	  "week": "2025-48",
//	  "metrics": {
//	    "files_over_500": 49,
//	    "files_over_1000": 7,
//	    "avg_complexity": 8.2,
//	    "functions_over_15": 12
//	  },
//	  "hotspots": [
//	    {"path": "...", "loc": 2562, "max_cc": 23, "score": 58926}
//	  ],
//	  "previous": { ... }  // Optional: previous snapshot for comparison
//	}
type ComplexityAdapter struct{}

// Name returns the adapter name.
func (a *ComplexityAdapter) Name() string {
	return "complexity-snapshot"
}

// Detect checks if the output is complexity snapshot JSON format.
func (a *ComplexityAdapter) Detect(firstLines []string) bool {
	if len(firstLines) == 0 {
		return false
	}

	// Join first lines and look for characteristic fields
	combined := strings.Join(firstLines, " ")

	// Complexity snapshots have these characteristic fields
	hasMetrics := strings.Contains(combined, `"metrics"`)
	hasHotspots := strings.Contains(combined, `"hotspots"`)
	hasComplexity := strings.Contains(combined, `"avg_complexity"`) ||
		strings.Contains(combined, `"files_over_500"`) ||
		strings.Contains(combined, `"max_cc"`)

	return hasMetrics && (hasHotspots || hasComplexity)
}

// complexitySnapshotJSON represents the expected JSON structure.
type complexitySnapshotJSON struct {
	Timestamp string `json:"timestamp"`
	Week      string `json:"week"`
	Metrics   struct {
		FilesOver500    int     `json:"files_over_500"`
		FilesOver1000   int     `json:"files_over_1000"`
		AvgComplexity   float64 `json:"avg_complexity"`
		FunctionsOver15 int     `json:"functions_over_15"`
	} `json:"metrics"`
	Hotspots []struct {
		Path  string `json:"path"`
		LOC   int    `json:"loc"`
		MaxCC int    `json:"max_cc"`
		Score int    `json:"score"`
	} `json:"hotspots"`
	Previous *struct {
		Metrics struct {
			FilesOver500    int     `json:"files_over_500"`
			FilesOver1000   int     `json:"files_over_1000"`
			AvgComplexity   float64 `json:"avg_complexity"`
			FunctionsOver15 int     `json:"functions_over_15"`
		} `json:"metrics"`
		Week string `json:"week"`
	} `json:"previous"`
	TrendWindow string `json:"trend_window"`
}

// Parse converts complexity snapshot JSON into a ComplexityDashboard pattern.
func (a *ComplexityAdapter) Parse(output io.Reader) (design.Pattern, error) {
	// Read all content
	var buf strings.Builder
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var data complexitySnapshotJSON
	if err := json.Unmarshal([]byte(buf.String()), &data); err != nil {
		return nil, err
	}

	// Build the dashboard
	dashboard := &design.ComplexityDashboard{
		Title:       "COMPLEXITY",
		TrendWindow: data.TrendWindow,
	}

	if dashboard.TrendWindow == "" {
		dashboard.TrendWindow = "4w"
	}

	// Build metrics with trend data
	dashboard.Metrics = []design.ComplexityMetric{
		{
			Label:       "Files >500 LOC",
			Current:     float64(data.Metrics.FilesOver500),
			LowerBetter: true,
		},
		{
			Label:       "Files >1000 LOC",
			Current:     float64(data.Metrics.FilesOver1000),
			LowerBetter: true,
		},
		{
			Label:       "Avg cyclomatic complexity",
			Current:     data.Metrics.AvgComplexity,
			LowerBetter: true,
		},
		{
			Label:       "Functions >15 complexity",
			Current:     float64(data.Metrics.FunctionsOver15),
			LowerBetter: true,
		},
	}

	// Add previous values if available
	if data.Previous != nil {
		dashboard.Metrics[0].Previous = float64(data.Previous.Metrics.FilesOver500)
		dashboard.Metrics[1].Previous = float64(data.Previous.Metrics.FilesOver1000)
		dashboard.Metrics[2].Previous = data.Previous.Metrics.AvgComplexity
		dashboard.Metrics[3].Previous = float64(data.Previous.Metrics.FunctionsOver15)
	}

	// Convert hotspots
	for _, h := range data.Hotspots {
		dashboard.Hotspots = append(dashboard.Hotspots, design.ComplexityHotspot{
			Path:          h.Path,
			LOC:           h.LOC,
			MaxComplexity: h.MaxCC,
			Score:         h.Score,
		})
	}

	return dashboard, nil
}
