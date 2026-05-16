# conversion-drift — repo

run_id: f62c7fc3af14
scope: project (requested) — note: linter is diff-scoped by design;
HEAD == origin/main so no PR diff exists. Reviewed the recent
boundary-touching commit range `83d29af~1...HEAD` (fo-u15.1.1
through fo-u15.1.3 — watch + state-sidecar work). This is the most
recent serialization-adjacent change set and the appropriate target
for a project-scope conversion-drift sweep.

Commits in range:
- cb99728 chore: close fo-u15.1.3
- 4bef86a feat(state): track test-outcome deltas (A.3)
- dcf45d5 docs(rules): note fsnotify dep added by fo-u15.1.2
- ded9ee3 feat(watch): fsnotify watcher + debounce (A.2)
- 83d29af fix(state): metrics-history accumulates runs (#258)

Files in scope (boundary-adjacent):
- pkg/state/state.go      (Run shape — sidecar persistence)
- pkg/state/diff.go       (Diff shape — Item construction from Run)
- pkg/state/headline.go   (Envelope — JSON wire shape for renderers)
- pkg/state/metrics_history.go (legacy flat → versioned envelope)
- pkg/report/report.go    (DiffSummary — public IR for renderers)
- cmd/fo/state.go         (envelope → DiffSummary projection)

---

### 1. [F1] `RunFromReport` — pass and skip conflated in persisted Tests map

**Helper:** `pkg/state/state.go:208-244` `RunFromReport`

**Old behavior:** `Run.Tests` did not exist. Test outcomes were not persisted at all.

**New behavior:** Only Fail/Panic/BuildError are persisted; **Pass and Skip are deliberately collapsed into "absent"**. Doc string says "absence means pass-or-skip for diff purposes."

**Risk:** This is a documented drift, but `FixedFailures` classification (`diff.go:106-110`) treats *any* absent test in the current run as a fix. A test that was failing yesterday, then skipped today (e.g. behind a build tag, on a different platform, gated off), will be reported as **FixedFailures** despite never actually being verified. Real-world failure mode: CI matrix flips a job from active to skipped — a green "1 fixed" banner appears, masking unverified regressions.

**Affected callers:** every site that constructs a Run from a Report — `pkg/state/state.go` (Append path), `pkg/state/diff.go:Classify` consuming it, `cmd/fo/watch.go` invoking the chain.

**Code:**
```go
// new in state.go:228-238
case report.OutcomeFail, report.OutcomePanic, report.OutcomeBuildError:
    tests[testKey(tr.Package, tr.Test)] = string(tr.Outcome)
case report.OutcomePass, report.OutcomeSkip:
    // passing and skipped tests are not stored
```

**Fix:** Either (a) persist skipped tests under a sentinel outcome (`"skip"`) so `FixedFailures` only fires on Pass; or (b) document the gap explicitly and add a regression test for the "fail → skip" transition asserting **no** FixedFailure is reported.

---

### 2. [F2] `Severity(outcome)` type confusion in `makeTestItem`

**Helper:** `pkg/state/diff.go:143-150` `makeTestItem`

**Old behavior:** `Item.Severity` was a `state.Severity` value derived from `report.Severity` via `severityFromReport` — domain values `"error"|"warning"|"note"|"info"`.

**New behavior:** Test items reuse the `Severity` field to hold the test outcome string verbatim: `Severity: Severity(outcome)` where `outcome` is `"fail"|"panic"|"build_error"`.

**Risk:** Same field, two domains. Any consumer that switches on `Item.Severity` (renderers in `pkg/view/`, future tally aggregators) now sees out-of-domain strings for test rows and either drops them silently or paints them as unknown. Roundtrip is asymmetric: marshal writes `"build_error"` into the `severity` JSON field; unmarshal accepts it but no code expects it.

**Affected callers:** any view that filters/groups Items by severity. `cmd/fo/state.go:convertItems` round-trips Severity into `report.Severity` for `DiffItem` — a `report.Severity("build_error")` is invalid per pkg/report contract.

**Code:**
```go
// diff.go:147
return Item{
    Fingerprint: key,
    RuleID:      test,
    File:        pkg,
    Severity:    Severity(outcome),  // <-- domain leak
    Class:       c,
}
```

**Fix:** Add a separate `Item.TestOutcome string` field, or normalize to `Severity("error")` for test rows and stash the outcome in a tag/metadata field. Add a roundtrip test asserting marshaled Item.Severity is in the Severity domain.

---

### 3. [F3] `testKey` parse asymmetry — subtests with `/` corrupt File/RuleID

**Helper:** `pkg/state/state.go:209-214` `testKey` encode vs `pkg/state/diff.go:131-141` decode in `makeTestItem`.

**Old behavior:** No key encoding existed.

**New behavior:** Encode: `pkg + "/" + test`. Decode: scan for the **last** `/` and split there.

**Risk:** Go subtests are `TestX/sub` and packages contain `/` (`github.com/foo/bar`). Encoded key `github.com/foo/bar/TestX/sub` decodes to `File="github.com/foo/bar/TestX"`, `RuleID="sub"`. The encode/decode pair is not a bijection. Renderers showing "package: github.com/foo/bar, test: TestX/sub" will mislabel every subtest result.

**Affected callers:** any renderer that shows `Item.File`/`Item.RuleID` for test rows — currently latent (no renderer wired yet at HEAD) but the watch loop persists corrupted strings.

**Code:**
```go
// encode (state.go:211-213)
return pkg + "/" + test
// decode (diff.go:134-140) — scans for *last* "/"
for j := len(key) - 1; j >= 0; j-- {
    if key[j] == '/' { pkg = key[:j]; test = key[j+1:]; break }
}
```

**Fix:** Use a separator that cannot appear in either side (e.g. `"\x00"` or `"::"`), or store `Package` and `Test` as distinct fields on Item rather than recomputing them. Add a roundtrip test with `Package="github.com/foo/bar"`, `Test="TestX/sub"`.

---

### 4. [F4] `LoadMetricsHistory` legacy migration fabricates GeneratedAt

**Helper:** `pkg/state/metrics_history.go:53-77` `LoadMetricsHistory`

**Old behavior:** `LoadMetrics` returned a flat `[]MetricSample` — no timestamp at all.

**New behavior:** Legacy flat files are migrated into a single `MetricsRun{GeneratedAt: time.Now().UTC(), Samples: legacy}`. The synthetic timestamp pretends old data was captured "now."

**Risk:** Sparkline/trend renderers (the stated motivation for `MaxMetricsHistory=30`) will mis-attribute the legacy sample to the current moment. Time-axis plots will show a flat line followed by a jump at "now," obscuring the actual age of the previous datapoint.

**Affected callers:** `LoadMetricsHistory` is called by `LoadMetrics` (newest-sample only — unaffected) and by any future trend renderer that reads `hist.Runs[i].GeneratedAt`.

**Code:**
```go
// metrics_history.go:73-76
return &MetricsFile{
    Version: MetricsSchemaVersion,
    Runs:    []MetricsRun{{GeneratedAt: time.Now().UTC(), Samples: legacy}},
}, nil
```

**Fix:** Use `time.Time{}` (zero) for legacy migration and document that callers must treat zero as "unknown age." Or skip the migration entirely — discard legacy flat data, log once, and start fresh — which preserves trend integrity at the cost of one lost datapoint.

---

### 5. [F5] `state.File.SchemaVersion` not bumped despite adding `Tests` field

**Helper:** `pkg/state/state.go:23` `SchemaVersion` (still `1`) + new `Run.Tests` field at `state.go:69`.

**Old behavior:** Run = `{GeneratedAt, DataHash, Findings}`. Schema v1.

**New behavior:** Run = `{GeneratedAt, DataHash, Findings, Tests}`. Still schema v1.

**Risk:** Forward-compatible (old binaries reading new files drop the unknown field), but a user who downgrades fo silently loses test-outcome history without any warning. Mirrors the pattern `MetricsSchemaVersion` does correctly. Also: a future incompatible Tests reshape will have no version to gate on.

**Affected callers:** sidecar load path `pkg/state/state.go:Load`, version check at `state.go:105`.

**Code:**
```go
// state.go:23
const SchemaVersion = 1   // unchanged
// state.go:65-70
type Run struct {
    GeneratedAt time.Time           `json:"generated_at"`
    DataHash    string              `json:"data_hash,omitempty"`
    Findings    map[string]Severity `json:"findings"`
    Tests       map[string]string   `json:"tests,omitempty"`  // NEW
}
```

**Fix:** Bump `SchemaVersion` to 2; document v1→v2 read path (Tests defaults to empty map). Or document explicitly that additive-only changes don't bump the version, and gate future field additions behind feature-version detection.

---

### 6. [F6] `DiffSummary` vs `Envelope` JSON shape asymmetry on test diffs

**Helper:** `pkg/report/report.go:101-103` (DiffSummary) vs `pkg/state/headline.go:54-56` (Envelope).

**Old behavior:** Pre-existing fields `New/Resolved/Regressed/Flaky` had matching `omitempty` policy across both structs.

**New behavior:** Both structs grew `NewFailures`/`FixedFailures`/`FlakyTests`, but with **divergent** JSON tags:
- `report.DiffSummary`: `json:"new_failures,omitempty"` (omits when empty)
- `state.Envelope`: `json:"new_failures"` (always emits, force non-nil via `nonNil()`)

**Risk:** Two on-wire shapes for the same logical payload. Consumers parsing `--format json` may see the field present (from Envelope path) or absent (from DiffSummary path) depending on which struct serialized. Schema docs (`pkg/report/report.schema.json`) will desync.

**Affected callers:** `cmd/fo/state.go:envelopeToDiffSummary` bridges Envelope→DiffSummary; any JSON renderer downstream.

**Code:**
```go
// report/report.go:101-103
NewFailures   []DiffItem `json:"new_failures,omitempty"`
FixedFailures []DiffItem `json:"fixed_failures,omitempty"`
FlakyTests    []DiffItem `json:"flaky_tests,omitempty"`

// state/headline.go:54-56
NewFailures   []Item `json:"new_failures"`
FixedFailures []Item `json:"fixed_failures"`
FlakyTests    []Item `json:"flaky_tests"`
```

**Fix:** Decide one policy (the pre-existing finding diffs use non-omitempty + `nonNil()`, which is the stable choice). Align DiffSummary tags to match Envelope. Update report.schema.json.

---

### 7. [F7] `AppendMetrics` file-mode tightened 0o644 → 0o600 silently

**Helper:** `pkg/state/metrics_history.go:AppendMetrics` (write at the bottom of the file).

**Old behavior:** `SaveMetrics` wrote with `0o644` — world-readable.

**New behavior:** `AppendMetrics` writes with `0o600` — owner-only.

**Risk:** Low for a single-user CLI, but: (a) Go's `os.WriteFile` only sets perms on **create**; existing 0o644 files retain old perms — the security goal isn't met for upgraders. (b) If a downstream tool (CI runner, file collector) reads the sidecar as a different user, it now breaks silently. This is a precondition-tightened change with no migration path.

**Affected callers:** every user who already has a `.fo/metrics-history.json` from the prior fo version.

**Code:**
```go
// was:
if err := os.WriteFile(path, data, 0o644); err != nil { ... }
// now:
if err := os.WriteFile(path, data, 0o600); err != nil { ... }
```

**Fix:** Add an explicit `os.Chmod(path, 0o600)` after write to actually enforce the new mode on pre-existing files; or document the change in the upgrade notes; or revert if not deliberate.

---

### 8. [F8] No roundtrip test for `Run` JSON with the new `Tests` field

**Helper:** `pkg/state/state.go` `Run` (Marshal/Unmarshal via the sidecar Load/Save path).

**Old behavior:** `Run` had Findings-only; the existing Load/Save tests covered roundtrip.

**New behavior:** Added `Tests map[string]string` field. No new test asserts that a saved `Run.Tests` survives Load → Classify intact. `TestRunFromReport_CapturesTestFailures` covers only the Report→Run projection, not the JSON roundtrip.

**Risk:** Nil-vs-empty-map distinction (`Tests` is `omitempty`; nil map and empty map both omit; but reload produces nil — downstream code must tolerate nil maps when ranging). Bucket-level: if `Tests` ever changes to a richer struct, the absence of a roundtrip baseline test makes the migration invisible.

**Affected callers:** sidecar Load path; Classify reads `prev.Tests` and `older[].Tests`.

**Fix:** Add `TestRun_TestsRoundtrip` in `pkg/state/state_test.go`: build a `Run` with Tests populated, marshal, unmarshal, assert equality including the subtest-with-slash case from F3.

---

### 9. [F9] `isTestFlaky` ignores `older` outcome — counts skipped as "was failing"

**Helper:** `pkg/state/diff.go:114-121` `isTestFlaky`.

**Old behavior:** N/A (new function).

**New behavior:** Reports flaky if the key appears in any older `Run.Tests` map.

**Risk:** Coupled with F1 (Tests stores only failures, skips are absent), `isTestFlaky` is correct *today*. But if F1 is fixed by storing skips under a sentinel (recommended), this function becomes wrong — it would treat "was-skipped, now-failing" as flaky. The two helpers' contracts are entangled by an unwritten invariant (only-failures-in-the-map).

**Affected callers:** `classifyTests` only.

**Fix:** When fixing F1, update `isTestFlaky` to check the outcome string, not just presence. Add a contract comment on `Run.Tests` ("values are always failing outcomes") so the invariant survives future edits.

---

### 10. [F10] Watch-loop drives Classify on every event — no debounce of conversion cost

**Helper:** Not a conversion helper per se, but the call chain `fswatch event → RunFromReport → Classify → Envelope → JSON write` runs on every fs event in `cmd/fo/watch.go`.

**Old behavior:** State sidecar updated once per CLI invocation.

**New behavior:** With fsnotify watch (A.2/A.3), the same Report→Run→Diff conversion now runs on a debounced event loop. Each pass mutates `metrics-history.json` (appending a new run, trimming oldest), so a 30-event burst can churn the history file 30×.

**Risk:** Lower-tier (no correctness drift), but the conversion pipeline was designed for one-shot CLI use and is now hit at watch frequency. If `RunFromReport` ever becomes lossy (e.g. F1's skip-conflation propagates), the lossy path runs many times per minute and the sidecar drifts faster than a human reviewer can audit.

**Fix:** Confirm debounce window in watch loop is conservative (≥500ms); add a fast-path in `AppendMetrics` that no-ops when the new run is byte-identical to `Runs[0]`.

---

## Summary

- 🟡 **P1 helpers changed:** 4 reviewed (`RunFromReport`, `makeTestItem`/`testKey`, `LoadMetricsHistory`, `AppendMetrics`). All on the persistence path → tier 🔴 by spec.
- 🟡 **P1 callsites adapted:** the new `Tests` map flows through `Classify` → `Envelope` → `DiffSummary` → renderers; renderers are not yet wired but the persisted shape is durable.
- 🟢 **P2 dep bump:** `fsnotify` added (commit ded9ee3) — not a serialization-touching dep; no boundary review needed.
- 🟡 **P3 tests:** roundtrip tests for `Run.Tests` JSON missing (F8); subtests-with-slash case missing (F3); fail→skip transition missing (F1).

Highest-leverage single fix: **F3** (subtest key collision) — silent and 100% reproducible the moment a subtest renderer ships.
