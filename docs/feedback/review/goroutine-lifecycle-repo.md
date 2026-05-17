# goroutine-lifecycle — repo

run_id: bd775e303d86-goroutine-lifecycle
target: repo
go.mod: `go 1.24.0` (loop-var capture rule downgraded)

## Scorecard

| Phase | Tier | Notes |
|---|---|---|
| P1 ownership | 🟢 | every prod goroutine has owner via ctx + close/resultCh/WaitGroup |
| P1 ctx honoring | 🟢 | all long-lived loops select on `ctx.Done()` |
| P1 shared state | 🟢 | no captured-pointer aliasing found; 1.24 loop-var safe |
| P2 magic buffers | 🟡 | one undocumented buffer (`snapshots`, N=8) |
| P2 time.Sleep as sync | 🟡 | 4 test sites use Sleep to wait for goroutine readiness |
| P2 lock reentry | 🟢 | not observed |
| P3 idiom inversions | 🟢 | WaitGroup / chan choices defensible |

5 findings.

---

### 1. [F1] `cmd/fo/main.go:920` — chan-magic-buffer

**Site:** `snapshots := make(chan report.Report, 8)`
**Owner:** producer goroutine at `main.go:933` closes `snapshots` on exit; consumer is `view.RenderStream` (line 959).
**Issue:** magic-buffer
**Evidence (Read-verified):** No comment near line 920 explains why 8. Adjacent comments justify `resultCh` (buffered=1, for race #267) and the streaming ruleset, but the choice between 0/1/4/8/16 for `snapshots` is silent. The coalescing test (`stream_coalesce_test.go`) demonstrates the channel must not block the parser under a slow renderer — implying the buffer is sized to swallow short bursts while `sendCoalesceSnapshot` drops duplicates. That design rationale belongs in a comment at the buffer.
**Code:**
```go
snapshots := make(chan report.Report, 8)
// resultCh carries the producer goroutine's terminal state. Using a
// single struct + blocking receive (a) ensures the producer is fully
// finished before main inspects parseErr (#267 race), and (b) lets
// main bound the wait via ctx + grace timeout so a wedged
// attachDiff/state.Save doesn't deadlock fo (#266).
type streamResult struct { ... }
resultCh := make(chan streamResult, 1)
```
**Fix:** Add a comment naming the design intent, e.g. `// 8 = burst tolerance for package-finish events; coalescer drops duplicates when renderer is slow.` Or reduce to 1 if the coalescer makes additional slots dead capacity.
**Tier:** 🟡 P2

---

### 2. [F2] `cmd/fo/stream_cancel_test.go:81` — time-sleep-as-sync

**Site:** `time.Sleep(50 * time.Millisecond)` between starting `runStreamCtx` goroutine and calling `cancel()`.
**Owner:** test goroutine via `done := make(chan int, 1)`.
**Issue:** time-sleep-as-sync
**Evidence (Read-verified):** Comment says `// Give the streamer a moment to consume the initial events.` Classic shape — wait for the other goroutine to reach a state. The test then asserts cancel honored within 500ms, so the sleep value is racing with the assertion bound. A deterministic signal (e.g. wait for first stdout write, or expose a `ready` channel from `runStreamCtx`) would be flake-proof.
**Code:**
```go
done := make(chan int, 1)
go func() {
    done <- runStreamCtx(ctx, prod, br, &stdout, theme.Mono(), "", true, false, &stderr)
}()
// Give the streamer a moment to consume the initial events.
time.Sleep(50 * time.Millisecond)
start := time.Now()
cancel()
```
**Fix:** Inject a signal — e.g., a `progressWriter` like `pkg/view/stream_test.go` uses; wait on the first signal, then cancel. Or accept the generous bound and tag with `//lintbrush:disable=goroutine-lifecycle:time-sleep-as-sync` + rationale.
**Tier:** 🟡 P2 (borderline — assertion has 500ms upper bound)

---

### 3. [F3] `cmd/fo/fswatch_test.go:125` — time-sleep-as-sync

**Site:** `time.Sleep(50 * time.Millisecond)` after `watchTree(ctx, dir)` and before writing the trigger file.
**Owner:** `watchTree` goroutine owned by ctx (timeout 3s); cleanup via `defer cancel()`.
**Issue:** time-sleep-as-sync
**Evidence (Read-verified):** Comment: `// Give the watcher a moment to start.` `fsnotify.NewWatcher` returns before its internal kqueue/inotify thread is fully armed; the test assumes 50ms is enough. Real flakes here are a well-known fsnotify pattern.
**Code:**
```go
events, err := watchTree(ctx, dir)
if err != nil { t.Fatalf(...) }
// Give the watcher a moment to start.
time.Sleep(50 * time.Millisecond)
if err := os.WriteFile(filepath.Join(dir, "x.go"), ...); err != nil { ... }
```
**Fix:** Either retry the file-write in a small loop until an event lands (bounded by the 3s ctx), or have `watchTree` return a `ready` channel closed after the first `addRecursive` succeeds. The 2s outer bound absorbs current 50ms slop, so this is borderline.
**Tier:** 🟡 P2 (borderline — outer assertion has 2s bound)

---

### 4. [F4] `cmd/fo/fswatch_test.go:150` — time-sleep-as-sync

**Site:** Same pattern as F3, in `TestWatchTree_IgnoresVendorDir`.
**Owner:** same as F3.
**Issue:** time-sleep-as-sync
**Evidence (Read-verified):** Identical shape and comment intent — the negative assertion ("should not emit") only has a 400ms ceiling. If watcher startup ever drifts past 400ms, this test silently passes for the wrong reason (event was never observable, not because vendor was filtered).
**Code:**
```go
events, err := watchTree(ctx, dir)
if err != nil { ... }
time.Sleep(50 * time.Millisecond)
if err := os.WriteFile(filepath.Join(vendor, "ignored.go"), ...); err != nil { ... }
select {
case <-events:
    t.Fatal("watchTree: should not emit for files under vendor/")
case <-time.After(400 * time.Millisecond):
```
**Fix:** Write a *non-ignored* sentinel file first, wait for its event to confirm the watcher is armed, then write the ignored file and assert no event. That actually tests "ignored", not "happened to race".
**Tier:** 🟡 P2 (negative-assertion flake risk is higher than F3)

---

### 5. [F5] `cmd/fo/watch_test.go:86` — time-sleep-as-sync

**Site:** `time.Sleep(time.Millisecond)` inside polling loop waiting for first invocation.
**Owner:** `watchLoop` goroutine via `done := make(chan struct{})`.
**Issue:** time-sleep-as-sync
**Evidence (Read-verified):**
```go
deadline := time.After(time.Second)
for calls.Load() == 0 {
    select {
    case <-deadline:
        t.Fatal("watchLoop: initial call never observed")
    default:
        time.Sleep(time.Millisecond)
    }
}
```
A busy-poll with 1ms sleep ticks; cheap and bounded by 1s deadline. Functionally fine but a `chan struct{}` close from the first call site would be both faster and deterministic.
**Fix:** Test wraps the callback: `firstCall := make(chan struct{}); fn := func() { calls.Add(1); select { case firstCall <- struct{}{}: default: } }`; then `<-firstCall`.
**Tier:** 🟡 P2 (borderline — bounded busy-poll is acceptable; flagged for principle)

---

## Notes (not findings)

- **`cmd/fo/watchkey.go:58` ctx-watcher goroutine** — fire-and-forget but ownership is precise: parks on `<-ctx.Done()`, then `restore()` (idempotent via `sync.Once`) and `_ = f.Close()`. Sibling `readKeys` goroutine unblocks via fd Close. No leak on cancel.
- **`pkg/testjson/parser.go:92` `scanLoop`** — fire-and-forget but owner is `Stream`, which calls `r.Close()` on ctx cancel; comment at line 78 explicitly calls out the leak hazard if a non-closable reader is passed. Design-aware.
- **`cmd/fo/main.go:933` producer goroutine** — owner is `runStreamCtx` via blocking `<-resultCh` with 2s grace after ctx cancel. Race #267 documented inline.
- **Loop-var capture** — `go.mod` declares `go 1.24.0`, so the rule is not actionable.
- **No `context.Background()` substitution** found in handler-spawned goroutines.

## Cap

5 findings (cap 10). Repo is already well-disciplined; remaining items are test-suite Sleep usage and one undocumented buffer constant.
