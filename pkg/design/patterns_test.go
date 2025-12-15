package design

import (
	"strings"
	"testing"
)

func TestSparkline_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess

	tests := []struct {
		name      string
		sparkline *Sparkline
		wantEmpty bool
		wantChars []rune
	}{
		{
			name: "empty values",
			sparkline: &Sparkline{
				Label:  "Test",
				Values: []float64{},
			},
			wantEmpty: true,
		},
		{
			name: "ascending values",
			sparkline: &Sparkline{
				Label:  "Build time",
				Values: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
				Unit:   "s",
			},
			wantChars: []rune{'▁', '▂', '▄', '▆', '█'},
		},
		{
			name: "descending values",
			sparkline: &Sparkline{
				Label:  "Memory",
				Values: []float64{5.0, 4.0, 3.0, 2.0, 1.0},
				Unit:   "MB",
			},
			wantChars: []rune{'█', '▆', '▄', '▂', '▁'},
		},
		{
			name: "constant values",
			sparkline: &Sparkline{
				Label:  "Stable",
				Values: []float64{3.0, 3.0, 3.0, 3.0},
				Unit:   "ms",
			},
			wantChars: []rune{'▁', '▁', '▁', '▁'}, // All values map to min when constant
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sparkline.Render(cfg)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty output, got %q", got)
				}
				return
			}

			// Verify output contains label
			if tt.sparkline.Label != "" && !strings.Contains(got, tt.sparkline.Label) {
				t.Errorf("output should contain label %q, got: %q", tt.sparkline.Label, got)
			}

			// Verify output contains sparkline characters
			for _, char := range tt.wantChars {
				if !strings.ContainsRune(got, char) {
					t.Errorf("output should contain sparkline char %c, got: %q", char, got)
				}
			}

			// Verify output contains unit and latest value
			if tt.sparkline.Unit != "" && !strings.Contains(got, tt.sparkline.Unit) {
				t.Errorf("output should contain unit %q, got: %q", tt.sparkline.Unit, got)
			}
		})
	}
}

func TestLeaderboard_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Style.Indentation = "  "

	tests := []struct {
		name        string
		leaderboard *Leaderboard
		wantContain []string
	}{
		{
			name: "empty items",
			leaderboard: &Leaderboard{
				Label: "Test",
				Items: []LeaderboardItem{},
			},
			wantContain: []string{},
		},
		{
			name: "slowest tests",
			leaderboard: &Leaderboard{
				Label:      "Slowest Tests",
				MetricName: "Duration",
				Direction:  "highest",
				TotalCount: 100,
				ShowRank:   true,
				Items: []LeaderboardItem{
					{Name: "TestLargeDataset", Metric: "5.2s", Value: 5.2, Rank: 1},
					{Name: "TestComplexQuery", Metric: "3.1s", Value: 3.1, Rank: 2},
					{Name: "TestNetworkCall", Metric: "2.8s", Value: 2.8, Rank: 3},
				},
			},
			wantContain: []string{
				"Slowest Tests",
				"top 3 of 100",
				"TestLargeDataset",
				"5.2s",
				"TestComplexQuery",
				"3.1s",
			},
		},
		{
			name: "largest files without rank",
			leaderboard: &Leaderboard{
				Label:      "Largest Binaries",
				MetricName: "Size",
				Direction:  "highest",
				ShowRank:   false,
				Items: []LeaderboardItem{
					{Name: "myapp", Metric: "45MB", Value: 45},
					{Name: "myctl", Metric: "12MB", Value: 12},
				},
			},
			wantContain: []string{
				"Largest Binaries",
				"myapp",
				"45MB",
				"myctl",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.leaderboard.Render(cfg)

			if len(tt.leaderboard.Items) == 0 {
				if got != "" {
					t.Errorf("expected empty output for empty items, got %q", got)
				}
				return
			}

			for _, want := range tt.wantContain {
				if !strings.Contains(got, want) {
					t.Errorf("output should contain %q, got: %q", want, got)
				}
			}
		})
	}
}

func TestTestTable_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Icons.Error = "✗"
	cfg.Icons.Warning = "⚠"
	cfg.Style.Indentation = "  "

	table := &TestTable{
		Label: "Test Results",
		Results: []TestTableItem{
			{Name: "pkg/api", Status: "pass", Duration: "2.1s", Count: 42},
			{Name: "pkg/db", Status: "fail", Duration: "0.5s", Count: 12, Details: "connection timeout"},
			{Name: "pkg/ui", Status: "skip", Duration: "0.0s", Count: 8},
		},
	}

	got := table.Render(cfg)

	wantContain := []string{
		"Test Results",
		"pkg/api",
		"2.1s",
		"42 tests",
		"pkg/db",
		"connection timeout",
		"pkg/ui",
	}

	for _, want := range wantContain {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q, got: %q", want, got)
		}
	}
}

