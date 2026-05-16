# slice-map review — diff 482cfd360229

Scope: 18 files under cmd/fo, pkg/cluster, pkg/report, pkg/sarif, pkg/scene, pkg/suppress, pkg/testjson, pkg/view, pkg/wrapper/wraparchlint. Files `pkg/testjson/stats.go` and `pkg/wrapper/wraparchlint/convert.go` listed in the target do not exist in the worktree; skipped.

**Verdict: 🟢 mostly clean.** No critical defects. A handful of P2/P3 cap-retention and consistency notes worth a follow-up. The in-place filter in `report.ApplyFilter` is the only finding with practical impact (watch-mode re-runs accumulate retained tail entries on `r.Findings` backing arrays); everything else is hygiene-grade.

Tier summary:

| Rule | Count |
|---|---|
| boundary-returns-internal-backing | 2 (P2) |
| cap-retention | 2 (P2/P3) |
| nil-vs-empty-mixed-returns | 1 (P3) |
| append-aliases | 0 |
| map-mutate-during-iter | 0 |
| missed-prealloc | 0 |

### 1. [F1] `pkg/report/filter.go:29-54` — capacity-retention-leak (in-place filter)

**Site:** `report.ApplyFilter`
**Issue:** cap-retention / reset-vs-realloc

`kept := r.Findings[:0]` then re-assigns `r.Findings = kept`. The tail slots `r.Findings[len(kept):cap(kept)]` still hold the original `Finding` struct values (each carrying `RuleID`, `File`, `Message`, `Fingerprint` strings). Those strings stay reachable through the backing array until the whole `Report` is dropped.

**Mutation impact:** In `fo watch` mode where Reports are re-rendered across runs and may be held briefly by the renderer, suppressed findings' strings stay pinned. Not a leak that grows unbounded (bounded by max findings in a single run), but it's a silent surprise for callers expecting filter to release memory. Also: any future code that appends to `r.Findings` after filter will silently overwrite suppressed-finding slots in shared backing — harmless here (no other aliases), but fragile.

**Code:**
```go
kept := r.Findings[:0]
for _, f := range r.Findings {
    idx := rs.Match(f.RuleID, f.File)
    if idx < 0 { kept = append(kept, f); continue }
    // ...
    stats.Total++
}
r.Findings = kept
```

**Fix:** zero the tail so the GC can reclaim string memory held by struct-valued elements:
```go
clear(r.Findings[len(kept):])  // Go 1.21+
r.Findings = kept
```
Or, if the intent is "produce a fresh slice", `r.Findings = slices.Clone(kept)` after the loop. The `clear` form keeps the cheap in-place path.

### 2. [F2] `pkg/sarif/aggregates.go:84-90` — boundary-returns-internal-backing (map-stored slice)

**Site:** `sarif.GroupByFile`
**Issue:** boundary-returns-internal-backing

`GroupedResults.Results: byFile[file]` returns the exact slice stored in the local map. The map itself dies when the function returns, but each returned `GroupedResults.Results` shares its backing with the `byFile` slot (which has unused tail capacity). Callers appending to `Results` may write into that backing.

**Mutation impact:** Today, no other reference to those slices survives, so append-into-shared-backing is moot. The concern is forward-compatibility: this is the only spot in the file that does NOT do a defensive copy (compare `TopFiles` line 46 which copies `*fi` by value). If a future refactor caches `byFile` or returns it alongside groups, the aliasing becomes real.

**Code:**
```go
groups := make([]GroupedResults, 0, len(byFile))
for _, file := range order {
    groups = append(groups, GroupedResults{
        Key:     file,
        Results: byFile[file],   // shared with map's backing
    })
}
```

**Fix:** either godoc-document "Results aliases byFile; do not mutate", or clone:
```go
Results: append([]Result(nil), byFile[file]...),
```
Given the function is one-shot and the map dies, doc-only is acceptable.

### 3. [F3] `pkg/scene/scene.go:146-160` — capacity-retention-leak (flushCmd drops exit trailer)

**Site:** `scene.parser.flushCmd`
**Issue:** cap-retention (minor)

When the exit trailer is detected at flush time, `p.curCmd.Output = p.curCmd.Output[:n-1]` drops the last element via reslice. The trailer string sits in the unused tail slot of the backing array. The reduced slice is then copied into a `Beat{Command: *p.curCmd}`. The Beat's `Command.Output` slice header retains the backing array including the dropped trailer.

