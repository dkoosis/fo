# fo-metrics Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `fo-metrics/v1` as fo's third input schema — a JSON format for scalar metrics, conformance checks, and summaries — with renderers for all three output modes and two `fo wrap` transformers.

**Architecture:** Replace the existing `internal/metrics` package (trixi-specific old schema) with a new `internal/fometrics` package implementing the `fo-metrics/v1` schema. Rewrite `mapMetricsSection` in `pkg/mapper/report.go` to parse the new schema and produce `Summary` + optional `TestTable` patterns. Add `fo wrap jscpd` and `fo wrap archlint` subcommands that convert native tool JSON to fo-metrics JSON via stdin/stdout.

**Tech Stack:** Go 1.24+, standard library only (encoding/json, fmt, strings). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-03-12-fo-metrics-pipeline.md`

---

## Chunk 1: Schema Package + Mapper

### Task 1: Define fo-metrics types and parser (`internal/fometrics`)

This package defines the `fo-metrics/v1` JSON schema as Go types with a `Parse()` function that validates the schema version field.

**Files:**
- Create: `internal/fometrics/fometrics.go`
- Create: `internal/fometrics/fometrics_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fometrics/fometrics_test.go
package fometrics

import (
	"strings"
	"testing"
)

func TestParse_ValidPass(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "eval",
		"status": "pass",
		"metrics": [
			{"name": "MRR", "value": 0.983, "threshold": 0.950, "direction": "higher_is_better"},
			{"name": "FPR", "value": 0.000, "threshold": 0.050, "unit": "%", "direction": "lower_is_better"}
		],
		"summary": "86 queries, 0 regressions",
		"details": []
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Schema != "fo-metrics/v1" {
		t.Errorf("schema = %q, want fo-metrics/v1", doc.Schema)
	}
	if doc.Tool != "eval" {
		t.Errorf("tool = %q, want eval", doc.Tool)
	}
	if doc.Status != "pass" {
		t.Errorf("status = %q, want pass", doc.Status)
	}
	if len(doc.Metrics) != 2 {
		t.Fatalf("metrics len = %d, want 2", len(doc.Metrics))
	}
	if doc.Metrics[0].Name != "MRR" || doc.Metrics[0].Value != 0.983 {
		t.Errorf("metric[0] = %+v", doc.Metrics[0])
	}
	if doc.Metrics[0].Threshold == nil || *doc.Metrics[0].Threshold != 0.950 {
		t.Error("metric[0] threshold should be 0.950")
	}
	if doc.Metrics[1].Unit != "%" {
		t.Errorf("metric[1] unit = %q, want %%", doc.Metrics[1].Unit)
	}
	if doc.Metrics[1].Direction != "lower_is_better" {
		t.Errorf("metric[1] direction = %q, want lower_is_better", doc.Metrics[1].Direction)
	}
	if doc.Summary != "86 queries, 0 regressions" {
		t.Errorf("summary = %q", doc.Summary)
	}
}

func TestParse_Defaults(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "dupl",
		"status": "pass",
		"metrics": [{"name": "clones", "value": 12}]
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := doc.Metrics[0]
	if m.Threshold != nil {
		t.Error("threshold should be nil by default")
	}
	if m.Direction != "higher_is_better" {
		t.Errorf("direction should default to higher_is_better, got %q", m.Direction)
	}
	if m.Unit != "" {
		t.Errorf("unit should default to empty, got %q", m.Unit)
	}
	if doc.Summary != "" {
		t.Errorf("summary should default to empty, got %q", doc.Summary)
	}
	if len(doc.Details) != 0 {
		t.Errorf("details should default to empty, got %d", len(doc.Details))
	}
}

func TestParse_WithDetails(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "dupl",
		"status": "warn",
		"metrics": [{"name": "clones", "value": 3}],
		"details": [
			{"message": "clone: a.go:1-10 <> b.go:1-10", "file": "a.go", "line": 1, "severity": "warn"},
			{"message": "dep violation", "category": "arch"}
		]
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Details) != 2 {
		t.Fatalf("details len = %d, want 2", len(doc.Details))
	}
	d := doc.Details[0]
	if d.Message != "clone: a.go:1-10 <> b.go:1-10" {
		t.Errorf("detail[0] message = %q", d.Message)
	}
	if d.File != "a.go" || d.Line != 1 || d.Severity != "warn" {
		t.Errorf("detail[0] = %+v", d)
	}
	if doc.Details[1].Category != "arch" {
		t.Errorf("detail[1] category = %q", doc.Details[1].Category)
	}
}

func TestParse_RejectsV2(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v2", "tool": "x", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for v2 schema")
	}
	if !strings.Contains(err.Error(), "unsupported schema") {
		t.Errorf("error = %q, want 'unsupported schema'", err)
	}
}

