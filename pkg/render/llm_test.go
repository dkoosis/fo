package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

func TestLLMRender_KeyUserVisibleOutput(t *testing.T) {
	t.Parallel()

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
						{Name: "store → eval", Status: pattern.StatusFail, Details: "forbidden dependency"},
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
						{Name: "TestParser", Status: pattern.StatusFail, Duration: "0.02s", Details: "panic: boom"},
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
						{Name: "RULE001:12:3", Status: pattern.StatusFail, Details: "bad thing"},
					},
				},
			},
			wants: []string{"SCOPE: 1 files, 1 diags", "## pkg/a.go", "ERR RULE001:12:3 bad thing"},
		},
		{
			name:     "empty patterns returns SARIF fallback with zero scope",
			patterns: nil,
			wants:    []string{"SCOPE: 0 files, 0 diags"},
		},
		{
			name: "sarif sorts ERR before WARN before NOTE",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "file.go",
					Results: []pattern.TestTableItem{
						{Name: "note1:1:1", Status: pattern.StatusPass},
						{Name: "err1:2:1", Status: pattern.StatusFail},
						{Name: "warn1:3:1", Status: pattern.StatusSkip},
					},
				},
			},
			wants: []string{"ERR err1:2:1", "WARN warn1:3:1", "NOTE note1:1:1"},
		},
		{
			name: "details truncated to 3 lines with overflow indicator",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "pkg/foo", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "failures",
					Results: []pattern.TestTableItem{
						{Name: "TestBig", Status: pattern.StatusFail, Details: "line1\nline2\nline3\nline4\nline5"},
					},
				},
			},
			wants: []string{"line1", "line2", "line3", "2 more lines"},
		},
		{
			name: "report errors grouped by tool",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label: "REPORT",
					Kind:  pattern.SummaryKindReport,
					Metrics: []pattern.SummaryItem{
						{Label: "lint", Value: "crashed", Kind: pattern.KindError},
					},
				},
				&pattern.Error{Source: "lint", Message: "config parse failed"},
			},
			wants: []string{"ERROR: config parse failed"},
		},
		{
			name: "test skip items render with SKIP prefix",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "test", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "skipped",
					Results: []pattern.TestTableItem{
						{Name: "TestSkipped", Status: pattern.StatusSkip},
					},
				},
			},
			wants: []string{"SKIP TestSkipped"},
		},
		{
			name: "sarif rule without line number renders without location",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "pkg/b.go",
					Results: []pattern.TestTableItem{
						{Name: "RULE999", Status: pattern.StatusFail, Details: "no location"},
					},
				},
			},
			wants: []string{"ERR RULE999 no location"},
		},
		{
			name: "sarif scope counts mixed severities",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "a.go",
					Results: []pattern.TestTableItem{
						{Name: "r1:1:1", Status: pattern.StatusFail},
						{Name: "r2:2:1", Status: pattern.StatusFail},
					},
				},
				&pattern.TestTable{
					Label: "b.go",
					Results: []pattern.TestTableItem{
						{Name: "r3:1:1", Status: pattern.StatusSkip},
					},
				},
			},
			wants: []string{"2 files", "3 diags", "2 err", "1 warn"},
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
