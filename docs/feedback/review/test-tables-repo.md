# test-tables — repo

Scope: project. Linter: test-tables. Mode: report.
go.mod: `go 1.24.0` — no legacy `tt := tt` rescope needed (none found, clean).

Overall: table-tests in this repo are mostly well-shaped. Most have `name` fields and use `t.Run(tt.name, ...)`. Findings below are concentrated in three files; the rest is clean.

### 1. [F1] per-case-branching — `testjson.TestParseStream_Behavior`

**Test:** `pkg/testjson/parser_test.go:10` (loop body at `:97-137`)
**Issue:** `per-case-branching`

The table has 5 rows but the body conditionally asserts on different fields per row, so the cases don't share a shape:

```go
if len(results) != tt.wantPackages { ... }
if tt.wantPackages == 0 { return }                              // shape-1 early return
got := results[0]
if got.Name != tt.wantPackageName { ... }
if got.Passed != tt.wantPassed { ... }
...
if tt.wantStatus != "" && got.Status() != tt.wantStatus { ... } // shape-2: status row
if tt.wantCoverage > 0 && (got.Coverage < tt.wantCoverage-0.01 ...) // shape-3: coverage row
if got.Panicked != tt.wantPanicked { ... }                      // shape-4: panic row
```

The struct has 10 fields, most rows leave 5+ at zero. "package with no test activity is skipped" diverges entirely (returns before the per-package asserts). "coverage is parsed" and "panic output marks package" each carry a single distinctive assertion guarded by a non-zero/empty check.

**Fix:** split into focused tests with uniform shapes:
- `TestParseStream_AggregatesPassFail` — base case asserting Passed/Failed/Status
- `TestParseStream_ParsesCoverage` — one or two cases, asserts Coverage
- `TestParseStream_DetectsPanic` — asserts Panicked
- `TestParseStream_SkipsMalformedLines` — asserts Malformed counter
- `TestParseStream_SkipsEmptyPackages` — asserts len(results)==0

Each then has its own narrow struct and body. The "if field != zero { assert }" pattern is the smell.

---

### 2. [F2] name-field-missing — `wrapdiag.TestFixCommandFor`

**Test:** `pkg/wrapper/wrapdiag/diag_test.go:176`
**Issue:** `table-name-field-missing`

```go
tests := []struct {
    tool, rule, file string
    want             string
}{
    {toolGolangciLint, "SA4006", mainGoName, "golangci-lint run --fix --enable-only=SA4006 main.go"},
    ...
}
for _, tt := range tests {
    got := fixCommandFor(tt.tool, tt.rule, tt.file)
    if got != tt.want { t.Errorf(...) }
}
```

7 rows, no `name` field, no `t.Run`. On failure the diagnostic depends on `t.Errorf` formatting the inputs — workable for this case, but inconsistent with the rest of the repo where `name + t.Run` is the norm. Failures don't surface in `go test -v` as named subtests.

**Fix:** add `name string` as first field; wrap loop body in `t.Run(tt.name, ...)`.

---

### 3. [F3] name-field-missing — `wrapdiag.TestParseDiagLine`

**Test:** `pkg/wrapper/wrapdiag/diag_test.go:197`
**Issue:** `table-name-field-missing`

Same shape as F2 — 8 rows, positional struct, no `name`, no `t.Run`. The struct fields are descriptive (`wantFile, wantLine, wantCol, wantMsg`) but a subtest name like "windows drive path" would make failures self-describing. Consistency with `TestNormalizeMessage` in `pkg/fingerprint/fingerprint_test.go` (which does use `name` + `t.Run`) argues for the same form here.

**Fix:** add `name string` as first field; use `t.Run(tt.name, ...)`.

---

### 4. [F4] style-inconsistent — `pkg/score` package

**Test:** `pkg/score/score_test.go` (lines 9, 25, 56)
**Issue:** `table-style-inconsistent`

Within one file:

- `TestSeverityWeight_MapsKnownLevels` uses `map[string]int{...}` then `for level, want := range cases`
- `TestFileCentrality_PrecedenceRules` and `TestScore_SeverityOccurrenceCentralityMatrix` use `[]struct{name, ...}{...}` with `t.Run(tc.name, ...)`

The map form is fine for trivial enum→weight mapping, but readers now have to predict which form a given test uses. The slice-of-struct form is the package convention everywhere else.

**Fix:** migrate the map case to the same `[]struct{name, level string; want int}` form with `t.Run(tc.name, ...)`. Cheap, makes the file uniform.

---

### 5. [F5] unused-field across rows — `testjson.TestStream_EventDeliveryAndMalformedCounting`

**Test:** `pkg/testjson/stream_test.go:12`
**Issue:** `table-unused-field` (partial — field used by 1 of 3 rows)

```go
tests := []struct {
    name          string
    input         string
    wantMalformed int
    wantEvents    int
    check         func(t *testing.T, events []TestEvent)
}{
    { name: "valid stream...", ..., check: func(...) { ... } },     // uses check
    { name: "malformed lines are skipped", ... },                   // check nil
    { name: "mixed malformed and valid lines", ... },               // check nil
}
```

Two of three rows leave `check` nil; only the first row has an event-content assertion. This is a soft per-case-branching smell — that one row wants a different shape (event-shape verification, not just counts). Body has `if tt.check != nil { tt.check(t, events) }`.

**Fix:** extract the event-content assertion into a dedicated test (`TestStream_PreservesEventOrderAndContent`); shrink this table to a uniform `(input, wantMalformed, wantEvents)` shape and drop the `check` field.

---

### 6. [F6] borderline two-row table — `sarif.TestTopFiles_ReturnsFilesSortedByIssueCountDescending`

**Test:** `pkg/sarif/aggregates_external_test.go:66`
**Issue:** `table-one-row` (borderline — 2 rows, threshold is `1`)

The table has exactly 2 cases ("no limit returns all", "positive limit truncates"). Not below threshold for the linter's `one-row` rule, but worth flagging: the two cases differ only in `limit` (0 vs 1) and the truncated `want` slice. A second-case-as-placeholder feel.

**Fix:** acceptable as-is. If a third behavior is added (e.g. `limit > N`), keep the table; otherwise consider two named subtests inline.

---

## Summary

| Tier | Count | Status |
|------|-------|--------|
| P1 per-case-branching | 1 (parser_test.go) + 1 partial (stream_test.go) | yellow |
| P1 one-row | 0 (one borderline) | green |
| P2 name-field-missing | 2 (wrapdiag) | yellow |
| P2 style-inconsistent | 1 (score) | yellow |
| P2 unused-field | 1 partial (stream_test.go) | green |
| P2 1.22 rescope | 0 | green |

Highest leverage: fix F1 (parser_test.go) — splitting that test will make each shape's assertions self-describing and remove the "conditional assertion on zero-value field" pattern. F2/F3/F4 are cheap mechanical fixes.
