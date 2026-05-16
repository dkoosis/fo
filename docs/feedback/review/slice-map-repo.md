# slice-map review — repo

scope: project · mode: report · run: f62c7fc3af14

Surveyed boundary returns, append sites, and sub-slice retention across `cmd/fo`, `pkg/report`, `pkg/sarif`, `pkg/testjson`, `pkg/state`, `pkg/view`. The codebase is generally hygienic: aggregators copy bytes (`pkg/testjson/parser.go:122`), nil-vs-empty is normalized at the JSON boundary (`pkg/state/headline.go:77`), and most accumulator loops preallocate. Findings below are concentrated on a few missed preallocations where the bound is known at loop entry. No P1 boundary-shares-backing or append-aliasing defects were found.

---

### 1. [F1] pkg/testjson/parser.go:401 — missed-prealloc on FailedTests

**Site:** `(*aggregator).results` · `pkg/testjson/parser.go:401`
**Issue:** missed-prealloc
**Mutation impact:** none (allocation churn only)

The outer slice `results` is preallocated (`make([]TestPackageResult, 0, len(a.order))`, line 379), but `r.FailedTests` is grown by append in a loop whose bound `len(pkg.failedOrder)` is known and used right above. Each failing-test batch reallocates 1-3 times depending on size.

```go
r := TestPackageResult{Name: pkg.name, ...}
// Build failed tests list in run order
for _, testName := range pkg.failedOrder {
    r.FailedTests = append(r.FailedTests, FailedTest{
        Name:   testName,
        Output: pkg.outputBuf[testName],
    })
}
results = append(results, r)
```

**Fix:** `r.FailedTests = make([]FailedTest, 0, len(pkg.failedOrder))` before the loop, or build the slice with index assignment.

---

### 2. [F2] pkg/state/headline.go:12 — over-allocated prealloc (minor)

**Site:** `Headline` · `pkg/state/headline.go:12`
**Issue:** missed-prealloc (inverse — over-prealloc, not under)
**Mutation impact:** none

`parts := make([]string, 0, 8)` is fine; flagged only as context. **However**, sibling `EnvelopeOf` (line 60) calls `nonNil` eight times, each branch allocating `[]Item{}` on nil input. Each is one allocation; the eight aggregate to 8 alloc/marshal at zero-diff. Not a defect — documenting the trade-off (LLM/shell parse stability) is in the godoc and outweighs the cost. **No fix.** Listed so the reviewer sees it was considered.

---

### 3. [F3] pkg/sarif/aggregates.go:85 — capacity-retention on truncated TopFiles

**Site:** `TopFiles` · `pkg/sarif/aggregates.go:85`
**Issue:** cap-retention
**Mutation impact:** the trimmed-tail backing array stays alive as long as the caller retains `files`

```go
files := make([]FileIssue, 0, len(byFile))
for _, fi := range byFile { files = append(files, *fi) }
slices.SortFunc(files, ...)
if limit > 0 && len(files) > limit {
    files = files[:limit]   // ← retains cap = len(byFile)
}
return files
```

When `byFile` has thousands of entries and the caller asks for the top 10, the returned slice pins a backing array sized for the full map. `FileIssue` is small (3 ints + string), so absolute waste is modest, but the pattern is exactly the cap-retention shape. Risk grows if the type fattens.

**Fix:** `files = slices.Clip(files[:limit])` or `files = append([]FileIssue(nil), files[:limit]...)`.

---

### 4. [F4] pkg/state/metrics_history.go:101 — capacity-retention on history trim

**Site:** `AppendMetrics` · `pkg/state/metrics_history.go:101`
**Issue:** cap-retention (transient)
**Mutation impact:** none in current call path — the trimmed slice is marshaled and discarded — but defensible code would clip.

```go
hist.Runs = append([]MetricsRun{...}, hist.Runs...)
if len(hist.Runs) > MaxMetricsHistory {
    hist.Runs = hist.Runs[:MaxMetricsHistory]   // ← retains larger backing
}
data, err := json.MarshalIndent(hist, "", "  ")
```

Today the function returns immediately after the write, so retention is irrelevant. Flagged because the trimming pattern is repeated below (line 87 returns `hist.Runs[0].Samples`, also a sub-slice of file-backed history) and the convention should be uniform.

**Fix:** `hist.Runs = slices.Clip(hist.Runs[:MaxMetricsHistory])`, or accept current code with a comment.

---

### 5. [F5] pkg/state/metrics_history.go:87 — boundary returns sub-slice into MetricsFile

**Site:** `LoadMetrics` · `pkg/state/metrics_history.go:87`
**Issue:** boundary-shares-backing
**Mutation impact:** caller mutating returned `[]MetricSample` mutates `hist.Runs[0].Samples`; since `hist` is local and dropped after the call this is harmless, but the contract is implicit.