func TestParse_RejectsMissingSchema(t *testing.T) {
	input := []byte(`{"tool": "x", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}

func TestParse_RejectsInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParse_AcceptsV1Minor(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1.2", "tool": "x", "status": "pass", "metrics": []}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("v1.2 should be accepted: %v", err)
	}
	if doc.Tool != "x" {
		t.Errorf("tool = %q", doc.Tool)
	}
}

func TestParse_RejectsMissingTool(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestParse_RejectsInvalidStatus(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1", "tool": "x", "status": "unknown", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}
```

The `"strings"` import is included above (needed by `TestParse_RejectsV2`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/fometrics/ -v -count=1`
Expected: compilation failure — package does not exist

- [ ] **Step 3: Write the implementation**

```go
// internal/fometrics/fometrics.go
package fometrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Document represents a fo-metrics/v1 JSON object.
type Document struct {
	Schema  string   `json:"schema"`
	Tool    string   `json:"tool"`
	Status  string   `json:"status"`
	Metrics []Metric `json:"metrics"`
	Summary string   `json:"summary,omitempty"`
	Details []Detail `json:"details,omitempty"`
}

// Metric is a single named metric with optional threshold and direction.
type Metric struct {
	Name      string   `json:"name"`
	Value     float64  `json:"value"`
	Threshold *float64 `json:"threshold,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	Direction string   `json:"direction,omitempty"`
}

// Detail is an itemized finding within a metrics report.
type Detail struct {
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`
}

// Parse decodes and validates a fo-metrics JSON document.
// Accepts fo-metrics/v1 and v1.x; rejects v2+ and missing/malformed schemas.
func Parse(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	if err := validate(&doc); err != nil {
		return nil, err
	}

	applyDefaults(&doc)
	return &doc, nil
}

func validate(doc *Document) error {
	// Schema version check
	switch {
	case doc.Schema == "":
		return fmt.Errorf("missing required field: schema")
	case doc.Schema == "fo-metrics/v1":
		// exact match, ok
	case strings.HasPrefix(doc.Schema, "fo-metrics/v1."):
		// minor version, ok
	case strings.HasPrefix(doc.Schema, "fo-metrics/"):
		return fmt.Errorf("unsupported schema: %s", doc.Schema)
	default:
		return fmt.Errorf("unsupported schema: %s", doc.Schema)
	}

	if doc.Tool == "" {
		return fmt.Errorf("missing required field: tool")
	}

	switch doc.Status {
	case "pass", "fail", "warn":
		// valid
	case "":
		return fmt.Errorf("missing required field: status")
	default:
		return fmt.Errorf("invalid status: %q (expected pass, fail, or warn)", doc.Status)
	}

	return nil
}

func applyDefaults(doc *Document) {
	for i := range doc.Metrics {
		if doc.Metrics[i].Direction == "" {
			doc.Metrics[i].Direction = "higher_is_better"
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/fometrics/ -v -count=1`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/fometrics/fometrics.go internal/fometrics/fometrics_test.go
git commit -m "feat: add internal/fometrics package with fo-metrics/v1 schema types and parser"
```

---

### Task 2: Refactor `mapSection` return + rewrite `mapMetricsSection`

Two changes in one task:

1. **Expand `mapSection` return** from `([]pattern.Pattern, bool, string)` to `([]pattern.Pattern, pattern.ItemKind, string)`. This lets the mapper express three states: pass (`KindSuccess`), fail (`KindError`), warn (`KindWarning`). Currently `FromReport` derives kind from a bool, losing the warn state. The refactor is mechanical — each existing mapper replaces `true` → `KindSuccess`, `false` → `KindError`.

2. **Replace `mapMetricsSection`** — swap old `metrics.Parse()` for `fometrics.Parse()`, use the new `ItemKind` return for warn support, and emit a synthetic fail item when `status:"fail"` + empty details (ensuring exit code 1).

**Files:**
- Modify: `pkg/mapper/report.go:1-14` (imports), `pkg/mapper/report.go:26-97` (`FromReport`, `mapSection`, all mapper returns), `pkg/mapper/report.go:148-194` (`mapMetricsSection`)
- Modify: `pkg/mapper/report_test.go` (add metrics mapper tests)

- [ ] **Step 1: Write the failing tests**

Add to `pkg/mapper/report_test.go`:

```go
func TestFromReport_MetricsPassSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"eval","status":"pass","metrics":[{"name":"MRR","value":0.983,"threshold":0.95,"direction":"higher_is_better"},{"name":"FPR","value":0.0,"threshold":0.05,"unit":"%","direction":"lower_is_better"}],"summary":"86 queries"}`
	sections := []report.Section{
		{Tool: "eval", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("expected success kind for passing metrics, got %q", sum.Metrics[0].Kind)
	}
	if !strings.Contains(sum.Metrics[0].Value, "MRR=0.983") {
		t.Errorf("expected MRR in label, got %q", sum.Metrics[0].Value)
	}
}

func TestFromReport_MetricsFailSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"eval","status":"fail","metrics":[{"name":"MRR","value":0.800,"threshold":0.95,"direction":"higher_is_better"}],"summary":"regression","details":[{"message":"MRR dropped","severity":"error"}]}`
	sections := []report.Section{
		{Tool: "eval", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("expected error kind for failing metrics, got %q", sum.Metrics[0].Kind)
	}
	// Should produce a TestTable from details
	var foundTable bool
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			foundTable = true
			if len(tt.Results) != 1 {
				t.Errorf("expected 1 detail item, got %d", len(tt.Results))
			}
			if tt.Results[0].Status != pattern.StatusFail {
				t.Errorf("severity error should map to StatusFail, got %q", tt.Results[0].Status)
			}
		}
	}
	if !foundTable {
		t.Error("expected TestTable from non-empty details")
	}
}

func TestFromReport_MetricsWarnSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"dupl","status":"warn","metrics":[{"name":"clones","value":5}],"details":[{"message":"clone pair","severity":"warn"}]}`
	sections := []report.Section{
		{Tool: "dupl", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	// warn renders yellow but doesn't fail the build
	if sum.Metrics[0].Kind != pattern.KindWarning {
		t.Errorf("warn status should map to KindWarning, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsFailNoDetails(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"check","status":"fail","metrics":[{"name":"score","value":0.5}]}`
	sections := []report.Section{
		{Tool: "check", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("fail status should map to KindError, got %q", sum.Metrics[0].Kind)
	}
	// Even with no details, a fail section must produce a pattern that triggers exit code 1
	var hasFailItem bool
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			for _, r := range tt.Results {
				if r.Status == pattern.StatusFail {
					hasFailItem = true
				}
			}
		}
	}
	if !hasFailItem {
		t.Error("status:fail with no details must emit a synthetic fail item for exit code 1")
	}
}

func TestFromReport_MetricsEmptyMetrics(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"check","status":"pass","metrics":[]}`
	sections := []report.Section{
		{Tool: "check", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("expected success, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsWithUnit(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"perf","status":"pass","metrics":[{"name":"latency","value":42.5,"unit":"ms"}]}`
	sections := []report.Section{
		{Tool: "perf", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Metrics[0].Value, "ms") {
		t.Errorf("expected unit suffix in label, got %q", sum.Metrics[0].Value)
	}
}

func TestFromReport_MetricsInvalidJSON(t *testing.T) {
	sections := []report.Section{
		{Tool: "bad", Format: "metrics", Content: []byte(`not json`)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("invalid JSON should produce error kind, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsDetailCategories(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"arch","status":"fail","metrics":[],"details":[
		{"message":"a->b","category":"deps","severity":"error"},
		{"message":"c->d","category":"deps","severity":"error"},
		{"message":"unused file","category":"hygiene"}
	]}`
	sections := []report.Section{
		{Tool: "arch", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	var table *pattern.TestTable
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			table = tt
		}
	}
	if table == nil {
		t.Fatal("expected TestTable from details")
	}
	if len(table.Results) != 3 {
		t.Errorf("expected 3 detail items, got %d", len(table.Results))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/mapper/ -run "TestFromReport_Metrics" -v -count=1`
Expected: FAIL — `mapMetricsSection` still calls old `metrics.Parse()`, the fo-metrics JSON won't parse with the old schema.

- [ ] **Step 3: Refactor `FromReport` and `mapSection` to use `ItemKind`**

In `pkg/mapper/report.go`, change `mapSection` signature and update `FromReport` to use it:

```go
// mapSection return type changes: bool → pattern.ItemKind
func mapSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
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
		return sectionError(sec.Tool, fmt.Errorf("unknown format %q", sec.Format)),
			pattern.KindError, fmt.Sprintf("unknown format %q", sec.Format)
	}
}
```

Update `FromReport` — replace the `sectionPass` bool logic:

```go
// Before:
sectionPatterns, sectionPass, scopeLabel := mapSection(sec)
if sectionPass { pass++ } else { fail++ }
kind := pattern.KindSuccess
if !sectionPass { kind = pattern.KindError }

// After:
sectionPatterns, kind, scopeLabel := mapSection(sec)
if kind == pattern.KindError { fail++ } else { pass++ }
```

The `kind` is used directly in the `SummaryItem`, no derivation needed.

Then update each existing mapper's return type. The changes are mechanical:

```go
// mapSARIFSection: change return type, replace bool returns
func mapSARIFSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	// ... existing code ...
	// was: return sectionError(...), false, "parse error"
	// now:
	return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	// ...
	// was: passed := stats.ByLevel["error"] == 0; return patterns, passed, label
	// now:
	kind := pattern.KindSuccess
	if stats.ByLevel["error"] > 0 { kind = pattern.KindError }
	return patterns, kind, label
}

// mapTestJSONSection: same pattern
func mapTestJSONSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	// was: return sectionError(...), false, ...
	// now: return sectionError(...), pattern.KindError, ...
	// was: return patterns, true, label  /  return patterns, false, label
	// now: return patterns, pattern.KindSuccess, label  /  return patterns, pattern.KindError, label
}

// mapArchLintSection: same pattern
func mapArchLintSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	// was: return nil, true, label
	// now: return nil, pattern.KindSuccess, label
	// was: return patterns, false, label
	// now: return patterns, pattern.KindError, label
}

// mapJSCPDSection: same pattern
func mapJSCPDSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	// was: return nil, true, "pass (0 clones)"
	// now: return nil, pattern.KindSuccess, "pass (0 clones)"
	// was: return patterns, true, label  (clones don't fail)
	// now: return patterns, pattern.KindSuccess, label
}

// mapTextSection: same pattern
func mapTextSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	// was: passed := sec.Status != statusFail; return nil, passed, label
	// now:
	kind := pattern.KindSuccess
	if sec.Status == statusFail { kind = pattern.KindError }
	return nil, kind, label
}
```

- [ ] **Step 4: Rewrite `mapMetricsSection` to use fo-metrics schema**

Replace the import of `"github.com/dkoosis/fo/internal/metrics"` with `"github.com/dkoosis/fo/internal/fometrics"` and rewrite `mapMetricsSection`:

```go
func mapMetricsSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	doc, err := fometrics.Parse(sec.Content)
	if err != nil {
		return sectionError(sec.Tool, err), pattern.KindError, fmt.Sprintf("parse error: %v", err)
	}

	// Map status to ItemKind — the spec says status is authoritative
	kind := mapMetricsStatus(doc.Status)

	// Build label from metrics: "PASS — MRR=0.983 FPR=0.000%"
	label := buildMetricsLabel(doc)

	var patterns []pattern.Pattern

	// Build TestTable from details if non-empty
	if len(doc.Details) > 0 {
		items := make([]pattern.TestTableItem, 0, len(doc.Details))
		for _, d := range doc.Details {
			status := mapDetailSeverity(d.Severity)
			name := d.Message
			if d.File != "" {
				loc := d.File
				if d.Line > 0 {
					loc = fmt.Sprintf("%s:%d", d.File, d.Line)
				}
				name = loc + " " + d.Message
			}
			items = append(items, pattern.TestTableItem{
				Name:   name,
				Status: status,
			})
		}
		patterns = append(patterns, &pattern.TestTable{
			Label:   sec.Tool + " details",
			Results: items,
		})
	}

	// Ensure status:"fail" produces exit code 1 even when details are empty.
	// exitCode() checks TestTable fail items and Error patterns — Summary alone
	// doesn't trigger it. Emit a synthetic fail item so the signal propagates.
	if doc.Status == "fail" && len(doc.Details) == 0 {
		patterns = append(patterns, &pattern.TestTable{
			Label: sec.Tool,
			Results: []pattern.TestTableItem{
				{Name: "metrics check failed", Status: pattern.StatusFail},
			},
		})
	}

	return patterns, kind, label
}

func mapMetricsStatus(status string) pattern.ItemKind {
	switch status {
	case "fail":
		return pattern.KindError
	case "warn":
		return pattern.KindWarning
	default:
		return pattern.KindSuccess
	}
}

func buildMetricsLabel(doc *fometrics.Document) string {
	prefix := strings.ToUpper(doc.Status)

	var parts []string
	for _, m := range doc.Metrics {
		formatted := formatMetricValue(m)
		parts = append(parts, fmt.Sprintf("%s=%s", m.Name, formatted))
	}

	label := prefix
	if len(parts) > 0 {
		label += " — " + strings.Join(parts, " ")
	}
	if doc.Summary != "" {
		label += " (" + doc.Summary + ")"
	}
	return label
}

func formatMetricValue(m fometrics.Metric) string {
	// Format the value: use integer format if it's a whole number, else 3 decimal places
	var s string
	if m.Value == float64(int64(m.Value)) {
		s = fmt.Sprintf("%d", int64(m.Value))
	} else {
		s = fmt.Sprintf("%.3f", m.Value)
	}
	if m.Unit != "" {
		s += m.Unit
	}
	return s
}

func mapDetailSeverity(severity string) pattern.TestStatus {
	switch severity {
	case "error":
		return pattern.StatusFail
	case "warn":
		return pattern.StatusSkip
	default:
		return pattern.StatusPass
	}
}
```

Also remove `"github.com/dkoosis/fo/internal/metrics"` from the import block and add `"github.com/dkoosis/fo/internal/fometrics"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/mapper/ -v -count=1`
Expected: all PASS (both new and existing tests)

- [ ] **Step 5: Run all tests to check for regressions**

Run: `go test ./... -count=1`
Expected: PASS. The `full.report` fixture uses the old metrics schema, so `TestRun_ReportFullFormats` will fail — that's expected and fixed in Task 3.

- [ ] **Step 6: Commit**

```bash
git add pkg/mapper/report.go pkg/mapper/report_test.go
git commit -m "feat: rewrite mapMetricsSection to use fo-metrics/v1 schema"
```

---

### Task 3: Update test fixtures to use fo-metrics/v1

The `full.report` fixture has `format:metrics` with old schema JSON. Update it to use the new fo-metrics/v1 format.

**Files:**
- Modify: `cmd/fo/testdata/full.report` (line 10 — the metrics section content)

- [ ] **Step 1: Update `full.report` fixture**

Replace line 10 (the `format:metrics` content) from:
```
{"scope":"86 queries · 51 nugs","columns":["MRR","P@5","P@10","NDCG5"],"rows":[{"name":"Overall","values":[0.983,0.227,0.119,0.961],"n":86}],"regressions":[]}
```

To:
```
{"schema":"fo-metrics/v1","tool":"eval","status":"pass","metrics":[{"name":"MRR","value":0.983,"threshold":0.95,"direction":"higher_is_better"},{"name":"P@5","value":0.227},{"name":"P@10","value":0.119},{"name":"NDCG5","value":0.961,"threshold":0.9,"direction":"higher_is_better"}],"summary":"86 queries, 0 regressions"}
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS. The `TestRun_ReportFullFormats` test checks for `"7 tools"`, `"all pass"`, and each tool name — these should all still hold.

- [ ] **Step 3: Commit**

```bash
git add cmd/fo/testdata/full.report
git commit -m "fix: update full.report fixture to fo-metrics/v1 schema"
```

---

### Task 4: Remove `internal/metrics` package (dead code)

After Task 2-3, nothing imports `internal/metrics` anymore. Remove it.

**Files:**
- Delete: `internal/metrics/metrics.go`
- Delete: `internal/metrics/metrics_test.go`

- [ ] **Step 1: Verify nothing imports the old package**

Run: `grep -r '"github.com/dkoosis/fo/internal/metrics"' --include='*.go' .`
Expected: no matches (Task 2 already switched the import)

- [ ] **Step 2: Delete the files**

```bash
rm internal/metrics/metrics.go internal/metrics/metrics_test.go
rmdir internal/metrics
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add -u internal/metrics/
git commit -m "cleanup: remove internal/metrics package, replaced by internal/fometrics"
```

---

## Chunk 2: Transformers (`fo wrap jscpd`, `fo wrap archlint`)

### Task 5: Add `fo wrap jscpd` transformer

Reads jscpd native JSON from stdin, writes fo-metrics/v1 JSON to stdout. Uses `internal/jscpd.Parse()` to parse the native format, then constructs and marshals a `fometrics.Document`.

**Files:**
- Create: `cmd/fo/wrap_jscpd.go`
- Create: `cmd/fo/wrap_jscpd_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// cmd/fo/wrap_jscpd_test.go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrapJscpd_EmptyDuplicates(t *testing.T) {
	input := `{"duplicates":[],"statistics":{}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"schema":"fo-metrics/v1"`) && !strings.Contains(out, `"schema": "fo-metrics/v1"`) {
		t.Errorf("missing schema field in output:\n%s", out)
	}
	if !strings.Contains(out, `"status":"pass"`) && !strings.Contains(out, `"status": "pass"`) {
		t.Errorf("expected status pass:\n%s", out)
	}
}

func TestWrapJscpd_WithClones(t *testing.T) {
	input := `{"duplicates":[{
		"format":"go",
		"lines":22,
		"firstFile":{"name":"a.go","startLoc":{"line":1},"endLoc":{"line":22}},
		"secondFile":{"name":"b.go","startLoc":{"line":10},"endLoc":{"line":31}}
	}],"statistics":{}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"warn"`) && !strings.Contains(out, `"status": "warn"`) {
		t.Errorf("expected status warn for clones:\n%s", out)
	}
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Errorf("expected clone files in details:\n%s", out)
	}
}

func TestWrapJscpd_InvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"jscpd"}, strings.NewReader("not json"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Error("expected error on stderr")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fo/ -run "TestWrapJscpd" -v -count=1`
Expected: FAIL — `runWrap` doesn't recognize "jscpd" yet

- [ ] **Step 3: Write the implementation**

```go
// cmd/fo/wrap_jscpd.go
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/internal/fometrics"
	"github.com/dkoosis/fo/internal/jscpd"
)

func runWrapJscpd(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: reading stdin: %v\n", err)
		return 1
	}

	result, err := jscpd.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: %v\n", err)
		return 1
	}

	doc := fometrics.Document{
		Schema: "fo-metrics/v1",
		Tool:   "jscpd",
		Status: "pass",
		Metrics: []fometrics.Metric{
			{Name: "clones", Value: float64(len(result.Clones))},
		},
	}

	if len(result.Clones) > 0 {
		doc.Status = "warn"
		for _, c := range result.Clones {
			doc.Details = append(doc.Details, fometrics.Detail{
				Message:  fmt.Sprintf("%s:%d-%d ↔ %s:%d-%d (%d lines, %s)", c.FileA, c.StartA, c.EndA, c.FileB, c.StartB, c.EndB, c.Lines, c.Format),
				File:     c.FileA,
				Line:     c.StartA,
				Severity: "warn",
			})
		}
	}

	doc.Summary = fmt.Sprintf("%d clones", len(result.Clones))

	out, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap jscpd: marshal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}
```

- [ ] **Step 4: Update `runWrap` dispatch in `cmd/fo/main.go`**

Replace the guard at line 319:

```go
// Before:
if len(args) == 0 || args[0] != "sarif" {
    fmt.Fprintf(stderr, "fo wrap: unknown subcommand (expected 'sarif')\n\n")
    fmt.Fprint(stderr, wrapUsage)
    return 2
}

// After:
if len(args) == 0 {
    fmt.Fprintf(stderr, "fo wrap: subcommand required (sarif, jscpd, archlint)\n\n")
    fmt.Fprint(stderr, wrapUsage)
    return 2
}
switch args[0] {
case "sarif":
    // fall through to existing sarif handling below
case "jscpd":
    return runWrapJscpd(stdin, stdout, stderr)
case "archlint":
    return runWrapArchlint(stdin, stdout, stderr)
default:
    fmt.Fprintf(stderr, "fo wrap: unknown subcommand %q (expected sarif, jscpd, archlint)\n\n", args[0])
    fmt.Fprint(stderr, wrapUsage)
    return 2
}
```

Note: `runWrapArchlint` won't exist yet — this step will only compile after Task 6. If you want to avoid a compile error between tasks, add a stub:

```go
// Stub in wrap_archlint.go — replaced in Task 6
package main

import (
	"fmt"
	"io"
)

func runWrapArchlint(_ io.Reader, _, stderr io.Writer) int {
	fmt.Fprintf(stderr, "fo wrap archlint: not yet implemented\n")
	return 2
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./cmd/fo/ -run "TestWrapJscpd" -v -count=1`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/fo/wrap_jscpd.go cmd/fo/wrap_jscpd_test.go cmd/fo/main.go
git commit -m "feat: add fo wrap jscpd transformer (jscpd JSON -> fo-metrics/v1)"
```

---

### Task 6: Add `fo wrap archlint` transformer

Same pattern as Task 5 but for go-arch-lint JSON.

**Files:**
- Create: `cmd/fo/wrap_archlint.go` (replaces stub if one was created)
- Create: `cmd/fo/wrap_archlint_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// cmd/fo/wrap_archlint_test.go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestWrapArchlint_Clean(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"pass"`) && !strings.Contains(out, `"status": "pass"`) {
		t.Errorf("expected status pass:\n%s", out)
	}
	if !strings.Contains(out, "violations") {
		t.Errorf("expected violations metric in output:\n%s", out)
	}
}

func TestWrapArchlint_WithViolations(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"search","FileRelativePath":"pkg/search/search.go","ResolvedImportName":"embedder"}
	],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[]}}`
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, `"status":"fail"`) && !strings.Contains(out, `"status": "fail"`) {
		t.Errorf("expected status fail:\n%s", out)
	}
	if !strings.Contains(out, "search") || !strings.Contains(out, "embedder") {
		t.Errorf("expected violation details in output:\n%s", out)
	}
}

func TestWrapArchlint_InvalidJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"archlint"}, strings.NewReader("bad"), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./cmd/fo/ -run "TestWrapArchlint" -v -count=1`
Expected: FAIL — the stub returns 2, not 0

