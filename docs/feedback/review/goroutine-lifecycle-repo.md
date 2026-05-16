# goroutine-lifecycle — repo

scope: project · mode: report · go.mod: `go 1.24.0`

Production goroutine launch sites (test files excluded):

| # | Site | Owner |
|---|------|-------|
| 1 | `pkg/testjson/parser.go:92` — `go scanLoop(...)` | ctx + caller drains `lines`; caller closes `r` on cancel |
| 2 | `cmd/fo/main.go:907` — anonymous parser goroutine in `runStreamCtx` | ctx + `finalCh`/`saveErrCh` rendezvous |
| 3 | `cmd/fo/fswatch.go:146` — `go runWatcher(...)` | ctx; closes `out` and watcher on return |
| 4 | `cmd/fo/fswatch.go:208` — `go runDebounce(...)` | ctx; closes `out` on return |
| 5 | `cmd/fo/watch.go:119` — anonymous `stdinTriggers` goroutine | ctx **only on send**; blocked Read survives cancel |

Overall the codebase shows deliberate lifecycle work — every prod goroutine takes a ctx, most pair Close on the source reader with cancel (see the explicit comment in `pkg/testjson/parser.go:75-78` referencing fo-u2w). Findings below are real but small.

---

### 1. [F1] stdinTriggers goroutine cannot be canceled while Read is blocked

- **Site:** `cmd/fo/watch.go:117-131` — `stdinTriggers`
- **Owner:** none — fire-and-forget; nothing waits, nothing forces an unblock
- **Issue:** `goroutine-ignores-ctx` (asymmetric: ctx guards the send but not the underlying Read)
- **Code:**

```go
func stdinTriggers(ctx context.Context, r io.Reader) <-chan struct{} {
    ch := make(chan struct{})
    go func() {
        defer close(ch)
        sc := bufio.NewScanner(r)
        for sc.Scan() {                       // blocks in Read; ctx cannot interrupt
            select {
            case ch <- struct{}{}:
            case <-ctx.Done():
                return
            }
        }
    }()
    return ch
}
```

- **Why it matters:** `runWatch` (watch.go:92) wires `signal.NotifyContext` and may switch to `-source stdin`. On SIGINT, ctx fires, `watchLoop` returns, the process exits — and only the process exit unblocks `sc.Scan()`. The same project caught this exact failure mode in `pkg/testjson/parser.go` (see fo-u2w comment) by requiring an `io.ReadCloser` whose `Close` actually unblocks Reads. `stdinTriggers` accepts a bare `io.Reader`, so it can never adopt the same fix.
- **Fix:** mirror the parser pattern — take `io.ReadCloser` (or an `io.Reader` plus a Close hook), register `context.AfterFunc(ctx, func() { _ = r.Close() })` before launching the goroutine, and document that callers must pass a closable source. For `os.Stdin`, closing it is the standard way to unblock.

---

### 2. [F2] runStreamCtx parser goroutine: parseErrCh send is not select-guarded

- **Site:** `cmd/fo/main.go:907-929`
- **Owner:** caller blocks on `<-finalCh`; ctx threads through
- **Issue:** `goroutine-no-owner` (partial) — under one specific path the goroutine writes before the reader exists
- **Code:**

```go
parseErrCh := make(chan error, 1)        // buffered 1, single writer → unblocked send is fine
saveErrCh  := make(chan error, 1)
go func() {
    defer close(snapshots)
    _, err := testjson.Stream(ctx, rc, ...)
    if err != nil && !errors.Is(err, context.Canceled) && ... {
        parseErrCh <- err                 // ok: buffered, single writer
    }
    r := testjson.ToReport(agg.Results())
    saveErrCh <- attachDiff(...)          // ok: buffered 1, single writer
    finalCh   <- r                        // ok: buffered 1
    select {                              // last snapshot — guarded
    case snapshots <- *r:
    case <-ctx.Done():
    }
}()
```

- **Why it matters:** Today the channel sizes match the writer count, so this is correct. It's brittle: any future code that calls `parseErrCh <- err` from a second path, or shrinks the buffer, deadlocks the goroutine on cancel because the caller does a non-blocking `select { case perr := <-parseErrCh: default: }` and may have already returned through the renderErr path.
- **Fix:** make the writes ctx-guarded for consistency with the snapshot send, or add a comment locking down the "buffered=writer count, single sender" invariant. Cheapest fix:

```go
select { case parseErrCh <- err: case <-ctx.Done(): }
select { case saveErrCh  <- aerr: case <-ctx.Done(): }
select { case finalCh    <- r:    case <-ctx.Done(): }
```

---

### 3. [F3] snapshots channel buffer of 8 is a magic number

- **Site:** `cmd/fo/main.go:902` — `snapshots := make(chan report.Report, 8)`
- **Issue:** `chan-magic-buffer`
- **Code:**

```go
snapshots := make(chan report.Report, 8)
finalCh   := make(chan *report.Report, 1)
parseErrCh := make(chan error, 1)
saveErrCh  := make(chan error, 1)
```

- **Why it matters:** The 1s are obviously single-shot; 8 is undocumented. The companion `sendCoalesceSnapshot` (main.go:949) treats fullness as a drop signal, so 8 is effectively a smoothing window for fast parser / slow renderer. That intent should be in a comment. Without it, the next reader has to reverse-engineer it (and might "fix" it to 0 or 100).
- **Fix:** one-line comment such as `// 8: smoothing window for parser bursts; sendCoalesceSnapshot drops oldest on overflow`.

---

### 4. [F4] runWatcher swallows w.Errors silently

- **Site:** `cmd/fo/fswatch.go:170-173`
- **Issue:** not a lifecycle bug per se, but the `_, ok := <-w.Errors` arm discards every fsnotify error without logging. Combined with "runtime errors swallowed" in the comment at fswatch.go:131, this means a watcher that loses its inotify slot fails silently and watch becomes a no-op for the rest of the session — caller never learns its owner is degraded.
- **Fix:** at minimum, log the first error to stderr; ideally surface it through a side channel so `runWatch` can decide to exit non-zero. Out of strict scope for this linter — flagged as Info.

---

No findings for:

- `goroutine-uses-background-ctx` — every prod goroutine takes the caller's ctx.
- `closure-captures-loop-var` — `go.mod` declares Go 1.24; per-iter scoping is in effect.
- `closure-captures-pointer` — no shared pointer-mutation pattern across launches found.
- `time-sleep-as-sync` — zero `time.Sleep` calls outside `*_test.go`.
- `lock-held-during-call-out` — no mutex usage in prod paths under review.
- `chan-instead-of-waitgroup` — done-signaling uses channels because they also carry data or pair with ctx cancellation; no WG-substitute pattern found.

Concurrency scoring (per linter rubric):

| Tier | Verdict |
|------|---------|
| P1 ownership | 🟡 — one fire-and-forget (F1) |
| P1 ctx | 🟡 — one ctx gap (F1) |
| P1 shared state | 🟢 |
| P2 magic | 🟡 — one buffer (F3) |
| P2 lock reentry | 🟢 |
| P3 idiom | 🟢 |
