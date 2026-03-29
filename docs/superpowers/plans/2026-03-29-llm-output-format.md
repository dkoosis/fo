# LLM Output Format Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the LLM renderer to produce a consistent, Claude-optimized output format with unified severity symbols, triage-first layout, and noise suppression.

**Architecture:** Single file rewrite of `pkg/render/llm.go` — replace all three render paths (SARIF, test, report) with the new format. Tests rewritten to match. No changes to interfaces, pattern types, or other renderers.

**Tech Stack:** Go, existing `pkg/pattern` types, `strings.Builder`

**Spec:** `docs/superpowers/specs/2026-03-29-llm-output-format.md`

---

### File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Rewrite | `pkg/render/llm.go` | LLM renderer — all three paths |
| Rewrite | `pkg/render/llm_test.go` | LLM renderer tests |

No new files. No other files touched.

---

### Task 1: Add constants and severity helpers

**Files:**
- Modify: `pkg/render/llm.go:1-15` (add constants, keep struct/constructor)

- [ ] **Step 1: Write failing test for severity symbol mapping**

Add to `pkg/render/llm_test.go`, replacing the entire file:

```go
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
		&pattern.Summary{
			Label: "test",
			Kind:  pattern.SummaryKindSARIF,
			Metrics: []pattern.SummaryItem{
				{Label: "errors", Value: "1", Kind: pattern.KindError},
			},
		},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/render/ -run TestLLM_VersionPreamble -v`
Expected: FAIL — output doesn't start with `fo:llm:v1`

- [ ] **Step 3: Add constants and version preamble**

Replace the top of `pkg/render/llm.go` (lines 1-15) with:

```go
package render

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/pattern"
)

const (
	formatVersion  = "fo:llm:v1"
	maxDetailLines = 5
	symError       = "✗"
	symWarning     = "⚠"
	symNote        = "ℹ"
	symPass        = "✔"
)

// LLM renders patterns as terse plain text optimized for LLM consumption.
// Zero ANSI codes, deterministic sort, severity-first, action-oriented.
type LLM struct{}

// NewLLM creates an LLM renderer.
func NewLLM() *LLM {
	return &LLM{}
}

// severitySymbol maps a SARIF-based status to a severity symbol.
func severitySymbol(status pattern.Status) string {
	switch status {
	case pattern.StatusFail:
		return symError
	case pattern.StatusSkip:
		return symWarning
	case pattern.StatusPass:
		return symNote
	default:
		return symNote
	}
}

// severityPriority returns sort priority (lower = more severe).
func severityPriority(sym string) int {
	switch sym {
	case symError:
		return 0
	case symWarning:
		return 1
	default:
		return 2
	}
}
```

- [ ] **Step 4: Stub Render to emit version preamble**

Replace the `Render` method with a temporary stub that just emits the preamble plus the old output. This keeps other tests from breaking while we rewrite incrementally:

```go
func (l *LLM) Render(patterns []pattern.Pattern) string {
	var sb strings.Builder
	sb.WriteString(formatVersion + "\n\n")

	// Separate by type
	var summaries []*pattern.Summary
	var tables []*pattern.TestTable
	var errors []*pattern.Error

	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.Summary:
			summaries = append(summaries, v)
		case *pattern.TestTable:
			tables = append(tables, v)
		case *pattern.Error:
			errors = append(errors, v)
		}
	}

	if len(summaries) > 0 {
		switch summaries[0].Kind {
		case pattern.SummaryKindReport:
			sb.WriteString(l.renderReport(summaries, tables, errors))
		case pattern.SummaryKindTest:
			sb.WriteString(l.renderTestOutput(summaries, tables))
		case pattern.SummaryKindSARIF:
			sb.WriteString(l.renderSARIFOutput(tables))
		default:
			sb.WriteString(l.renderSARIFOutput(tables))
		}
	} else {
		sb.WriteString(l.renderSARIFOutput(tables))
	}

	return sb.String()
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./pkg/render/ -run TestLLM_VersionPreamble -v`
Expected: PASS

Run: `go test ./pkg/render/ -run TestLLM_ZeroANSI -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): add LLM format constants, version preamble, severity helpers"
```

---

