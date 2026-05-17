# io-parallel — repo review

- **Run:** `bd775e303d86-io-parallel`
- **Date:** 2026-05-17
- **Scope:** whole repo
- **Findings:** 0

## Summary

No findings. `fo` is a streaming presentation filter — stdin → IR (`Report`) → stdout
render. The smell io-parallel hunts (sequential independent RPC/HTTP/DB calls in a
single function) requires multi-client orchestration that this codebase does not do.

## Evidence (Read-verified)

- `head -1 go.mod` → `module github.com/dkoosis/fo` (single module, no service clients).
- `rg 'errgroup\.|golang\.org/x/sync'` → 0 matches. No parallel-IO library in use.
- `rg 'http\.|client\.|\.Get\(|\.Post\(|\.Exec\(|\.Query\(|\.Fetch\('` → 0 matches in
  non-test source. No HTTP, RPC, or DB clients.
- I/O surface is filesystem-only and minimal:
  - `pkg/state/state.go:94` `os.ReadFile` — single sidecar `.fo/last-run.json`.
  - `pkg/state/state.go:172` `os.Open` — directory handle for atomic rename.
  - `pkg/state/metrics_history.go:55` `os.ReadFile` — single history file.
  - `cmd/fo/suppress.go:46`, `cmd/fo/suppress_cmd.go:200` `os.Open` — single
    suppress-config read.
  - `cmd/fo/fswatch.go:39` `os.Open` — `.gitignore`.
  - `cmd/fo/fswatch.go:98` `filepath.WalkDir` — watch-mode tree scan (inherently
    serial directory traversal; not parallelizable per io-parallel's contract).
  - `cmd/fo/watch.go:217` `exec.CommandContext` — single user-supplied command per
    watch tick.
- Goroutines that exist (`pkg/testjson/parser.go`, `cmd/fo/watch.go`,
  `cmd/fo/watchkey.go`, `cmd/fo/main.go:933`, `cmd/fo/fswatch.go`) are single
  scanner/reader producers, not parallel-IO fanouts. Out of scope per linter
  prelude ("defer goroutine lifecycle/safety to /review goroutine-lifecycle").

## Rule-by-rule

| Rule | Result |
|------|--------|
| `sequential-independent-rpc` | N/A — no RPC/HTTP clients |
| `phased-mixed-deps` | N/A — no multi-call I/O functions |
| `loop-independent-io` | N/A — no per-item I/O loops |
| `sequential-db-reads` | N/A — no DB |
| `not-using-errgroup-for-parallel-io` | N/A — no raw-goroutine parallel I/O |

## Architectural note

Per `.claude/rules/north-star.md`, fo's contract is "stdin → IR → render. ✗ owning
tool invocation." Callers run tools and pipe results in; fo never orchestrates
remote calls. io-parallel will remain N/A for this repo as long as that contract
holds. Consider skipping this linter in fo's lintbrush rotation.
