---
review_date: 2026-05-17
run_id: bd775e303d86-concurrency-safety
linter: concurrency-safety
target: repo
race_detector_run: true
race_detector_result: clean (no races across all packages, 5m timeout)
packages_analyzed: cmd/fo, pkg/testjson, pkg/view, pkg/state
severity_counts: { critical: 0, high: 0, medium: 3, info: 2 }
---

# Concurrency-safety — fo repo

**Headline:** `go test -race ./...` is **clean** across all 20+ packages (≈42s for pkg/testjson, the hot one). The concurrency surface is small and disciplined: every `go func()` ships with a termination contract, every long-lived loop selects on `ctx.Done()`, channel ownership is single-producer in every case I traced, and `context.AfterFunc` / `signal.NotifyContext` are used correctly. Findings below are all Medium/Info — latent risks and idiom nits, no proven races or leaks.

---

### 1. [F1] `cmd/fo/main.go:1051` — context-cancel-leak (adjacent)

**Diagnosis:** `runTestJSONPipeline` registers `context.AfterFunc(ctx, func() { _ = c.Close() })` to close stdin on cancel, AND wraps stdin in a `bufioReadCloser{closer: closerOf(stdin)}` which `testjson.Stream`'s `drainLines` also closes on ctx cancel (parser.go:136). Both fire on the cancel path.

**Why it matters:** For `*os.File` the second Close returns `os.ErrClosed` and is swallowed by `_ =`. Tolerable. But any future stdin substitute whose `Close()` is not idempotent (e.g. a pipe wrapper that frees an underlying resource) will misbehave silently. The two cancel hooks duplicate intent across package boundaries — easy to miss when refactoring either side.

**Evidence (Read-verified):**
- `cmd/fo/main.go:1051` — `stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })`
- `cmd/fo/main.go:1058` — `rc := &bufioReadCloser{Reader: br, closer: closerOf(stdin)}`
- `pkg/testjson/parser.go:135-137` — `case <-ctx.Done(): _ = r.Close(); return ...`

**Fix:** Pick one closer. Preferred: drop the `AfterFunc` in `runTestJSONPipeline` and rely on `testjson.Stream`'s built-in cancel-close path (it's already the documented contract — see Stream doc at parser.go:74-78). Alternatively pass a no-op closer to `bufioReadCloser` and keep `AfterFunc` as the single owner. The "honors ctx cancellation by closing stdin" claim should belong to exactly one layer.

**Tier:** Medium (latent; current behavior tolerated because `*os.File.Close` is idempotent-ish).

---

### 2. [F2] `cmd/fo/main.go:933` — goroutine-leak on grace-timeout path

**Diagnosis:** `runStreamCtx` bounds the wait for the producer goroutine at 2s after ctx cancel (line 971: `case <-time.After(2 * time.Second)`). On that branch fo returns 2 and the producer goroutine continues running — possibly blocked inside `attachDiff` → `state.Save` (file I/O). When `main` exits, the OS reaps it; in any long-lived host (tests, future embedding) the goroutine outlives its caller.

**Why it matters:** Today fo is a one-shot CLI so the leak is bounded by process lifetime. The code comment at line 924-926 acknowledges this trade-off ("a wedged disk doesn't hang fo forever"). The risk shifts the moment anything calls `runStreamCtx` outside `main` — tests already do (`stream_cancel_test.go`).

**Evidence (Read-verified):**
- `cmd/fo/main.go:931` — `resultCh := make(chan streamResult, 1)` (buffer 1 ensures producer can send without blocking)
- `cmd/fo/main.go:966-975` — grace-timeout select
- `cmd/fo/main.go:947-952` — producer calls `applySuppress` and `attachDiff` (both touch the filesystem) after parser drain

**Fix:** Either (a) move `attachDiff` / `applySuppress` out of the producer goroutine into `runStreamCtx` after the resultCh receive — then the goroutine only does fast in-memory work and the grace window can shrink to <100ms; or (b) accept the leak and document it on the function signature (not just on resultCh).

**Tier:** Medium (architectural latent risk, not a current-behavior bug).

---

### 3. [F3] `pkg/testjson/parser.go:79` — context-not-propagated (contract enforced by convention)

**Diagnosis:** The function comment warns "r MUST be an `io.ReadCloser` whose Close actually interrupts in-flight Read calls — `*bufio.Reader` wrapped in `io.NopCloser` will leak the scanner goroutine on cancel (fo-u2w)". This is the only safety belt. A caller passing a no-op-Close ReadCloser builds a leak with no compile-time or test-time signal.

