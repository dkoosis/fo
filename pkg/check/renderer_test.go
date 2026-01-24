package check

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestRenderer_Render(t *testing.T) {
	report := &Report{
		Schema:  SchemaID,
		Tool:    "filesize",
		Status:  StatusWarn,
		Summary: "3 red files",
		Metrics: []Metric{
			{Name: "red_files", Value: 3, Threshold: 0},
			{Name: "total_loc", Value: 48230, Unit: "lines"},
		},
		Items: []Item{
			{Severity: SeverityError, Label: "server.go", Value: "1847 LOC"},
			{Severity: SeverityWarning, Label: "handler.go", Value: "612 LOC"},
		},
		Trend: []int{45000, 46200, 47100, 48230},
	}

	config := DefaultRendererConfig()
	theme := design.DefaultConfig()
	renderer := NewRenderer(config, theme)

	output := renderer.Render(report)

	// Verify output contains expected elements
	if !strings.Contains(output, "filesize") {
		t.Error("output should contain tool name 'filesize'")
	}
	if !strings.Contains(output, "3 red files") {
		t.Error("output should contain summary '3 red files'")
	}
	if !strings.Contains(output, "red_files") {
		t.Error("output should contain metric 'red_files'")
	}
	if !strings.Contains(output, "server.go") {
		t.Error("output should contain item 'server.go'")
	}
	// Trend should show sparkline and values
	if !strings.Contains(output, "trend") {
		t.Error("output should contain trend")
	}
}

func TestRenderer_RenderEmpty(t *testing.T) {
	config := DefaultRendererConfig()
	theme := design.DefaultConfig()
	renderer := NewRenderer(config, theme)

	output := renderer.Render(nil)
	if output != "" {
		t.Errorf("Render(nil) = %q, want empty string", output)
	}
}

func TestRenderer_Sparkline(t *testing.T) {
	config := DefaultRendererConfig()
	theme := design.DefaultConfig()
	renderer := NewRenderer(config, theme)

	// Test sparkline generation
	sparkline := renderer.buildSparkline([]int{1, 2, 3, 4, 5, 6, 7, 8})
	if sparkline == "" {
		t.Error("sparkline should not be empty")
	}
	// Verify it contains block characters
	if !strings.ContainsAny(sparkline, "▁▂▃▄▅▆▇█") {
		t.Errorf("sparkline should contain block characters, got %q", sparkline)
	}
}

func TestRenderer_SparklineFlatLine(t *testing.T) {
	config := DefaultRendererConfig()
	theme := design.DefaultConfig()
	renderer := NewRenderer(config, theme)

	// Test flat line (all same values)
	sparkline := renderer.buildSparkline([]int{5, 5, 5, 5, 5})
	if sparkline == "" {
		t.Error("sparkline should not be empty for flat line")
	}
}

func TestRenderer_ItemLimit(t *testing.T) {
	report := &Report{
		Schema: SchemaID,
		Tool:   "test",
		Status: StatusWarn,
		Items:  make([]Item, 50), // 50 items
	}
	for i := range report.Items {
		report.Items[i] = Item{Severity: SeverityWarning, Label: "item"}
	}

	config := DefaultRendererConfig()
	config.ItemLimit = 5 // Limit to 5
	theme := design.DefaultConfig()
	renderer := NewRenderer(config, theme)

	output := renderer.Render(report)

	// Should indicate truncation
	if !strings.Contains(output, "more items") {
		t.Error("output should indicate truncated items")
	}
}
