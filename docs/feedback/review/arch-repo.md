# arch · repo · /Users/vcto/Projects/fo

RUN_ID: f62c7fc3af14
linter: arch (project, report-only)
target: repo
module: `github.com/dkoosis/fo`

## Summary

| Dimension | Tier | Note |
|-----------|------|------|
| Conformance | green | no cycles; no `.go-arch-lint.yml` (gap, not a violation) |
| Coupling | green | no danger-zone pkgs; top cross-pkg pair `cmd/fo→pkg/testjson` = 27 calls (<50 threshold) |
| API Surface | yellow | 1 pkg with mixed-family surface; rest <=35 |
| Pkg Health | green | no god pkgs (cmd/fo exempt); 2 lazy/seam-borderline wrappers |
| Structural | yellow | no `.go-arch-lint.yml` to lock the declared layering in `.claude/rules/CLAUDE.md` |

Overall: **yellow**. Repo topology is clean (no cycles, no reverse-DAG since single module, no orphans, no god pkgs). One pkg mixes unrelated families; one structural gap (missing layering config) leaves the documented architecture unenforced.

## Findings

### 1. [F1] `pkg/state/state.go:23` — pkg-surface-bloat

**Diagnosis.** `pkg/state` exports 41 symbols spanning three distinct families: sidecar persistence (`File`, `Envelope`, `Load`, `Save`, `Reset`, `Run`, `RunFromReport`, `Path`, `Dir`, `SchemaVersion`, `DefaultPath`, `MaxHistory`, `ErrVersionSkew`, `ErrDurabilityDegraded`); diff classification (`Class`, `Class*` consts, `Diff`, `Item`, `Classify`, `Severity`, `Sev*` consts, `Headline`); and metrics-history (`MetricSample`, `MetricDelta`, `MetricsRun`, `MetricsFile`, `LoadMetricsHistory`, `LoadMetrics`, `AppendMetrics`, `DiffMetrics`, `MetricsHistoryPath`, `MaxMetricsHistory`, `MetricsSchemaVersion`).

**Why.** Readers can't predict which file owns which behavior; consumers (currently only `cmd/fo`) pull a wide surface they don't all need. Three orthogonal lifecycles share one import path — touching one family forces re-reading the others.

