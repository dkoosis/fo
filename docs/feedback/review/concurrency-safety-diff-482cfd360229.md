---
review: concurrency-safety
review_date: 2026-05-16
race_detector_run: false
race_detector_log: (not run — diff-scoped audit)
scope: focused diff (PR #281 watch/watchkey + adjacent files)
severity_counts:
  critical: 0
  high: 2
  medium: 3
  info: 1
packages_analyzed:
  - cmd/fo (main.go, suppress.go, watch.go, watchkey.go)
  - pkg/cluster, pkg/scene, pkg/sarif, pkg/testjson (toreport)
  - pkg/report, pkg/suppress, pkg/view (scene_*), pkg/wrapper/wraparchlint
verdict: minor
notes: |
  fo-oq9 (sync.Once for term.Restore in watchkey) is already filed and not
  re-reported. Findings below are adjacent issues in the same diff.
  pkg/cluster, pkg/scene, pkg/sarif, pkg/testjson/toreport, pkg/view/scene_*,
  pkg/report, pkg/suppress, pkg/wrapper/wraparchlint, pkg/report/filter,
  cmd/fo/suppress.go contain no goroutines, channels, or shared mutable
  state — pure data transformation. Concurrency surface is confined to
  cmd/fo/{main,watch,watchkey}.go.
---

# Concurrency-safety audit — diff 482cfd360229

## Findings

### 1. [F1] `cmd/fo/watchkey.go:53-57` — data-race

**File:** `cmd/fo/watchkey.go:53`
**Function:** `keyControl` (the `go func() { <-ctx.Done(); _ = f.Close() }()` watcher)
**Severity:** High
**Category:** race

**Sequence:**
1. `keyControl` launches goroutine G1 that calls `f.Close()` on ctx cancel.
2. `readKeys` (G2) is concurrently calling `r.Read(buf)` on the same `*os.File`.
3. After ctx cancellation, G1's `f.Close()` and G2's blocked `Read` race on the file descriptor state.

In Go, `*os.File.Close` + `*os.File.Read` are safe — the runtime serializes them — but `f.Close()` here runs unconditionally on *every* ctx cancel, including the normal `q`/Ctrl-C path where `readKeys` has *already* returned (the `cancel()` it calls triggers ctx.Done and G1 then closes the fd). That double-shutdown sequence is benign for `*os.File` but the `restore` closure that follows can race: `restoreTTY` is called from the deferred path in `runWatch` (main goroutine) and there is no happens-before edge guaranteeing `f.Close()` completes before `term.Restore(fd, oldState)` runs on the same fd.

Worse: after `f.Close()`, `fd` is no longer owned by this process; if the runtime reuses it before `term.Restore` fires, `term.Restore` will reconfigure an unrelated descriptor (`tcsetattr` on the recycled fd). This is the classic close-then-use-fd UAF.

**Code:**
```go
go func() {
    <-ctx.Done()
    _ = f.Close()      // closes fd
}()
go readKeys(f, out, cancel)
// ...later, runWatch defers:
defer restoreTTY()     // → term.Restore(fd, oldState) on a possibly-recycled fd
```

**Fix:** Sequence Close *after* Restore, not before; or guard Close with the same `sync.Once` proposed for restore (fo-oq9) and run Restore first:
```go
restore := func() {
    once.Do(func() {
        _ = term.Restore(fd, oldState)  // restore BEFORE close
        _ = f.Close()
    })
}
// drop the separate close goroutine; use a Read deadline or non-blocking poll instead,
// or accept that readKeys parks until the next keypress (the process is exiting).
```

**Repro:** `go test -race -run TestWatch ./cmd/fo/...` with rapid ctx cancellation under a TTY harness.

---

### 2. [F2] `cmd/fo/watchkey.go:53` — goroutine-leak

**File:** `cmd/fo/watchkey.go:53`
**Function:** `keyControl` (close-on-cancel goroutine)
**Severity:** High
**Category:** leak

**Sequence:**
1. Caller invokes `keyControl(ctx, …)` and decides not to use the returned channel (e.g. `opts.source == "stdin"` — the early-return paths handle this, fine).
2. In the *active* path, `runWatch` returns normally (watchLoop exits because triggers closed, not ctx cancel). `stop()` then runs from `signal.NotifyContext`'s defer.
3. The close-on-cancel goroutine wakes, calls `f.Close()` on `os.Stdin` — closing the process's stdin descriptor as a side effect of normal exit.

Closing `os.Stdin` on every normal `fo watch` exit is observable: any caller that pipes into `fo watch -- cmd` and then continues reading from a tee'd stdin sees EOF prematurely. The goroutine itself doesn't leak (ctx.Done fires when `stop()` runs), but it has an unintended side effect coupled to the leak-prevention mechanism.

**Code:** `cmd/fo/watchkey.go:53-56`

**Fix:** Only close on actual cancellation, not normal exit. Use a sentinel channel:
```go
done := make(chan struct{})
defer close(done)  // in restore, signal normal exit
go func() {
    select {
    case <-ctx.Done():
        _ = f.Close()
    case <-done:
    }
}()
```
Mirror the pattern already used in `stdinTriggers` (watch.go:177-185).

**Repro:** `printf 'q\n' | fo watch -- echo hi; cat <<<'leftover'` — observe leftover behavior on stdin after watch exits.

---

### 3. [F3] `cmd/fo/main.go:1028-1035` — race / double-close

**File:** `cmd/fo/main.go:1028`
**Function:** `runTestJSONPipeline`
**Severity:** Medium
**Category:** race

**Sequence:**
1. `context.AfterFunc(ctx, func() { _ = c.Close() })` registers a Close on ctx cancel using the stdin closer.
2. `bufioReadCloser.Close()` *also* calls `closerOf(stdin).Close()` — the *same* underlying closer.
3. If `testjson.Stream` calls `rc.Close()` (e.g. on its own cancel path) while AfterFunc is concurrently firing, two goroutines call `Close()` on the same `*os.File`.

`*os.File.Close` is documented as safe to call repeatedly (returns `ErrClosed`), but the contract for *arbitrary* `io.Closer` (e.g. a pipe wrapper in tests) is not. The defer-based `stopClose` only deregisters AfterFunc if it hasn't already started running — there is no synchronization between an in-progress AfterFunc callback and the deferred `rc.Close()` path.

**Code:**
```go
if c, ok := stdin.(io.Closer); ok {
    stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
    defer stopClose()
}
rc := &bufioReadCloser{Reader: br, closer: closerOf(stdin)}  // same closer
```

**Fix:** Pick one owner. Either rely on `rc.Close()` (called by testjson.Stream on cancel) and drop the AfterFunc, or guard the closer with `sync.Once`:
```go
var closeOnce sync.Once
closeFn := func() { closeOnce.Do(func() { _ = c.Close() }) }
context.AfterFunc(ctx, closeFn)
rc := &bufioReadCloser{Reader: br, closer: closerFunc(closeFn)}
```

**Repro:** `go test -race ./cmd/fo/... -run TestRunStream` with a custom closer that panics on second call.

---

### 4. [F4] `cmd/fo/main.go:915-934` — channel-block-forever (suspected)

**File:** `cmd/fo/main.go:929-933`
**Function:** `runStreamCtx` producer goroutine
**Severity:** Medium
**Category:** deadlock

**Sequence:**
1. Producer finishes pipeline, sends `streamResult` to `resultCh` (cap=1, non-blocking).
2. Producer then attempts `snapshots <- *r` to push a final snapshot.
3. If `view.RenderStream` has already returned (it bails on ctx.Done or stream errors) and ctx is *not yet cancelled*, the producer blocks on `snapshots <-` forever — nobody is draining `snapshots`.

The select guards with `<-ctx.Done()` but only the normal-cancel path. If `RenderStream` returns due to its *own* error (`renderErr != nil` path that isn't context.Canceled) before ctx is cancelled, the main goroutine then blocks on `res = <-resultCh` waiting for the producer that is itself blocked on the snapshots send.

**Code:**
```go
resultCh <- streamResult{...}              // OK, buffered cap=1
select {
case snapshots <- *r:                       // can block if RenderStream is gone but ctx is live
case <-ctx.Done():
}
```

**Fix:** Add a `default` case (drop the final snapshot if no consumer), or close `snapshots` *before* sending result and have RenderStream signal it's done via a separate signal:
```go
select {
case snapshots <- *r:
case <-ctx.Done():
default:  // renderer gone, drop final snapshot
}
```

**Repro:** Inject a RenderStream that returns non-cancel error immediately; observe runStreamCtx hang until the 2s timeout fires.

---

### 5. [F5] `cmd/fo/watch.go:172-200` — context-not-propagated

**File:** `cmd/fo/watch.go:187`
**Function:** `stdinTriggers` scanner goroutine
**Severity:** Medium
**Category:** leak

**Sequence:**
1. `stdinTriggers` launches the scanner goroutine reading from `r`.
2. If `r` is *not* an `io.Closer` (test reader, third-party wrapper), there is no close goroutine.
3. On ctx cancel, the scanner is parked in `sc.Scan()` reading `r`. Nothing unblocks it. Goroutine leaks for the rest of the process lifetime.

The comment at line 168-171 acknowledges this: "for non-closable readers (strings.Reader in tests, pipes we don't own) the reader goroutine remains parked until the next byte or EOF — by then the process is usually exiting anyway."

"Usually exiting" is fine in `main`, but `runWatch` is also reachable from tests where the process is *not* exiting — each test that hits the non-closable branch leaks a goroutine for the test binary's lifetime, masking other leaks under `goleak`.

**Code:** `cmd/fo/watch.go:177-185, 187-199`

**Fix:** In tests, always pass a closable reader (wrap `strings.Reader` in a `nopCloser` that gets explicitly closed by the test). Or change `stdinTriggers` to require an `io.ReadCloser`. Either way, document the test-only constraint inline.

**Repro:** `go test -count=100 ./cmd/fo/... -run TestWatchLoop` under goleak — count parked goroutines grows linearly.

---

### 6. [F6] `cmd/fo/watchkey.go:41-48` — race (restore flag)

**File:** `cmd/fo/watchkey.go:41`
**Function:** `keyControl.restore` closure
**Severity:** Medium
**Category:** race

**Sequence:**
1. `restored bool` is captured by the `restore` closure.
2. fo-oq9 already notes the need for `sync.Once`. Adjacent issue: even with `sync.Once`, multiple call sites currently exist — `defer restoreTTY()` in `runWatch` is the only documented caller, but `cancel()` from `readKeys` triggers ctx.Done which fires the close-goroutine, which does *not* call restore.

The current `restored bool` is read+written without synchronization. The race is between a `defer restoreTTY()` on the main goroutine and any future caller (an audit-vulnerable surface as code grows). Not a present bug given the single caller, but the pattern invites the regression.

**Fix:** Use `sync.Once` (fo-oq9 covers this).

This finding is adjacent context for fo-oq9 — the `restored bool` is the specific primitive that needs replacement. Not double-reporting; flagging the implementation choice.

---

### 7. [F7] `cmd/fo/watchkey.go:99-129` — info: fanIn correctness

**File:** `cmd/fo/watchkey.go:99`
**Function:** `fanIn`
**Severity:** Info
**Category:** leak

**Sequence:** Reviewed for correctness. The pattern is sound:
- Closes `out` when both `a` and `b` are nil (drained).
- Honors ctx.Done at every select.
- Nil-channel idiom correctly blocks the disabled case.

No defect. Noting because fan-in with two open-ended sources is a classic leak shape and this implementation gets it right — useful baseline for future reviewers.

---

## Summary

Verdict: **minor**. No P0 (race/deadlock proven). Two P1-High findings in `watchkey.go` (F1 close-then-restore UAF window; F2 unintended `os.Stdin` close on normal exit). Three P1-Medium findings (F3 double-close path in runTestJSONPipeline; F4 final-snapshot send can block; F5 scanner leak in non-closable test path). One Info (F7 fanIn baseline).

Recommend fixing F1 + F2 alongside fo-oq9 (they share the restore/close sequencing surface). F3 + F4 are pre-existing in main.go but are cheap to harden. F5 is a test-hygiene cleanup, not production risk.
