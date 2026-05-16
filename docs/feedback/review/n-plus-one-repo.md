# n-plus-one — repo

run_id: f62c7fc3af14
scope: project
mode: report
target: /Users/vcto/Projects/fo

## Summary

**No findings.**

The N+1 query/RPC pattern does not apply to this codebase. `fo` is a stdin→stdout build-output renderer (SARIF + `go test -json` → human/llm/json). It has no database client, no RPC client, no service wrappers, no repos. Its only I/O surfaces:

- **stdin** — `internal/boundread` / `internal/lineread`, read once at startup
- **state sidecar** — `pkg/state/state.go`, `pkg/state/metrics_history.go`: single `os.ReadFile`/`os.WriteFile` on `.fo/last-run.json` per run, not per-row
- **fsnotify watcher** — `cmd/fo/fswatch.go`: one `filepath.WalkDir` at startup to seed watch dirs; a single `os.Stat` per create-event (canonical FS traversal, not a per-row lookup against an outer query)
- **subprocess** — `cmd/fo/watch.go:144`: one `exec.CommandContext` per debounced trigger (one process per event, not N per outer query)

## Per-phase results

| Phase | Result |
|---|---|
| P1 direct (loop body calls I/O parameterized by loop var) | none — non-test loops iterate parsed in-memory structures (SARIF runs/results, testjson events, report findings, render rows); the I/O happens before/after the loop, not inside |
| P1 indirect (wrapper hides per-row I/O) | none — no service/repo layer; wrappers under `pkg/wrapper/*` are pure converters (bytes→SARIF→Report) |
| P1 nested N×M | none |
| P2 errgroup-masks-batch | none — `rg errgroup` returns 0 hits in non-test code |
| P3 preload candidates | n/a — no repo API to extend |

## Verification commands

```bash
rg -n 'errgroup|g\.Go\(' --type go -g '!*_test.go'       # 0 hits
rg -n 'for .* range' --type go -g '!*_test.go' -A 8 \
  | rg -B 1 'os\.(Open|ReadFile|Stat|WriteFile|Create)|exec\.Command|filepath\.Walk'
# only hit: cmd/fo/fswatch.go:98 — filepath.WalkDir as the loop body itself (canonical traversal, not N+1)
```

## Tier: green

Architecture precludes the pattern. Revisit only if `fo` grows a network client, persistent store, or per-finding enrichment via external service.