**Why it matters:** `cmd/fo` honors the contract today (`bufioReadCloser` propagates to the real stdin closer). A future caller in any new entry point (subcommand, library use) will need to re-discover this rule from prose. The existing `stream_cancel_test.go` only covers the positive case — there is no negative test asserting the leak warning.

**Evidence (Read-verified):**
- `pkg/testjson/parser.go:74-82` — doc comment with the "MUST" clause
- `pkg/testjson/stream_cancel_test.go:38-62` — only tests a closer that DOES unblock
- `cmd/fo/main.go:1058,1081-1086` — caller-side compliance

**Fix:** Rename the parameter from `r io.ReadCloser` to `r CancelableReader` (typedef in the same package) and put the contract on the type's doc comment. Catches the next reader at API-shape time. Pragmatic; no behavior change.

**Tier:** Medium (correctness contract enforced only by reviewer attention).

---

### 4. [F4] `cmd/fo/watch.go:175` — goroutine-leak (documented, non-closable readers)

**Diagnosis:** When `r` does not implement `io.Closer`, ctx cancel cannot interrupt the blocking `sc.Scan()`. The goroutine remains parked until the next byte or EOF. The function's own doc comment acknowledges this.

**Why it matters:** In production (`os.Stdin` is `*os.File`, closable) this is benign. In tests using `strings.Reader` / `bytes.Reader` it can leak goroutines per test if cleanup doesn't drive things to EOF — masks future leaks behind the noise floor.

**Evidence (Read-verified):**
- `cmd/fo/watch.go:180-188` — closer goroutine guarded by type assertion
- `cmd/fo/watch.go:190-203` — scanner goroutine has no ctx-bound abort for in-flight Read

**Fix:** Add `goleak.VerifyNone(t)` (uber-go/goleak) to one cmd/fo test that exercises `stdinTriggers` so a future test that forgets to drive EOF fails loudly. No production code change required.

**Tier:** Info (documented behavior, no production failure mode).

---

### 5. [F5] `cmd/fo/watchkey.go:58` — process-global side effect on stdin

**Diagnosis:** When `keyControl` is wired (TTY-attached watch mode), a goroutine awaits ctx.Done then closes the stdin `*os.File`. Correct for unblocking the raw-read goroutine, but the closed descriptor is process-global: any sibling goroutine or library code that later reads `os.Stdin` sees EOF/ErrClosed. Today nothing else reads stdin during watch (confirmed: `runWatch` routes stdin only into keyControl OR stdinTriggers via `opts.source` check at watch.go:108), so safe.

**Why it matters:** This is the only place fo closes a descriptor the caller owns. If a future feature (config reload from stdin, an embedded host that retains stdin) shares stdin with watch, the close becomes a use-after-free in the read-from-stdin sense.

**Evidence (Read-verified):**
- `cmd/fo/watchkey.go:58-62` — `go func() { <-ctx.Done(); restore(); _ = f.Close() }()`
- `cmd/fo/watch.go:107-116` — mutual exclusion of stdin readers

**Fix:** Add a comment at the goroutine flagging the process-global side effect so a future contributor doesn't quietly add a second stdin reader. No behavior change.

**Tier:** Info.

---

## Notes / non-findings

- **`sendCoalesceSnapshot` (main.go:997-1017)** — single-producer drop-oldest is correctly implemented; the comment explicitly calls out the single-producer invariant. Clean.
- **`debounce` Stop/Drain (fswatch.go:240-250)** — the timer reset pattern is safe because the `<-timerC` select arm nils both `timer` and `timerC`, so `resetTimer` is only called when the timer is still armed. Clean.
- **`fanIn` / `fanInStep` (watchkey.go:121-161)** — explicit state machine, nil-channel idiom used correctly for "source closed". Clean.
- **`aggregator`** — explicitly single-threaded ("Not safe for concurrent use; callers serialize event delivery"); only consumer is `testjson.Stream`'s drainLines which calls back synchronously. Clean.
- **No `sync.Map`, no `errgroup`, no `singleflight`, no manual lock ordering** — the codebase deliberately stays inside the streaming-filter envelope. Matches north-star.md "no TUI, no event loop, no interactive state."

## Race detector

```
$ go test -race -timeout=5m ./...
... all packages OK; no DATA RACE warnings ...
ok  github.com/dkoosis/fo/pkg/testjson           42.267s
ok  github.com/dkoosis/fo/pkg/view               11.813s
ok  github.com/dkoosis/fo/pkg/wrapper/wraparchlint 23.340s
ok  github.com/dkoosis/fo/pkg/sarif               3.255s
ok  github.com/dkoosis/fo/pkg/state               3.535s
... (full output: race detector ran clean across all 20+ packages)
```

No `DATA RACE` diagnostics emitted in this session.
