# goroutine-lifecycle — fo (project scope)

Run: `7858b3a8ea1b` · reviewer: goroutine-lifecycle · scope: project · `go.mod`: `go 1.24.0`

## Summary

fo's production concurrency is carefully owned. Every `go` launch in non-test
code is ctx-scoped: `runWatcher`, `runDebounce`, `fanInLoop`, `readKeys`, the
`scanLoop` reader, and the `stream.go` producer each close their output channel
on `ctx.Done()` or are unblocked by a paired closer goroutine. Ownership is real
(channel-close handshake + a bounded 2s grace wait in `runStreamCtx`), context is
honored, and shared-state hand-offs copy slices before publishing
(`aggregator.results()`, pkg/testjson/parser.go:390,405). The hard paths carry
prior-bug-audit refs (#262–#267, fo-op6, fo-u2w, fo-4qh). `go 1.24.0` moots
pre-1.22 loop-variable capture.

`time.Sleep` appears only in `_test.go` files. All production buffered channels
are size 1 with justifying comments except one (`snapshots`, N=8). No lock-reentry
or `context.Background()` substitution in spawned goroutines was found.

Three borderline findings below. No action-tier smell survived verification.
Note: the prior run's vendor-ignore negative-assertion sleep is now **fixed** — a
positive control was added (fo-u60, fswatch_test.go:151–162), so it is not
re-filed.

## Scorecard

| Phase | Tier | Notes |
|---|---|---|
| P1 ownership | 🟢 | every prod goroutine owned via ctx + channel-close / resultCh |
| P1 ctx honoring | 🟢 | long-lived loops select on `ctx.Done()`; blocked Reads unblocked via Close |
| P1 shared state | 🟢 | slices copied before publish; 1.24 loop-var safe |
| P2 magic buffers | 🟡 | one undocumented buffer (`snapshots`, N=8) |
| P2 sleep-as-sync | 🟡 | 2 test readiness-sleeps (generous bounds) |
| P2 lock reentry | 🟢 | not observed |
| P3 idiom inversions | 🟢 | WaitGroup / chan choices defensible |

---

### 1. [F1] `cmd/fo/stream.go:72` — chan-magic-buffer

**Diagnosis.** The streaming-snapshot channel is buffered at a bare literal `8`
with no comment explaining that depth.

**Why.** `sendCoalesceSnapshot` (stream.go:151–171) drops the oldest queued
snapshot when the channel is full rather than blocking the parser, so the buffer
depth is a real tuning knob — it sets how many stale per-package snapshots may
queue before the coalescer starts dropping. `8` is neither the synchronous handoff
(`0`) nor the single-shot signal (`1`) the rule treats as self-justifying, and no
comment says why 8 rather than 4 or 32. Mild: a wrong value degrades
latency/freshness, not correctness. The `resultCh` one line down is the contrasting
good pattern — size 1 with an inline rationale.

**Evidence.** cmd/fo/stream.go:72 (verbatim):

```go
	snapshots := make(chan report.Report, 8)
```

cmd/fo/stream.go:83 (verbatim) — justified sibling:

```go
	resultCh := make(chan streamResult, 1)
```

**Fix.** Add a one-line comment tying `8` to the coalescing burst-tolerance intent,
or reduce toward 1 if the drop-oldest coalescer makes extra slots dead capacity.

**Tier.** borderline

---

### 2. [F2] `cmd/fo/fswatch_test.go:125` — time-sleep-as-sync

**Diagnosis.** `TestWatchTree_DetectsFileWrite` sleeps a fixed 50ms to wait for the
fsnotify watcher to arm before writing the trigger file.

**Why.** `fsnotify.NewWatcher` returns before its kqueue/inotify thread is fully
registered; the test assumes 50ms covers that gap. This is the classic fsnotify
readiness-race — under load or a slow CI box the write can land before the watcher
is armed, and the event is missed. The sibling test `TestWatchTree_IgnoresVendorDir`
already solved exactly this by writing a control file and waiting for its event
(fo-u60); this positive-path test still uses the blind sleep. The 2s assertion
ceiling absorbs current slop, so this is borderline, not action.

**Evidence.** cmd/fo/fswatch_test.go:124–128 (verbatim):

```go
	// Give the watcher a moment to start.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
```

**Fix.** Apply the fo-u60 pattern: write a sentinel file and wait for its event to
confirm the watcher is armed, then run the real assertion — or retry the write in a
loop bounded by the 3s ctx.

**Tier.** borderline

---

### 3. [F3] `cmd/fo/stream_cancel_test.go:84` — time-sleep-as-sync

**Diagnosis.** The cancel test sleeps a fixed 50ms so the streamer "consumes the
initial events" before it calls `cancel()`.

**Why.** The sleep waits for the producer goroutine (launched at line 76) to reach
a state, then races against a 500ms upper-bound assertion on cancel latency. If the
consume takes longer than 50ms the test measures cancel from the wrong start point;
if it's much faster the sleep is wasted wall-clock. A deterministic ready signal
(first stdout write, or an exposed ready channel) removes both the flake and the
delay. Borderline — the 500ms/2s bounds make current failure unlikely.

**Evidence.** cmd/fo/stream_cancel_test.go:83–86 (verbatim):

```go
	// Give the streamer a moment to consume the initial events.
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()
```

**Fix.** Inject a signal — e.g. a `progressWriter` like `pkg/view/stream_test.go`
uses — wait on the first render signal, then `cancel()`. Or accept the generous
bound and annotate with `//lintbrush:disable=goroutine-lifecycle:time-sleep-as-sync`
plus a one-line reason.

**Tier.** borderline

---

## Verified clean (no finding)

- **Ownership.** `watchTree`/`debounce`/`fanIn`/`keyControl` goroutines close their
  output channel on `ctx.Done()` (fswatch.go:152,214; watchkey.go:70,122);
  `keyControl`'s reader (watchkey.go:63) is unblocked by the paired ctx-watch
  goroutine (watchkey.go:58) closing the fd. `runStreamCtx` bounds the producer wait
  with a 2s grace window (stream.go:119–129). No unowned fire-and-forget.
- **Context honoring.** `scanLoop` (parser.go:96) and the `stdinTriggers` scanner
  (watch.go:190) can't be interrupted mid-`Read` by ctx alone; both are documented
  (fo-u2w; watch.go:170–174) and unblocked in production via `Close()` on the real
  `*os.File` stdin.
- **Shared state.** `aggregator.results()` copies `panicOutput` and per-test output
  slices before returning (parser.go:390,405); the `stream.go` producer publishes to
  `resultCh` before any concurrent read of `*r`. No unsynchronized pointer share.
- **watch_test.go:86 busy-poll** — not flagged: the loop polls the real condition
  (`calls.Load() == 0`) bounded by a 1s deadline, not a blind fixed wait.
- **Loop-var capture** — `go.mod` `go 1.24.0`; rule not actionable.

## Cap

3 findings (cap 10). Repo is well-disciplined; remaining items are two bounded
test-suite readiness-sleeps and one undocumented buffer constant.
