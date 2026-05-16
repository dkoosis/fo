---
review_date: 2026-05-16
linter: concurrency-safety
scope: project
target: repo
race_detector_run: true
race_detector_result: clean
race_detector_log: (inline; no findings)
packages_analyzed:
  - cmd/fo
  - pkg/testjson
  - pkg/view
  - pkg/state
severity_counts:
  critical: 0
  high: 0
  medium: 2
  info: 3
overall_tier: green
---

# Concurrency safety — repo

`go test -race ./...` ran clean across all 23 packages (~37s for cmd/fo + pkg/testjson, the only goroutine-heavy units). The concurrency surface is small and well-bounded: production goroutines live in `cmd/fo/fswatch.go` (watcher + debouncer), `cmd/fo/watch.go` (stdin trigger), `cmd/fo/main.go` (`runStream` parser), and `pkg/testjson/parser.go` (`scanAsync`). No `sync.Mutex`/`RWMutex`/`WaitGroup` in production code — coordination is exclusively channels + context. Channel ownership is single-producer everywhere; no multi-closer risk.

Findings below are all P1–P3 — latent shape issues, not confirmed defects. Nothing in P0.

## Findings

### 1. [F1] runStream producer can block on `snapshots` send after renderer returns

- **Rule:** channel-block-forever
- **File:** `cmd/fo/main.go:925`
- **Function:** `runStreamCtx` (anonymous producer goroutine)
- **Severity:** Medium
- **Category:** leak (transient — bounded by deferred `stop()`)
- **Sequence:**
  1. `testjson.Stream` returns; producer sends `parseErrCh`/`saveErrCh`/`finalCh` (all cap 1, non-blocking).
  2. Producer enters `select { case snapshots <- *r: case <-ctx.Done(): }`.
  3. Main goroutine has already read `<-finalCh` and may return on `renderErr != nil` path *before* draining `snapshots`; the buffered chan (cap 8) can still hold the last in-flight snapshot, so the send blocks.
  4. Only `defer stop()` in `runStream` (line 883) eventually cancels ctx and unblocks the producer.
- **Code:**
  ```go
  // cmd/fo/main.go:902-929
  snapshots := make(chan report.Report, 8)
  ...
  go func() {
      defer close(snapshots)
      ...
      saveErrCh <- attachDiff(...)
      finalCh   <- r
      select {
      case snapshots <- *r:   // can block if buffer full + renderer returned
      case <-ctx.Done():
      }
  }()
  renderErr := view.RenderStream(ctx, stdout, snapshots, t, width)
  final := <-finalCh
  ```
- **Impact:** Goroutine remains scheduled until the deferred `stop()` fires on function return — practically not a leak, but the producer is doing useful cleanup (`close(snapshots)`) that is delayed. Becomes a real leak if a future refactor moves `runStreamCtx` into a longer-lived context (e.g. `runWatch` loop) where the cancel isn't immediate.
- **Fix:** Drop the trailing `snapshots <- *r` — the final report is already delivered via `finalCh`. Or move the send before `finalCh <- r` so the renderer (still draining at that point) consumes it. Confirmed via code path; not race-detector observable because the send eventually succeeds via ctx.Done.
- **Repro:** Build with `runStreamCtx` extracted into a long-lived ctx; or inject a `RenderStream` that returns early on first snapshot — producer will sit on the send until ctx cancellation.

### 2. [F2] `runDebounce` swallows a final emission when input closes mid-burst and ctx cancels

- **Rule:** channel-block-forever (edge case)
- **File:** `cmd/fo/fswatch.go:253`
- **Function:** `flushPending`
- **Severity:** Medium
- **Category:** correctness (lost event under shutdown)
- **Sequence:**
  1. Input channel `in` closes while a debounce timer is armed.
  2. `runDebounce` calls `flushPending`, which selects `out <- struct{}{}` vs `ctx.Done()`.
  3. If ctx is also canceled (typical shutdown — fs watcher closes its output channel on ctx cancel; see `runWatcher:152`), both cases are ready and Go picks pseudo-randomly. The trailing change can be silently dropped.
- **Code:**
  ```go
  // cmd/fo/fswatch.go:252-261
  func flushPending(ctx context.Context, timer *time.Timer, out chan<- struct{}) {
      if timer == nil { return }
      select {
      case out <- struct{}{}:
      case <-ctx.Done():
      }
  }
  ```
- **Impact:** Under `fo watch` shutdown a pending burst is lost. Mostly harmless (process is exiting), but if `watchLoop` is reused in a longer-lived caller the lost trigger means a stale "last result". Latent.
- **Fix:** Either document that shutdown drops pending emissions, or prefer the send by attempting non-blocking `out <- struct{}{}` first, then fall back to the select. Acceptable as-is if a comment notes the intent.
- **Repro:** Send N events to `in`, close `in`, immediately call `cancel()` from another goroutine. Observe whether the final emission lands. Non-deterministic.

