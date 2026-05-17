# slice-map review — fo repo

Run ID: `bd775e303d86-slice-map`
Date: 2026-05-17
Target: whole repo
Scope: read-only

---

### 1. [F1] `pkg/testjson/parser.go:396` — boundary-returns-internal-backing

**Site:** `(*aggregator).results` → `TestPackageResult.PanicOutput`
**Issue:** `boundary-shares-backing`
**Mutation impact:** `aggregator.Results()` is part of the public streaming API (`Aggregator` type alias + `NewAggregator` + `ProcessEvent`). After a snapshot, the aggregator may continue receiving events; `appendCapped` on `pkg.panicOutput` will potentially mutate the same backing array exposed to the caller's `TestPackageResult.PanicOutput`. Snapshot consumers see surprising late mutations.

```go
r := TestPackageResult{
    ...
    PanicOutput: pkg.panicOutput,   // shared slice — caller + producer alias
}
// then later, ProcessEvent → appendCapped(pkg.panicOutput, ...) may grow in-place
```

**Fix:** `PanicOutput: append([]string(nil), pkg.panicOutput...)` (or `slices.Clone`). Same pattern needed at line 403 for `Output: pkg.outputBuf[testName]` — that map's value slices are also live and mutated by subsequent events.

**Tier:** 🟡

---

### 2. [F2] `pkg/testjson/parser.go:403` — boundary-returns-internal-backing

**Site:** `(*aggregator).results` → `FailedTest.Output`
**Issue:** `boundary-shares-backing`
**Mutation impact:** Same root cause as F1 — `FailedTest.Output` is bound directly to `pkg.outputBuf[testName]`, which is the live `[]string` that `appendCapped` continues to extend (or in-place mutate the truncation sentinel slot) for any further events on the same test name. Read F1 + F2 together: both leak the aggregator's mutable internal state across the snapshot boundary.

```go
for _, testName := range pkg.failedOrder {
    r.FailedTests = append(r.FailedTests, FailedTest{
        Name:   testName,
        Output: pkg.outputBuf[testName],   // shared with pkg
    })
}
```

**Fix:** Clone per slot: `Output: append([]string(nil), pkg.outputBuf[testName]...)`.

**Tier:** 🟡

---

### 3. [F3] `pkg/sarif/aggregates.go:88` — boundary-returns-internal-backing

**Site:** `GroupByFile` → `GroupedResults.Results`
**Issue:** `boundary-shares-backing`
**Mutation impact:** `Results: byFile[file]` is the very `[]Result` accumulated via `append` into the temporary `byFile` map. The map is local so no future producer mutation, but callers receive a slice whose `cap` may exceed `len` (typical `append` doubling). A consumer doing `groups[i].Results = append(groups[i].Results, extra)` could clobber an adjacent group's backing array if it shares capacity — not possible here because each file got its own backing via map-key isolation, but the open-ended `Results []Result` invites caller-side `append` mistakes when callers re-export the slice.

```go
groups = append(groups, GroupedResults{
    Key:     file,
    Results: byFile[file],          // exposes cap-extended slice
})
```

**Fix:** `Results: append([]Result(nil), byFile[file]...)` or use `slices.Clip(byFile[file])` to bound capacity to length. Cheap because this only runs once per render.

**Tier:** 🟢

---

### 4. [F4] `pkg/state/metrics_history.go:88` — boundary-returns-internal-backing

**Site:** `LoadMetrics` → `[]MetricSample`
**Issue:** `boundary-shares-backing`
**Mutation impact:** Returns `hist.Runs[0].Samples` straight from the deserialized envelope. `hist` is local and dropped, so producer-side mutation is impossible. However, the godoc says callers "only care about the latest snapshot." If the caller appends to the returned slice, it can spill into the (now-discarded) `Runs[0]` struct's residual capacity — harmless functionally but the contract is unclear. The doc neither labels read-only nor copies.

**Fix:** Document "callers MUST NOT append/mutate" OR `return slices.Clone(hist.Runs[0].Samples), nil`. Tiny slice, copy is free.

**Tier:** 🟢

---

### 5. [F5] `cmd/fo/suppress_cmd.go:105` — reset-vs-realloc

**Site:** `runSuppressAdd` — `filtered := existing[:0]`
**Issue:** `reset-vs-realloc` / `append-aliases-shared-backing`
**Mutation impact:** `filtered := existing[:0]` reuses the backing array of `existing` (returned from `loadFile`). The subsequent `for _, s := range existing` iterates the SAME backing array that `append(filtered, s)` writes into. Today this is safe because the loop only ever drops elements (filtered len ≤ existing index at any point) and reads each `s` by value before the write. But it's the textbook setup for the "filter-in-place corrupts iterator" footgun — fragile under future edits (e.g., expanding to keep one rule plus a derived synthetic rule would clobber the next read).

```go
filtered := existing[:0]
for _, s := range existing {        // reads from same backing array
    if s.RuleID != rule.RuleID {
        filtered = append(filtered, s)   // writes into same backing array
    }
}
filtered = append(filtered, rule)   // may write into existing[len(filtered)]
```

**Fix:** Allocate explicitly: `filtered := make([]suppress.Suppression, 0, len(existing)+1)`. Suppress files are tiny; the allocation is free and the read/write aliasing risk disappears.

**Tier:** 🟢

---

### 6. [F6] `pkg/state/headline.go:79` — nil-vs-empty-mixed-returns

**Site:** `nonNil` documents the convention but `Diff` itself doesn't
**Issue:** `nil-vs-empty-mixed-returns`
**Mutation impact:** `nonNil` exists precisely because upstream `Diff` fields are inconsistently `nil` vs `[]Item{}` — JSON marshals differently and the codebase has had to add a normalization helper at the envelope boundary. The underlying producers (`pkg/state/diff.go`) still mix conventions: `d.New = append(d.New, ...)` starts from `nil` and yields `[]Item{}` only after the first append; an empty diff leaks `nil` through. The asymmetry is contained today by `EnvelopeOf`, but any other consumer of `Diff` directly (e.g., a future caller skipping the envelope) inherits the inconsistency.

**Fix:** Either pre-initialize all `Diff` slice fields to `make([]Item, 0)` in the constructor, OR add a godoc on `Diff` stating "all slice fields may be nil; callers MUST use `EnvelopeOf` or treat nil as empty." Lean toward documenting — the envelope is the established boundary.

**Tier:** 🟢

---

## Summary

- 2 🟡 boundary findings in `pkg/testjson` (F1, F2) — same root cause; the public `Aggregator.Results()` API leaks its internal `panicOutput` / `outputBuf` slices that `appendCapped` continues to mutate after a snapshot. Subtle because `ParseStream` happens to discard the aggregator immediately, masking the bug. Streaming consumers (the use case the public API was added for) will hit it.
- 4 🟢 lower-severity (F3-F6) — capacity exposure, undocumented contracts, in-place filter that's currently safe but fragile, nil-vs-empty leakage outside the envelope.
- No 🔴 findings. No append-aliasing-active bugs, no capacity-retention leaks pinning large buffers, no map-grow-during-iter. `copyBytes` (`pkg/testjson/parser.go:122`) and the explicit `Members: append([]string(nil), g.Members...)` clone at `pkg/testjson/toreport.go:164` show the codebase knows the pattern — F1/F2 are the missed instances of the same idiom.

Counts by issue: `boundary-shares-backing` 4 · `reset-vs-realloc` 1 · `nil-vs-empty-mixed-returns` 1.
