# QA Report Pipeline — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Pipe `make all` output from trixi (and any Go project) through fo, rendering unified QA results as Claude-optimized (default/piped) or human-friendly (TTY) output.

**Architecture:** A shell script runs each QA tool with structured output (SARIF, go test -json), emits results in a delimited multi-section format, and pipes to fo. fo detects the multi-section format, dispatches each section to existing parsers, aggregates patterns, and renders a unified report. No new dependencies.

**Tech Stack:** Go (fo), Bash (qa-report script), existing fo parsers (SARIF, testjson, wrap)

---

## Context

### Problem

`make all` runs 8 tools sequentially (vet, lint, test, eval, vuln, arch, dupl, install), each producing different text output. fo currently accepts exactly 2 formats: SARIF and go test -json. There's no way to pipe mixed tool output through fo for a unified report.

### Tool Output Capabilities

| Tool | Native structured output | Strategy |
|------|-------------------------|----------|
| go vet | text only | `fo wrap sarif --tool govet` |
| golangci-lint | `--output.sarif.path=stdout` | SARIF direct |
| go test | `-json` flag | testjson direct |
| go test (eval) | `-json` flag (custom metrics in output) | testjson as separate section |
| govulncheck | `-format sarif` | SARIF direct |
| go-arch-lint | `--json` flag | new `archlint` section format |
| jscpd | `--reporters json` | new `jscpd` section format |
| snipe index | no meaningful output | skip |
| go install | no meaningful output | skip |

### Output Modes (already handled by fo)

- **Piped (default):** LLM mode — terse, deterministic, zero ANSI, Claude-optimized
- **TTY:** Terminal mode — lipgloss colors, Unicode icons, human-scannable
- **Explicit:** `--format json` for automation

### Desired LLM Output (clean run)

```
REPORT: 7 tools — all pass

vet: 0 diags
lint: 0 diags
test: PASS — 60 tests, 12 packages
eval: PASS — MRR=0.983 P@5=0.227 NDCG5=0.961 (86 queries, no regressions)
vuln: 0 diags
arch: pass (3 checks, 0 violations)
dupl: pass (0 clones)
```

### Desired LLM Output (failing run)

```
REPORT: 7 tools — 3 fail, 4 pass

vet: 0 diags

lint: 3 err, 2 warn

## internal/store/store.go
  ERR revive:42:5 exported function Foo should have comment
  ERR unused:88:2 field Bar is unused

## pkg/search/hybrid.go
  WARN goconst:15:3 string "hello" has 3 occurrences

test: FAIL — 2 failed, 58 passed

  FAIL TestParser/malformed_input (0.02s)
    expected nil, got error: unexpected token
  FAIL TestStream/cancel (0.15s)
    context deadline exceeded

eval: FAIL — MRR=0.970 P@5=0.210 NDCG5=0.940 (86 queries)
  regression: entity MRR 0.938→0.875 (-0.063)
  regression: entity NDCG5 0.954→0.900 (-0.054)

vuln: 0 diags

arch: FAIL — 1 violation
  store → eval (forbidden by .go-arch-lint.yml)

dupl: 2 clones
  .go-arch-lint-target.yml:4-102 ↔ .go-arch-lint.yml:3-101 (98 lines)
  docs/progress.md:415-435 ↔ docs/progress.md:290-310 (20 lines)
```

---

## Delimited Report Format

New input format for fo. Each section starts with a delimiter line:

```
--- tool:<name> format:<format> [status:<pass|fail>] ---
<tool output verbatim>
```

Rules:
- Delimiter regex: `^--- tool:(\w[\w-]*) format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`
- Content between delimiters is the raw tool output, untouched
- Empty sections (delimiter followed by next delimiter) are valid (0 output = pass)

### Section Formats

| Format | Content | Pass/fail derived from |
|--------|---------|----------------------|
| `sarif` | Complete SARIF 2.1.0 JSON document | Parsed results (errors = fail) |
| `testjson` | go test -json NDJSON lines | Parsed test results |
| `text` | Raw text | `status` field in delimiter (required) |
| `metrics` | Metrics JSON (see below) | Regressions array |
| `archlint` | go-arch-lint `--json` output | `Payload.ArchHasWarnings` |
| `jscpd` | jscpd JSON report | `duplicates` array length |

### Metrics JSON Schema (generic, for benchmarks/eval)

```json
{
  "scope": "86 queries · 51 nugs · hybrid",
  "columns": ["MRR", "P@5", "P@10", "NDCG5"],
  "rows": [
    {"name": "Overall", "values": [0.983, 0.227, 0.119, 0.961], "n": 86},
    {"name": "entity", "values": [0.938, 0.200, 0.100, 0.954], "n": 8}
  ],
  "regressions": [
    {"group": "entity", "metric": "MRR", "from": 0.938, "to": 0.875}
  ]
}
```

Trixi's eval test outputs this format. The schema is generic — any project can use it for benchmark metrics.

> **Note:** The `deltas` field was removed from the schema. Regressions carry `from`/`to` values which are sufficient — a separate deltas array duplicates information and no rendering code consumes it.

### go-arch-lint JSON Schema (subset fo parses)

```json
{
  "Type": "models.Check",
  "Payload": {
    "ArchHasWarnings": false,
    "ArchWarningsDeps": [
      {"ComponentA": {"Name": "store"}, "ComponentB": {"Name": "eval"}, "FileA": "...", "FileB": "..."}
    ],
    "Qualities": [
      {"ID": "component_imports", "Used": true}
    ]
  }
}
```

> **Note:** The `ComponentA`/`ComponentB` fields are objects with a `Name` key, not bare strings. Tests and parser must reflect the actual go-arch-lint output structure.

### jscpd JSON Schema (subset fo parses)

```json
{
  "duplicates": [
    {
      "format": "go",
      "lines": 20,
      "firstFile": {"name": "a.go", "start": 10, "end": 30},
      "secondFile": {"name": "b.go", "start": 5, "end": 25}
    }
  ]
}
```

### Example Report

```
--- tool:vet format:sarif ---
{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}
--- tool:lint format:sarif ---
{"version":"2.1.0","$schema":"...","runs":[...]}
--- tool:test format:testjson ---
{"Time":"...","Action":"start","Package":"..."}
{"Time":"...","Action":"pass","Package":"...","Elapsed":0.5}
--- tool:eval format:metrics ---
{"scope":"86 queries · 51 nugs","columns":["MRR","P@5","P@10","NDCG5"],"rows":[{"name":"Overall","values":[0.983,0.227,0.119,0.961],"n":86}],"regressions":[]}
--- tool:vuln format:sarif ---
{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govulncheck"}},"results":[]}]}
--- tool:arch format:archlint ---
{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"Qualities":[{"ID":"component_imports","Used":true}]}}
--- tool:dupl format:jscpd ---
{"duplicates":[],"statistics":{}}
```

