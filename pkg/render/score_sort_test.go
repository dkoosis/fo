package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

// TestLLM_SortsSARIFDiagsByScoreDescending verifies that the LLM renderer
// orders diagnostics by Score desc, not by raw severity. A high-occurrence
// warning should outrank a low-occurrence error.
func TestLLM_SortsSARIFDiagsByScoreDescending(t *testing.T) {
	t.Parallel()

	// Construct two patterns with hand-set Score so we don't depend on the
	// mapper. The high-score row must appear in output before the low-score row.
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label:   "Analysis: 2 issues",
			Kind:    pattern.SummaryKindSARIF,
			Metrics: nil,
		},
		&pattern.TestTable{
			Label: "pkg/a/a.go",
			Results: []pattern.TestTableItem{
				{Name: "LOW:1:1", Status: pattern.StatusFail, Details: "low-priority error", Score: 3.0},
			},
		},
		&pattern.TestTable{
			Label: "pkg/b/b.go",
			Results: []pattern.TestTableItem{
				{Name: "HIGH:1:1", Status: pattern.StatusSkip, Details: "high-priority warning", Score: 99.0},
			},
		},
	}

	out := render.NewLLM().Render(patterns)

	hi := strings.Index(out, "high-priority warning")
	lo := strings.Index(out, "low-priority error")
	if hi < 0 || lo < 0 {
		t.Fatalf("missing expected entries; output:\n%s", out)
	}
	if hi > lo {
		t.Fatalf("HIGH-score row should render before LOW-score row\nhi=%d lo=%d output:\n%s", hi, lo, out)
	}
}