- [ ] **Step 3: Write the implementation**

```go
// cmd/fo/wrap_archlint.go
package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/internal/archlint"
	"github.com/dkoosis/fo/internal/fometrics"
)

func runWrapArchlint(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: reading stdin: %v\n", err)
		return 1
	}

	result, err := archlint.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: %v\n", err)
		return 1
	}

	doc := fometrics.Document{
		Schema: "fo-metrics/v1",
		Tool:   "go-arch-lint",
		Status: "pass",
		Metrics: []fometrics.Metric{
			{Name: "violations", Value: float64(len(result.Violations)), Direction: "lower_is_better"},
			{Name: "checks", Value: float64(len(result.Checks))},
		},
	}

	if result.HasWarnings {
		doc.Status = "fail"
		for _, v := range result.Violations {
			doc.Details = append(doc.Details, fometrics.Detail{
				Message:  fmt.Sprintf("%s → %s", v.From, v.To),
				File:     v.FileFrom,
				Severity: "error",
				Category: "dependency",
			})
		}
	}

	doc.Summary = fmt.Sprintf("%d violations, %d checks", len(result.Violations), len(result.Checks))

	out, err := json.Marshal(doc)
	if err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint: marshal: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", out)
	return 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./cmd/fo/ -run "TestWrapArchlint" -v -count=1`
