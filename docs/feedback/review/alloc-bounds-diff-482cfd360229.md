# alloc-bounds — focused diff review

**Verdict: 🟡 yellow.** No fresh unbounded allocations on a hot path beyond the already-tracked `outputBuf` map (issue #257). All "external N" sites in the diff are gated upstream by `boundread.DefaultMax` (256 MiB on byte input) or by `lineread`'s 64 KiB-grown scanner with explicit caps. fo is a CLI consuming stdin/files — there is no HTTP/RPC trust boundary, so the threat model is "hostile SARIF / `go test -json` / `.fo/ignore` file produced by a misbehaving wrapper or attacker who can already write to the local FS." Within that model, three sites amplify N×M growth without any internal ceiling and are worth tightening, plus several P2/P3 observations.

Counts: P1 alloc 0 unbounded fresh / 2 loose · P1 read-all 0 · P1 fanout 0 · P2/P3 3.

---

### 1. [F1] `pkg/cluster/cluster.go:106-151` — unbounded-slice-alloc (loose)

**Site:** `byKey`, `recs`, `groups`, `taken`, `clusters`, `keys` in `RunWith` and `mostCommon`.
**N source:** `len(inputs)` → `attachClusters` (`pkg/testjson/toreport.go:102`) → `len(r.Tests)` → derived from every test event parsed off stdin. Adversary-influenced `go test -json` stream can manufacture arbitrarily many synthetic `Test` records (one per fake `fail` action) up to the stream byte cap. With ~50 bytes per JSON event, 256 MiB of stdin → ≈5M `Input` records; each `record` carries the full per-test `Output` blob (already capped per-test, but ×N records). `RunWith` then allocates seven slices/maps sized by N and a union-find quadratic in practice for `unionBy`'s `groups` map keyed by signal strings.

```go
// pkg/cluster/cluster.go:106
byKey := make(map[string]Input, len(inputs))
...
recs := make([]record, len(keys))      // :116
uf   := newUnionFind(len(recs))        // :122 → 2 × []int sized N
groups := make(map[int][]int)          // :144
...
clusters := make([]Cluster, 0, len(groups))  // :151
```

**Fix:** Bound at the boundary. In `attachClusters`, cap inputs to a constant (e.g. `maxClusterInputs = 10_000`) and emit a `Notice` when truncated. Rationale: clustering output is meant to summarize human-readable failure modes; >10k failing tests in one run is already a "fix your build" signal, not a clustering problem. Cheaper than per-allocation guards and preserves Determinism (truncate after sort by stable key).

**Severity:** Medium.

---

### 2. [F2] `pkg/testjson/parser.go:237,247-248` — unbounded-map-alloc

**Site:** `aggregator.packages`, `pkgState.outputBuf`, `pkgState.outputBufBytes`.
**N source:** `processEvent` calls `getOrCreate(e.Package)` on every event; package name is attacker-controlled JSON. Per-test `outputBuf` keys are `e.Test` — also attacker-controlled. Per-test byte cap exists (`appendCapped` → `maxPerTestOutputBytes`), but **the number of distinct keys is uncapped**: 5M synthetic events with unique `Test` names produce 5M map entries × map overhead.

```go
// pkg/testjson/parser.go:237
packages: make(map[string]*pkgState),
// :247
outputBuf:      make(map[string][]string),
outputBufBytes: make(map[string]int),
```

**Fix:** Add `maxPackages` and `maxTestsPerPackage` constants; once exceeded, drop further `output`/`fail` events for new keys and stamp a `Notice` on the Report. Issue #257 is the parent for `outputBuf` byte growth; this finding is the **key-count** dimension, which is orthogonal — file as a sibling bead or fold into #257 explicitly.

**Severity:** Medium (overlaps known #257; mention the key-count axis there if not already).

---

### 3. [F3] `pkg/sarif/aggregates.go:18,67` — unbounded-map-alloc

**Site:** `byFile` in `TopFiles` and `GroupByFile`.
**N source:** every `Result.Locations[0].PhysicalLocation.ArtifactLocation.URI` in a parsed SARIF doc. SARIF bytes are bounded by `boundread.DefaultMax` (256 MiB), but a pathological SARIF can pack ~250 byte results → ≈1M distinct fake `file://attacker-controlled-path-N` URIs → 1M map entries plus 1M `[]Result` slices in `GroupByFile`.

```go
// pkg/sarif/aggregates.go:18
byFile := make(map[string]*FileIssue)
// :67
byFile := make(map[string][]Result)
```

**Fix:** `TopFiles` already takes `limit`; honor it during accumulation by bounding `len(byFile)` (heap-of-N or two-pass with min-count threshold). For `GroupByFile`, cap `len(byFile)` and append surplus results into a synthetic `"…N more files"` group. Rationale: a leaderboard with 100k file rows is already useless to a human/LLM — the renderer truncates downstream anyway.

**Severity:** Medium.

---

### 4. [F4] `pkg/suppress/match.go:48-93` — quadratic-string-build / recursive backtracking

**Site:** `globHere` is naive recursive `**` matcher; nested wildcards exhibit catastrophic backtracking.
**N source:** `.fo/ignore` patterns are local-user-controlled, but `Match` runs once per Finding per loaded suppression. For a Report with 100k findings × 100 suppressions × a pattern like `**/**/**/foo`, runtime explodes.

**Fix:** Either (a) compile patterns once with `doublestar` or `path/filepath.Match` semantics + memoization, or (b) reject patterns with more than two `**` segments at Parse time. Rationale: `.fo/ignore` is shared between humans and LLM-generated suppressions; cheap to abuse accidentally.

**Severity:** Low. Local-user trust boundary; treat as defense-in-depth.

---

### 5. [F5] `cmd/fo/main.go:902` — chan-buffer-from-input (false-positive nearby)

**Site:** `snapshots := make(chan report.Report, 8)`. Constant, fine. Noted only to confirm it was scrutinized and **not** flagged. Same for `resultCh := make(chan streamResult, 1)` (:913).

**Severity:** Info (no action).

---

### 6. [F6] `pkg/wrapper/wraparchlint/archlint.go:31` — json-decode-unbounded-array (Info)

**Site:** `json.Unmarshal(data, &raw)` into struct with `ArchWarningsDeps []…` field; no post-decode length check before fanning into `b.AddResultWithFix` once per element.
**N source:** stdin → `boundread.All` (already capped to 256 MiB by default).

**Fix:** Add `if len(raw.Payload.ArchWarningsDeps) > maxArchFindings { … }` (e.g. 10k) before the loop, with a Notice. Low priority because the SARIF builder is in-memory only and the byte cap is the real ceiling. Same pattern applies to every wrapper not in this diff — worth a follow-up audit across `pkg/wrapper/*`.

**Severity:** Info.

---

## Notes / scope

- `pkg/testjson/stats.go` and `pkg/wrapper/wraparchlint/convert.go` do not exist in the tree — skipped.
- `bufio.Scanner` sites in `pkg/scene/scene.go`, `pkg/suppress/suppress.go`, `cmd/fo/watch.go` all call `Scanner.Buffer(..., 1<<20)` or `maxLine` — explicit caps, **good**.
- `cmd/fo/watchkey.go` channels and 1-byte read buf are constant-sized — fine.
- `pkg/report/filter.go` / `pkg/cluster/{frame,id,normalize}.go` / `pkg/view/scene_*.go` / `pkg/scene/scene.go` / `cmd/fo/suppress.go` did not surface alloc sites worth flagging.
- All findings hinge on the trust model: **fo's `go test -json` and SARIF inputs are de facto trusted because they come from local tools**, but malicious/buggy producers + CI memory budgets make the bounds worth tightening anyway.
