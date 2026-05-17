# /review all — fo @ main (2026-05-17)

run_id: `bd775e303d86`  ·  linters: 23 project-scope  ·  total findings: **71**

Per-linter reports live in `docs/feedback/review/<linter>-repo.md`. This page stitches the scorecard and surfaces files where multiple linters converged.

## Scorecard

| linter | tier | N | headline finding |
|---|---|---:|---|
| alloc-bounds | 🟢 | 2 | `pkg/sarif/reader.go:15` public `Read(io.Reader)` has no in-package cap; production callers route through `boundread` but the contract isn't enforced in code (borderline). |
| api-surface | 🟢 | 10 | All 10 are P2 exported-but-unreferenced: `cluster.Extract`, `state.Headline`, `state.LoadMetricsHistory`/`MetricsFile`/`MetricsRun`, and the `Err{NoHeader,NoRows,MalformedRow}` trio duplicated in `pkg/tally`/`pkg/metrics`/`pkg/status`. |
| arch | 🟡 | 4 | P0: no `.go-arch-lint.yml` — layering enforced only by prose in CLAUDE.md. P2: `pkg/state` mixes run persistence + metrics history + presentation (`Headline`) under two distinct `SchemaVersion` consts. |
| change-smells | 🟡 | 3 | `cmd/fo/main.go` divergent-change (1339 LOC, 5+ verb clusters); `runStream{,Ctx,Batch}` share a 6-param trailing tail (extract `streamOpts`); wrapper shotgun-surgery across `pkg/wrapper/*` (extract `wrapper.Run[T]`). |
| concurrency-safety | 🟢 | 5 | Race detector clean across all packages. `runTestJSONPipeline` duplicates stdin-close hooks (F1); producer goroutine can leak past 2s grace timeout on cancel (F2); `testjson.Stream` "Close must unblock Read" contract is prose-only (F3). |
| conversion-drift | 🔴 | 4 | F1 (🔴): `pkg/suppress/suppress.go` rejects `until=0001-01-01` in `Parse` but `Format()` still emits it (broken roundtrip), and `Parse` aborts on first error so one stale rule silently disables every later suppression. |
| ctx-value | 🟢 | 0 | No `context.WithValue` producers or `ctx.Value(...)` consumers — `context.Context` is used purely for cancel/deadline propagation. |
| domain-vocab | 🔴 | 6 | `theme.Default(bool)` exported API with bare-literal call sites; `noState, stateStrict bool` pair propagated across `runStream`/`runStreamCtx`/`runStreamBatch`/`attachDiff` (bundle into `StateOpts`). |
| errors-design | 🟡 | 5 | Three exported sentinels (`sarif.ErrMissingSARIFVersion`, the `tally.Err*` trio, `wrapleaderboard.Err{NoRows,MalformedRow}`) have no out-of-package consumers — unexport. Two wrap-redundancy clusters in `wraparchlint`/`wrapjscpd`/`pkg/scene`. |
| goroutine-lifecycle | 🟢 | 5 | Strong overall. P2: undocumented magic buffer (`snapshots`, N=8) at `cmd/fo/main.go:920`; four `time.Sleep`-as-sync patterns in tests, including a negative-assertion at `fswatch_test.go:150` that could silently pass for the wrong reason. |
| io-parallel | 🟢 | 0 | Linter inapplicable — fo is a streaming filter with no HTTP/RPC/DB clients and no errgroup usage. Recommend dropping from rotation while north-star "no tool invocation" holds. |
| json-shape | 🔴 | 3 | Lossy-omitempty on canonical IR: `Report.Suppressed`, `Finding.Score`+`TestResult.Score`, `MetricDelta.New` — each drops a meaningful zero/false on the wire. |
| n-plus-one | 🟢 | 0 | Architecture precludes the pattern — no DB/RPC, no errgroup, all FS reads are single-shot. |
| pointer-value | 🟡 | 1 | `pkg/wrapper/wrapdiag/diag.go:35-41` has 4 `*string` fields that are pure read-only derefs (fossil of removed `*flag.FlagSet` plugin path). |
| slice-map | 🟡 | 6 | F1/F2: `pkg/testjson/parser.go:396,403` `Aggregator.Results()` exposes live `panicOutput` and `outputBuf[name]` slices that `appendCapped` keeps mutating after snapshot — defeats streaming-API contract. |
| solid | 🟢 | 3 | F1: `tally.Tally.RenderLLM` (parser DTO + renderer); F2: `status.Status.ToViewRows` exists only to dodge a one-way import; F3: dead `report *report.Finding` back-pointer on `state.Item`. |
| sqlite | 🟢 | 0 | fo has no DB layer. |
| test-effectiveness | 🟡 | 10 | F1: `pkg/view/invariants_test.go:48-52` silently `return`s on type-assertion fail — invariant test is a no-op that still counts as coverage; mechanical fix via `t.Skipf` already used elsewhere in same file. Dominant smell across rest: under-asserted output. |
| test-tables | 🟢 | 2 | F1: `pkg/testjson/parser_test.go:20` `wantSkipped` field never populated non-zero; F2 (info): two tables in `pkg/wrapper/wrapdiag/diag_test.go` lack `name` field (exempted). |
| truthful-names | 🟢 | 3 | F1 `suppress.Ruleset.Rules []Suppression` ("Rule" appears nowhere else in pkg); F2 `state.AppendMetrics` actually prepends+trims+RMWs; F3 `sarif.FileIssue` is an aggregate counts struct named like a single issue. |
| tx-boundary | 🟢 | 0 | No DB layer; sidecar persistence already uses atomic write-temp-rename. |
| vestige-pair | 🟢 | 0 | Only candidate (`wrapjscpd.jscpd struct{}`) is exempted — carries real method + multiple test callers. |
| zero-sentinel | 🟢 | 3 | Already strong (Suppression.Until is `*time.Time`, fo-7jv hardened deserialization). Gaps: `ApplyFilter`/`classifyFinding` accept zero `now` without default; SARIF `Region.StartLine` + `Finding.Line` use `int+omitempty`; `TestPackageResult.Status()` returns PASS for zero-test packages. |