**Mutation impact:** trivial — one string per command beat, only for commands that carried an `(exit N)` trailer. The string is short ("(exit 0)" etc). Not worth fixing in isolation, but if scene output gets large (long capture sessions), tail accumulates.

**Code:**
```go
if n := len(p.curCmd.Output); n > 0 {
    if exit, ok, err := parseExitTrailer(p.curCmd.Output[n-1]); err == nil && ok {
        p.curCmd.Exit = exit
        p.curCmd.Output = p.curCmd.Output[:n-1]   // tail slot still pinned
    }
}
p.curAct.Beats = append(p.curAct.Beats, Beat{Kind: BeatCommand, Command: *p.curCmd})
```

**Fix (optional):** full-slice expression to bound cap to len, or explicit clear:
```go
p.curCmd.Output[n-1] = ""           // release the trailer string
p.curCmd.Output = p.curCmd.Output[:n-1:n-1]
```

### 4. [F4] `pkg/sarif/aggregates.go:52-54` — capacity-retention-leak (TopFiles truncate)

**Site:** `sarif.TopFiles`
**Issue:** cap-retention (minor)

`files = files[:limit]` after sorting drops the tail but keeps the backing. The dropped `FileIssue` entries are value structs with file-path strings; they stay reachable through the returned slice's backing. Returned slice's `cap > len` also means a caller appending can overwrite the discarded values silently.

**Mutation impact:** memory footprint is bounded by `len(byFile)` for the duration the caller holds the result; usually small. The append-clobber risk is theoretical (no other aliases survive).

**Code:**
```go
if limit > 0 && len(files) > limit {
    files = files[:limit]
}
return files
```

**Fix:** `files = slices.Clone(files[:limit])` or `files = files[:limit:limit]`. The latter is allocation-free and prevents the silent append-clobber.

### 5. [F5] `pkg/suppress/suppress.go:107-129` — nil-vs-empty-mixed-returns

**Site:** `suppress.Parse`
**Issue:** nil-vs-empty-mixed-returns (P3, low impact)

`var out []Suppression` then conditional appends. If input is empty or all comments/blank, returns `(nil, nil)`. Otherwise returns `([]Suppression{...}, nil)`. Callers using `len(out) == 0` see no difference, but the rule comment in the spec recommends explicit consistency.

**Mutation impact:** `cmd/fo/suppress.go:52` already guards with `len(rules) == 0`, so this is fine in practice. Flagging only to track the convention.

**Fix:** godoc one-liner — "Returns nil when no suppressions parsed" — or normalize to `[]Suppression{}` at return. Either is fine; lowest priority.

## Notes (not findings)

- `pkg/testjson/toreport.go:141` already does the right thing: `Members: append([]string(nil), g.Members...)` — defensive copy of cluster members into the report. Good template for the rest of the codebase.
- `pkg/cluster/cluster.go` is clean throughout: every returned slice (`keys`, `recs`, `Members`, `clusters`) is freshly `make`'d with known capacity; no sub-slicing of caller input; `Cluster.Members` set from freshly-made `keys`. `Run` documented as returning non-nil empty slice on empty input (line 99); consistent.
- `pkg/scene/scene.go:IsHeader` (line 84-106) slices bytes off the input `data` to scan lines; only used for prefix matching and reports a bool; no slice escapes the function. Safe.
- `pkg/wrapper/wraparchlint/archlint.go` reads bounded bytes via `boundread.All`, unmarshals into a local struct, builds SARIF fresh. No boundary sharing.
- `pkg/cluster/cluster.go:283-287` `unionFind.find` does path compression on the parent slice (`u.parent[x] = u.parent[u.parent[x]]`). The slice is package-internal and owned by the unionFind value; no aliasing concern.
- `cmd/fo/watch.go:191` and `pkg/scene/scene.go:119` both bound the bufio.Scanner buffer explicitly — good.

## Don't-flag rationale

- `r.Notices = append(r.Notices, ...)` in filter.go — single owner (`*Report`), no shared backing. Standard append.
- `r.Tests = append(r.Tests, ...)` in toreport.go — building a fresh `*Report`; no caller-provided slice.
- `actorPalette` in `pkg/view/scene_human.go` — package-level read-only data; index modulo'd into. Safe.