---

## Task Order

> **Dependency-correct execution order.** Parsers (Tasks 3–5) must exist before the mapper (Task 6) can compile. CLI wiring (Task 7) requires both. Integration tests (Task 9) exercise the full pipeline. Trixi-side changes (Tasks 10–12) come last since they depend on fo being functional.

| Phase | Tasks | What |
|-------|-------|------|
| **Foundation** | 1, 2, 3, 4, 5 | detect, section parser, metrics/archlint/jscpd parsers |
| **Assembly** | 6, 7, 8 | mapper, CLI wiring, LLM renderer |
| **Verification** | 9 | integration tests + terminal verification + JSON golden test |
| **Trixi** | 10, 11, 12 | shell script, Makefile, eval metrics flag |

---

## Tasks

### Task 1: Report Format Detection

**Files:**
- Modify: `internal/detect/detect.go`
- Modify: `internal/detect/detect_test.go`

**Step 1: Write the failing test**

```go
func TestSniff_Report(t *testing.T) {
	input := []byte("--- tool:vet format:sarif ---\n{\"version\":\"2.1.0\"}")
	got := detect.Sniff(input)
	if got != detect.Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportWithStatus(t *testing.T) {
	input := []byte("--- tool:arch format:text status:pass ---\nAll checks passed.")
	got := detect.Sniff(input)
	if got != detect.Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportMetricsFormat(t *testing.T) {
	input := []byte("--- tool:eval format:metrics ---\n{\"scope\":\"86 queries\"}")
	got := detect.Sniff(input)
	if got != detect.Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportArchlintFormat(t *testing.T) {
	input := []byte("--- tool:arch format:archlint ---\n{}")
	got := detect.Sniff(input)
	if got != detect.Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}

func TestSniff_ReportJscpdFormat(t *testing.T) {
	input := []byte("--- tool:dupl format:jscpd ---\n{}")
	got := detect.Sniff(input)
	if got != detect.Report {
		t.Errorf("Sniff() = %v, want Report", got)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/detect/ -run TestSniff_Report -v`
Expected: FAIL — `detect.Report` undefined

**Step 3: Add Report format constant**

In `detect.go`, add `Report` to the Format enum.

**Step 4: Implement detection**

> **CRITICAL:** The current `Sniff()` has an early-return guard: `if data[0] != '{' { return Unknown }`. Report delimiters start with `---`, not `{`. The report check MUST be inserted before this guard.

In `Sniff()`, after the leading-whitespace trim and the `len(data) == 0` check, but **before** the `data[0] != '{'` guard:

```go
// Check for report delimiter before requiring '{' — reports start with '---'
if firstLine := extractFirstLine(data); reportDelimiterRe.Match(firstLine) {
	return Report
}

// Must start with '{' for SARIF or go test -json
if data[0] != '{' {
	return Unknown
}
```

Use `regexp.MustCompile` for the delimiter pattern — must include **all six** section formats:
```go
var reportDelimiterRe = regexp.MustCompile(
	`^--- tool:\w[\w-]* format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`,
)
```

Add `extractFirstLine` helper:
```go
func extractFirstLine(data []byte) []byte {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return data[:i]
		}
	}
	return data
}
```

> **Why not `firstLine` (existing)?** There is no existing `firstLine` function in detect.go. The plan previously referenced one that doesn't exist. We define `extractFirstLine` explicitly.

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/detect/ -v`
Expected: all PASS (including existing tests — the `{` guard still protects SARIF/GoTestJSON paths)

**Step 6: Commit**

```
feat(detect): add Report format detection for multi-tool input
```

---

### Task 2: Section Parser

**Files:**
- Create: `pkg/report/report.go`
- Create: `pkg/report/report_test.go`

**Step 1: Write the failing test**

```go
package report

import "testing"

func TestParse_SingleSARIF(t *testing.T) {
	input := "--- tool:lint format:sarif ---\n{\"version\":\"2.1.0\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("got %d sections, want 1", len(sections))
	}
	if sections[0].Tool != "lint" {
		t.Errorf("tool = %q, want lint", sections[0].Tool)
	}
	if sections[0].Format != "sarif" {
		t.Errorf("format = %q, want sarif", sections[0].Format)
	}
}

func TestParse_MultipleSections(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n{}\n--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n--- tool:arch format:text status:pass ---\nOK\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 3 {
		t.Fatalf("got %d sections, want 3", len(sections))
	}
	if sections[2].Status != "pass" {
		t.Errorf("status = %q, want pass", sections[2].Status)
	}
}

func TestParse_EmptySection(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n--- tool:lint format:sarif ---\n{}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 2 {
		t.Fatalf("got %d sections, want 2", len(sections))
	}
	if len(sections[0].Content) != 0 {
		t.Errorf("section 0 content should be empty, got %d bytes", len(sections[0].Content))
	}
}

func TestParse_PreservesTrailingNewlineInContent(t *testing.T) {
	// NDJSON expects trailing newline — parser must not strip all of them
	input := "--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n"
	sections, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	// Content should be the raw line(s) with exactly one trailing newline trimmed
	// (the one added by bytes.Split), not aggressively stripped
	content := string(sections[0].Content)
	if content != "{\"Action\":\"pass\"}" {
		t.Errorf("content = %q", content)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/report/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Implement the parser**

```go
package report

import (
	"bytes"
	"fmt"
	"regexp"
)

var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`,
)

// Section represents one tool's output within a report.
type Section struct {
	Tool    string // e.g. "lint", "test", "vuln"
	Format  string // "sarif", "testjson", "text", "metrics", "archlint", "jscpd"
	Status  string // "pass" or "fail" (required for text, derived for others)
	Content []byte // raw tool output
}

// Parse splits delimited report input into sections.
func Parse(data []byte) ([]Section, error) {
	lines := bytes.Split(data, []byte("\n"))
	var sections []Section
	var current *Section

	for _, line := range lines {
		if m := delimiterRe.FindSubmatch(line); m != nil {
			if current != nil {
				current.Content = trimTrailingNewline(current.Content)
				sections = append(sections, *current)
			}
			current = &Section{
				Tool:   string(m[1]),
				Format: string(m[2]),
				Status: string(m[3]), // empty string if not present
			}
			continue
		}
		if current != nil {
			current.Content = append(current.Content, line...)
			current.Content = append(current.Content, '\n')
		}
	}
	if current != nil {
		current.Content = trimTrailingNewline(current.Content)
		sections = append(sections, *current)
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no sections found in report input")
	}
	return sections, nil
}

// trimTrailingNewline removes exactly one trailing newline byte, if present.
// Unlike bytes.TrimRight("\n"), this preserves intentional multiple newlines
// in tool output (e.g., NDJSON streams).
func trimTrailingNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}
```

> **Fix applied:** Replaced `bytes.TrimRight(current.Content, "\n")` with `trimTrailingNewline` that removes exactly one trailing `\n`. The aggressive trim would strip all trailing newlines, which could corrupt multi-line tool output.

> **Fix applied:** Removed `bytes.TrimSpace(line)` before regex matching. Delimiter lines must be exact — no leading/trailing whitespace tolerance. This prevents accidental matches on tool output that happens to contain delimiter-like text with leading spaces.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/report/ -v`
Expected: all PASS

