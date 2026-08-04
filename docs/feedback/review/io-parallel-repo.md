# io-parallel — repo review

- **Run:** `7858b3a8ea1b`
- **Date:** 2026-07-27
- **Scope:** whole repo (project)
- **Findings:** 0

## Summary

Zero findings. This linter hunts sequential, independent RPC/HTTP/DB calls in one
function (three-plus I/O calls that don't feed each other, or a loop of per-item
independent I/O). `fo` has no such surface: it is a single-process streaming filter
(stdin → parse → `Report` IR → render → stdout), per
`.claude/rules/north-star.md` — "✗ Owning tool invocation — fo reads stdin, callers
run the tools." There are no service clients, no database, and no `errgroup` /
`sync/WaitGroup` parallel-I/O machinery anywhere in the module.

## Evidence (Read-verified)

- `head -1 go.mod` → `module github.com/dkoosis/fo` — single module, no RPC/DB
  client deps in the tree.
- `rg -n 'client\.\w+\(' --type go` → 0 matches.
- `rg -n 'errgroup\.|golang\.org/x/sync' --type go` → 0 matches. No parallel-I/O
  library imported anywhere.
- `rg -n 'http\.(Get|Post|Do)\(' --type go` → 0 matches. No HTTP client calls.
- `rg -n 'exec\.Command' --type go` → 3 matches, none in the same function: two
  are test-harness invocations (`pkg/view/pipeline_golden_test.go:40,81`), one is
  `cmd/fo/watch.go:217` — a single `exec.CommandContext` per watch tick, not a
  fan-out of independent calls.
- The one function chain with several sequential local-file I/O calls —
  `cmd/fo/main.go:349-354` calling, in order, `recordFullLog` →
  `attachDiff` → `assignAndPersistIDs` → `recordRun` (each defined in
  `cmd/fo/state.go`, each reading/writing a distinct `.fo/*.json` sidecar) — is
  not a `sequential-independent-rpc` candidate on inspection:
  - `recordFullLog` (`cmd/fo/state.go:58-76`) and `attachDiff`
    (`cmd/fo/state.go:18-51`) both `append` to the shared `r.Notices` slice on
    the same `*report.Report` passed in from `main.go`. Running them concurrently
    without added synchronization would race on that shared slice header —
    exactly the kind of "shared mutable side effect" the linter's own
    `sequential-independent-rpc` rule excludes ("ordered side effects").
  - `assignAndPersistIDs` (`cmd/fo/state.go:84-100`) mutates `r.Findings`/`r.Tests`
    in place (`report.AssignShortIDsStable`), so it is not read-only with respect
    to the other three.
  - All four operate on sub-kilobyte local JSON sidecars (`.fo/last-run.json`,
    `.fo/full.log`, `.fo/findings.json`, `.fo/run-log.json`) — not a
    latency-sensitive path per the linter's own framing ("independent I/O is a
    free win in latency-sensitive paths"). Parallelizing would trade a real (if
    small) correctness risk for a gain too small to matter.
- No `for _, x := range xs { io.Call(x) }` pattern exists over per-item I/O
  anywhere in `cmd/fo` or `pkg/wrapper/*` — checked each wrapper's `Convert`
  (wrapleaderboard, wrapdiag, waraparchlint, wrapgobench, wrapcoverprofile,
  wrapcover, wraparchlinttext, wrapjscpd): each is a single buffered
  reader → line-loop → single writer, no per-line network/DB call.

## Rule-by-rule

| Rule | Result |
|------|--------|
| `sequential-independent-rpc` | N/A — no RPC/HTTP clients; nearest candidate (`cmd/fo/main.go` state-sidecar chain) has real shared-state ordering constraints |
| `phased-mixed-deps` | N/A — no multi-call I/O function with mixed deps |
| `loop-independent-io` | N/A — no per-item I/O loop found in wrappers or cmd |
| `sequential-db-reads` | N/A — no DB |
| `not-using-errgroup-for-parallel-io` | N/A — no raw-goroutine parallel I/O anywhere |

## Architectural note

Unchanged since the last pass: fo's contract (north-star.md) keeps it a
single-process filter that never orchestrates remote calls, so this linter stays
structurally N/A for this repo. Re-run only if a future feature adds a service
client or a per-item fetch loop (e.g., a wrapper that resolves external
references over the network).
