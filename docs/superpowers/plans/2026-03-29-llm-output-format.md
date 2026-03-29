# LLM Output Format Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite the LLM renderer to produce a consistent, Claude-optimized output format with unified severity symbols, triage-first layout, and noise suppression.

**Architecture:** Single file rewrite of `pkg/render/llm.go` — replace all three render paths (SARIF, test, report) with the new format. Tests rewritten to match. No changes to interfaces, pattern types, or other renderers.

**Tech Stack:** Go, existing `pkg/pattern` types, `strings.Builder`

**Spec:** `docs/superpowers/specs/2026-03-29-llm-output-format.md`

---

### Precondition: Status Enum Dual-Meaning

The `pattern.Status` type (`pass`/`fail`/`skip`) carries different semantics depending on the mapper:

| Context | `StatusFail` | `StatusSkip` | `StatusPass` |
|---------|-------------|-------------|-------------|
| SARIF mapper | SARIF `error` | SARIF `warning` | SARIF `note` |
| TestJSON mapper | test failure | (unused) | test passed |

This dual-meaning is an existing coupling between the SARIF mapper and renderers. The LLM renderer handles it by using `severitySymbol()` only for SARIF-context items and handling test-context items separately (always `✗` for failures, dropping passes/skips). The renderer **must not** pass test-mode StatusPass/StatusSkip items through `severitySymbol()` — they would be misclassified as `ℹ`/`⚠`.

**How the renderer distinguishes SARIF vs test tables:** `parseRuleLocation(item.Name)` returns `line > 0` for SARIF items (names are `rule:line:col`). Test items have names like `TestFoo`, `PANIC`, `BUILD ERROR` — never containing `:`-separated numbers. This heuristic is reliable because Go test function names cannot contain colons.

---

### File Structure

| Action | File | Responsibility |
|--------|------|----------------|
| Rewrite | `pkg/render/llm.go` | LLM renderer — all three paths |
| Rewrite | `pkg/render/llm_test.go` | LLM renderer tests |
| Update | `cmd/fo/main_test.go` | Integration test assertions |

---

### Task 1: Add constants, severity helpers, version preamble

**Files:**
- Modify: `pkg/render/llm.go:1-15` (add constants, keep struct/constructor)
- Modify: `pkg/render/llm_test.go` (append new tests, keep old tests passing)

- [ ] **Step 1: Write failing tests**

Append to the existing `pkg/render/llm_test.go` (do NOT replace the file — old tests must keep passing during incremental rewrite):

```go
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
```

- [ ] **Step 2: Run tests to verify preamble test fails**

Run: `go test ./pkg/render/ -run TestLLM_VersionPreamble -v`
Expected: FAIL — output doesn't start with `fo:llm:v1`

- [ ] **Step 3: Add constants and helpers to llm.go**

Replace lines 1-15 of `pkg/render/llm.go` with:

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

// severitySymbol maps a SARIF-context Status to a severity symbol.
// ONLY use for SARIF-mode items where StatusFail=error, StatusSkip=warning, StatusPass=note.
// For test-mode items, always use symError for failures and drop passes/skips.
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