func TestSummary_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Icons.Error = "✗"
	cfg.Icons.Warning = "⚠"
	cfg.Icons.Info = "ℹ"
	cfg.Style.Indentation = "  "

	summary := &Summary{
		Label: "Build Summary",
		Metrics: []SummaryItem{
			{Label: "Total Tests", Value: "142", Type: "info"},
			{Label: "Passed", Value: "138", Type: "success"},
			{Label: "Failed", Value: "4", Type: "error"},
		},
	}

	got := summary.Render(cfg)

	wantContain := []string{
		"Build Summary",
		"Total Tests",
		"142",
		"Passed",
		"138",
		"Failed",
		"4",
	}

	for _, want := range wantContain {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q, got: %q", want, got)
		}
	}
}

func TestComparison_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Style.Indentation = "  "

	comparison := &Comparison{
		Label: "Performance Changes",
		Changes: []ComparisonItem{
			{Label: "Build time", Before: "5.2s", After: "4.1s", Change: -1.1, Unit: "s"},
			{Label: "Binary size", Before: "42MB", After: "45MB", Change: 3, Unit: "MB"},
			{Label: "Test coverage", Before: "85%", After: "87%", Change: 2, Unit: "%"},
		},
	}

	got := comparison.Render(cfg)

	wantContain := []string{
		"Performance Changes",
		"Build time",
		"5.2s → 4.1s",
		"Binary size",
		"42MB → 45MB",
		"Test coverage",
		"85% → 87%",
	}

	for _, want := range wantContain {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q, got: %q", want, got)
		}
	}
}

func TestInventory_Render(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Info = "•"
	cfg.Style.Indentation = "  "

	inventory := &Inventory{
		Label: "Build Artifacts",
		Items: []InventoryItem{
			{Name: "myapp", Size: "2.3MB", Path: "./bin/myapp"},
			{Name: "myctl", Size: "1.1MB", Path: "./bin/myctl"},
			{Name: "docs", Size: "450KB"},
		},
	}

	got := inventory.Render(cfg)

	wantContain := []string{
		"Build Artifacts",
		"myapp",
		"2.3MB",
		"./bin/myapp",
		"myctl",
		"1.1MB",
		"docs",
		"450KB",
	}

	for _, want := range wantContain {
		if !strings.Contains(got, want) {
			t.Errorf("output should contain %q, got: %q", want, got)
		}
	}
}

func TestTestTable_RenderCompact(t *testing.T) {
	cfg := &Config{
		IsMonochrome: true,
	}
	cfg.Icons.Success = IconCharSuccess
	cfg.Icons.Error = "✗"
	cfg.Icons.Warning = "⚠"
	cfg.Style.Indentation = "  "

	// Create test table with many results
	results := []TestTableItem{
		{Name: "pkg/api", Status: "pass", Duration: "2.1s"},
		{Name: "pkg/db", Status: "pass", Duration: "1.8s"},
		{Name: "pkg/auth", Status: "fail", Duration: "0.5s"},
		{Name: "pkg/utils", Status: "pass", Duration: "0.3s"},
		{Name: "pkg/models", Status: "pass", Duration: "0.4s"},
		{Name: "pkg/views", Status: "skip", Duration: "0.0s"},
	}

	tests := []struct {
		name     string
		density  DensityMode
		wantCols int // Expected number of items per line
	}{
		{
			name:     "compact mode",
			density:  DensityCompact,
			wantCols: 3,
		},
		{
			name:     "balanced mode",
			density:  DensityBalanced,
			wantCols: 2,
		},
		{
			name:     "detailed mode",
			density:  DensityDetailed,
			wantCols: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := &TestTable{
				Label:   "Test Results",
				Results: results,
				Density: tt.density,
			}

			got := table.Render(cfg)

			// Verify output contains expected items
			if !strings.Contains(got, "pkg/api") {
				t.Errorf("output should contain pkg/api, got: %q", got)
			}

			// Count lines (excluding header)
			lines := strings.Split(strings.TrimSpace(got), "\n")
			// Filter out header
			dataLines := 0
			for _, line := range lines {
				if strings.Contains(line, "pkg/") {
					dataLines++
				}
			}

			expectedLines := (len(results) + tt.wantCols - 1) / tt.wantCols
			if dataLines != expectedLines {
				t.Errorf("%s: expected %d data lines, got %d", tt.name, expectedLines, dataLines)
			}
		})
	}
}

