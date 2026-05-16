# /review all — fo @ 2026-05-16

run_id: `f62c7fc3af14`
target: `/Users/vcto/Projects/fo` (whole repo)
linters: 22 project-scope
total findings: 95 across 16 linters (6 zero-finding)

## Scorecard

| linter | n | tier | top finding | report |
|---|---:|---|---|---|
| alloc-bounds | 2 | 🟢 | `cmd/fo/watch.go:121` `stdinTriggers` uses default 64KB scanner buffer | `review/alloc-bounds-repo.md` |
| api-surface | 5 | 🟡 | `pkg/view` exports paired Foo/FooMode wrappers w/ no external callers | `review/api-surface-repo.md` |
| arch | 3 | 🟡 | `pkg/state` mixes persistence + diff + metrics-history (surface bloat) | `review/arch-repo.md` |
| change-smells | 5 | 🟡 | Divergent change on `cmd/fo/main.go` | `review/change-smells-repo.md` |
| concurrency-safety | 5 | 🟢 | `runStream` producer can transiently block on `snapshots` after renderer returns | `review/concurrency-safety-repo.md` |
| conversion-drift | 10 | 🟡 | `testKey` parse asymmetry — subtest with `/` corrupts File/RuleID | `review/conversion-drift-repo.md` |
| ctx-value | 0 | 🟢 | (no `context.WithValue` in repo) | `review/ctx-value-repo.md` |
| domain-vocab | 5 | 🟡 | `runStream{,Ctx,Batch}` carry adjacent `noState, stateStrict bool` (bool-trap) | `review/domain-vocab-repo.md` |
| errors-design | 9 | 🟡 | 3 exported sentinels w/ zero `errors.Is` callers; unexport candidates | `review/errors-design-repo.md` |
| goroutine-lifecycle | 4 | 🟡 | `stdinTriggers` goroutine cannot be canceled while `Read` is blocked | `review/goroutine-lifecycle-repo.md` |
| io-parallel | 0 | 🟢 | (stdin→stdout pipeline; no RPC/HTTP/DB) | `review/io-parallel-repo.md` |
| json-shape | 7 | 🟡 | `DiffSummary` test-outcome deltas use `omitempty` — absent vs empty collide | `review/json-shape-repo.md` |
| n-plus-one | 0 | 🟢 | (no DB/RPC layer) | `review/n-plus-one-repo.md` |
| pointer-value | 5 | 🟡 | `wrapdiag.diag` uses `*string` for every flag field; forces heap escape | `review/pointer-value-repo.md` |
| slice-map | 10 | 🟡 | `pkg/testjson/parser.go:401` missed prealloc on `FailedTests` | `review/slice-map-repo.md` |
| solid | 3 | 🟡 | `pkg/tally.Tally` (parser DTO) carries `RenderLLM` (renderer method) | `review/solid-repo.md` |
| sqlite | 0 | 🟢 | (no SQLite usage) | `review/sqlite-repo.md` |
| test-effectiveness | 10 | 🟠 | `TestE2E_Pipeline_ContractSurface` accepts both exit 0 and 1 for every fixture | `review/test-effectiveness-repo.md` |
| test-tables | 6 | 🟡 | `TestParseStream_Behavior` per-case branching; split shape-uniform | `review/test-tables-repo.md` |
| truthful-names | 2 | 🟢 | `pkg/state.Item` name doesn't match content (back-pointer + report-shaped fields) | `review/truthful-names-repo.md` |
| tx-boundary | 0 | 🟢 | (no transactional persistence) | `review/tx-boundary-repo.md` |
| zero-sentinel | 4 | 🟡 | merged `Report.GeneratedAt` left zero when all sections fail to parse | `review/zero-sentinel-repo.md` |

Tier legend: 🟢 nothing material / 🟡 yellow worth-fixing / 🟠 active issue / 🔴 fire.

## Cross-linter hotspots

Cite by linter:finding-id.

- **`cmd/fo/watch.go` `stdinTriggers` goroutine** — alloc-bounds:F1 (scanner buffer), concurrency-safety:F3 (uncancelable), goroutine-lifecycle:F1. Three linters converge on this one fire-and-forget goroutine. Strongest signal in the report.
- **`pkg/state` surface** — arch:F1 (surface bloat), solid:F3 (dead back-pointer field), truthful-names:F1 (name↔content drift), zero-sentinel:F2/F3/F4 (`GeneratedAt time.Time` without `IsZero` guard at write seams), json-shape:F4/F5/F6 (omitempty drift + permissive decode). The state package is the densest hot-spot — wants a focused pass.
- **`runStream{,Ctx,Batch}` signatures** — domain-vocab:F1 (bool-trap), concurrency-safety:F1/F2 (block/dropped emission). Refactor candidate.
- **Test contract surface** — test-effectiveness:F1, test-tables:F1. The e2e contract test currently asserts almost nothing.
- **Exported-but-unreferenced** — api-surface:F1-F5, errors-design:F1-F3. 8 candidates for unexport across `pkg/view`, `pkg/wrapper/wrapleaderboard`, `pkg/status`, `pkg/sarif`.

## Recommended next moves

1. **Single highest-leverage cluster**: the `stdinTriggers` goroutine fix (alloc-bounds:F1 + concurrency-safety:F3 + goroutine-lifecycle:F1). One small PR closes three findings.
2. **`pkg/state` focused refactor**: address truthful-names:F1, solid:F3, arch:F1, zero-sentinel:F2/F3/F4, json-shape:F4/F5/F6 together. Likely a single epic.
3. **Test-contract fix**: test-effectiveness:F1 — assert exit-code expectation per fixture instead of accepting either.
4. **Unexport sweep**: 8 candidates, mechanical.

## Notes

- 6 linters with zero findings: ctx-value, io-parallel, n-plus-one, sqlite, tx-boundary, plus none-hit branches. fo is a stdin→stdout renderer; persistence/RPC/concurrency surface is intentionally small.
- Snipe bundle cache: `/tmp/snipe-bundle-f62c7fc3af14` (cleanup below).
- Each linter's full report + sidecar at `docs/feedback/review/<linter>-repo.md` and `.jsonl`.

## Rate findings

```
/assess-feedback <linter> --run-id=f62c7fc3af14
```

Run per-linter — six outcomes per finding written back to its sidecar, accept-ratio logged to trixi.
