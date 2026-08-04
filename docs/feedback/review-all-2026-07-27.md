# /review all — 2026-07-27

run_id: `7858b3a8ea1b` · lens picker: Correctness & safety, Design & architecture, Clarity & craft, Tests

30 linters selected — 25 ran · 2 n/a · 3 out-of-scope (dropped this pass, see note) — 48 findings total.

**Out-of-scope note:** `tx-boundary` and `n-plus-one` dropped — fo has no database/ORM/transaction code (`rg` confirmed no `sqlite`, `database/sql`, `gorm`, `sqlx`, `Begin`/`Transaction` hits) so these would reliably find nothing. `craft` (file-scope, no explicit target) dropped — default target is branch-changed `.go` files via merge-base, and this session's branch was already merged to `main`, so the diff is empty. `sqlite`/`ctx-value` ran their `applies_when` gate and self-skipped as `n/a` (no sqlite, no `context.WithValue` in the repo) — listed below, not counted as dropped.

`fix-security` is normally apply-mode (auto-commit + auto-PR); this run was forced report-only by operator decision — findings below, nothing applied.

7 of the 25 are pkg-scope linters (`clarity`, `cohesion`, `domain-model`, `error-semantics`, `fix-security`, `sim-pair`, `testability`) run as ONE condensed pass over the whole repo, not the full per-package `next`-queued auto-sweep — operator chose condensed depth to bound cost (~27 agents instead of ~230).

## Scorecard

| Linter | Scope | Findings | Report |
|---|---|---|---|
| alloc-bounds | project | 2 | [alloc-bounds-repo.md](review/alloc-bounds-repo.md) |
| api-surface | project | 1 | [api-surface-repo.md](review/api-surface-repo.md) |
| arch | project | 3 | [arch-repo.md](review/arch-repo.md) |
| change-smells | project | 0 | [change-smells-repo.md](review/change-smells-repo.md) |
| concurrency-safety | project | 0 | [concurrency-safety-repo.md](review/concurrency-safety-repo.md) |
| conversion-drift | project | 0 | [conversion-drift-repo.md](review/conversion-drift-repo.md) |
| ctx-value | project | n/a | applies_when: no `context.WithValue` in repo |
| domain-vocab | project | 2 | [domain-vocab-repo.md](review/domain-vocab-repo.md) |
| errors-design | project | 5 | [errors-design-repo.md](review/errors-design-repo.md) |
| goroutine-lifecycle | project | 3 | [goroutine-lifecycle-repo.md](review/goroutine-lifecycle-repo.md) |
| io-parallel | project | 0 | [io-parallel-repo.md](review/io-parallel-repo.md) |
| json-shape | project | 3 | [json-shape-repo.md](review/json-shape-repo.md) |
| pointer-value | project | 1 | [pointer-value-repo.md](review/pointer-value-repo.md) |
| slice-map | project | 1 | [slice-map-repo.md](review/slice-map-repo.md) |
| solid | project | 0 | [solid-repo.md](review/solid-repo.md) |
| sqlite | project | n/a | applies_when: no sqlite in repo |
| test-effectiveness | project | 1 | [test-effectiveness-repo.md](review/test-effectiveness-repo.md) |
| test-tables | project | 1 | [test-tables-repo.md](review/test-tables-repo.md) |
| truthful-names | project | 3 | [truthful-names-repo.md](review/truthful-names-repo.md) |
| zero-sentinel | project | 1 | [zero-sentinel-repo.md](review/zero-sentinel-repo.md) |
| clarity | pkg (condensed) | 4 | [clarity-repo.md](review/clarity-repo.md) |
| cohesion | pkg (condensed) | 2 | [cohesion-repo.md](review/cohesion-repo.md) |
| domain-model | pkg (condensed) | 4 | [domain-model-repo.md](review/domain-model-repo.md) |
| error-semantics | pkg (condensed) | 3 | [error-semantics-repo.md](review/error-semantics-repo.md) |
| fix-security | pkg (condensed, report-only) | 4 | [fix-security-repo.md](review/fix-security-repo.md) |
| sim-pair | pkg (condensed) | 0 | [sim-pair-repo.md](review/sim-pair-repo.md) |
| testability | pkg (condensed) | 4 | [testability-repo.md](review/testability-repo.md) |

## Top findings by tier

**Action-tier, worth landing soon:**
- `fix-security` F1-F2, F4: `cmd/fo/render.go:192`, `cmd/fo/suppress_cmd.go:230`, `pkg/view/pipeline_golden_test.go:99` — G306 world-writable-ish file permissions on written files.
- `fix-security` F3: `pkg/state/metrics_history.go:64` — unhandled error (G104).
- `arch` F1-F2: `.go-arch-lint.yml`, `pkg/tally/tally.go:28` — layering violations against the declared architecture.
- `alloc-bounds` F1-F2: `pkg/testjson/parser.go:185`, `pkg/sarif/reader.go:28` — unbounded map/array allocation sized by external input.
- `errors-design`/`error-semantics`: sentinel errors (`state.ErrVersionSkew`, `multiplex.ErrNoSections`, `sarif.ErrNestingTooDeep`, hygiene-format sentinels) exported but never checked by any caller via `errors.Is`.

**Borderline / structural, worth a look:**
- `domain-model`: 3× stringly-typed enum (`report.go`, `sarif/types.go`, `testjson/types.go`).
- `testability`: `pkg/state` has globals-as-implicit-deps + a missing service seam; `pkg/hygiene` + 4 wrapper packages share the same globals pattern.
- `truthful-names`: terminology drift in `pkg/suppress/match.go`, `pkg/sarif/aggregates.go`; imprecise name in `pkg/state/metrics_history.go`.

## Cross-linter hotspots

_Locations cited by 2+ linters. One fix may close multiple findings._

| # linters | location | linter:finding:rule |
|-----------|----------|---------------------|
| 2 | pkg/state | arch:F3:pkg-surface-bloat, cohesion:F1:lcom4-split-candidate |

**Manual note (script matches exact `file:line`, so this undercounts):** `pkg/state` is independently flagged by **6** of the 25 linters — `arch` (pkg-surface-bloat), `cohesion` (lcom4-split-candidate: "three parallel, independent persistence subsystems in one package"; also a `pkg/status`/`pkg/state` terminology homonym), `errors-design` (sentinel-without-callers, `state.go:95`), `error-semantics` (internal-sentinel-leaked-exported, same line), `testability` (globals-as-implicit-deps + missing-service-seam), and `slice-map` (boundary-returns-internal-backing, `diff.go:156`). Six independent lenses converging on one package is the strongest signal in this run: `pkg/state` has grown into last-run diffing + snapshot/explain + run-log + full-log persistence in one package with package-level globals and no service seam. Splitting it (or introducing a `State` interface/struct to replace the globals) would plausibly close 6+ findings at once. Candidate for a `/review craft` or design pass on `pkg/state` specifically before further additions land there (this session's own PR #288 added a 4th persistence concern — full.log — to the same package).

## Next step

Rate findings per linter: `/assess-feedback <linter> --run-id=7858b3a8ea1b`, or `/assess-feedback --all --run-id=7858b3a8ea1b` for the whole run.
