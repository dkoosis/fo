# arch · repo

run-id: bd775e303d86-arch · target: repo (whole) · mode: report

Overall: **green-yellow**. Small repo (20 pkgs, ~9.5k LOC). No import cycles, no >50 coupling pairs (excl. tests), no orphans, no danger-zone Ca+I combos. Two structural notes worth surfacing: the absence of a `.go-arch-lint.yml` (declared layering is informal) and multi-domain mixing inside `pkg/state`.

| Dimension | Score | Notes |
|---|---|---|
| Conformance | yellow | no `.go-arch-lint.yml`; layering exists only in `CLAUDE.md` prose |
| Coupling | green | max cross-pkg call pair (non-test) = 25 (cmd/fo→testjson) |
| API Surface | green-yellow | pkg/state ~36 non-test exports across 3 domains |
| Pkg Health | green-yellow | pkg/state multi-domain; pkg/view LCOM4=14 (defer to cohesion) |
| Structural | green | no cycles, no orphans, no reverse-DAG (single module) |

## Findings

### 1. [F1] `.go-arch-lint.yml:0` — layering-violation

**Diagnosis.** No `.go-arch-lint.yml` (or `-target.yml`) in repo root. The architecture-layering contract documented in `/Users/vcto/Projects/fo/.claude/rules/CLAUDE.md` (stdin→read→sniff→parse→Report→diff→render→exit, plus "Report is the IR" / "wrappers don't render" rules) is enforced only by code review.

**Why.** `arch.rules.md#layering-violation` treats the YAML as source-of-truth for intended layering. With nothing to check against, every layering question is an opinion. The north-star doc lists six load-bearing decisions any of which a future renderer-that-imports-state or wrapper-that-imports-view could quietly violate.

**Evidence.** `ls /Users/vcto/Projects/fo/.go-arch-lint*.yml` → no matches. `/Users/vcto/Projects/fo/.claude/rules/CLAUDE.md` lines 15-58 carry the prose contract.

**Fix.** Add `.go-arch-lint.yml` codifying: (a) `pkg/wrapper/*` may import `pkg/sarif`, `pkg/report`, `pkg/metrics`, `pkg/status`, `pkg/tally` — never `pkg/view`/`pkg/paint`/`pkg/theme`; (b) parsers (`pkg/sarif`, `pkg/testjson`) never import `pkg/view`; (c) `internal/*` is leaf-only; (d) `cmd/fo` is the only composition root. Wire `go-arch-lint check` into the target that already runs `make report`.

**Tier.** P0 (conformance) — gap, not regression.

---

### 2. [F2] `pkg/state/state.go:1` — pkg-surface-bloat

**Diagnosis.** `pkg/state` exports ~36 non-test symbols spanning three distinct domains: run persistence (`Run`, `RunFromReport`, `Save`, `Load`, `Envelope`, `File`, `Diff`, `Item`, `Severity`+constants, `Classify`+classes, `Class*` consts), metrics history (`MetricsRun`, `MetricSample`, `MetricsFile`, `MetricDelta`, `AppendMetrics`, `LoadMetricsHistory`, `DiffMetrics`, `MetricsHistoryPath`, `MetricsSchemaVersion`, `MaxMetricsHistory`), and presentation (`Headline`). Different lifecycles, different callers, different invariants.

**Why.** `arch.rules.md#pkg-surface-bloat` flags multi-domain pkgs because consumers can't predict where behavior lives. `Headline` in particular is a presentation helper riding inside a persistence pkg; metrics history is a parallel storage system with its own schema version (`MetricsSchemaVersion` vs `SchemaVersion`). A consumer importing `pkg/state` for `Run.Save` drags the metrics history surface and a presentation formatter.

**Evidence.** Files: `pkg/state/state.go` (279 LOC, run+diff+severity), `pkg/state/metrics_history.go` (parallel persisted format), `pkg/state/headline.go` (presentation), `pkg/state/diff.go` (classification). Two `SchemaVersion` constants confirm two independent persisted formats. `sqlite3 .snipe/index.db` shows 70 raw exports including tests, ~36 production exports.

**Fix.** Three sub-pkgs along the verb-clusters already present in the filenames: `pkg/state` (run + diff + severity + classify), `pkg/state/metrics` (metrics history), and move `Headline` into `pkg/view` or a new `pkg/state/summary` — it formats, doesn't store. Keep `state.Save/Load` as the only persistence entry points.