## Cross-linter hotspots

No exact `file:line` convergence across linters. At **file** granularity, 10 files were cited by 2+ linters — these are the highest-leverage targets:

| # linters | file | linters |
|---:|---|---|
| 4 | `cmd/fo/main.go` | change-smells, concurrency-safety, domain-vocab, goroutine-lifecycle |
| 4 | `pkg/state/metrics_history.go` | api-surface, json-shape, slice-map, truthful-names |
| 3 | `pkg/cluster/cluster.go` | api-surface, arch, domain-vocab |
| 2 | `pkg/report/filter.go` | conversion-drift, zero-sentinel |
| 2 | `pkg/sarif/aggregates.go` | slice-map, truthful-names |
| 2 | `pkg/sarif/reader.go` | alloc-bounds, errors-design |
| 2 | `pkg/scene/scene.go` | arch, errors-design |
| 2 | `pkg/state/headline.go` | api-surface, slice-map |
| 2 | `pkg/state/state.go` | alloc-bounds, arch |
| 2 | `pkg/testjson/parser.go` | concurrency-safety, slice-map |

`cmd/fo/main.go` and `pkg/state/*` are the structural hotspots — splitting main.go (change-smells F1) and decomposing `pkg/state` (arch F2) would close findings across multiple linters at once.

## Critical (🔴) items worth triaging first

1. **conversion-drift F1** — `pkg/suppress/suppress.go` Parse/Format roundtrip break + first-error abort silently disables every later suppression rule.
2. **json-shape F1-F3** — lossy `omitempty` on canonical IR fields (`Report.Suppressed`, `*.Score`, `MetricDelta.New`) means JSON consumers can't distinguish absent from explicit-zero.
3. **domain-vocab F1, F2** — bool-pair propagation across `runStream*` and `theme.Default(bool)` API smell are correctness-adjacent (one wrong boolean trail and state classification flips).

## Next

→ `/assess-feedback <linter>` to rate findings per report; high-leverage clusters to consider:
  - **cmd/fo/main.go split** (change-smells F1) — closes 3+ findings.
  - **pkg/state decomposition** (arch F2 + truthful-names F2 + api-surface dead exports) — coherent epic.
  - **suppress roundtrip fix** (conversion-drift F1) — small, high-conviction correctness fix.
  - **Drop io-parallel/sqlite/tx-boundary/n-plus-one** from fo's rotation (architecturally inapplicable; cost ≈ zero signal).