func TestPattern_Interface(t *testing.T) {
	// Verify all pattern types implement the Pattern interface
	cfg := &Config{IsMonochrome: true}

	var _ Pattern = &Sparkline{}
	var _ Pattern = &Leaderboard{}
	var _ Pattern = &TestTable{}
	var _ Pattern = &Summary{}
	var _ Pattern = &Comparison{}
	var _ Pattern = &Inventory{}
	var _ Pattern = &QualityReport{}
	var _ Pattern = &ComplexityDashboard{}
	var _ Pattern = &Housekeeping{}

	// Verify they can all be rendered
	patterns := []Pattern{
		&Sparkline{Values: []float64{1, 2, 3}},
		&Leaderboard{Items: []LeaderboardItem{{Name: "test", Metric: "1", Rank: 1}}},
		&TestTable{Results: []TestTableItem{{Name: "test", Status: "pass"}}},
		&Summary{Metrics: []SummaryItem{{Label: "test", Value: "1"}}},
		&Comparison{Changes: []ComparisonItem{{Label: "test", Before: "1", After: "2"}}},
		&Inventory{Items: []InventoryItem{{Name: "test", Size: "1MB"}}},
		&QualityReport{ServerName: "test", Categories: []QualityCategory{{Name: "test", Passed: 1, Total: 1}}},
		&ComplexityDashboard{Metrics: []ComplexityMetric{{Label: "test", Current: 10}}},
		&Housekeeping{Checks: []HousekeepingCheck{{Name: "test", Status: "pass"}}},
	}

	for i, p := range patterns {
		output := p.Render(cfg)
		if output == "" {
			t.Errorf("pattern %d produced empty output", i)
		}
	}
}

func TestPatternType_AllPatternTypes(t *testing.T) {
	types := AllPatternTypes()
	expectedCount := 9 // Updated to include ComplexityDashboard and Housekeeping
	if len(types) != expectedCount {
		t.Errorf("AllPatternTypes() returned %d types, want %d", len(types), expectedCount)
	}

	// Verify all expected types are present
	expectedTypes := map[PatternType]bool{
		PatternTypeSparkline:           true,
		PatternTypeLeaderboard:         true,
		PatternTypeTestTable:           true,
		PatternTypeSummary:             true,
		PatternTypeComparison:          true,
		PatternTypeInventory:           true,
		PatternTypeQualityReport:       true,
		PatternTypeComplexityDashboard: true,
		PatternTypeHousekeeping:        true,
	}

	for _, pt := range types {
		if !expectedTypes[pt] {
			t.Errorf("AllPatternTypes() returned unexpected type: %s", pt)
		}
		delete(expectedTypes, pt)
	}

	if len(expectedTypes) > 0 {
		t.Errorf("AllPatternTypes() missing types: %v", expectedTypes)
	}
}

func TestPatternType_IsValidPatternType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid sparkline", "sparkline", true},
		{"valid leaderboard", "leaderboard", true},
		{"valid test-table", "test-table", true},
		{"valid summary", "summary", true},
		{"valid comparison", "comparison", true},
		{"valid inventory", "inventory", true},
		{"valid quality-report", "quality-report", true},
		{"valid complexity-dashboard", "complexity-dashboard", true},
		{"valid housekeeping", "housekeeping", true},
		{"invalid type", "invalid", false},
		{"empty string", "", false},
		{"case sensitive", "Sparkline", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPatternType(tt.input)
			if got != tt.expected {
				t.Errorf("IsValidPatternType(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestPattern_PatternType(t *testing.T) {
	tests := []struct {
		name     string
		pattern  Pattern
		expected PatternType
	}{
		{"Sparkline", &Sparkline{}, PatternTypeSparkline},
		{"Leaderboard", &Leaderboard{}, PatternTypeLeaderboard},
		{"TestTable", &TestTable{}, PatternTypeTestTable},
		{"Summary", &Summary{}, PatternTypeSummary},
		{"Comparison", &Comparison{}, PatternTypeComparison},
		{"Inventory", &Inventory{}, PatternTypeInventory},
		{"QualityReport", &QualityReport{}, PatternTypeQualityReport},
		{"ComplexityDashboard", &ComplexityDashboard{}, PatternTypeComplexityDashboard},
		{"Housekeeping", &Housekeeping{}, PatternTypeHousekeeping},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pattern.PatternType()
			if got != tt.expected {
				t.Errorf("%s.PatternType() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
