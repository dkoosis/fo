package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

func TestLLMRender_KeyUserVisibleOutput(t *testing.T) {
	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wants    []string
	}{
		{
			name: "report includes per-tool summary and failed checks",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label: "REPORT: 3 tools — 1 fail, 2 pass",
					Kind:  pattern.SummaryKindReport,
					Metrics: []pattern.SummaryItem{
						{Label: "vet", Value: "0 diags", Kind: pattern.KindSuccess},
						{Label: "lint", Value: "2 err", Kind: pattern.KindError},
					},
				},
				&pattern.TestTable{
					Label:  "lint violations",
					Source: "lint",
					Results: []pattern.TestTableItem{
						{Name: "store → eval", Status: "fail", Details: "forbidden dependency"},
					},
				},
			},
			wants: []string{"REPORT: 3 tools — 1 fail, 2 pass", "vet: 0 diags", "## lint violations", "FAIL store → eval", "forbidden dependency"},
		},
		{
			name: "test summary mode includes scope and status lines",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "pkg/foo", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "failed tests",
					Results: []pattern.TestTableItem{
						{Name: "TestParser", Status: "fail", Duration: "0.02s", Details: "panic: boom"},
					},
				},
			},
			wants: []string{"SCOPE: pkg/foo", "failed tests", "FAIL TestParser (0.02s)", "panic: boom"},
		},
		{
			name: "sarif mode derives scope and groups by file",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "pkg/a.go",
					Results: []pattern.TestTableItem{
						{Name: "RULE001:12:3", Status: "fail", Details: "bad thing"},
					},
				},
			},
			wants: []string{"SCOPE: 1 files, 1 diags", "## pkg/a.go", "ERR RULE001:12:3 bad thing"},
		},
	}

	r := render.NewLLM()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := r.Render(tc.patterns)
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected output to contain %q, got:\n%s", want, out)
				}
			}
		})
	}
}
