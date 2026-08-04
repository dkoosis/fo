# Testability review — whole repo (condensed pass)

RUN_ID: 7858b3a8ea1b
Linter: testability (scope widened from `pkg` to whole repo for this run)

## Method note

Phase 0 gate applied per-package: `cmd/fo` is exempt as the binary entry point
(reduced scope, main.go wiring only — nothing non-wiring found there worth
flagging). No package in the repo matched the adapter-basename list or the
<3-exported-funcs carve-out. `pkg/state` is functionally a persistence
adapter (its own doc-comment: "persists a sidecar record") but is not a
name-list match and is itself invoked directly from `cmd/fo` — I used the
tiebreaker (name-based) judgment for its Phase-2/4 findings below rather than
the literal caller-side test, since fo's `cmd/` layer is deliberately thin
wiring and treating every symbol it calls as "adapter" would exempt the
entire `pkg/` tree from this linter, which is not the intent for a
flat-architecture CLI tool. No `exec`/`http`/`net`/`sql` handles exist
anywhere outside `cmd/fo/watch.go` (exempt, cmd/ scope), so shell/network/db
rule-ids don't fire in this repo at all.

---

### 1. [F1] `pkg/hygiene/hygiene.go:90` + 4 wrapper packages — globals-as-implicit-deps

**Diagnosis:** `hygiene.Scan` and four `pkg/wrapper/*` `Convert` functions write their
oversize-line-dropped warning straight to the global `os.Stderr` instead of
an injected `io.Writer`, unlike the sibling `pkg/wrapper/wrapdiag` package,
which solves the identical problem correctly.

**Why:** `os.Stderr` is a global, non-injectable stream. A caller — or a
test — can't capture, redirect, or suppress the warning without swapping the
real process stderr. `pkg/wrapper/wrapdiag` already carries a
`DiagOpts.Stderr io.Writer` field for exactly this diagnostic, wired through
to a private field and asserted on directly in
`TestDiagConvert_OversizeLineWarnsStderr` via a `bytes.Buffer`. The other
four `Convert` functions have no equivalent test for their own drop-warning
path — the hardcoded `os.Stderr` is why: there's nothing to inject a buffer
into. This is the same defect repeated five times in one repo, with the
fix already checked in next door.

**Evidence:**

```
pkg/hygiene/hygiene.go:90:
    fmt.Fprintf(os.Stderr, "%s: dropped %d line(s) exceeding %d bytes\n", spec.Name, dropped, lineread.MaxLineLen)

pkg/wrapper/wrapleaderboard/wrapleaderboard.go:79:
    fmt.Fprintf(os.Stderr, "wrap leaderboard: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)

pkg/wrapper/wrapcover/wrapcover.go:40  (same pattern)
pkg/wrapper/wrapgobench/wrapgobench.go:51  (same pattern)
pkg/wrapper/wraparchlinttext/wraparchlinttext.go:101  (same pattern)
```

Correct sibling pattern, `pkg/wrapper/wrapdiag/convert.go:10-15`:
```go
type DiagOpts struct {
	...
	Stderr  io.Writer
}
```
— consumed and tested via `pkg/wrapper/wrapdiag/oversize_test.go:34-53`
(`TestDiagConvert_OversizeLineWarnsStderr`, asserts `errBuf.String()` contains
the warning).

**Fix:** Give `hygiene.Spec` a `Stderr io.Writer` field (nil-safe, defaulting
to a no-op or the real `os.Stderr` at the `cmd/fo` call site) and thread the
same field through the four wrapper `Convert`/`Opts` signatures, mirroring
`wrapdiag.DiagOpts.Stderr`. Report-only — this is a one-line-signature change
repeated across 5 files; leave the patch to a human/implementer pass.

**Tier:** action

---

### 2. [F2] `pkg/state/state.go:201` — globals-as-implicit-deps

**Diagnosis:** `Save`'s parent-directory fsync is indirected through a
mutable package-level var (`syncDir`) instead of a constructor-injected
dependency, and the tests that need to swap it are explicitly marked
non-parallel as a direct consequence.

