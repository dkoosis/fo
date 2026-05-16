# goroutine-lifecycle — diff 482cfd360229

**Date:** 2026-05-16
**Module:** github.com/dkoosis/fo (go 1.24.0)
**Scope:** 21 files (cmd/fo + pkg/{cluster,report,sarif,scene,suppress,testjson,view,wrapper/wraparchlint})

## Verdict

**Tier: 🟢 (with 2 🟡 borderline)**

Goroutines are confined to `cmd/fo/{watch.go,watchkey.go,main.go}`. The rest of the diff has no concurrency. Owners are mostly explicit and ctx-honoring; #266/#267 work already paid down the producer-side debt at `main.go:915`. Two design smells worth a comment, neither a bug.

| P | tier | notes |
|---|------|-------|
| P1 ownership | 🟡 | 1 fire-and-forget (readKeys) — best-effort by design |
| P1 ctx | 🟢 | every long-lived goroutine selects on `ctx.Done()` |
| P1 shared state | 🟢 | no closure-captures-loop-var, no pointer share without sync |
| P2 magic | 🟡 | one unjustified buffer size (`snapshots, 8`) |
| P2 lock reentry | 🟢 | no mutexes in diff |
| P3 idiom | 🟢 | fanIn/done patterns are correct primitive choices |

The `keyControl` restore-race is **fo-oq9** — not re-reported here.

## Findings

### 1. [F1] `cmd/fo/main.go:902` — chan-magic-buffer

- **Site:** `snapshots := make(chan report.Report, 8)`
- **Owner:** producer goroutine at `main.go:915` closes the channel via `defer close(snapshots)`; consumer is `view.RenderStream` at 936.
- **Issue:** `magic-buffer` — buffer of 8 has no comment explaining the choice. The producer sends "one snapshot per finished package"; 8 is neither 0 (sync handoff) nor 1 (single signal) nor an explicit backpressure design.
- **Code:**

```go
// main.go:902
snapshots := make(chan report.Report, 8)
// ... 915
go func() {
    defer close(snapshots)
    r, parseErr := runTestJSONPipeline(ctx, stdin, br, func(snap report.Report) {
        sendCoalesceSnapshot(ctx, snapshots, snap)
    })
```

- **Fix:** add a one-line comment explaining what 8 buys (e.g. "absorb burst of 8 package-finish events while RenderStream paints a frame, beyond that sendCoalesceSnapshot drops to keep the parser non-blocking"). The drop-on-full behavior in `sendCoalesceSnapshot` (line 974) is the actual backpressure design — wire the rationale to the buffer constant.

---

### 2. [F2] `cmd/fo/watchkey.go:57` — goroutine-no-owner (borderline)

- **Site:** `go readKeys(f, out, cancel)`
- **Owner:** none explicit. `keyControl` returns `(<-chan struct{}, func())`; the func only restores the TTY — it does not `Wait()` for `readKeys` to exit. Exit relies on the sibling goroutine at line 53 closing `f` after `ctx.Done()`, which makes `Read` return an error, which makes `readKeys` close `out` and return.
- **Issue:** `no-owner` — works in practice but the contract is "best effort"; a caller cannot deterministically observe goroutine exit. Combined with the sibling closer goroutine at line 53 (also unowned), keyControl spawns two goroutines whose only join point is process exit.
- **Code:**

```go
// watchkey.go:50
out := make(chan struct{}, 1)
// Best-effort: a blocking Read on the raw TTY can't be interrupted by ctx
// alone. Closing the descriptor from another goroutine unblocks it.
go func() {
    <-ctx.Done()
    _ = f.Close()
}()
go readKeys(f, out, cancel)
return out, restore
```

- **Fix:** if borderline-keep — add a comment to the rules.md style ("//lintbrush:disable=goroutine-lifecycle:no-owner — best-effort TTY reader; ctx-Done closes fd; restore is idempotent"). If tightening — return a `func() { <-doneReadKeys }` companion so `runWatch` can `defer` it after `restoreTTY` and the caller has a real join. Cost is small; payoff is determinism in tests.

---

### 3. [F3] `cmd/fo/watch.go:178` — Info (not flagged)

- **Site:** stdin-closer goroutine inside `stdinTriggers`.
- **Owner:** ✓ paired with `done` channel closed by the scanner goroutine at line 189. Clean two-goroutine handshake.
- **Note:** listed for the reviewer's audit trail; no action.

---

## Cross-references

- **fo-oq9** — keyControl restore race (deferred; not re-reported per task instructions).
- **#266, #267** — already addressed the producer/consumer race at `main.go:915` (resultCh + 2s grace). Verified clean.

## Cap

3 findings emitted (2 actionable, 1 info). Well under the 18-cap.
