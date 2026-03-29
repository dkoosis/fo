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
			name: "test summary mode shows triage line and failure details",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label: "FAIL (0.5s)",
					Kind:  pattern.SummaryKindTest,
					Metrics: []pattern.SummaryItem{
						{Label: "Failed", Value: "1/1 tests", Kind: pattern.KindError},
						{Label: "Packages", Value: "1", Kind: pattern.KindInfo},
					},
				},
				&pattern.TestTable{
					Label: "FAIL pkg/foo (1/1 failed)",
					Results: []pattern.TestTableItem{
						{Name: "TestParser", Status: pattern.StatusFail, Duration: "0.02s", Details: "panic: boom"},
					},
				},
			},
			wants: []string{"1 ✗ / 1 tests", "pkg/foo", "TestParser", "panic: boom"},
		},
		{
			name: "sarif mode shows triage counts and findings",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "pkg/a.go",
					Results: []pattern.TestTableItem{
						{Name: "RULE001:12:3", Status: pattern.StatusFail, Details: "bad thing"},
					},
				},
			},
			wants: []string{"1 ✗ 0 ⚠", "pkg/a.go:12:3 RULE001 — bad thing"},
		},
		{
			name:     "empty patterns returns zero triage counts",
			patterns: nil,
			wants:    []string{"0 ✗ 0 ⚠"},
		},
		{
			name: "sarif sorts ✗ before ⚠ before ℹ",
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
			wants: []string{"✗ file.go:2:1 err1", "⚠ file.go:3:1 warn1", "ℹ file.go:1:1 note1"},
		},
		{
			name: "details truncated to 5 lines with overflow indicator",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "FAIL", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "failures",
					Results: []pattern.TestTableItem{
						{Name: "TestBig", Status: pattern.StatusFail, Details: "line1\nline2\nline3\nline4\nline5\nline6\nline7"},
					},
				},
			},
			wants: []string{"line1", "line2", "line3", "line4", "line5", "2 more lines"},
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
			name: "test skip items suppressed in output",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "PASS", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "skipped",
					Results: []pattern.TestTableItem{
						{Name: "TestSkipped", Status: pattern.StatusSkip},
					},
				},
			},
			wants: []string{"0 ✗ /"},
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
			wants: []string{"✗ pkg/b.go RULE999 — no location"},
		},
		{
			name: "sarif mixed severities counted in triage",
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
			wants: []string{"2 ✗ 1 ⚠"},
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

func TestLLM_VersionPreamble(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()
	out := r.Render(nil)
	if !strings.HasPrefix(out, "fo:llm:v1\n") {
		t.Fatalf("expected version preamble, got:\n%s", out)
	}
}

func TestLLM_ZeroANSI(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()
	out := r.Render([]pattern.Pattern{
		&pattern.TestTable{
			Label: "file.go",
			Results: []pattern.TestTableItem{
				{Name: "rule:1:1", Status: pattern.StatusFail, Details: "bad"},
			},
		},
	})
	if strings.Contains(out, "\033[") {
		t.Fatal("LLM output contains ANSI escape codes")
	}
}

func TestLLM_SARIF_TriageLine(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wants    []string
	}{
		{
			name:     "empty input shows zero counts",
			patterns: nil,
			wants:    []string{"0 ✗ 0 ⚠"},
		},
		{
			name: "errors and warnings counted",
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
			wants: []string{"2 ✗ 1 ⚠"},
		},
		{
			name: "note count shown when non-zero",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "c.go",
					Results: []pattern.TestTableItem{
						{Name: "r1:1:1", Status: pattern.StatusPass},
					},
				},
			},
			wants: []string{"0 ✗ 0 ⚠ 1 ℹ"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render(tc.patterns)
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestLLM_Test_TriageLine(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name    string
		patterns []pattern.Pattern
		wants   []string
		rejects []string
	}{
		{
			name: "failures show count with symbol",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label:   "FAIL 2/5 tests, 1 packages affected (1.0s)",
					Kind:    pattern.SummaryKindTest,
					Metrics: []pattern.SummaryItem{
						{Label: "Failed", Value: "2/5 tests", Kind: pattern.KindError},
						{Label: "Packages", Value: "1", Kind: pattern.KindInfo},
					},
				},
				&pattern.TestTable{
					Label: "FAIL pkg/handler (2/5 failed)",
					Results: []pattern.TestTableItem{
						{Name: "TestA", Status: pattern.StatusFail},
						{Name: "TestB", Status: pattern.StatusFail},
					},
				},
			},
			wants: []string{"2 ✗ /"},
		},
		{
			name: "all pass shows zero errors, no package listing",
			patterns: []pattern.Pattern{
				&pattern.Summary{
					Label:   "PASS (1.0s)",
					Kind:    pattern.SummaryKindTest,
					Metrics: []pattern.SummaryItem{
						{Label: "Passed", Value: "2/2 tests", Kind: pattern.KindSuccess},
						{Label: "Packages", Value: "2", Kind: pattern.KindInfo},
					},
				},
				&pattern.TestTable{
					Label:   "Passing Packages (2)",
					Results: []pattern.TestTableItem{
						{Name: "pkg/a", Status: pattern.StatusPass},
						{Name: "pkg/b", Status: pattern.StatusPass},
					},
				},
			},
			wants:   []string{"0 ✗ /"},
			rejects: []string{"pkg/a", "pkg/b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render(tc.patterns)
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected %q, got:\n%s", want, out)
				}
			}
			for _, reject := range tc.rejects {
				if strings.Contains(out, reject) {
					t.Fatalf("unexpected %q in:\n%s", reject, out)
				}
			}
		})
	}
}

