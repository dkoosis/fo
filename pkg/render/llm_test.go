package render

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
)

func TestLLM_RenderReport(t *testing.T) {
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — all pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: "success"},
				{Label: "arch", Value: "pass", Kind: "success"},
			},
		},
	}
	r := NewLLM()
	out := r.Render(patterns)
	if !strings.Contains(out, "REPORT:") {
		t.Errorf("expected REPORT header in output:\n%s", out)
	}
	if !strings.Contains(out, "vet: 0 diags") {
		t.Errorf("expected tool summary line in output:\n%s", out)
	}
}

func TestLLM_RenderReportWithFailures(t *testing.T) {
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 3 tools — 1 fail, 2 pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: "success"},
				{Label: "lint", Value: "2 err", Kind: "error"},
				{Label: "arch", Value: "pass", Kind: "success"},
			},
		},
		&pattern.TestTable{
			Label:  "lint violations",
			Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "store → eval", Status: "fail", Details: "forbidden dependency"},
			},
		},
	}
	r := NewLLM()
	out := r.Render(patterns)
	if !strings.Contains(out, "1 fail") {
		t.Errorf("expected failure count in output:\n%s", out)
	}
	if !strings.Contains(out, "lint violations") {
		t.Errorf("expected table label in output:\n%s", out)
	}
	if !strings.Contains(out, "FAIL store → eval") {
		t.Errorf("expected violation detail in output:\n%s", out)
	}
}
