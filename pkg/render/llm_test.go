package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

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

func TestLLM_Report_TriageAndSections(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 3 tools — 1 fail, 2 pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: pattern.KindSuccess},
				{Label: "lint", Value: "2 err", Kind: pattern.KindError},
				{Label: "test", Value: "PASS", Kind: pattern.KindSuccess},
			},
		},
		&pattern.TestTable{
			Label: "internal/store/store.go", Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "error return not checked"},
				{Name: "unused:90:6", Status: pattern.StatusFail, Details: "func `helper` unused"},
			},
		},
	})

	if !strings.Contains(out, "2 ✗ 0 ⚠") {
		t.Fatalf("expected triage counts, got:\n%s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Fatalf("expected ✔ for passing tools, got:\n%s", out)
	}
	if !strings.Contains(out, "## lint") {
		t.Fatalf("expected ## lint section, got:\n%s", out)
	}
	if strings.Contains(out, "## vet") {
		t.Fatalf("clean tool vet should not have section, got:\n%s", out)
	}
	if strings.Contains(out, "## test") {
		t.Fatalf("clean tool test should not have section, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ internal/store/store.go:42:5 errcheck — error return not checked") {
		t.Fatalf("expected formatted finding, got:\n%s", out)
	}
}

func TestLLM_Report_AllPass(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 3 tools — all pass",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: pattern.KindSuccess},
				{Label: "lint", Value: "0 diags", Kind: pattern.KindSuccess},
				{Label: "test", Value: "PASS", Kind: pattern.KindSuccess},
			},
		},
	})

	if !strings.Contains(out, "0 ✗ 0 ⚠") {
		t.Fatalf("expected zero counts, got:\n%s", out)
	}
	if !strings.Contains(out, "vet lint test ✔") {
		t.Fatalf("expected all tools passing, got:\n%s", out)
	}
	if strings.Contains(out, "##") {
		t.Fatalf("all-pass should have no sections, got:\n%s", out)
	}
}

func TestLLM_Report_MixedSARIFAndTest(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — 2 fail",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "lint", Value: "1 err", Kind: pattern.KindError},
				{Label: "test", Value: "FAIL — 1 failed", Kind: pattern.KindError},
			},
		},
		&pattern.TestTable{
			Label: "internal/store.go", Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "error return not checked"},
			},
		},
		&pattern.TestTable{
			Label: "FAIL pkg/handler (1/3 failed)", Source: "test",
			Results: []pattern.TestTableItem{
				{Name: "TestDelete", Status: pattern.StatusFail, Duration: "0.2s", Details: "expected 204, got 500"},
				{Name: "TestCreate", Status: pattern.StatusPass, Duration: "0.1s"},
			},
		},
	})

	if !strings.Contains(out, "## lint") {
		t.Fatalf("expected ## lint, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ internal/store.go:42:5 errcheck") {
		t.Fatalf("expected SARIF-format lint finding, got:\n%s", out)
	}
	if !strings.Contains(out, "## test") {
		t.Fatalf("expected ## test, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ pkg/handler TestDelete") {
		t.Fatalf("expected test-format finding, got:\n%s", out)
	}
	if strings.Contains(out, "TestCreate") {
		t.Fatalf("passing test should be suppressed, got:\n%s", out)
	}
}

func TestLLM_Report_ErrorPattern(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "lint", Value: "crashed", Kind: pattern.KindError},
			},
		},
		&pattern.Error{Source: "lint", Message: "config parse failed"},
	})

	if !strings.Contains(out, "1 ✗") {
		t.Fatalf("expected error count, got:\n%s", out)
	}
	if !strings.Contains(out, "## lint") {
		t.Fatalf("expected lint section, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ config parse failed") {
		t.Fatalf("expected error finding, got:\n%s", out)
	}
}

func TestLLM_Report_NoteCountOmission(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "lint", Value: "1 err", Kind: pattern.KindError},
			},
		},
		&pattern.TestTable{
			Label: "a.go", Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "r:1:1", Status: pattern.StatusFail, Details: "bad"},
			},
		},
	})
	if strings.Contains(out, "ℹ") {
		t.Fatalf("ℹ count should be omitted when zero, got:\n%s", out)
	}
}

func TestLLM_Report_ToolOrder(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "1 warn", Kind: pattern.KindWarning},
				{Label: "lint", Value: "1 err", Kind: pattern.KindError},
			},
		},
		&pattern.TestTable{
			Label: "a.go", Source: "vet",
			Results: []pattern.TestTableItem{
				{Name: "printf:1:1", Status: pattern.StatusSkip, Details: "format"},
			},
		},
		&pattern.TestTable{
			Label: "b.go", Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "unused:1:1", Status: pattern.StatusFail, Details: "unused"},
			},
		},
	})

	vetIdx := strings.Index(out, "## vet")
	lintIdx := strings.Index(out, "## lint")
	if vetIdx == -1 || lintIdx == -1 {
		t.Fatalf("expected both sections, got:\n%s", out)
	}
	if vetIdx > lintIdx {
		t.Fatalf("vet should appear before lint (delimiter order), got:\n%s", out)
	}
}

func TestLLM_FixCommand_SARIF(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	t.Run("non-empty FixCommand renders fenced bash block", func(t *testing.T) {
		t.Parallel()
		out := r.Render([]pattern.Pattern{
			&pattern.TestTable{
				Label: "store.go",
				Results: []pattern.TestTableItem{
					{
						Name:       "errcheck:42:5",
						Status:     pattern.StatusFail,
						Details:    "error return not checked",
						FixCommand: "go fix ./...",
					},
				},
			},
		})
		if !strings.Contains(out, "```bash") {
			t.Fatalf("expected fenced bash block, got:\n%s", out)
		}
		if !strings.Contains(out, "go fix ./...") {
			t.Fatalf("expected fix command in output, got:\n%s", out)
		}
	})

	t.Run("empty FixCommand renders nothing", func(t *testing.T) {
		t.Parallel()
		out := r.Render([]pattern.Pattern{
			&pattern.TestTable{
				Label: "store.go",
				Results: []pattern.TestTableItem{
					{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "msg"},
				},
			},
		})
		if strings.Contains(out, "```") {
			t.Fatalf("expected no fenced block when FixCommand empty, got:\n%s", out)
		}
	})
}

func TestLLM_FixCommand_TestFailure(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "tests (1.0s)",
			Kind:  pattern.SummaryKindTest,
			Metrics: []pattern.SummaryItem{
				{Label: "Failed", Value: "1/1", Kind: pattern.KindError},
				{Label: "Passed", Value: "0/1"},
				{Label: "Packages", Value: "1"},
			},
		},
		&pattern.TestTable{
			Label: "FAIL pkg/foo (0.1s)",
			Results: []pattern.TestTableItem{
				{
					Name:       "TestBad",
					Status:     pattern.StatusFail,
					Details:    "expected 200",
					FixCommand: "go test -run TestBad ./pkg/foo",
				},
			},
		},
	})

	if !strings.Contains(out, "```bash") {
		t.Fatalf("expected fenced bash block for test failure, got:\n%s", out)
	}
	if !strings.Contains(out, "go test -run TestBad ./pkg/foo") {
		t.Fatalf("expected fix command in output, got:\n%s", out)
	}
}