**Evidence.**
- 41 exported non-test symbols (`snipe` symbols table, `name GLOB '[A-Z]*' AND name NOT LIKE 'Test%'`).
- File-level split already implicit: `state.go` (278 LOC), `diff.go`, `metrics_history.go` (136 LOC), `headline.go`.
- 14 call edges `cmd/fo → pkg/state` confirm single-consumer fan-in.
- Architecture doc (`.claude/rules/CLAUDE.md`) describes `pkg/state` as "sidecar `.fo/last-run.json` for diff classification" — the metrics-history sub-family was bolted on later (fo-2nj, #258 in recent commits).

**Fix.** Split along family lines — keep persistence in `pkg/state`, move diff classification to `pkg/state/diff` (or merge into `pkg/fingerprint`/`pkg/score` next to its peers), move metrics history to `pkg/metrics` (which already exists at 8 exports and is the natural home). Or, if splitting is too disruptive, demote families' internal helpers to unexported and keep only the narrow surface `cmd/fo` actually wires.

**Rule:** `pkg-surface-bloat`

---

### 2. [F2] `.claude/rules/CLAUDE.md:1` — layering-violation (gap)

**Diagnosis.** Project documents a layered architecture (`stdin → read → sniff → parse → Report (IR) → diff → render → exit`) and an explicit pkg-role table, but there is no `.go-arch-lint.yml` or `.go-arch-lint-target.yml`. The layering exists only in prose; nothing prevents a future change from importing `pkg/sarif` directly from `pkg/view`, or `pkg/state` from `pkg/report`, etc.

**Why.** Without a config, the catalog rule fires as a *gap* rather than a violation — but it is the highest-leverage structural finding here because every other dimension is green and the docs already encode the rules. Tier-2 (target) config would let drift accumulate visibly instead of silently.

**Evidence.**
- No `.go-arch-lint*.yml` in repo root.
- `.claude/rules/CLAUDE.md` lines documenting the pipeline + the `Package Structure` table with explicit roles per pkg.
- Current topology happens to conform (pagerank: `pkg/report` is top hub with Ca=5, instability=0; renderers depend on report, not the reverse), so a config codifies the current shape without forcing a refactor.

**Fix.** Add `.go-arch-lint-target.yml` first (advisory, no failures) mapping the seven pipeline stages to component constraints — e.g. `view` may depend on `report|paint|theme` but not `sarif|testjson|state`; `sarif|testjson` may depend on `report` but not on each other; `wrapper/*` may depend on `sarif|tally|metrics|status` but not on `view|state`. Once green for a release cycle, promote to `.go-arch-lint.yml`.

**Rule:** `layering-violation`

---

### 3. [F3] `pkg/wrapper/wrapcover/wrapcover.go:1` — lazy-package

**Diagnosis.** `pkg/wrapper/wrapcover` exports 2 non-test symbols (`Convert`, `key`) with Ca=1 (only `cmd/fo`). Mirror analysis: `pkg/wrapper/wrapgobench` exports 3 (`Convert`, `unitKey`, plus regex vars) with Ca=1. Both are below the catalog threshold of `<3 exported AND Ca<2`.

**Why.** Two notes here: (a) the catalog rule's own "don't flag" carve-out applies — `pkg/wrapper/*` is a deliberate seam for the `fo wrap <name>` dispatch, and per the architecture doc adding a wrapper means "new package under `pkg/wrapper/`, expose `Convert`". (b) However, both `wrapcover` and `wrapgobench` are *strictly thinner* than their peers (`wraparchlint`, `wrapdiag`, `wrapjscpd` each have 7-15 exports and 88-150 LOC of real work) — they're the boundary cases for whether the seam pattern is paying off.

**Evidence.**
- `wrapcover/wrapcover.go`: 2 exports, Ca=1, 1 production .go file (other is its test).
- `wrapgobench`: 3 exports, Ca=1.
- Compare `wraparchlint`: 7 exports, Ca=1, but ships a `convert.go` + `archlint.go` split; `wraparchlinttext`: 3 exports, Ca=1 — same border.
- The wrapper dispatch in `cmd/fo/main.go` is the single caller, so folding into `cmd/fo/wrap_*.go` would lose nothing structurally.

**Fix.** Hold as-is (seam exemption applies and the pattern is uniform across 7 wrappers, which is its own virtue). If the seam ever earns a non-cmd caller (e.g. a library mode), the pattern pays off retroactively. Re-check in 6 months: any wrapper still at <=3 exports / Ca=1 with no second caller is a real lazy-package candidate.

**Rule:** `lazy-package`

---

## Not flagged (with reason)

- **cmd/fo as god-package.** 80 exported (mostly tests), Ce=17, ~3.8k LOC. Catalog explicitly carves out `cmd/*` composition roots. Production-only export count is 6.
- **cmd/fo → pkg/testjson coupling.** 27 cross-pkg calls. Below the 50-call coupling-hotspot threshold and the relationship is intentional (cmd/fo is the only consumer of the parser).
- **pkg/sarif, pkg/testjson, pkg/view at 35/32/32 exports.** All single-family (SARIF types, testjson parser, renderers). Yellow tier, no surface-bloat split.
- **No orphans.** All non-`cmd/*`, non-`*_test` pkgs have at least one importer.
- **No cycles.** `snipe metrics --kind=cycles` returned empty SCC list.
- **No reverse-DAG.** Single module (`github.com/dkoosis/fo`), no `go.work`.

## Data sources

- Pre-built bundle: `metrics-ca`, `metrics-ce`, `metrics-instability`, `metrics-pagerank` from `/tmp/snipe-bundle-f62c7fc3af14/`.
- Live: `snipe metrics --kind=cycles` (empty); `snipe lifecycle Report --depth 2` (Mutate=0, healthy); direct SQL over `.snipe/index.db` for tightly-coupled pairs and exported-symbol counts (filtering `Test*` prefixes).