Expected: all PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/fo/wrap_archlint.go cmd/fo/wrap_archlint_test.go
git commit -m "feat: add fo wrap archlint transformer (go-arch-lint JSON -> fo-metrics/v1)"
```

---

### Task 7: Update usage strings and wrapUsage

Now that all three `fo wrap` subcommands exist, update the help text.

**Files:**
- Modify: `cmd/fo/main.go:46-93` (usage constant), `cmd/fo/main.go:295-316` (wrapUsage constant)

- [ ] **Step 1: Update `usage` constant**

In the SUBCOMMANDS section (around line 68), add after the `fo wrap sarif` block:

```
  fo wrap jscpd     Convert jscpd JSON report to fo-metrics
  fo wrap archlint  Convert go-arch-lint JSON to fo-metrics
```

Also update the description line (around line 9) and INPUT FORMATS to mention fo-metrics:

```
INPUT FORMATS (auto-detected from stdin)
  SARIF 2.1.0     Static analysis results (golangci-lint, gosec, etc.)
  go test -json   Test execution stream (supports live + batch)
  fo-metrics/v1   Scalar metrics, conformance, summaries (eval, jscpd, go-arch-lint)
  report          Multi-tool delimited report (--- tool:X format:Y ---)
```

- [ ] **Step 2: Update `wrapUsage` constant**

Add the jscpd and archlint subcommands to the wrap help text. Change the title line and add brief descriptions.

- [ ] **Step 3: Run tests**

Run: `go test ./cmd/fo/ -v -count=1`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/fo/main.go
git commit -m "docs: update --help text for fo wrap jscpd and fo wrap archlint"
```

