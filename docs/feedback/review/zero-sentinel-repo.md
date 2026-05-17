# zero-sentinel — fo repo

run_id: bd775e303d86-zero-sentinel
date: 2026-05-17
scope: whole repo
findings: 3 (cap 10)

Overall tier: 🟢 — fo treats zero values carefully. `Suppression.Until` is `*time.Time`; `RunFromReport` IsZero-guards before persisting; fo-7jv already hardened the 0001-year deserialization path. The three findings below are latent contract gaps, not active bugs.

---

### 1. [F1] `pkg/report/filter.go:22` — time-zero-as-missing

**Diagnosis.** `ApplyFilter(r, rs, now time.Time)` and `classifyFinding(..., now time.Time)` accept a zero `time.Time` silently. If a caller ever passes `time.Time{}`, the year-0001 reference flips `Suppression.Expired` behaviour across the whole ruleset — every Until-bearing rule reports the opposite of its real state.

**Why.** The function is a pure-data API on the public package — nothing in its signature, doc, or body asserts `!now.IsZero()`. Today all four call sites pass `time.Now()`, but the trap is one refactor away (e.g., a future "evaluate as of report.GeneratedAt" change where GeneratedAt happens to be zero on a parse-failure path).

**Evidence (Read-verified).**
```go
// pkg/report/filter.go:22
func ApplyFilter(r *Report, rs *suppress.Ruleset, now time.Time) FilterStats {
    ...
    activeIdx, expiredIdx := classifyFinding(rs, f.RuleID, f.File, now)

// pkg/report/filter.go:55-66
func classifyFinding(rs *suppress.Ruleset, ruleID, file string, now time.Time) (activeIdx, expiredIdx int) {
    ...
    if rs.Rules[i].Expired(now) { ... }

// pkg/suppress/suppress.go:50-59 — Expired computes today from now.UTC().Date();
// zero now yields today=0001-01-01, after which today.After(*Until) is false
// for any sane Until, silently extending every suppression past its real expiry.
```

**Fix.** Either (a) add a defensive default at the top of `ApplyFilter`: `if now.IsZero() { now = time.Now().UTC() }` (preserves the lenient contract), or (b) rename the parameter `evalAt` and document the precondition; in tests, assert `!evalAt.IsZero()`. (a) is the safer choice for a public-package function.

**Tier.** 🟡 — latent, not active. Public API surface raises the cost of a future mistake.

---

### 2. [F2] `pkg/sarif/types.go:86-89` `pkg/report/report.go:37` — optional-value-without-pointer

**Diagnosis.** `Region.StartLine/StartColumn/EndLine/EndColumn` and `Finding.Line/Col` are plain `int` with `omitempty`. Zero means both "absent from source" and "line 0" — but line 0 is not a valid source location. The SARIF spec uses absence-of-field to mean "no precise location"; with `omitempty + int`, fo cannot distinguish a tool that emitted `startLine: 0` (malformed) from one that omitted the field.

**Why.** `Result.Line()` (pkg/sarif/types.go:97) returns `r.Locations[0].PhysicalLocation.Region.StartLine` directly. Downstream renderers display `file:0` as if it were a real location. When a new wrapper (jscpd, archlint) is added and forgets to set StartLine, the rendered output silently shows `file:0:0:` — the reader can't tell whether the tool failed to locate the finding or pointed at the file header.

**Evidence (Read-verified).**
```go
// pkg/sarif/types.go:86-89
type Region struct {
    StartLine   int `json:"startLine,omitempty"`
    StartColumn int `json:"startColumn,omitempty"`
    ...
}

// pkg/sarif/types.go:93-98
func (r *Result) Line() int {
    if len(r.Locations) == 0 { return 0 }
    return r.Locations[0].PhysicalLocation.Region.StartLine
}

// pkg/report/report.go:37-38
Line  int `json:"line,omitempty"`
Col   int `json:"col,omitempty"`
```

**Fix.** Either treat 0 as "absent" by convention everywhere (document in `report.Finding` and in `pkg/sarif/types.go`; renderers must skip `file:0` formatting), or change to `*int` at the IR boundary. The lighter fix is documentation + a render-side guard: when Line==0, print `file` without a `:line` suffix.

**Tier.** 🟡 — production path (every SARIF wrapper hits it) but cosmetic, not a correctness bug.

---

### 3. [F3] `pkg/testjson/types.go:72-80` — optional-value-without-pointer (Status semantics)

**Diagnosis.** `TestPackageResult.Status()` returns `StatusPass` when `Passed == Failed == Skipped == 0`. A zero-test package (e.g., `-run` pattern matched nothing, or a package built but no tests ran) is indistinguishable from a one-pass-zero-fail success. The reader sees "PASS" for a package that ran nothing.

**Why.** `TotalTests()` exists but `Status()` doesn't consult it. The view layer relies on `Status()` to pick the green checkmark vs. red X. A package whose tests were all filtered out renders as a clean pass, masking a likely user mistake.

**Evidence (Read-verified).**
```go
// pkg/testjson/types.go:72-80
func (r *TestPackageResult) Status() Status {
    if r.BuildError != "" || r.Panicked || r.Failed > 0 {
        return StatusFail
    }
    if r.Passed == 0 && r.Skipped > 0 {
        return StatusSkip
    }
    return StatusPass        // ← also returned when Passed==Failed==Skipped==0
}
```

**Fix.** Add an explicit branch: `if r.Passed == 0 && r.Failed == 0 && r.Skipped == 0 { return StatusSkip }` (or introduce a `StatusEmpty` if the renderer wants to distinguish "no tests" from "all skipped"). Aligns with `go test`'s own `ok pkg [no test files]` convention.

**Tier.** 🟡 — affects reader interpretation in a legitimate edge case.

---

## Not flagged (positive controls)

- `pkg/state/state.go:241-244` — RunFromReport guards `r.GeneratedAt.IsZero()` before persisting. ✓
- `pkg/suppress/suppress.go:43` — `Until *time.Time` correctly nullable. ✓
- `pkg/suppress/suppress.go:173-175` — explicit `t.Year() <= 1` reject in Parse (fo-7jv hardening). ✓
- `pkg/state/metrics_history.go:73,100` — GeneratedAt populated unconditionally on append. ✓
- `pkg/report/report.go:84` `Report.GeneratedAt` as plain `time.Time` — populated unconditionally by every ToReport path (`pkg/sarif/toreport.go:23`, `pkg/testjson/toreport.go:34`), so absent-vs-zero ambiguity cannot arise. ✓
- No `uuid` library in use; UUID-shaped strings are regex-normalized, not used as IDs. ✓
- No raw map-index-then-deref patterns in non-test code. ✓

## Don't-flag exclusions applied

- `pkg/testjson/types.go:37` `TestEvent.Time` — populated by `go test -json`; never consumed downstream where zero would change behaviour.
- `pkg/testjson/types.go:53` `Coverage float64` — currently unused by renderers; flag deferred until a consumer treats 0.0 specially.
- All `if s == ""` checks reviewed are guards on user input or parser tokens where empty == absent is the domain convention.
