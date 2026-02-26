package render

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
)

func TestTerminal_RenderReportPatterns(t *testing.T) {
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — all pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: "success"},
				{Label: "test", Value: "PASS — 60 tests", Kind: "success"},
			},
		},
	}
	r := NewTerminal(MonoTheme(), 80)
	out := r.Render(patterns)
	if !strings.Contains(out, "REPORT:") {
		t.Errorf("expected REPORT in output:\n%s", out)
	}
}
