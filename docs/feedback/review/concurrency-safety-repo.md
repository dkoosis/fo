# concurrency-safety — fo repo (project scope)

RUN_ID: 7858b3a8ea1b · mode: report · linter: concurrency-safety

## Verdict: no findings

The production concurrency surface is small and unusually hardened — nearly every
goroutine, channel, and timer carries a bug-fix citation for a prior race/leak/deadlock
(`#257 #262 #266 #267`, `fo-op6 fo-4qh fo-u2w fo-gn0 fo-2sk fo-58k`). I traced every
production `go`-launch, channel, mutex, and timer and found nothing that survives the
Diagnosis/Why/Evidence/Fix bar. Padding the report would distort the signal, so it stays
empty (harness: zero findings is a complete result).

## Race detector

```
go test -race ./cmd/fo/       → ok    (66.9s, no DATA RACE)
go test -race ./pkg/view/     → ok    (10.4s, no DATA RACE)
go test -race ./pkg/testjson/ → FAIL  (90s timeout, NOT a race)
```

The `pkg/testjson` FAIL is a wall-clock timeout: `TestStreamMode_LargePerTestOutputBounded`
and `TestPanicOutput_Bounded` push ~1 MiB buffers through the parser and run past the 90s
`-timeout` under race instrumentation. The failing goroutines are `[runnable]` inside
`encoding/json.Unmarshal` — no `DATA RACE` banner in any run. Not a concurrency defect.
Full-repo `go test -race ./...` was cut by the harness 2-minute wall; the three hotspot
packages above cover the entire production goroutine surface.

## Surface reviewed (production, non-test)

| File | Primitive | Assessment |
|---|---|---|
| `cmd/fo/stream.go` | producer goroutine + `resultCh` (buf 1) + `snapshots` (buf 8) | Correct. All `r` mutations (`attachDiff`/`assignAndPersistIDs`/`recordRun`) happen-before the `resultCh` send; main reads `r` only after receiving. Grace-timeout path (`#266`) returns without touching `r`. No race, no leak. |
| `cmd/fo/stream.go` | `sendCoalesceSnapshot` drop-oldest | Single-producer invariant documented and holds; the non-blocking drop/retry cannot race another sender. |
| `cmd/fo/fswatch.go` | `runDebounce` timer + `resetTimer` | Textbook `Stop()`/drain pattern. The `case <-timerC` branch nils the timer, so `resetTimer` never drains an already-consumed timer — no `<-timerC` deadlock. |
| `cmd/fo/fswatch.go` | `runWatcher` fsnotify loop | Selects on `ctx.Done()` on both the event forward and the receive; closes `out` and the watcher on exit. Terminates on cancel. |
| `cmd/fo/watch.go` | `stdinTriggers` reader + closer goroutines | Production stdin is `*os.File` (io.Closer) → ctx-cancel closes it, unblocking the scanner. Non-closable-reader park is documented, test/pipe-only, and process-lifetime — a `Don't flag` case per the goroutine-leak rule. |
| `cmd/fo/watch.go` | `watchLoop` single-flight | Selects `ctx.Done()` vs `triggers`; synchronous `runOnce`. No shared state. |
| `cmd/fo/watchkey.go` | `keyControl`/`readKeys`/`fanIn` | `restore` guarded by `sync.Once`; ctx goroutine restores-then-closes fd (correct order, documented); `readKeys` exits on the resulting Read error and closes `out`. `fanIn` nils closed sources and honors ctx. No leak. |
| `pkg/testjson/parser.go` | `Stream`/`scanLoop`/`drainLines` scanner goroutine | Lines copied via `copyBytes` before crossing the channel; `sendResult` selects on `ctx.Done()`; `drainLines` closes `r` on cancel. Aggregator is single-goroutine (Stream calls `fn` synchronously) — matches its documented "not safe for concurrent use" contract. |
| `pkg/testjson/parser.go` | `results()` snapshot copies | Each snapshot deep-copies `panicOutput`/`outputBuf` slices (lines 390, 405), so streaming consumers holding an earlier snapshot never alias the still-mutating aggregator backing arrays. |
| `pkg/view/stream.go` | `RenderStreamMode` consumer | Single consumer, no shared state, selects on `ctx.Done()`. Clean. |

No `atomic.*`, no `sync.Map`, no `errgroup`, no cross-package channel ownership, no
`time.Sleep`-as-sync in production code.

trixi log-skill concurrency-safety findings 0 --run-id "7858b3a8ea1b"