**Tier.** P2 (API surface).

---

### 3. [F3] `pkg/cluster/cluster.go:1` — lazy-package (borderline)

**Diagnosis.** `pkg/cluster` has Ca=1 (only `pkg/testjson` imports it), ~14 non-test exports, ~800 LOC including extensive property tests. Borderline against the lazy-pkg rule (<3 exports AND Ca<2) — the export count clears the bar but the single consumer triggers a look.

**Why.** `arch.rules.md#lazy-package` says deliberate seams (plugin boundaries, DI ports, build-tag gates) get a pass. Cluster is a deliberate seam — test-failure deduplication engine with its own normalize+frame extraction pipeline and property-tested invariants. With a single consumer and no extension point, the boundary buys test isolation but not modularity. Worth a note, not a fix.

**Evidence.** `sqlite3 .snipe/index.db "SELECT DISTINCT importer_pkg FROM imports WHERE pkg_path='github.com/dkoosis/fo/pkg/cluster'"` → `pkg/testjson` only. metrics-ca.txt rank 11: Ca=1. metrics-instability shows I=0 (stable leaf).

**Fix.** Leave as-is — property tests and orthogonal normalize/frame modes justify the boundary. Revisit if a second consumer never materialises and the surface shrinks; then fold into `pkg/testjson/internal/cluster`.

**Tier.** P3 (structural, informational).

---

### 4. [F4] `pkg/scene/scene.go:1` — pkg-surface-bloat (mild)

**Diagnosis.** `pkg/scene` has 14 non-test exports including 5 sentinel error vars (`ErrMalformedAct`, `ErrMalformedActor`, `ErrMalformedExit`, `ErrNoHeader`, `ErrUnknownAttr`) for a single consumer (`pkg/view`). Single domain (scene parsing), so not multi-family bloat — but the sentinel-error surface is large for one importer.

**Why.** `arch.rules.md#pkg-surface-bloat` covers wide consumer surface. Sentinels exported "in case the consumer wants to switch on them" that the consumer never branches on = avoidable export and an API stability liability.

**Evidence.** `sqlite3 .snipe/index.db "SELECT DISTINCT importer_pkg FROM imports WHERE pkg_path='github.com/dkoosis/fo/pkg/scene'"` → `pkg/view` plus tests. Sentinels listed via `sqlite3` exports query above.

**Fix.** Either keep one `ErrMalformed` umbrella + `errors.Is` paths, or unexport the four narrow variants and surface only `ErrNoHeader` + `ErrUnknownAttr` if those carry distinct caller semantics. Audit the four `Err*` call sites in `pkg/view` first.

**Tier.** P3 (surface trim).

---

## Empty / informational buckets

- **dependency-cycle** — none in the import graph. The 2 call-graph SCCs in `metrics-cycles.txt` are deliberate recursion: `Render`↔`renderDelta` (delta wraps an inner ViewSpec re-rendered through the same dispatch in `pkg/view/render.go:21` + `pkg/view/delta.go:13`), and `run`↔`runWatch`↔`runChildAndRender` (watch loop in `cmd/fo/watch.go:91-222` calls back into the render pipeline per trigger). Both intended.
- **reverse-dag-import** — single-module repo, n/a.
- **coupling-hotspot** — max non-test cross-pkg call count is 25 (cmd/fo→testjson); threshold is 50. Ca/I danger zone empty: `internal/lineread` Ca=10 has I=0 (leaf reader, stable as it should be); `pkg/report` Ca=5 I=0.167. No fragile coordinators (max non-cmd Ce=5 in `pkg/testjson`).
- **god-package** — `cmd/fo` (Ca=0, Ce=19, 81 exports, 3.1k LOC) is the composition root and exempt by rule. No other pkg combines high Ca + high LOC + wide surface.
- **orphan-package** — none. `pkg/cluster` and `pkg/scene` each have exactly one production consumer; not zero.
- **lifecycle-split-ownership** — `Report` (the IR) is constructed by `pkg/sarif`, `pkg/testjson`, and `pkg/report.ParseSections`; mutated by `pkg/state` (diff classification attaches `DiffSummary`); consumed read-only by `pkg/view`. Single mutation site, clean create→mutate→read pipeline. No split ownership.