### Task 2: Rewrite standalone SARIF path

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderSARIFOutput`, replace `sarifScope`)
- Modify: `pkg/render/llm_test.go` (add SARIF-specific tests)

- [ ] **Step 1: Write failing tests for new SARIF format**

Append to `pkg/render/llm_test.go`:

```go
func TestLLM_SARIF_TriageLine(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wants    []string
		rejects  []string
	}{
		{
			name:     "empty input shows zero counts",
			patterns: nil,
			wants:    []string{"0 ✗ 0 ⚠"},
		},
		{
			name: "error and warning counts",
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
			for _, reject := range tc.rejects {
				if strings.Contains(out, reject) {
					t.Fatalf("unexpected %q in:\n%s", reject, out)
				}
			}
		})
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
			name: "error finding with file:line:col",
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
			name: "severity sort: errors before warnings before notes",
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/render/ -run TestLLM_SARIF -v`
Expected: FAIL — old format doesn't use `✗`/`⚠`/`ℹ` symbols

- [ ] **Step 3: Rewrite renderSARIFOutput**

Replace `renderSARIFOutput`, `sarifScope`, `parseRuleLocation`, `llmLevelPriority` in `llm.go` with:

```go
func (l *LLM) renderSARIFOutput(tables []*pattern.TestTable) string {
	var sb strings.Builder

	// Count by severity
	var errCount, warnCount, noteCount int
	type diagEntry struct {
		file    string
		sym     string
		rule    string
		line    int
		col     int
		message string
	}

	var diags []diagEntry
	for _, t := range tables {
		for _, item := range t.Results {
			sym := severitySymbol(item.Status)
			switch sym {
			case symError:
				errCount++
			case symWarning:
				warnCount++
			default:
				noteCount++
			}

			rule, line, col := parseRuleLocation(item.Name)
			diags = append(diags, diagEntry{
				file:    t.Label,
				sym:     sym,
				rule:    rule,
				line:    line,
				col:     col,
				message: item.Details,
			})
		}
	}

	// Triage line
	sb.WriteString(fmt.Sprintf("%d %s %d %s", errCount, symError, warnCount, symWarning))
	if noteCount > 0 {
		sb.WriteString(fmt.Sprintf(" %d %s", noteCount, symNote))
	}
	sb.WriteString("\n")

	if len(diags) == 0 {
		return sb.String()
	}

	// Sort: severity → file → line → rule
	sort.Slice(diags, func(i, j int) bool {
		pi, pj := severityPriority(diags[i].sym), severityPriority(diags[j].sym)
		if pi != pj {
			return pi < pj
		}
		if diags[i].file != diags[j].file {
			return diags[i].file < diags[j].file
		}
		if diags[i].line != diags[j].line {
			return diags[i].line < diags[j].line
		}
		return diags[i].rule < diags[j].rule
	})

	sb.WriteString("\n")
	for _, d := range diags {
		sb.WriteString("  " + d.sym + " ")
		if d.line > 0 {
			sb.WriteString(fmt.Sprintf("%s:%d:%d %s", d.file, d.line, d.col, d.rule))
		} else {
			sb.WriteString(d.file + " " + d.rule)
		}
		if d.message != "" {
			sb.WriteString(" — " + d.message)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
```

Keep `parseRuleLocation` unchanged (it's still correct). Remove `sarifScope` and `llmLevelPriority` (replaced by helpers from Task 1).

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/render/ -run TestLLM_SARIF -v`
Expected: PASS

Run: `go test ./pkg/render/ -run TestLLM_VersionPreamble -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM SARIF output with severity symbols and triage line"
```

---

### Task 3: Rewrite standalone go test path

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderTestOutput`)
- Modify: `pkg/render/llm_test.go` (add test-specific tests)

- [ ] **Step 1: Write failing tests for new test format**

Append to `pkg/render/llm_test.go`:

```go
func TestLLM_Test_TriageLine(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name     string
		patterns []pattern.Pattern
		wants    []string
		rejects  []string
	}{
		{
			name: "failures show count with symbol",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "FAIL 2/5 tests, 1 packages affected (1.0s)", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label: "FAIL pkg/handler (2/5 failed)",
					Results: []pattern.TestTableItem{
						{Name: "TestA", Status: pattern.StatusFail},
						{Name: "TestB", Status: pattern.StatusFail},
					},
				},
			},
			wants: []string{"2 ✗"},
		},
		{
			name: "all pass shows zero errors",
			patterns: []pattern.Pattern{
				&pattern.Summary{Label: "PASS (1.0s)", Kind: pattern.SummaryKindTest},
				&pattern.TestTable{
					Label:   "Passing Packages (2)",
					Results: []pattern.TestTableItem{
						{Name: "pkg/a", Status: pattern.StatusPass},
						{Name: "pkg/b", Status: pattern.StatusPass},
					},
				},
			},
			wants:   []string{"0 ✗"},
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
		&pattern.Summary{Label: "FAIL 1/3 tests, 1 packages affected (0.5s)", Kind: pattern.SummaryKindTest},
		&pattern.TestTable{
			Label: "FAIL pkg/handler (1/3 failed)",
			Results: []pattern.TestTableItem{
				{Name: "TestBad", Status: pattern.StatusFail, Duration: "0.3s", Details: "handler_test.go:45: expected 404, got 500"},
				{Name: "TestGood", Status: pattern.StatusPass, Duration: "0.1s"},
				{Name: "TestSkipped", Status: pattern.StatusSkip},
			},
		},
	})

	// Failed test visible with symbol and details
	if !strings.Contains(out, "✗") {
		t.Fatalf("expected ✗ symbol, got:\n%s", out)
	}
	if !strings.Contains(out, "TestBad") {
		t.Fatalf("expected TestBad, got:\n%s", out)
	}
	if !strings.Contains(out, "handler_test.go:45") {
		t.Fatalf("expected failure detail, got:\n%s", out)
	}
	// Passing and skipped tests suppressed
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

	// First 5 lines shown
	for _, want := range []string{"line1", "line2", "line3", "line4", "line5"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q, got:\n%s", want, out)
		}
	}
	// Line 6+ truncated
	if strings.Contains(out, "line6") {
		t.Fatalf("line6 should be truncated, got:\n%s", out)
	}
	if !strings.Contains(out, "2 more lines") {
		t.Fatalf("expected overflow indicator, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/render/ -run TestLLM_Test -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderTestOutput**

Replace `renderTestOutput` in `llm.go`:

```go
func (l *LLM) renderTestOutput(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	// Count failures and total tests across all tables
	var failCount, totalCount int
	var failedPkgCount int
	for _, t := range tables {
		hasFail := false
		for _, item := range t.Results {
			totalCount++
			if item.Status == pattern.StatusFail {
				failCount++
				hasFail = true
			}
		}
		if hasFail {
			failedPkgCount++
		}
	}

	// Extract duration and package count from summary label if available
	duration := ""
	pkgCount := 0
	if len(summaries) > 0 {
		label := summaries[0].Label
		// Extract duration: look for (Ns) or (N.Ns) pattern
		if idx := strings.LastIndex(label, "("); idx >= 0 {
			if end := strings.Index(label[idx:], ")"); end >= 0 {
				duration = label[idx+1 : idx+end]
			}
		}
		// Count packages from summary metrics
		for _, m := range summaries[0].Metrics {
			if m.Label == "Packages" {
				pkgCount, _ = strconv.Atoi(m.Value)
			}
		}
	}
	if pkgCount == 0 {
		pkgCount = len(tables)
	}

	// Triage line
	sb.WriteString(fmt.Sprintf("%d %s / %d tests %d pkg", failCount, symError, totalCount, pkgCount))
	if duration != "" {
		sb.WriteString(" (" + duration + ")")
	}
	sb.WriteString("\n")

	// Only render failed tests
	if failCount == 0 {
		return sb.String()
	}

	sb.WriteString("\n")
	for _, t := range tables {
		for _, item := range t.Results {
			if item.Status != pattern.StatusFail {
				continue
			}
			sb.WriteString("  " + symError + " ")
			// Extract package from table label
			pkg := t.Label
			sb.WriteString(pkg + " " + item.Name)
			if item.Duration != "" {
				sb.WriteString(" (" + item.Duration + ")")
			}
			sb.WriteString("\n")
			writeDetails(&sb, item.Details)
		}
	}

	return sb.String()
}
```

- [ ] **Step 4: Update writeDetails to use maxDetailLines**

Replace `writeDetails` in `llm.go`:

```go
func writeDetails(sb *strings.Builder, details string) {
	if details == "" {
		return
	}
	lines := strings.Split(details, "\n")
	show := maxDetailLines
	if len(lines) < show {
		show = len(lines)
	}
	for _, line := range lines[:show] {
		sb.WriteString("    " + line + "\n")
	}
	if len(lines) > maxDetailLines {
		fmt.Fprintf(sb, "    ... (%d more lines)\n", len(lines)-maxDetailLines)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./pkg/render/ -run TestLLM_Test -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM test output — failures only, symbol triage"
```

---

### Task 4: Rewrite report path

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderReport`)
- Modify: `pkg/render/llm_test.go` (add report-specific tests)

- [ ] **Step 1: Write failing tests for new report format**

Append to `pkg/render/llm_test.go`:

```go
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
			Label:  "internal/store/store.go",
			Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "error return not checked"},
				{Name: "unused:90:6", Status: pattern.StatusFail, Details: "func `helper` unused"},
			},
		},
	})

	// Triage line: lint fails, vet and test pass
	if !strings.Contains(out, "2 ✗ 0 ⚠") {
		t.Fatalf("expected triage counts, got:\n%s", out)
	}
	if !strings.Contains(out, "lint") {
		t.Fatalf("expected lint in failing tools, got:\n%s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Fatalf("expected ✔ for passing tools, got:\n%s", out)
	}

	// Only lint gets a section — vet and test are clean
	if !strings.Contains(out, "## lint") {
		t.Fatalf("expected ## lint section, got:\n%s", out)
	}
	if strings.Contains(out, "## vet") {
		t.Fatalf("clean tool vet should not have section, got:\n%s", out)
	}
	if strings.Contains(out, "## test") {
		t.Fatalf("clean tool test should not have section, got:\n%s", out)
	}

	// Finding format
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
	// No ## sections
	if strings.Contains(out, "##") {
		t.Fatalf("all-pass should have no sections, got:\n%s", out)
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

	// Error patterns make the tool fail
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

	// vet section should appear before lint (report delimiter order)
	vetIdx := strings.Index(out, "## vet")
	lintIdx := strings.Index(out, "## lint")
	if vetIdx == -1 || lintIdx == -1 {
		t.Fatalf("expected both sections, got:\n%s", out)
	}
	if vetIdx > lintIdx {
		t.Fatalf("vet should appear before lint (delimiter order), got:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/render/ -run TestLLM_Report -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderReport**

Replace `renderReport` in `llm.go`:

```go
func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable, errors []*pattern.Error) string {
	var sb strings.Builder

	reportSummary := summaries[0]

	// Group tables and errors by tool (Source field)
	tablesByTool := make(map[string][]*pattern.TestTable)
	for _, t := range tables {
		if t.Source != "" {
			tablesByTool[t.Source] = append(tablesByTool[t.Source], t)
		}
	}
	errorsByTool := make(map[string][]*pattern.Error)
	for _, e := range errors {
		errorsByTool[e.Source] = append(errorsByTool[e.Source], e)
	}

	// Count findings per severity and classify tools
	var errTotal, warnTotal, noteTotal int
	var failingTools, passingTools []string

	for _, m := range reportSummary.Metrics {
		tool := m.Label
		toolErrors := 0
		toolHasErrorPattern := len(errorsByTool[tool]) > 0

		for _, t := range tablesByTool[tool] {
			for _, item := range t.Results {
				sym := severitySymbol(item.Status)
				switch sym {
				case symError:
					errTotal++
					toolErrors++
				case symWarning:
					warnTotal++
				default:
					noteTotal++
				}
			}
		}

		// Error patterns count as errors
		errTotal += len(errorsByTool[tool])
		toolErrors += len(errorsByTool[tool])

		if toolErrors > 0 || toolHasErrorPattern {
			failingTools = append(failingTools, tool)
		} else {
			passingTools = append(passingTools, tool)
		}
	}

	// Triage line
	sb.WriteString(fmt.Sprintf("%d %s %d %s", errTotal, symError, warnTotal, symWarning))
	if noteTotal > 0 {
		sb.WriteString(fmt.Sprintf(" %d %s", noteTotal, symNote))
	}
	if len(failingTools) > 0 {
		sb.WriteString(" | " + strings.Join(failingTools, " "))
	}
	sb.WriteString(" | " + strings.Join(passingTools, " ") + " " + symPass)
	sb.WriteString("\n")

	// Tool sections — only for tools with findings, in report delimiter order
	for _, m := range reportSummary.Metrics {
		tool := m.Label
		toolTables := tablesByTool[tool]
		toolErrors := errorsByTool[tool]
		if len(toolTables) == 0 && len(toolErrors) == 0 {
			continue
		}

		sb.WriteString("\n## " + tool + "\n")

		// Error patterns first
		for _, e := range toolErrors {
			sb.WriteString("  " + symError + " " + e.Message + "\n")
		}

		// SARIF-style findings — collect, sort, render
		type diagEntry struct {
			sym     string
			file    string
			rule    string
			line    int
			col     int
			message string
		}
		var diags []diagEntry
		for _, t := range toolTables {
			for _, item := range t.Results {
				rule, line, col := parseRuleLocation(item.Name)
				diags = append(diags, diagEntry{
					sym:     severitySymbol(item.Status),
					file:    t.Label,
					rule:    rule,
					line:    line,
					col:     col,
					message: item.Details,
				})
			}
		}

		sort.Slice(diags, func(i, j int) bool {
			pi, pj := severityPriority(diags[i].sym), severityPriority(diags[j].sym)
			if pi != pj {
				return pi < pj
			}
			if diags[i].file != diags[j].file {
				return diags[i].file < diags[j].file
			}
			if diags[i].line != diags[j].line {
				return diags[i].line < diags[j].line
			}
			return diags[i].rule < diags[j].rule
		})

		for _, d := range diags {
			sb.WriteString("  " + d.sym + " ")
			if d.line > 0 {
				sb.WriteString(fmt.Sprintf("%s:%d:%d %s", d.file, d.line, d.col, d.rule))
			} else {
				sb.WriteString(d.file + " " + d.rule)
			}
			if d.message != "" {
				sb.WriteString(" — " + d.message)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/render/ -run TestLLM_Report -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test -race ./...`
Expected: All pass. Some old tests in `cmd/fo/main_test.go` will now fail because they assert old LLM format strings — that's expected and handled in Task 5.

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM report output — triage line, tool sections, suppress clean"
```

---

### Task 5: Remove old LLM tests and update integration tests

**Files:**
- Modify: `pkg/render/llm_test.go` (remove old test function)
- Modify: `cmd/fo/main_test.go` (update assertions for new format)

- [ ] **Step 1: Remove old test function**

Delete `TestLLMRender_KeyUserVisibleOutput` from `pkg/render/llm_test.go` entirely — it tests the old format. The new tests from Tasks 1-4 replace it.

- [ ] **Step 2: Update cmd/fo integration tests**

Read `cmd/fo/main_test.go` and update all assertions that check LLM output. The key changes:

- Replace `"SCOPE:"` checks with triage line checks (`"✗"`, `"⚠"`)
- Replace `"ERR "` / `"WARN "` / `"NOTE "` with `"✗ "` / `"⚠ "` / `"ℹ "`
- Replace `"FAIL "` / `"PASS "` / `"SKIP "` in test output with `"✗ "`
- Add `"fo:llm:v1"` check to at least one test
- Replace `"REPORT:"` check with triage line check
- Remove assertions for passing test output (now suppressed)

For each test, read the current assertions and update to match the new format. Run each test individually after updating to verify.

- [ ] **Step 3: Run full test suite**

Run: `go test -race ./...`
Expected: All pass

Run: `golangci-lint run ./...`
Expected: 0 issues

- [ ] **Step 4: Commit**

```bash
git add pkg/render/llm_test.go cmd/fo/main_test.go
git commit -m "test: update all LLM output assertions for new format"
```

---

### Task 6: Clean up dead code and verify

**Files:**
- Modify: `pkg/render/llm.go` (remove dead functions)

- [ ] **Step 1: Remove dead functions**

Remove these functions if they still exist in `llm.go` after the rewrites:
- `sarifScope` (replaced by inline triage line builder)
- `llmLevelPriority` (replaced by `severityPriority`)

- [ ] **Step 2: Run full QA**

Run: `make qa`
Expected: All pass — build, test, vet, lint clean

- [ ] **Step 3: Verify output manually with demo fixtures**

```bash
cat cmd/fo/testdata/demo/sarif-mixed.json | ./fo --format llm
cat cmd/fo/testdata/demo/gotest-mixed.json | ./fo --format llm
cat cmd/fo/testdata/demo/gotest-clean.json | ./fo --format llm
cat cmd/fo/testdata/demo/report-full.report | ./fo --format llm
```

Verify each output matches the spec format visually.

- [ ] **Step 4: Commit and push**

```bash
git add pkg/render/llm.go
git commit -m "refactor(render): remove dead LLM helpers"
git push
```