**Step 5: Commit**

```
feat(report): add delimited multi-section parser
```

---

### Task 3: Metrics Section Parser (fo)

**Files:**
- Create: `internal/metrics/metrics.go`
- Create: `internal/metrics/metrics_test.go`

> **Placement:** `internal/` not `pkg/`. These parsers are fo-internal concerns tightly coupled to the report feature — not reusable library-quality packages for external consumers. The `pkg/` convention in this codebase is reserved for the two core parsers (sarif, testjson) that have independent utility.

**Step 1: Write the failing test**

```go
package metrics

import "testing"

func TestParse(t *testing.T) {
	input := []byte(`{
		"scope": "86 queries · 51 nugs",
		"columns": ["MRR", "P@5", "P@10", "NDCG5"],
		"rows": [
			{"name": "Overall", "values": [0.983, 0.227, 0.119, 0.961], "n": 86},
			{"name": "entity", "values": [0.938, 0.200, 0.100, 0.954], "n": 8}
		],
		"regressions": []
	}`)
	report, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scope != "86 queries · 51 nugs" {
		t.Errorf("scope = %q", report.Scope)
	}
	if len(report.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(report.Rows))
	}
	if len(report.Columns) != 4 {
		t.Errorf("got %d columns, want 4", len(report.Columns))
	}
}

func TestParse_WithRegressions(t *testing.T) {
	input := []byte(`{
		"scope": "86 queries",
		"columns": ["MRR"],
		"rows": [{"name": "Overall", "values": [0.900], "n": 86}],
		"regressions": [{"group": "entity", "metric": "MRR", "from": 0.938, "to": 0.875}]
	}`)
	report, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Regressions) != 1 {
		t.Fatalf("got %d regressions, want 1", len(report.Regressions))
	}
	if report.Regressions[0].Metric != "MRR" {
		t.Errorf("regression metric = %q, want MRR", report.Regressions[0].Metric)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
```

**Step 2: Implement**

```go
package metrics

import "encoding/json"

// Report represents a generic metrics report (eval, benchmarks, etc.).
type Report struct {
	Scope       string       `json:"scope"`
	Columns     []string     `json:"columns"`
	Rows        []Row        `json:"rows"`
	Regressions []Regression `json:"regressions"`
}

type Row struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
	N      int       `json:"n,omitempty"`
}

type Regression struct {
	Group  string  `json:"group"`
	Metric string  `json:"metric"`
	From   float64 `json:"from"`
	To     float64 `json:"to"`
}

// Parse decodes metrics JSON into a Report.
func Parse(data []byte) (*Report, error) {
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/metrics/ -v`

**Step 4: Commit**

```
feat(metrics): add generic metrics JSON parser for eval/benchmark sections
```

---

### Task 4: go-arch-lint JSON Parser (fo)

**Files:**
- Create: `internal/archlint/archlint.go`
- Create: `internal/archlint/archlint_test.go`

**Step 1: Write the failing test**

```go
package archlint

import "testing"

func TestParse_Clean(t *testing.T) {
	input := []byte(`{
		"Type": "models.Check",
		"Payload": {
			"ArchHasWarnings": false,
			"ArchWarningsDeps": [],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"Qualities": [
				{"ID": "component_imports", "Used": true},
				{"ID": "deepscan", "Used": true}
			]
		}
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasWarnings {
		t.Error("expected no warnings")
	}
	if len(result.Violations) != 0 {
		t.Errorf("got %d violations, want 0", len(result.Violations))
	}
	if len(result.Checks) != 2 {
		t.Errorf("got %d checks, want 2", len(result.Checks))
	}
}

