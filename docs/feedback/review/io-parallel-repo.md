# io-parallel review — repo

run_id: f62c7fc3af14
scope: project
target: /Users/vcto/Projects/fo

## Summary

No findings. The io-parallel linter targets sequential independent I/O — RPC orchestras, parallel DB reads, fanout-able loops. fo is a stdin-to-stdout data pipeline with no surface for the smells this linter looks for.

## Orientation

- `rg 'http\.|client\.\w+\(|\.Exec\(|\.Query\(|rpc\.' --type go -g '!*_test.go'` — zero matches.
- `rg 'errgroup|sync\.WaitGroup|go func\(' --type go -g '!*_test.go'` — two `go func()` sites only:
  - `cmd/fo/main.go:907` — single goroutine reading from stdin (not parallel I/O).
  - `cmd/fo/watch.go:119` — single goroutine in the watch loop runner (not parallel I/O).
- File I/O is confined to:
  - `pkg/state/state.go` Load/Save — one read, one atomic write, sequenced by data dependency (load → mutate → save).
  - `pkg/state/metrics_history.go` — read/append/write to one sidecar.
  - `cmd/fo/watch.go` — `exec.CommandContext` runs the user-supplied command (the contract; serializing it is intentional).
  - `cmd/fo/fswatch.go` — open `.gitignore` once.

## Why nothing fires

- **sequential-independent-rpc / sequential-db-reads / phased-mixed-deps**: no RPC, HTTP, or DB clients in the tree. The architecture (CLAUDE.md) is `stdin → parse → diff → render → stdout`.
- **loop-independent-io**: the only ranged loops walk parsed report sections (`pkg/report`, `pkg/view`) — pure in-memory transforms, no per-iteration I/O.
- **not-using-errgroup-for-parallel-io**: there is no parallel I/O, with or without errgroup.

## Verdict

io-parallel is not a meaningful axis for this codebase. If fo ever grows network adapters or concurrent file scanning across many roots, re-run; for the current renderer pipeline, sequential is by design and the right choice.