---

## Chunk 3: E2E Integration Test

### Task 8: Add E2E test with fo-metrics in report format

Validate the full pipeline: report with `format:metrics` section → mapper → renderer → output.

**Files:**
- Modify: `cmd/fo/main_test.go`

- [ ] **Step 1: Write the E2E test**

Add to `cmd/fo/main_test.go`:

```go
func TestRun_ReportWithFoMetrics(t *testing.T) {
	metricsJSON := `{"schema":"fo-metrics/v1","tool":"eval","status":"pass","metrics":[{"name":"MRR","value":0.983,"threshold":0.95,"direction":"higher_is_better"},{"name":"FPR","value":0.0,"threshold":0.05,"direction":"lower_is_better"}],"summary":"86 queries"}`
	input := "--- tool:eval format:metrics ---\n" + metricsJSON + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "eval:") {
		t.Errorf("expected eval tool in output:\n%s", out)
	}
	if !strings.Contains(out, "MRR") {
		t.Errorf("expected MRR metric in output:\n%s", out)
	}
}

func TestRun_ReportWithFoMetricsFailing(t *testing.T) {
	metricsJSON := `{"schema":"fo-metrics/v1","tool":"eval","status":"fail","metrics":[{"name":"MRR","value":0.8}],"details":[{"message":"MRR dropped below threshold","severity":"error"}]}`
	input := "--- tool:eval format:metrics ---\n" + metricsJSON + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "MRR dropped") {
		t.Errorf("expected detail message in output:\n%s", out)
	}
}

func TestRun_FoMetricsFailNoDetails(t *testing.T) {
	metricsJSON := `{"schema":"fo-metrics/v1","tool":"check","status":"fail","metrics":[{"name":"score","value":0.5}]}`
	input := "--- tool:check format:metrics ---\n" + metricsJSON + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 for status:fail even without details; stderr: %s", code, stderr.String())
	}
}

func TestRun_FoMetricsRejectsV2(t *testing.T) {
	metricsJSON := `{"schema":"fo-metrics/v2","tool":"x","status":"pass","metrics":[]}`
	input := "--- tool:x format:metrics ---\n" + metricsJSON + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (error pattern from unsupported schema); stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "unsupported schema") {
		t.Errorf("expected 'unsupported schema' error in output:\n%s", out)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./cmd/fo/ -run "TestRun_.*FoMetrics|TestRun_.*Metrics" -v -count=1`