**Why:** This is the textbook shape the rule protects against: tests can't
substitute the dependency without `var`-swap gymnastics, and concurrent
tests interfere with each other. The code's own test file proves it —
`pkg/state/state_test.go:105` and `:137` both carry the comment "Not
parallel: mutates package-level syncDir," and each test does
`orig := syncDir; t.Cleanup(func() { syncDir = orig }); syncDir = func(...) {...}`
to fake the fsync path. A one-method interface passed to `Save` (or a small
`Store` struct — see F4) would let these tests run with `t.Parallel()` like
the rest of the suite.

**Evidence:**

```
pkg/state/state.go:200-201:
	// package var so tests can assert it's invoked with the right path
	// without requiring real fault injection.
	var syncDir = func(dir string) error {

pkg/state/state_test.go:104-111:
func TestSave_FsyncsParentDir(t *testing.T) {
	// Not parallel: mutates package-level syncDir.
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "last.json")
	wantDir := filepath.Join(dir, "sub")

	orig := syncDir
	t.Cleanup(func() { syncDir = orig })
```

**Fix:** Replace the package var with a one-method interface
(`type dirSyncer interface { Sync(dir string) error }`) taken as a parameter
on `Save` (or held as a field once a `Store` type exists, F4) with the
current function as the default/real implementation. Removes the shared
mutable state and the parallel-test restriction. Report-only.

**Tier:** action

---

### 3. [F3] `pkg/sarif/toreport.go:23`, `pkg/testjson/toreport.go:34` — clock-in-policy

**Diagnosis:** `sarif.ToReport` and `testjson.ToReport` stamp
`GeneratedAt: time.Now().UTC()` unconditionally, with no way for a caller to
supply a fixed time — unlike every other place in the repo that produces the
same field.

**Why:** `Report.GeneratedAt` is load-bearing downstream: `pkg/state`'s own
`RunFromReport` and `RunLogEntryFromReport` both read `r.GeneratedAt` and
only fall back to `time.Now().UTC()` when it `IsZero()` — that's the
established, already-shipped pattern for this exact field
(`pkg/state/state.go:274-276`, `pkg/state/runlog.go:52-55`). `ToReport` is
the one place that produces `GeneratedAt` in the first place, and it doesn't
follow its own downstream consumers' convention. No test in either
`toreport_test.go` constructs a `Report` via `ToReport` and inspects
`GeneratedAt` — the only `GeneratedAt` assertions in the repo build `Report`
literals directly (`pkg/report/schema_test.go:73`), so the untested clock
reach is real, not just theoretical.

**Evidence:**

```
pkg/sarif/toreport.go:21-24:
func ToReport(doc *Document) *report.Report {
	r := &report.Report{
		GeneratedAt: time.Now().UTC(),
	}

pkg/testjson/toreport.go:31-35:
func ToReport(results []TestPackageResult) *report.Report {
	r := &report.Report{
		Tool:        "go test",
		GeneratedAt: time.Now().UTC(),
	}
```

Sibling pattern already in the repo, `pkg/state/runlog.go:51-55`:
```go
func RunLogEntryFromReport(r *report.Report) RunLogEntry {
	e := RunLogEntry{At: r.GeneratedAt, Tool: r.Tool, RuleCounts: map[string]int{}}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
```

**Fix:** Accept an optional `now time.Time` (or `now func() time.Time`)
parameter on `ToReport`/`ToReportWithMeta` in both packages, defaulting to
`time.Now().UTC()` only when the caller passes the zero value — same
convention already proven in `pkg/state`. Report-only; touches four exported
signatures across two packages, a human-reviewed API change.

**Tier:** borderline — the field is metadata rather than branch-controlling,
so the practical blast radius is small; flagged because the fix pattern is
already established twice elsewhere in the same repo and the inconsistency
is easy to miss.

---

### 4. [F4] `pkg/state` (whole package) — missing-service-seam