// isSARIFTable returns true if the table contains SARIF-style rule:line:col items.
func isSARIFTable(t *pattern.TestTable) bool {
	for _, item := range t.Results {
		_, line, _ := parseRuleLocation(item.Name)
		if line > 0 {
			return true
		}
	}
	// If no items parsed with line numbers, check if any name contains ':'
	// (SARIF rules like "errcheck" without location). Test names never have ':'.
	for _, item := range t.Results {
		if strings.Contains(item.Name, ":") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Prepend version preamble in Render method**

Modify the `Render` method to prepend the version line:

```go
func (l *LLM) Render(patterns []pattern.Pattern) string {
	var sb strings.Builder
	sb.WriteString(formatVersion + "\n\n")

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

- [ ] **Step 5: Run all render tests**

Run: `go test -race ./pkg/render/...`
Expected: All pass (old tests still work, new preamble test passes)

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): add LLM format constants, version preamble, severity helpers"
```

---

### Task 2: Rewrite standalone SARIF path

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderSARIFOutput`, remove `sarifScope`, `llmLevelPriority`)
- Modify: `pkg/render/llm_test.go` (append SARIF tests)

- [ ] **Step 1: Write failing tests**

Append to `pkg/render/llm_test.go`:

```go
func TestLLM_SARIF_TriageLine(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name    string
		patterns []pattern.Pattern
		wants   []string
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

func TestLLM_SARIF_FindingFormat(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	tests := []struct {
		name  string
		patterns []pattern.Pattern
		wants []string
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/render/ -run TestLLM_SARIF -v`
Expected: FAIL — old format uses ERR/WARN/NOTE text, not symbols

- [ ] **Step 3: Extract shared diagnostic helpers**

Add these shared types and helpers to `llm.go` (used by both SARIF and report paths — correction #8):

```go
// diagEntry represents a single diagnostic finding for LLM rendering.
type diagEntry struct {
	sym     string
	file    string
	rule    string
	line    int
	col     int
	message string
}

// collectSARIFDiags extracts diagnostic entries from SARIF-mode tables.
func collectSARIFDiags(tables []*pattern.TestTable) (diags []diagEntry, errCount, warnCount, noteCount int) {
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
				sym: sym, file: t.Label, rule: rule,
				line: line, col: col, message: item.Details,
			})
		}
	}
	return
}

// sortDiags sorts diagnostics by severity desc → file asc → line asc → rule asc.
func sortDiags(diags []diagEntry) {
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
}

// renderDiags writes sorted diagnostic lines to a builder.
func renderDiags(sb *strings.Builder, diags []diagEntry) {
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

// triageCounts formats the triage count string, omitting ℹ when zero.
func triageCounts(errCount, warnCount, noteCount int) string {
	s := fmt.Sprintf("%d %s %d %s", errCount, symError, warnCount, symWarning)
	if noteCount > 0 {
		s += fmt.Sprintf(" %d %s", noteCount, symNote)
	}
	return s
}
```

- [ ] **Step 4: Rewrite renderSARIFOutput**

Replace `renderSARIFOutput` in `llm.go`:

```go
func (l *LLM) renderSARIFOutput(tables []*pattern.TestTable) string {
	var sb strings.Builder

	diags, errCount, warnCount, noteCount := collectSARIFDiags(tables)
	sb.WriteString(triageCounts(errCount, warnCount, noteCount) + "\n")

	if len(diags) == 0 {
		return sb.String()
	}

	sortDiags(diags)
	sb.WriteString("\n")
	renderDiags(&sb, diags)

	return sb.String()
}
```

Remove `sarifScope` and `llmLevelPriority` (now dead code).

- [ ] **Step 5: Run all render tests**

Run: `go test -race ./pkg/render/...`
Expected: New SARIF tests pass. Some old `TestLLMRender_KeyUserVisibleOutput` subtests will now fail (they assert old format strings) — that's expected, they'll be removed in Task 5.

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM SARIF path with severity symbols and shared diag helpers"
```

---

### Task 3: Rewrite standalone go test path

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderTestOutput`, update `writeDetails`)
- Modify: `pkg/render/llm_test.go` (append test-specific tests)

- [ ] **Step 1: Write failing tests**

Append to `pkg/render/llm_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify failures**

Run: `go test ./pkg/render/ -run TestLLM_Test -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderTestOutput**

Replace `renderTestOutput` in `llm.go`. Note: the test mapper puts package info in `TestTable.Label` as `"FAIL pkg/handler (2/5 failed)"` or `"PANIC pkg/store"`. We extract the package name by taking the label and stripping the prefix/suffix. The `Source` field is only set in report mode.

```go
func (l *LLM) renderTestOutput(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	// Count failures and totals from summary metrics (reliable source)
	failCount, totalTests, pkgCount := 0, 0, 0
	duration := ""
	if len(summaries) > 0 {
		for _, m := range summaries[0].Metrics {
			switch m.Label {
			case "Failed":
				fmt.Sscanf(m.Value, "%d/", &failCount)
			case "Passed":
				fmt.Sscanf(m.Value, "%d/%d", new(int), &totalTests)
			case "Packages":
				fmt.Sscanf(m.Value, "%d", &pkgCount)
			case "Panics":
				fmt.Sscanf(m.Value, "%d", &failCount) // panics are failures
			case "Build Errors":
				n := 0
				fmt.Sscanf(m.Value, "%d", &n)
				failCount += n
			}
		}
		// Extract duration from summary label: "FAIL ... (1.0s)" or "PASS (1.0s)"
		label := summaries[0].Label
		if idx := strings.LastIndex(label, "("); idx >= 0 {
			if end := strings.Index(label[idx:], ")"); end >= 0 {
				duration = label[idx+1 : idx+end]
			}
		}
		// totalTests from "Passed" metric's denominator; fallback to counting items
		if totalTests == 0 {
			for _, t := range tables {
				totalTests += len(t.Results)
			}
		}
	}

	// Triage line
	sb.WriteString(fmt.Sprintf("%d %s / %d tests %d pkg", failCount, symError, totalTests, pkgCount))
	if duration != "" {
		sb.WriteString(" (" + duration + ")")
	}
	sb.WriteString("\n")

	// Only render failed tests
	hasFailures := false
	for _, t := range tables {
		for _, item := range t.Results {
			if item.Status != pattern.StatusFail {
				continue
			}
			if !hasFailures {
				sb.WriteString("\n")
				hasFailures = true
			}
			// Extract package from table label: "FAIL pkg/handler (2/5 failed)" → "pkg/handler"
			// or "PANIC pkg/store" → "pkg/store"
			pkg := extractPackage(t.Label)
			sb.WriteString("  " + symError + " " + pkg + " " + item.Name)
			if item.Duration != "" {
				sb.WriteString(" (" + item.Duration + ")")
			}
			sb.WriteString("\n")
			writeDetails(&sb, item.Details)
		}
	}

	return sb.String()
}

// extractPackage pulls the package name from a test table label.
// Handles: "FAIL pkg/handler (2/5 failed)", "PANIC pkg/store", "BUILD FAIL pkg/x", "Passing Packages (3)"
func extractPackage(label string) string {
	// Strip known prefixes
	for _, prefix := range []string{"PANIC ", "BUILD FAIL ", "FAIL "} {
		if strings.HasPrefix(label, prefix) {
			rest := label[len(prefix):]
			// Strip trailing " (N/M failed)" or similar
			if idx := strings.Index(rest, " ("); idx >= 0 {
				return rest[:idx]
			}
			return rest
		}
	}
	// "Passing Packages (N)" — shouldn't reach here since we skip passing
	return label
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

Run: `go test -race ./pkg/render/...`
Expected: New tests pass. Some old tests in `TestLLMRender_KeyUserVisibleOutput` may fail (expected).

- [ ] **Step 6: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM test output — failures only, symbol triage, extractPackage"
```

---

### Task 4: Rewrite report path with SARIF/test branching

**Files:**
- Modify: `pkg/render/llm.go` (rewrite `renderReport`)
- Modify: `pkg/render/llm_test.go` (append report tests including mixed SARIF+test)

This is the most complex task. The report renderer must: build a triage line with tool classification, emit `##` sections only for tools with findings, and **branch between SARIF and test rendering per tool** (correction #2).

- [ ] **Step 1: Write failing tests**

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
	if !strings.Contains(out, symPass) {
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
	if !strings.Contains(out, "vet lint test "+symPass) {
		t.Fatalf("expected all tools passing, got:\n%s", out)
	}
	if strings.Contains(out, "##") {
		t.Fatalf("all-pass should have no sections, got:\n%s", out)
	}
}

func TestLLM_Report_MixedSARIFAndTest(t *testing.T) {
	t.Parallel()
	r := render.NewLLM()

	// Simulates: make report with lint (SARIF) + test (testjson) both failing
	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — 2 fail",
			Kind:  pattern.SummaryKindReport,
			Metrics: []pattern.SummaryItem{
				{Label: "lint", Value: "1 err", Kind: pattern.KindError},
				{Label: "test", Value: "FAIL — 1 failed", Kind: pattern.KindError},
			},
		},
		// SARIF-style table (lint)
		&pattern.TestTable{
			Label: "internal/store.go", Source: "lint",
			Results: []pattern.TestTableItem{
				{Name: "errcheck:42:5", Status: pattern.StatusFail, Details: "error return not checked"},
			},
		},
		// Test-style tables (test) — from testjson mapper
		&pattern.TestTable{
			Label: "FAIL pkg/handler (1/3 failed)", Source: "test",
			Results: []pattern.TestTableItem{
				{Name: "TestDelete", Status: pattern.StatusFail, Duration: "0.2s", Details: "expected 204, got 500"},
				{Name: "TestCreate", Status: pattern.StatusPass, Duration: "0.1s"},
			},
		},
	})

	// lint section should use SARIF format
	if !strings.Contains(out, "## lint") {
		t.Fatalf("expected ## lint, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ internal/store.go:42:5 errcheck") {
		t.Fatalf("expected SARIF-format lint finding, got:\n%s", out)
	}

	// test section should use test format (not rule:line:col)
	if !strings.Contains(out, "## test") {
		t.Fatalf("expected ## test, got:\n%s", out)
	}
	if !strings.Contains(out, "✗ pkg/handler TestDelete") {
		t.Fatalf("expected test-format finding, got:\n%s", out)
	}
	// Passing test suppressed
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

	// Report with only errors (no notes) — ℹ count should be absent
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
	if strings.Contains(out, symNote) {
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
```

- [ ] **Step 2: Run to verify failures**

Run: `go test ./pkg/render/ -run TestLLM_Report -v`
Expected: FAIL

- [ ] **Step 3: Rewrite renderReport with SARIF/test branching**

Replace `renderReport` in `llm.go`:

```go
func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable, errors []*pattern.Error) string {
	var sb strings.Builder

	reportSummary := summaries[0]

	// Group tables and errors by tool
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

	// Count findings and classify tools
	var errTotal, warnTotal, noteTotal int
	var failingTools, passingTools []string

	for _, m := range reportSummary.Metrics {
		tool := m.Label
		toolErrors := len(errorsByTool[tool])

		for _, t := range tablesByTool[tool] {
			for _, item := range t.Results {
				if isSARIFTable(t) {
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
				} else {
					// Test-mode: only count failures
					if item.Status == pattern.StatusFail {
						errTotal++
						toolErrors++
					}
				}
			}
		}

		// Error patterns count as errors
		errTotal += len(errorsByTool[tool])

		if toolErrors > 0 {
			failingTools = append(failingTools, tool)
		} else {
			passingTools = append(passingTools, tool)
		}
	}

	// Triage line — guard empty groups (correction #3)
	sb.WriteString(triageCounts(errTotal, warnTotal, noteTotal))
	if len(failingTools) > 0 {
		sb.WriteString(" | " + strings.Join(failingTools, " "))
	}
	if len(passingTools) > 0 {
		sb.WriteString(" | " + strings.Join(passingTools, " ") + " " + symPass)
	}
	sb.WriteString("\n")

	// Tool sections — only tools with findings, in report delimiter order
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

		// Branch: SARIF vs test rendering
		hasSARIF := false
		for _, t := range toolTables {
			if isSARIFTable(t) {
				hasSARIF = true
				break
			}
		}

		if hasSARIF {
			// SARIF-style: collect, sort, render as diagnostics
			diags, _, _, _ := collectSARIFDiags(toolTables)
			sortDiags(diags)
			renderDiags(&sb, diags)
		} else {
			// Test-style: render failures with details, skip passes
			for _, t := range toolTables {
				for _, item := range t.Results {
					if item.Status != pattern.StatusFail {
						continue
					}
					pkg := extractPackage(t.Label)
					sb.WriteString("  " + symError + " " + pkg + " " + item.Name)
					if item.Duration != "" {
						sb.WriteString(" (" + item.Duration + ")")
					}
					sb.WriteString("\n")
					writeDetails(&sb, item.Details)
				}
			}
		}
	}

	return sb.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./pkg/render/ -run TestLLM_Report -v`
Expected: PASS

Run: `go test -race ./pkg/render/...`
Expected: New tests pass. Old `TestLLMRender_KeyUserVisibleOutput` subtests may still fail.

- [ ] **Step 5: Commit**

```bash
git add pkg/render/llm.go pkg/render/llm_test.go
git commit -m "feat(render): rewrite LLM report path — tool classification, SARIF/test branching"
```

---

### Task 5: Remove old tests and update integration tests

**Files:**
- Modify: `pkg/render/llm_test.go` (remove old test function)
- Modify: `cmd/fo/main_test.go` (update LLM assertions for new format)

- [ ] **Step 1: Remove old test function**

Delete `TestLLMRender_KeyUserVisibleOutput` from `pkg/render/llm_test.go` entirely. All its coverage is replaced by the tests from Tasks 1-4.

- [ ] **Step 2: Update cmd/fo integration tests**

Read `cmd/fo/main_test.go` and update all assertions for LLM output. Key changes:

- Add `"fo:llm:v1"` preamble check to `TestJTBD_RenderSARIFLintResults`
- Replace `"SCOPE:"` checks with triage line checks (`"✗"`, `"⚠"`)
- Replace `"ERR "` with `"✗ "`, `"WARN "` with `"⚠ "`
- Replace `"FAIL TestName"` with `"✗"` + test name
- Replace `"SCOPE: PASS"` / `"SCOPE: FAIL"` with `"0 ✗"` / `"N ✗"`
- Remove assertions for `"Passing"` section (now suppressed)
- Replace `"REPORT:"` check with triage line format
- Replace `"SKIP "` assertions — skipped tests are now dropped
- Update `"ERROR:"` to `"✗ "` for error patterns

For each test function, run individually after updating:
```bash
go test ./cmd/fo/ -run TestJTBD_RenderSARIFLintResults -v
```

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

### Task 6: Clean up and verify

**Files:**
- Modify: `pkg/render/llm.go` (remove any remaining dead code)

- [ ] **Step 1: Remove dead functions**

Check for and remove these if they still exist:
- `sarifScope` (replaced by `triageCounts`)
- `llmLevelPriority` (replaced by `severityPriority`)

- [ ] **Step 2: Run full QA**

Run: `make qa`
Expected: Build + test + vet + lint all pass

- [ ] **Step 3: Build binary and verify output manually**

```bash
go build -o fo ./cmd/fo
cat cmd/fo/testdata/demo/sarif-mixed.json | ./fo --format llm
cat cmd/fo/testdata/demo/gotest-mixed.json | ./fo --format llm
cat cmd/fo/testdata/demo/gotest-clean.json | ./fo --format llm
cat cmd/fo/testdata/demo/report-full.report | ./fo --format llm
```

Verify each output matches the spec. Specifically check:
- Version preamble `fo:llm:v1` on first line
- Triage line with `✗`/`⚠` counts
- No `ERR`/`WARN`/`FAIL`/`PASS`/`SCOPE:` tokens in output
- Clean gotest shows one triage line only
- Report groups by tool, SARIF and test formatted differently

- [ ] **Step 4: Commit and push**

```bash
git add pkg/render/llm.go
git commit -m "refactor(render): remove dead LLM helpers"
git push
```