Expected: all PASS

- [ ] **Step 3: Run full suite one final time**

Run: `go test ./... -count=1`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/fo/main_test.go
git commit -m "test: add E2E tests for fo-metrics in report pipeline"
```

---

## Chunk 4: Cleanup and Verification

### Task 9: Remove dead `mapArchLintSection` and `mapJSCPDSection` code paths

With `fo wrap jscpd` and `fo wrap archlint` producing fo-metrics JSON, the Makefile will eventually use `format:metrics` for these tools. However, the `format:archlint` and `format:jscpd` tokens are still valid in `report.DelimiterRe` and still have mapper functions. These remain as backward compatibility — **do not remove yet**. The spec says fo-metrics is additive.

This task is just verification — no code changes needed unless the `format:archlint` and `format:jscpd` code paths have been explicitly deprecated.

- [ ] **Step 1: Verify backward compatibility**

Run: `go test ./cmd/fo/ -run "TestRun_ReportFullFormats" -v -count=1`
Expected: PASS — `full.report` still has `format:archlint` and `format:jscpd` sections that go through the native parsers.

- [ ] **Step 2: Run lint**

Run: `golangci-lint run --output.text.path=stdout ./...`
Expected: clean (no new warnings from the changes)

- [ ] **Step 3: Run the binary manually against the full report fixture**

Run: `cat cmd/fo/testdata/full.report | go run ./cmd/fo/ --format llm`
Expected: renders all 7 tools with proper labels. The `eval` section should show `MRR=0.983` etc from the new fo-metrics format.

---

## Implementation Notes

### What `internal/archlint` and `internal/jscpd` become
These packages are now used in two places:
1. `mapArchLintSection`/`mapJSCPDSection` in `pkg/mapper/report.go` — for backward compat with `format:archlint`/`format:jscpd` sections
2. `runWrapArchlint`/`runWrapJscpd` in `cmd/fo/` — for the transformer subcommands

This is fine. They're small, focused parsers. No refactoring needed.

### What doesn't change
- `pkg/render/terminal.go` — the existing `renderSummary` already works for fo-metrics because the mapper constructs `Summary` patterns with the standard `SummaryItem` structure. The spec's compact terminal layout is aspirational — the current renderer handles it adequately.
- `pkg/render/llm.go` — the existing `renderReport` already handles fo-metrics because it renders `SummaryItem.Label + ": " + SummaryItem.Value` per tool, which produces `eval: PASS — MRR=0.983 ...`. The detail `TestTable` is rendered below the tool summary via `tablesByTool`.
- `pkg/render/json.go` — passthrough, no changes needed.
- `internal/detect/` — `format:metrics` is already a valid token in `DelimiterRe`.
- `internal/report/` — no changes needed.

### Exit code behavior
- `status:"fail"` → `KindError` on Summary item. If details exist with `severity:"error"`, `StatusFail` items in TestTable → `exitCode()` returns 1. If details are empty, a synthetic fail item is emitted → exit 1. Both paths covered.
- `status:"warn"` → `KindWarning` on Summary item (renders yellow). Details with `severity:"warn"` → `StatusSkip` → `exitCode()` returns 0. Warnings don't fail the build.
- `status:"pass"` → `KindSuccess`. No details typically. Exit 0.

### Deferred work (not in scope)
- **Per-metric threshold coloring**: The spec says metrics breaching thresholds should get `KindWarning`/`KindError` styling individually. This plan puts all metrics into a flat label string in the Summary, so per-metric color is lost. To implement: the mapper would need to emit individual `SummaryItem`s per metric (each with its own Kind based on threshold comparison), requiring a different Summary layout. Deferred to a renderer enhancement pass.
- **Compact terminal layout**: The spec shows a single-line `eval  pass  MRR=0.983 ...` format. The current `renderSummary` renders each metric on its own line with icons. The compact layout would need a `SummaryKindMetrics` dispatch in the terminal renderer. Deferred — current layout is functional.