**Diagnosis:** Ten exported free functions (`Load`, `Save`, `Reset`,
`LoadMetricsHistory`, `AppendMetrics`, `LoadRunLog`, `SaveRunLog`,
`SaveFullLog`, `LoadSnapshot`, `SaveSnapshot`) each reach the filesystem
directly, plus the path-resolution helpers (`Dir`, `Path`,
`MetricsHistoryPath`, `RunLogPath`, `FullLogPath`, `SnapshotPath`) and the
`syncDir` var (F2) — all sharing the same underlying resource (the sidecar
directory + its durability/sync behavior) with no struct anywhere in the
package that holds it.

**Why:** This is the Phase-4 heuristic by the book: every infra-reaching
free function in the package shares one dependency set (the resolved
`FO_STATE_DIR`, plus the fsync stub), and nothing groups them. Extracting a
single `Clock` or `Syncer` interface (F2) helps `Save` alone but leaves nine
other functions independently re-deriving `Dir()` and repeating the
open/write/rename/sync dance. A `Store` (or similarly domain-named) struct
holding the resolved dir + sync dependency, with the ten functions becoming
methods, would let a test construct one `Store` with a fake dir + fake
syncer instead of relying on `t.Setenv("FO_STATE_DIR", ...)` (used in
`pkg/state/fulllog_test.go:11,31`) and the `syncDir` package-var swap (F2)
side by side.

**Evidence:**

```
pkg/state/state.go:49   func Dir() string { ... os.Getenv("FO_STATE_DIR") ... }
pkg/state/state.go:57   func Path() string { return filepath.Join(Dir(), "last-run.json") }
pkg/state/state.go:60   func MetricsHistoryPath() string { return filepath.Join(Dir(), "metrics-history.json") }
pkg/state/state.go:101  func Load(path string) (*File, error) { ... os.Open ... }
pkg/state/state.go:144  func Save(path string, f *File) error { ... os.CreateTemp ... }
pkg/state/state.go:201  var syncDir = func(dir string) error { ... }
pkg/state/state.go:215  func Reset(path string) error { ... os.Remove ... }
pkg/state/runlog.go:26  func RunLogPath() string { return filepath.Join(Dir(), "run-log.json") }
pkg/state/fulllog.go:14 func FullLogPath() string { return filepath.Join(Dir(), "full.log") }
pkg/state/snapshot.go:23 func SnapshotPath() string { return filepath.Join(Dir(), "findings.json") }
```

No struct anywhere in `pkg/state/*.go` holds `Dir()`'s resolved value or the
sync dependency — every one of the ten path/IO functions re-resolves or
re-implements it independently.

**Fix:** report-only — file a bd follow-up for the human-pass refactor.
Sketch: `type Store struct { dir string; sync func(string) error }` with
`NewStore(dir string) *Store` (defaulting `dir` via the existing `Dir()`
logic when empty) and `NewTestStore(t *testing.T) *Store` for tests; convert
the ten free functions to methods. Affects every call site in `cmd/fo`
(`state.Load`, `state.Save`, `state.Reset`, `state.AppendMetrics`, etc.) —
multi-file, not a single-finding patch.

**Tier:** borderline — real signal, but this is a bigger redesign than the
other three findings and the current free-function shape works today; a
human should weigh the churn against the parallel-testing payoff (F2) before
committing to it.

---

## Self-reflection

- 4 findings, all evidence read at absolute paths and quoted verbatim.
- No shell/network/db-in-policy candidates exist in this repo outside
  `cmd/fo` (exempt) — those rule-ids don't appear because the repo genuinely
  doesn't reach those handles from policy code.
- Skipped as non-findings after inspection: `pkg/theme.Default`'s
  `os.Getenv("NO_COLOR")` (deliberate documented fix, `renderScene` already
  takes `theme.Theme` as a parameter one level down — the seam already
  exists); `pkg/report.ApplyFilter` and `pkg/state.RunFromReport` /
  `RunLogEntryFromReport` (already follow the correct
  accept-or-default-when-zero clock pattern — cited above as the positive
  precedent for F3); `pkg/view/pipeline_golden_test.go`'s `exec.Command` use
  (legitimate whole-binary golden test, not a pure-rule test needing shell).