func TestParse_WithViolation(t *testing.T) {
	input := []byte(`{
		"Type": "models.Check",
		"Payload": {
			"ArchHasWarnings": true,
			"ArchWarningsDeps": [
				{
					"ComponentA": {"Name": "store"},
					"ComponentB": {"Name": "eval"},
					"FileA": "internal/store/store.go",
					"FileB": "internal/eval/eval.go"
				}
			],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"Qualities": [{"ID": "component_imports", "Used": true}]
		}
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasWarnings {
		t.Error("expected warnings")
	}
	if len(result.Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(result.Violations))
	}
	if result.Violations[0].From != "store" || result.Violations[0].To != "eval" {
		t.Errorf("violation = %s → %s", result.Violations[0].From, result.Violations[0].To)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
```

**Step 2: Implement**

```go
package archlint

import "encoding/json"

// Result represents parsed go-arch-lint check output.
type Result struct {
	HasWarnings bool
	Violations  []Violation
	Checks      []Check
}

type Violation struct {
	From     string // component name
	To       string // component name
	FileFrom string
	FileTo   string
}

type Check struct {
	ID   string
	Used bool
}

// Parse decodes go-arch-lint --json output into a Result.
func Parse(data []byte) (*Result, error) {
	var raw struct {
		Payload struct {
			ArchHasWarnings  bool `json:"ArchHasWarnings"`
			ArchWarningsDeps []struct {
				ComponentA struct{ Name string } `json:"ComponentA"`
				ComponentB struct{ Name string } `json:"ComponentB"`
				FileA      string                `json:"FileA"`
				FileB      string                `json:"FileB"`
			} `json:"ArchWarningsDeps"`
			Qualities []struct {
				ID   string `json:"ID"`
				Used bool   `json:"Used"`
			} `json:"Qualities"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	r := &Result{HasWarnings: raw.Payload.ArchHasWarnings}
	for _, d := range raw.Payload.ArchWarningsDeps {
		r.Violations = append(r.Violations, Violation{
			From: d.ComponentA.Name, To: d.ComponentB.Name,
			FileFrom: d.FileA, FileTo: d.FileB,
		})
	}
	for _, q := range raw.Payload.Qualities {
		r.Checks = append(r.Checks, Check{ID: q.ID, Used: q.Used})
	}
	return r, nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/archlint/ -v`

**Step 4: Commit**

```
feat(archlint): add go-arch-lint JSON parser
```

---

### Task 5: jscpd JSON Parser (fo)

**Files:**
- Create: `internal/jscpd/jscpd.go`
- Create: `internal/jscpd/jscpd_test.go`

**Step 1: Write the failing test**

```go
package jscpd

import "testing"

func TestParse_NoClones(t *testing.T) {
	input := []byte(`{"duplicates": [], "statistics": {}}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Clones) != 0 {
		t.Errorf("got %d clones, want 0", len(result.Clones))
	}
}

func TestParse_WithClones(t *testing.T) {
	input := []byte(`{
		"duplicates": [
			{
				"format": "go",
				"lines": 20,
				"firstFile": {"name": "a.go", "start": 10, "end": 30,
					"startLoc": {"line": 10, "column": 1},
					"endLoc": {"line": 30, "column": 5}},
				"secondFile": {"name": "b.go", "start": 5, "end": 25,
					"startLoc": {"line": 5, "column": 1},
					"endLoc": {"line": 25, "column": 5}}
			}
		]
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Clones) != 1 {
		t.Fatalf("got %d clones, want 1", len(result.Clones))
	}
	c := result.Clones[0]
	if c.FileA != "a.go" || c.FileB != "b.go" {
		t.Errorf("files = %s, %s", c.FileA, c.FileB)
	}
	if c.Lines != 20 {
		t.Errorf("lines = %d, want 20", c.Lines)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
```

**Step 2: Implement**

```go
package jscpd

import "encoding/json"

// Result represents parsed jscpd duplicate detection output.
type Result struct {
	Clones []Clone
}

type Clone struct {
	Format string
	Lines  int
	FileA  string
	StartA int
	EndA   int
	FileB  string
	StartB int
	EndB   int
}

// Parse decodes jscpd JSON report into a Result.
func Parse(data []byte) (*Result, error) {
	var raw struct {
		Duplicates []struct {
			Format     string `json:"format"`
			Lines      int    `json:"lines"`
			FirstFile  struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"firstFile"`
			SecondFile struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"secondFile"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	r := &Result{}
	for _, d := range raw.Duplicates {
		r.Clones = append(r.Clones, Clone{
			Format: d.Format, Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line, EndA: d.FirstFile.EndLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return r, nil
}
```

**Step 3: Run tests**

Run: `go test ./internal/jscpd/ -v`

**Step 4: Commit**

```
feat(jscpd): add jscpd JSON report parser
```

---

### Task 6: Report Mapper

**Files:**
- Create: `pkg/mapper/report.go`
- Create: `pkg/mapper/report_test.go`

> **Prerequisite:** Tasks 3, 4, 5 must be complete — this task imports `internal/metrics`, `internal/archlint`, `internal/jscpd`.

**Step 1: Write the failing test**

```go
package mapper

import (
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/report"
)

func TestFromReport_TextPassSection(t *testing.T) {
	sections := []report.Section{
		{Tool: "vuln", Format: "text", Status: "pass", Content: []byte("No vulnerabilities.")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) < 1 {
		t.Fatal("expected at least 1 pattern")
	}
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Metrics[0].Kind != "success" {
		t.Errorf("expected success kind, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_SARIFSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","$schema":"https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json","runs":[{"tool":{"driver":{"name":"govet","rules":[]}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected patterns")
	}
}

func TestFromReport_MalformedSectionReportsError(t *testing.T) {
	sections := []report.Section{
		{Tool: "lint", Format: "sarif", Content: []byte("not valid json{{{")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal("FromReport should not return top-level error for section failures")
	}
	// The top-level summary should mark the section as failed
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Metrics[0].Kind != "error" {
		t.Errorf("malformed section should be marked error, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MultiSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "arch", Format: "text", Status: "pass", Content: []byte("OK")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	sum := patterns[0].(*pattern.Summary)
	if len(sum.Metrics) != 2 {
		t.Errorf("expected 2 tool metrics, got %d", len(sum.Metrics))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/mapper/ -run TestFromReport -v`
Expected: FAIL — `FromReport` undefined

**Step 3: Implement the mapper**

```go
package mapper

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/internal/archlint"
	"github.com/dkoosis/fo/internal/jscpd"
	"github.com/dkoosis/fo/internal/metrics"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
)

// FromReport converts multi-section report data into patterns.
// Individual section parse failures are reported as error patterns, not
// as a top-level error — a malformed lint section shouldn't hide passing tests.
func FromReport(sections []report.Section) ([]pattern.Pattern, error) {
	var allPatterns []pattern.Pattern
	var toolSummaries []pattern.SummaryItem
	pass, fail := 0, 0

	for _, sec := range sections {
		sectionPatterns, sectionPass, scopeLabel := mapSection(sec)

		if sectionPass {
			pass++
		} else {
			fail++
		}

		kind := "success"
		if !sectionPass {
			kind = "error"
		}
		toolSummaries = append(toolSummaries, pattern.SummaryItem{
			Label: sec.Tool,
			Value: scopeLabel,
			Kind:  kind,
		})

		allPatterns = append(allPatterns, sectionPatterns...)
	}

	// Build top-level summary
	label := fmt.Sprintf("REPORT: %d tools", len(sections))
	if fail == 0 {
		label += " — all pass"
	} else {
		parts := []string{}
		if fail > 0 {
			parts = append(parts, fmt.Sprintf("%d fail", fail))
		}
		if pass > 0 {
			parts = append(parts, fmt.Sprintf("%d pass", pass))
		}
		label += " — " + strings.Join(parts, ", ")
	}

	topSummary := &pattern.Summary{
		Label:   label,
		Metrics: toolSummaries,
	}

	return append([]pattern.Pattern{topSummary}, allPatterns...), nil
}

// mapSection dispatches to format-specific mappers.
// Returns (patterns, passed, scopeLabel).
// On parse error: returns an empty pattern list, passed=false, and a
// descriptive scopeLabel indicating the failure.
func mapSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	switch sec.Format {
	case "sarif":
		return mapSARIFSection(sec)
	case "testjson":
		return mapTestJSONSection(sec)
	case "metrics":
		return mapMetricsSection(sec)
	case "archlint":
		return mapArchLintSection(sec)
	case "jscpd":
		return mapJSCPDSection(sec)
	case "text":
		return mapTextSection(sec)
	default:
		return nil, false, fmt.Sprintf("unknown format %q", sec.Format)
	}
}

// mapSARIFSection parses SARIF content and converts to patterns.
func mapSARIFSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	doc, err := sarif.ReadBytes(sec.Content)
	if err != nil {
		return nil, false, fmt.Sprintf("parse error: %v", err)
	}
	stats := sarif.ComputeStats(doc)
	patterns := FromSARIF(doc)

	passed := stats.ByLevel["error"] == 0
	label := fmt.Sprintf("%d diags", stats.TotalIssues)
	if stats.TotalIssues > 0 {
		var parts []string
		if n := stats.ByLevel["error"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d err", n))
		}
		if n := stats.ByLevel["warning"]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d warn", n))
		}
		label = strings.Join(parts, ", ")
	}
	return patterns, passed, label
}

// mapTestJSONSection parses go test -json content and converts to patterns.
func mapTestJSONSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	results, err := testjson.ParseBytes(sec.Content)
	if err != nil {
		return nil, false, fmt.Sprintf("parse error: %v", err)
	}
	stats := testjson.ComputeStats(results)
	patterns := FromTestJSON(results)

	passed := stats.Failed == 0 && stats.BuildErrors == 0 && stats.Panics == 0
	if passed {
		label := fmt.Sprintf("PASS — %d tests, %d packages", stats.TotalTests, stats.Packages)
		return patterns, true, label
	}
	label := fmt.Sprintf("FAIL — %d failed, %d passed", stats.Failed, stats.Passed)
	return patterns, false, label
}

// mapMetricsSection parses metrics JSON and converts to patterns.
func mapMetricsSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	m, err := metrics.Parse(sec.Content)
	if err != nil {
		return nil, false, fmt.Sprintf("parse error: %v", err)
	}

	passed := len(m.Regressions) == 0

	// Build scope label from first row (Overall) values
	var label string
	if len(m.Rows) > 0 && len(m.Columns) > 0 {
		row := m.Rows[0]
		var parts []string
		for i, col := range m.Columns {
			if i < len(row.Values) {
				parts = append(parts, fmt.Sprintf("%s=%.3f", col, row.Values[i]))
			}
		}
		prefix := "PASS"
		if !passed {
			prefix = "FAIL"
		}
		label = fmt.Sprintf("%s — %s (%s", prefix, strings.Join(parts, " "), m.Scope)
		if passed {
			label += ", no regressions)"
		} else {
			label += ")"
		}
	}

	var patterns []pattern.Pattern
	// Add regression details as TestTable rows
	if len(m.Regressions) > 0 {
		var items []pattern.TestTableItem
		for _, r := range m.Regressions {
			items = append(items, pattern.TestTableItem{
				Name:    fmt.Sprintf("regression: %s %s", r.Group, r.Metric),
				Status:  "fail",
				Details: fmt.Sprintf("%.3f→%.3f (%.3f)", r.From, r.To, r.To-r.From),
			})
		}
		patterns = append(patterns, &pattern.TestTable{
			Label:   sec.Tool + " regressions",
			Results: items,
		})
	}

	return patterns, passed, label
}

// mapArchLintSection parses go-arch-lint JSON and converts to patterns.
func mapArchLintSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	result, err := archlint.Parse(sec.Content)
	if err != nil {
		return nil, false, fmt.Sprintf("parse error: %v", err)
	}

	passed := !result.HasWarnings
	if passed {
		label := fmt.Sprintf("pass (%d checks, 0 violations)", len(result.Checks))
		return nil, true, label
	}

	var items []pattern.TestTableItem
	for _, v := range result.Violations {
		items = append(items, pattern.TestTableItem{
			Name:   fmt.Sprintf("%s → %s", v.From, v.To),
			Status: "fail",
		})
	}
	patterns := []pattern.Pattern{
		&pattern.TestTable{
			Label:   sec.Tool + " violations",
			Results: items,
		},
	}
	label := fmt.Sprintf("FAIL — %d violation", len(result.Violations))
	if len(result.Violations) != 1 {
		label += "s"
	}
	return patterns, false, label
}

// mapJSCPDSection parses jscpd JSON and converts to patterns.
func mapJSCPDSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	result, err := jscpd.Parse(sec.Content)
	if err != nil {
		return nil, false, fmt.Sprintf("parse error: %v", err)
	}

	if len(result.Clones) == 0 {
		return nil, true, "pass (0 clones)"
	}

	var items []pattern.TestTableItem
	for _, c := range result.Clones {
		items = append(items, pattern.TestTableItem{
			Name:    fmt.Sprintf("%s:%d-%d ↔ %s:%d-%d", c.FileA, c.StartA, c.EndA, c.FileB, c.StartB, c.EndB),
			Status:  "skip", // advisory — clones are warnings, not errors
			Details: fmt.Sprintf("%d lines (%s)", c.Lines, c.Format),
		})
	}
	patterns := []pattern.Pattern{
		&pattern.TestTable{
			Label:   sec.Tool + " clones",
			Results: items,
		},
	}
	label := fmt.Sprintf("%d clones", len(result.Clones))
	return patterns, true, label // clones don't fail the report by default
}

// mapTextSection handles text sections with explicit pass/fail status.
func mapTextSection(sec report.Section) ([]pattern.Pattern, bool, string) {
	passed := sec.Status != "fail"
	label := sec.Status
	if len(sec.Content) > 0 {
		// Use first line of content as additional context
		firstLine := string(sec.Content)
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		if len(firstLine) > 60 {
			firstLine = firstLine[:60] + "…"
		}
		label = sec.Status + " — " + firstLine
	}
	return nil, passed, label
}
```

> **Fixes applied:**
> 1. All six helper functions are now fully implemented (previously hand-waved as "delegate to existing parsers").
> 2. Parse errors in individual sections produce `passed=false` with descriptive scopeLabel instead of silently treating them as passing.
> 3. `FromReport` returns `([]pattern.Pattern, error)` for consistency with Go conventions, though the error is currently always nil (section errors are folded into patterns).
> 4. `sectionScopeLabel` is eliminated — each helper returns its own scope label as the third return value.
> 5. jscpd clones default to `passed=true` (advisory only). The mapper uses status "skip" for individual clone items so they render as warnings but don't trigger exit code 1.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mapper/ -run TestFromReport -v`
Expected: all PASS

**Step 5: Commit**

```
feat(mapper): add FromReport for multi-section report input
```

---

### Task 7: CLI Wiring

**Files:**
- Modify: `cmd/fo/main.go`

**Step 1: Write the failing test**

Add to `cmd/fo/main_test.go`:

```go
func TestRun_ReportFormat(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n" +
		`{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}` + "\n" +
		"--- tool:arch format:text status:pass ---\nAll checks passed.\n"
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "REPORT:") {
		t.Errorf("output should contain REPORT header, got:\n%s", stdout.String())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/fo/ -run TestRun_ReportFormat -v`
Expected: FAIL — `unrecognized input format`

**Step 3: Add Report case to parseInput**

```go
case detect.Report:
	sections, err := report.Parse(input)
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing report: %v\n", err)
		return nil, 2
	}
	patterns, mapErr := mapper.FromReport(sections)
	if mapErr != nil {
		fmt.Fprintf(stderr, "fo: mapping report: %v\n", mapErr)
		return nil, 2
	}
	return patterns, -1
```

Add import for `"github.com/dkoosis/fo/pkg/report"`.

**Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fo/ -v`
Expected: all PASS

**Step 5: Commit**

```
feat(cli): wire up Report format in parseInput
```

---

### Task 8: LLM Renderer — Report-Aware Rendering

**Files:**
- Modify: `pkg/render/llm.go`
- Create: `pkg/render/llm_test.go` (if not exists)

**Step 1: Write the failing test**

```go
func TestLLM_RenderReport(t *testing.T) {
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — all pass",
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: "success"},
				{Label: "arch", Value: "pass", Kind: "success"},
			},
		},
	}
	r := render.NewLLM()
	out := r.Render(patterns)
	if !strings.Contains(out, "REPORT:") {
		t.Errorf("expected REPORT header in output:\n%s", out)
	}
	if !strings.Contains(out, "vet: 0 diags") {
		t.Errorf("expected tool summary line in output:\n%s", out)
	}
}
```

**Step 2: Run test to verify behavior**

Run: `go test ./pkg/render/ -run TestLLM_RenderReport -v`

The existing code paths detect "test vs SARIF" based on Summary labels. A report Summary (label starts with "REPORT:") falls through to SARIF path, which ignores summaries. Need a third code path.

**Step 3: Add report rendering path**

> **Design note:** The existing renderer dispatches on string prefix matching (`"PASS"`/`"FAIL"` → test, else → SARIF). Adding a third `"REPORT:"` check compounds this fragile pattern. The right long-term fix is to add a `Kind` field to `pattern.Summary` (values: `"sarif"`, `"test"`, `"report"`). However, that's a refactor touching existing code. For now, we match the existing convention and add a `// TODO: replace string-prefix dispatch with Summary.Kind field` comment.

In `Render()`, detect report by checking for "REPORT:" prefix in any summary label:

```go
// TODO: replace string-prefix dispatch with Summary.Kind field
isReport := false
for _, s := range summaries {
	if strings.HasPrefix(s.Label, "REPORT:") {
		isReport = true
		break
	}
}
if isReport {
	return l.renderReport(summaries, tables)
}
```

Implement `renderReport` that:
1. Prints the REPORT summary label as first line
2. Blank line
3. Prints each tool metric as `<tool>: <value>` on its own line
4. For sections with issues (associated TestTable items), renders them after the tool summary line with a blank line separator
5. Blank line between sections that have detail

```go
func (l *LLM) renderReport(summaries []*pattern.Summary, tables []*pattern.TestTable) string {
	var sb strings.Builder

	// Find the report summary (first one with REPORT: prefix)
	var reportSummary *pattern.Summary
	for _, s := range summaries {
		if strings.HasPrefix(s.Label, "REPORT:") {
			reportSummary = s
			break
		}
	}
	if reportSummary == nil {
		return ""
	}

	sb.WriteString(reportSummary.Label + "\n")

	// Build a map of tables by label prefix for associating with tools
	tablesByTool := make(map[string][]*pattern.TestTable)
	for _, t := range tables {
		// Table labels are prefixed with tool name (e.g., "lint violations")
		for _, m := range reportSummary.Metrics {
			if strings.HasPrefix(t.Label, m.Label) {
				tablesByTool[m.Label] = append(tablesByTool[m.Label], t)
			}
		}
	}

	for _, m := range reportSummary.Metrics {
		sb.WriteString("\n" + m.Label + ": " + m.Value + "\n")

		// Render associated tables
		for _, t := range tablesByTool[m.Label] {
			sb.WriteString("\n")
			for _, item := range t.Results {
				prefix := "  "
				switch item.Status {
				case "fail":
					prefix = "  FAIL "
				case "skip":
					prefix = "  "
				}
				sb.WriteString(prefix + item.Name)
				if item.Duration != "" {
					sb.WriteString(" (" + item.Duration + ")")
				}
				sb.WriteString("\n")
				if item.Details != "" {
					lines := strings.Split(item.Details, "\n")
					max := 3
					if len(lines) < max {
						max = len(lines)
					}
					for _, line := range lines[:max] {
						sb.WriteString("    " + line + "\n")
					}
					if len(lines) > 3 {
						sb.WriteString(fmt.Sprintf("    ... (%d more lines)\n", len(lines)-3))
					}
				}
			}
		}
	}

	return sb.String()
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/render/ -v`
Expected: all PASS

**Step 5: Commit**

```
feat(render): add report-aware LLM rendering path
```

---

### Task 9: Integration Tests + Verification

**Files:**
- Create: `cmd/fo/testdata/clean.report`
- Create: `cmd/fo/testdata/failing.report`
- Modify: `cmd/fo/main_test.go`
- Modify: `pkg/render/terminal.go` (if needed)

> **Fix applied:** Test fixtures live in `cmd/fo/testdata/` (not `pkg/report/testdata/`). Go's test framework sets the working directory to the package under test, so `testdata/` within `cmd/fo/` is always reachable as a relative path. The previous plan used `../../pkg/report/testdata/` which breaks under `go test ./...` from the repo root.

**Step 1: Create test fixture — clean report**

A minimal report with 3 sections, all passing. Include real SARIF (empty results), real testjson (one passing test), and text (pass).

**Step 2: Create test fixture — failing report**

Same structure but with lint errors in SARIF and a failing test in testjson.

**Step 3: Write integration tests**

```go
func TestRun_ReportClean(t *testing.T) {
	input, err := os.ReadFile("testdata/clean.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("clean report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "all pass") {
		t.Errorf("expected 'all pass' in output:\n%s", out)
	}
}

func TestRun_ReportFailing(t *testing.T) {
	input, err := os.ReadFile("testdata/failing.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("failing report exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
}

func TestRun_ReportJSON(t *testing.T) {
	input, err := os.ReadFile("testdata/clean.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("JSON report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	// Verify JSON is valid and contains patterns
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Errorf("expected JSON output, got:\n%s", stdout.String())
	}
}
```

> **Fix applied:** Added `TestRun_ReportJSON` — the original plan had no verification that `--format json` works with report patterns. The JSON renderer serializes patterns automatically, but this test confirms the structure is valid end-to-end.

**Step 4: Verify terminal rendering**

```go
func TestTerminal_RenderReportPatterns(t *testing.T) {
	patterns := []pattern.Pattern{
		&pattern.Summary{
			Label: "REPORT: 2 tools — all pass",
			Metrics: []pattern.SummaryItem{
				{Label: "vet", Value: "0 diags", Kind: "success"},
				{Label: "test", Value: "PASS — 60 tests", Kind: "success"},
			},
		},
	}
	r := render.NewTerminal(render.MonoTheme(), 80)
	out := r.Render(patterns)
	if !strings.Contains(out, "REPORT:") {
		t.Errorf("expected REPORT in output:\n%s", out)
	}
}
```

No structural changes needed — the existing Terminal renderer already handles Summary and TestTable patterns generically.

**Step 5: Run tests**

Run: `go test ./cmd/fo/ -run TestRun_Report -v && go test ./pkg/render/ -run TestTerminal_RenderReport -v`
Expected: all PASS

**Step 6: Commit**

```
test: add integration tests for report format end-to-end
```

---

### Task 10: qa-report.sh for Trixi

**Files:**
- Create: `/Users/vcto/Projects/trixi/scripts/qa-report.sh`

This is a standalone shell script. No Go code.

**Step 1: Create the script**

```bash
#!/bin/bash
# qa-report.sh — Run QA tools and emit fo report format.
# Usage: ./scripts/qa-report.sh | fo
# TTY:   human-friendly colored output
# Piped: Claude-optimized terse output

set -o pipefail

_log="$(mktemp)"
trap 'rm -f "$_log"' EXIT

# Log diagnostic messages (visible in debug mode, hidden from pipeline)
log() { echo "[qa-report] $*" >> "$_log"; }

emit_sarif() {
    local name="$1"; shift
    echo "--- tool:$name format:sarif ---"
    if ! "$@" 2>>"$_log"; then
        log "$name: tool exited with code $?"
    fi
}

emit_testjson() {
    local name="$1"; shift
    echo "--- tool:$name format:testjson ---"
    if ! "$@" 2>>"$_log"; then
        log "$name: tool exited with code $?"
    fi
}

emit_json() {
    local name="$1" fmt="$2"; shift 2
    echo "--- tool:$name format:$fmt ---"
    if ! "$@" 2>>"$_log"; then
        log "$name: tool exited with code $?"
    fi
}

emit_text() {
    local name="$1"; shift
    local output
    output=$("$@" 2>&1)
    local rc=$?
    local status="pass"
    [ $rc -ne 0 ] && status="fail"
    echo "--- tool:$name format:text status:$status ---"
    [ -n "$output" ] && echo "$output"
}

# --- Tools ---

# go vet → wrap to SARIF
emit_sarif "vet" bash -c 'go vet ./... 2>&1 | fo wrap sarif --tool govet'

# golangci-lint → native SARIF
emit_sarif "lint" golangci-lint run --output.sarif.path=stdout --output.text.path=stderr ./...

# go test → native JSON (excludes eval tests)
emit_testjson "test" go test -json -race -timeout=5m -count=1 $(go list ./... | grep -v /eval)

# eval → metrics JSON (separate section for retrieval quality metrics)
echo "--- tool:eval format:metrics ---"
go test -run TestEval -count=1 -v ./internal/eval/ -fo-metrics 2>>"$_log" || true

# govulncheck → native SARIF
emit_sarif "vuln" govulncheck -format sarif ./...

# go-arch-lint → native JSON
emit_json "arch" "archlint" go-arch-lint check --json

# jscpd → JSON report (write to temp, cat, cleanup)
_jscpd_dir=$(mktemp -d)
jscpd . --reporters json --output "$_jscpd_dir" >>"$_log" 2>&1 || true
echo "--- tool:dupl format:jscpd ---"
cat "$_jscpd_dir/jscpd-report.json" 2>/dev/null || echo '{"duplicates":[]}'
rm -rf "$_jscpd_dir"

# Dump log on debug
if [ "${QA_DEBUG:-}" = "1" ]; then
    echo "--- qa-report.sh log ---" >&2
    cat "$_log" >&2
fi
```

> **Fix applied:** Replaced `2>/dev/null` with `2>>"$_log"` throughout. Tool stderr is now captured to a temp log file instead of being silently discarded. If `QA_DEBUG=1` is set, the log is dumped to stderr at the end for troubleshooting. This preserves the clean stdout pipeline while making failures diagnosable.

**Note on eval:** The `-fo-metrics` flag is a new test flag added to trixi's TestEval that outputs the generic metrics JSON format to stdout instead of the human-readable table. This requires a small change to trixi's `internal/eval/eval_test.go` (see Task 12).

**Step 2: Make executable**

```bash
chmod +x scripts/qa-report.sh
```

**Step 3: Test manually**

```bash
cd /Users/vcto/Projects/trixi
./scripts/qa-report.sh | head -5
# Should see: --- tool:vet format:sarif ---

# Debug mode:
QA_DEBUG=1 ./scripts/qa-report.sh | head -5
```

**Step 4: Commit**

```
feat: add qa-report.sh for fo-powered QA output
```

---

### Task 11: Makefile Integration (Trixi)

**Files:**
- Modify: `/Users/vcto/Projects/trixi/Makefile`

**Step 1: Add new targets**

```makefile
.PHONY: qa-fo all-fo

# QA with fo-rendered output (Claude-friendly when piped, human-friendly on TTY)
qa-fo: snipe-index
	@./scripts/qa-report.sh | fo

# Everything with fo-rendered output
all-fo: qa-fo install
	@echo "=== all pass ==="
```

**Step 2: Test**

```bash
# Human-friendly (TTY)
make qa-fo

# Claude-friendly (piped)
make qa-fo 2>&1 | cat
```

**Step 3: Commit**

```
feat: add qa-fo and all-fo make targets
```

---

### Task 12: Eval Metrics Output (trixi)

**Files:**
- Modify: `/Users/vcto/Projects/trixi/internal/eval/eval_test.go`
- Modify: `/Users/vcto/Projects/trixi/internal/eval/eval.go` (if needed)

Add a `-fo-metrics` test flag that outputs the generic metrics JSON format instead of the human-readable table.

**Step 1: Add flag and conditional output**

```go
var foMetrics = flag.Bool("fo-metrics", false, "output metrics in fo-compatible JSON format")

func TestEval(t *testing.T) {
	// ... existing test logic producing report ...

	if *foMetrics {
		printFOMetrics(report)
		return
	}
	// ... existing table rendering ...
}

func printFOMetrics(r *Report) {
	columns := []string{"MRR", "P@5", "P@10", "NDCG5"}
	var rows []map[string]interface{}
	// Overall row
	rows = append(rows, map[string]interface{}{
		"name": "Overall", "n": r.QueryCount,
		"values": []float64{r.MRR, r.P5, r.P10, r.NDCG5},
	})
	// Per-kind rows — sort keys for deterministic output
	kinds := make([]string, 0, len(r.ByKind))
	for k := range r.ByKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		km := r.ByKind[kind]
		rows = append(rows, map[string]interface{}{
			"name": kind, "n": km.N,
			"values": []float64{km.MRR, km.P5, km.P10, km.NDCG5},
		})
	}

	out := map[string]interface{}{
		"scope":       fmt.Sprintf("%d queries · %d nugs · %s", r.QueryCount, r.CorpusSize, r.Mode),
		"columns":     columns,
		"rows":        rows,
		"regressions": r.Regressions, // populated by Compare()
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
```

> **Fix applied:** Added `sort.Strings(kinds)` for deterministic per-kind row ordering. Map iteration order is random in Go — without sorting, the metrics JSON would differ between runs, making diffs noisy and test assertions fragile.

> **Fix applied:** Removed the `deltas` field from the output. Per the schema simplification (see Metrics JSON Schema note above), `from`/`to` on regressions is sufficient.

**Step 2: Test**

```bash
cd /Users/vcto/Projects/trixi
go test -run TestEval -count=1 ./internal/eval/ -fo-metrics 2>/dev/null | jq .
```

**Step 3: Commit**

```
feat(eval): add -fo-metrics flag for structured metrics output
```

---

## Summary of Changes

### fo (this repo)

| File | Action | Purpose |
|------|--------|---------|
| `internal/detect/detect.go` | modify | Add `Report` format + detection (before `{` guard) |
| `internal/detect/detect_test.go` | modify | Tests for report detection (all 6 formats) |
| `pkg/report/report.go` | create | Section parser for delimited format |
| `pkg/report/report_test.go` | create | Parser tests (incl. trailing newline preservation) |
| `internal/metrics/metrics.go` | create | Generic metrics JSON parser |
| `internal/metrics/metrics_test.go` | create | Metrics parser tests |
| `internal/archlint/archlint.go` | create | go-arch-lint JSON parser |
| `internal/archlint/archlint_test.go` | create | Arch-lint parser tests |
| `internal/jscpd/jscpd.go` | create | jscpd JSON report parser |
| `internal/jscpd/jscpd_test.go` | create | jscpd parser tests |
| `pkg/mapper/report.go` | create | Section → pattern mapper (all 6 formats, full impl) |
| `pkg/mapper/report_test.go` | create | Mapper tests (incl. malformed section handling) |
| `pkg/render/llm.go` | modify | Report-aware rendering path |
| `pkg/render/llm_test.go` | create | LLM render tests |
| `cmd/fo/main.go` | modify | Wire Report format in parseInput |
| `cmd/fo/main_test.go` | modify | Integration tests (LLM + JSON + Terminal) |
| `cmd/fo/testdata/*.report` | create | Test fixtures (in correct location) |

### trixi (../trixi)

| File | Action | Purpose |
|------|--------|---------|
| `scripts/qa-report.sh` | create | Shell wrapper emitting report format (with stderr logging) |
| `Makefile` | modify | Add `qa-fo` and `all-fo` targets |
| `internal/eval/eval_test.go` | modify | Add `-fo-metrics` JSON output flag |

### Not changed

- No new Go dependencies (encoding/json, regexp are stdlib)
- No changes to existing SARIF or testjson parsers
- No changes to `fo wrap sarif`
- Existing `make qa` and `make all` targets unchanged

## Design Decisions

1. **`internal/` for report-specific parsers.** `metrics`, `archlint`, `jscpd` are tightly coupled to the report feature and not independently useful. They go in `internal/` to keep `pkg/` reserved for core parsers with external utility (sarif, testjson). The section parser (`pkg/report`) stays in `pkg/` because it defines the report format contract.

2. **Section parse errors → failed sections, not aborted reports.** If one tool produces malformed output, the rest of the report still renders. The broken section appears as failed with a "parse error" scope label. This matches the resilience principle: fo should always produce useful output.

3. **jscpd clones are advisory (pass=true, status="skip").** Duplication is a code smell, not a build failure. Clone items render as warnings but don't trigger exit code 1. If CI enforcement is needed later, add a `--strict-dupl` flag or read severity from the delimiter.

4. **Batch-only, no streaming.** QA runs are batch by nature — all tools finish before rendering. No streaming support needed.

5. **String-prefix dispatch in LLM renderer.** This is acknowledged tech debt. The TODO comment marks it for replacement with a `Summary.Kind` field when the renderer is refactored. Doing it now would touch stable code paths for SARIF and test output.

## Remaining Open Questions

1. **Streaming**: Confirmed batch-only. No action needed.

2. **jscpd clone severity**: Resolved — advisory by default (see Design Decision #3).

3. **Summary.Kind refactor**: Deferred. Tracked via TODO comment in `llm.go`. Consider doing this when adding unit tests for `pkg/render` (backlog item).