```go
return hist.Runs[0].Samples, nil
```

Today `hist` is unreferenced after return, so the slice effectively owns its backing. Future refactor (caching `hist` in a struct, returning runs from cached state) would silently expose the sub-slice. The convention rule `nil`-or-clone keeps the contract resilient.

**Fix:** document "callers may mutate" or `return slices.Clone(hist.Runs[0].Samples), nil`. Document is cheaper here.

---

### 6. [F6] pkg/state/diff.go:157 — older runs sub-slice

**Site:** `priorRuns` · `pkg/state/diff.go:157`
**Issue:** boundary-shares-backing (low risk, internal)
**Mutation impact:** `older` is `prev.Runs[1:]`; consumers `isFlaky` / `isTestFlaky` only read. Internal package-private function with two known callers; safe today.

```go
older = prev.Runs[1:]
```

No fix needed — flagged for awareness. If a future caller appends to `older`, it will overwrite `prev.Runs`'s tail (cap is preserved). The full-slice expression `prev.Runs[1:len(prev.Runs):len(prev.Runs)]` would protect against that at zero runtime cost.

---

### 7. [F7] pkg/view/pickview.go:324 — append-into-map without prealloc (acceptable)

**Site:** `groupFindingsByPackage` · `pkg/view/pickview.go:320-326`
**Issue:** missed-prealloc on map-value slice (deliberate)
**Mutation impact:** none

```go
out := make(map[string][]report.Finding)
for _, f := range fs {
    key := packageOf(f.File)
    out[key] = append(out[key], f)
}
```

The per-key bound isn't known up-front, so this is the canonical Go pattern. **No fix.** Listed because it appears in the rg sweep; explicitly approving so future audits don't relitigate.

---

### 8. [F8] cmd/fo/main.go:744-760 — repeated append on merged.Findings without prealloc

**Site:** `parseSections`/multiplex merge · `cmd/fo/main.go:741-763`
**Issue:** missed-prealloc
**Mutation impact:** none

`merged := &report.Report{Tool: "multi"}` starts both `Findings` and `Tests` nil; the loop then appends one-or-more findings per section plus `sub.Findings...`. Section count is `len(sections)` and is known. With ~10+ sections the slice reallocates 3-4 times.

```go
merged := &report.Report{Tool: "multi"}
for _, sec := range sections {
    if f, ok := sectionStatusFinding(sec); ok {
        merged.Findings = append(merged.Findings, f)
    }
    ...
    merged.Findings = append(merged.Findings, sub.Findings...)
    merged.Tests = append(merged.Tests, sub.Tests...)
}
```

**Fix:** approximate prealloc — `merged.Findings = make([]report.Finding, 0, len(sections))` (per-section sub-counts unknown, but the section-status finding alone is bounded by `len(sections)`).

---

### 9. [F9] pkg/view/pickview.go:343 — sortedKeys is correctly preallocated

**Site:** `sortedKeys` · `pkg/view/pickview.go:340-347`
**Issue:** none (positive control)
**Mutation impact:** none

Flagged only to confirm the audited "iterate-map-into-slice" pattern is preallocated here, demonstrating the codebase knows the idiom. The F1/F8 misses are inconsistencies with this baseline, not architectural blind spots.

---

### 10. [F10] pkg/testjson/parser.go:230 — appendCapped sentinel adds without bound check on cap

**Site:** `appendCapped` · `pkg/testjson/parser.go:223-233`
**Issue:** none — defensible
**Mutation impact:** none

```go
if used+add > maxPerTestOutputBytes {
    return append(buf, truncationSentinel), maxPerTestOutputBytes + 1
}
return append(buf, line), used + add
```

The slice grows naturally; the cap is on **bytes**, not slots. The function is single-writer per test-name (no aliasing). Listed to confirm the bytes-cap mechanism doesn't have a slot-cap leak. **No fix.**

---

## Summary

| Tier | Issue | Findings | Verdict |
|------|-------|----------|---------|
| P1 | boundary-shares-backing | F5, F6 | 🟡 — both low risk, internal/transient |
| P1 | append-aliases | (none) | 🟢 |
| P1 | cap-retention | F3, F4 | 🟡 — F3 worth fixing; F4 cosmetic |
| P2 | nil-vs-empty | (none — handled in nonNil) | 🟢 |
| P3 | missed-prealloc | F1, F8 | 🟡 — known bounds available |

**Recommended actions, in priority order:** F1 (clear win), F3 (defensive clip), F8 (defensible prealloc). F4/F5/F6 are conventions to adopt if/when the surrounding code is touched. F2/F7/F9/F10 are positive controls — no action.