### 3. [F3] `stdinTriggers` goroutine survives until child reader EOFs even after ctx cancel

- **Rule:** goroutine-leak (bounded)
- **File:** `cmd/fo/watch.go:117-131`
- **Function:** `stdinTriggers`
- **Severity:** Info
- **Category:** leak (cooperative)
- **Sequence:**
  1. `bufio.Scanner.Scan()` blocks on `os.Stdin.Read`.
  2. ctx is canceled (SIGINT). The goroutine only checks ctx inside the inner send-select; it does not interrupt the in-flight `Read`.
  3. Goroutine remains blocked in `Scan()` until stdin closes (EOF) or yields a line.
- **Impact:** For `fo watch -source stdin`, Ctrl-C cancels the run loop but leaves this goroutine parked on stdin until the parent shell closes it (which usually happens right after). In practice it exits cleanly because `runWatch` returns and process termination tears the goroutine down.
- **Fix:** Mirror the pattern used in `runStreamCtx` — use `context.AfterFunc(ctx, func() { _ = stdinCloser.Close() })` to force the read to unblock. Only worth doing if `stdinTriggers` is ever reused in a non-process-lifetime context.
- **Repro:** N/A — observable only as goroutine in pprof at moment of cancel.

### 4. [F4] `runStream` reads `parseErrCh` with `default` — silent race on slow goroutine

- **Rule:** channel-block-forever (anti-pattern)
- **File:** `cmd/fo/main.go:933-938`
- **Function:** `runStreamCtx`
- **Severity:** Info
- **Category:** correctness (probabilistic, currently impossible)
- **Sequence:**
  1. Producer order: `parseErrCh <- err` (only if err) → `saveErrCh <- ...` → `finalCh <- r`.
  2. Main reads `<-finalCh` (blocks), so by the time the `select` on `parseErrCh` runs, the send (if any) has already happened. Today this is safe.
  3. If the producer order is ever reordered (e.g. `finalCh <- r` moves above `parseErrCh <- err`), the `default` branch silently swallows the error. No tests would catch it.
- **Code:**
  ```go
  // cmd/fo/main.go:931-938
  renderErr := view.RenderStream(ctx, stdout, snapshots, t, width)
  final := <-finalCh
  select {
  case perr := <-parseErrCh:
      fmt.Fprintf(stderr, "fo: %v\n", perr)
      return 2
  default:
  }
  ```
- **Fix:** Either close `parseErrCh` in the producer on the no-error path (so main can use a blocking `<-parseErrCh` that returns the zero value), or document the strict ordering invariant near the channel declarations.
- **Repro:** Reorder producer sends; observe parse errors silently dropped.

### 5. [F5] `runChildAndRender` passes ctx to `exec.CommandContext` but ignores child exit signal

- **Rule:** context-not-propagated (mild)
- **File:** `cmd/fo/watch.go:144-147`
- **Function:** `runChildAndRender`
- **Severity:** Info
- **Category:** context
- **Sequence:**
  1. `exec.CommandContext(ctx, ...)` is used — good; ctx cancel will SIGKILL the child.
  2. `_ = c.Run()` discards the error; if ctx canceled mid-run, the buffered stdout may be partial but is still rendered via `run()`.
  3. The follow-on `run()` call doesn't take ctx, so a slow render after cancel doesn't abort.
- **Impact:** Minor — render path is CPU-bound and short. Worth noting because the watch loop's "single-flight" guarantee (`watchLoop:68` comment) presumes the run completes; if a render hangs, ctx cancel does not interrupt it.
- **Fix:** None required for current behavior. If render hang ever becomes a concern, plumb ctx into `run()`.

## Patterns observed (positive)

- All production goroutines own exactly one channel and close it via `defer` on the goroutine body.
- `sendCoalesceSnapshot` (main.go:955) correctly drops stale snapshots using single-producer invariant; explicitly documents the invariant.
- `Stream` (testjson/parser.go:79) cleanly hands ctx-cancel to a `r.Close()` call that interrupts in-flight `Read` (fo-u2w noted in comment) — exemplary cancellation hygiene.
- No `time.Sleep` used for synchronization in production code.
- No `sync.Map` misuse — no mutexes at all in production.
- No errgroup, no multi-lock acquisition — lock-order deadlock surface is zero.

## Coverage notes

- Race detector run: `go test -race -timeout=2m -count=1 ./...` → all 23 packages green (~37s longest: cmd/fo and pkg/testjson).
- Snipe cache loaded from `/tmp/snipe-bundle-f62c7fc3af14`.
- No P0 findings. Overall tier: 🟢.