func TestLLM_Test_FailuresOnly(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label:   "FAIL 1/3 tests, 1 packages affected (0.5s)",
			Kind:    pattern.SummaryKindTest,
			Metrics: []pattern.SummaryItem{
				{Label: "Failed", Value: "1/3 tests", Kind: pattern.KindError},
				{Label: "Packages", Value: "1", Kind: pattern.KindInfo},
			},
		},
		&pattern.TestTable{
			Label: "FAIL pkg/handler (1/3 failed)",
			Results: []pattern.TestTableItem{
				{Name: "TestBad", Status: pattern.StatusFail, Duration: "0.3s", Details: "handler_test.go:45: expected 404, got 500"},
				{Name: "TestGood", Status: pattern.StatusPass, Duration: "0.1s"},
				{Name: "TestSkipped", Status: pattern.StatusSkip},
			},
		},
	})

	if !strings.Contains(out, "✗") {
		t.Fatalf("expected ✗ symbol, got:\n%s", out)
	}
	if !strings.Contains(out, "TestBad") {
		t.Fatalf("expected TestBad, got:\n%s", out)
	}
	if !strings.Contains(out, "handler_test.go:45") {
		t.Fatalf("expected failure detail, got:\n%s", out)
	}
	if strings.Contains(out, "TestGood") {
		t.Fatalf("passing test should be suppressed, got:\n%s", out)
	}
	if strings.Contains(out, "TestSkipped") {
		t.Fatalf("skipped test should be suppressed, got:\n%s", out)
	}
}

func TestLLM_Test_DetailTruncation(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{Label: "FAIL", Kind: pattern.SummaryKindTest},
		&pattern.TestTable{
			Label: "failures",
			Results: []pattern.TestTableItem{
				{Name: "TestBig", Status: pattern.StatusFail, Details: "line1\nline2\nline3\nline4\nline5\nline6\nline7"},
			},
		},
	})

	for _, want := range []string{"line1", "line2", "line3", "line4", "line5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line6") {
		t.Fatalf("line6 should be truncated, got:\n%s", out)
	}
	if !strings.Contains(out, "2 more lines") {
		t.Fatalf("expected overflow indicator, got:\n%s", out)
	}
}

func TestLLM_SARIF_FindingFormat(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wants    []string
	}{
		{
			name: "error with file:line:col",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "store.go",
					Results: []pattern.TestTableItem{
						{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "error return not checked"},
					},
				},
			},
			wants: []string{"✗ store.go:42:5 errcheck — error return not checked"},
		},
		{
			name: "warning finding",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "main.go",
					Results: []pattern.TestTableItem{
						{Name: "printf:10:3", Status: pattern.StatusSkip, Details: "format mismatch"},
					},
				},
			},
			wants: []string{"⚠ main.go:10:3 printf — format mismatch"},
		},
		{
			name: "rule without line number",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Label: "pkg/b.go",
					Results: []pattern.TestTableItem{
						{Name: "RULE999", Status: pattern.StatusFail, Details: "no location"},
					},
				},
			},
			wants: []string{"✗ pkg/b.go RULE999 — no location"},
		},
		{
			name: "severity sort: ✗ before ⚠ before ℹ",
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
			wants: []string{"✗ file.go:2:1 err1", "⚠ file.go:3:1 warn1", "ℹ file.go:1:1 note1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render(tc.patterns)
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected %q, got:\n%s", want, out)
				}
			}
		})
	}
}
