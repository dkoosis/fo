# Go Codebase True-Bug Audit (3 Passes)

> Note: the requested `snipe` tool is not available in this environment (`snipe: command not found`). Reachability and flow tracing were done with `rg -n` caller/callee searches plus direct source tracing.

## PASS 1 — True Bugs Audit (Correctness & Reliability)

### System Map

- **Entrypoints**
  - CLI main path: `cmd/fo/main.go:main -> run`.
  - Streaming path: `run -> runStream -> stream.Run -> testjson.Stream`.
  - Batch path: `run -> runBatch -> parseInput -> mapper/render`.
  - Subcommand path: `run -> runWrap` for `fo wrap sarif`.
- **Concurrency model**
  - Single-threaded event loop for rendering; cancellation via `signal.NotifyContext` in stream mode.
- **Persistence / state**
  - In-memory aggregation/buffering (`pkg/testjson` and `pkg/stream` maps/slices).
  - No DB/cache/filesystem writes in normal render path.
- **External boundaries**
  - STDIN/STDOUT streams.
  - OS signals.
- **Error conventions**
  - Exit code `2` for tool errors, `1` for detected failures, `0` clean.

### Findings (ranked)

#### 1) Streaming cancellation can hang until next input line
- **Severity:** High
- **Evidence + reachability:**
  - Reachability chain: `cmd/fo/main.go:runStream` -> `pkg/stream/stream.go:Run` -> `pkg/testjson/parser.go:Stream` (verified via `rg -n "runStream\(|stream\.Run\(|testjson\.Stream\("`).
  - `testjson.Stream` checks `ctx.Err()` only *inside* the `for scanner.Scan()` loop body, after `Scan()` returns. A blocked `Scan()` on a quiet pipe/socket is not interruptible by context.
- **Mechanism:** context cancellation does not break blocking scanner read, so Ctrl-C may not terminate promptly in stream mode.
- **Concrete scenario:** User runs `go test -json ./... | fo` where upstream stalls; Ctrl-C sent to `fo` context cancels, but `fo` remains blocked waiting for another line/EOF.
- **Minimal fix + tests:**
  - Move streaming decode to a goroutine and select on `ctx.Done()` vs decoded event channel; close reader on cancel when possible.
  - Add test with a custom reader that blocks forever; assert stream exits on context timeout.
- **Confidence:** High

#### 2) Scanner token limit causes hard failure on long JSON lines (output loss)
- **Severity:** High
- **Evidence + reachability:**
  - `pkg/testjson/parser.go` sets scanner max token to `1024*1024` in both `ParseStream` and `Stream`.
  - `bufio.Scanner` returns `ErrTooLong` for longer lines; this terminates parsing and returns error in `Stream`, producing exit code 2.
- **Mechanism:** one oversized test output line (>1 MiB) aborts entire parse/render path.
- **Concrete scenario:** verbose tests emit a huge JSON `Output` event line (large logs or stack dumps); fo aborts mid-run and loses remaining results.
- **Minimal fix + tests:**
  - Replace scanner with `bufio.Reader.ReadBytes('\n')` / decoder stream handling without strict token cap.
  - Add regression test generating >1MiB event line for both `ParseStream` and `Stream`.
- **Confidence:** High

#### 3) `fo wrap sarif` has default 64KiB scanner limit; large diagnostics break conversion
- **Severity:** Medium
- **Evidence + reachability:**
  - `runWrap` uses `bufio.NewScanner(stdin)` without `scanner.Buffer(...)` override.
  - Path is reachable from CLI (`run` dispatches to `runWrap` when first arg is `wrap`).
- **Mechanism:** long diagnostic lines trigger scanner token-too-long, stopping conversion and returning exit code 2.
- **Concrete scenario:** tool emits long one-line diagnostics (minified linter output, generated-file warnings); output SARIF truncates unexpectedly.
- **Minimal fix + tests:**
  - Set scanner buffer and max token similar to testjson (or move to `bufio.Reader`).
  - Add wrap test with >64KiB diagnostic line.
- **Confidence:** High

#### 4) Buffered per-test output is unbounded in streaming mode (memory blow-up)
- **Severity:** Medium
- **Evidence + reachability:**
  - `pkg/stream/stream.go:handleOutput` appends every output line into `s.outputBuf[key]`.
  - Data is only released on pass/fail/skip handlers; long-running noisy tests can accumulate arbitrarily.
- **Mechanism:** no cap/eviction/streaming truncation for output buffers.
- **Concrete scenario:** soak/integration test emits continuous logs for minutes before finishing; `fo` RSS grows until OOM or heavy GC.
- **Minimal fix + tests:**
  - Cap lines/bytes per test buffer, keeping tail only (e.g., last N lines).
  - Add test asserting cap enforcement and bounded memory behavior.
- **Confidence:** Medium

#### 5) Windows file-only diagnostics with backslashes are dropped in `parseDiagLine`
- **Severity:** Medium
- **Evidence + reachability:**
  - `parseDiagLine` file-only branch accepts `.go` suffix or `/` in path, but not `\\` separator.
  - Called directly from `runWrap` scan loop.
- **Mechanism:** file-only lines like `C:\repo\pkg\x.go` fail matcher and are silently ignored.
- **Concrete scenario:** on Windows, `gofmt -l` piped to `fo wrap sarif` omits findings entirely.
- **Minimal fix + tests:**
  - Accept `strings.Contains(trimmed, "\\")` in file-only path detection.
  - Add test case for Windows backslash path.
- **Confidence:** High

---

## PASS 2 — Concurrency & Lifecycle Audit

### Concurrency Roots Inventory

1. **Signal cancellation root**
   - Started in `runStream` via `signal.NotifyContext`.
   - Consumed by `stream.Run` and then `testjson.Stream`.
2. **No background worker goroutines** in normal codepath; event handling is synchronous.
3. **Lifecycle symmetry**
   - Start: `runStream` constructs context and calls `stream.Run`.
   - Stop: context cancel / EOF / scanner error -> return code.

### Findings (ranked)

#### 1) Cancellation lifecycle mismatch: context exists but read loop is not cancellation-responsive during blocked read
- **Severity:** High
- **Evidence + lifecycle trace:** `runStream` creates cancelable context, but `testjson.Stream` checks it only after `scanner.Scan()` returns.
- **Mechanism:** blocked I/O prevents lifecycle shutdown from propagating.
- **Timeline failure scenario:** SIGINT at T0 -> context canceled at T0+ε -> parser still blocked in `Scan` indefinitely until producer emits line or closes pipe.
- **Minimal fix:** use cancelable read architecture (`select` on done + channel) and/or close underlying reader when canceled.
- **Test strategy:** integration test with blocking reader and short timeout context; ensure timely return.
- **Confidence:** High

#### 2) Backpressure/memory hazard from unbounded buffered logs per active test
- **Severity:** Medium
- **Evidence + lifecycle trace:** output is appended in `handleOutput`, released only on terminal test events.
- **Mechanism:** lifecycle of a long-running test can outlive practical memory budget.
- **Timeline scenario:** T0 run test starts; T1..Tn emits logs continuously; Tn+1 pass/fail delayed -> buffer grows unbounded.
- **Minimal fix:** bounded ring buffer per test and optional package-level cap.
- **Test strategy:** synthetic stream with many output events before pass; assert capped retained lines.
- **Confidence:** Medium

---

## PASS 3 — Persistence & Boundary Audit

### Boundary Inventory

- **Input boundaries**
  - STDIN read for batch (`io.ReadAll`), streaming parser (`bufio.Scanner`), wrap parser (`bufio.Scanner`).
- **Output boundary**
  - STDOUT render/write in all modes.
- **Serialization boundaries**
  - JSON decode for SARIF and go-test events.
- **No DB/cache/network/file-mutation boundaries** in runtime flow.

### Findings (ranked)

#### 1) Oversized boundary payloads are not robustly handled (scanner hard limits)
- **Severity:** High
- **Evidence + boundary trace:**
  - `fo` entrypoint -> streaming boundary (`testjson.Stream`) or wrap boundary (`runWrap`) -> `bufio.Scanner` max token errors.
- **Mechanism:** boundary parser aborts on long lines, causing partial processing/lost findings.
- **Concrete scenario:** CI log injector or tool outputs very long single-line JSON/diagnostic; fo exits 2 and loses downstream reporting.
- **Minimal fix:** replace scanner-based line handling with reader-based approach and explicit max-size policy + truncation.
- **Test plan:** boundary fuzz/regression tests for 64KiB, 1MiB, and >1MiB lines across stream/wrap paths.
- **Confidence:** High

#### 2) Batch mode `io.ReadAll(stdin)` has no size guard (potential OOM)
- **Severity:** Medium
- **Evidence + boundary trace:** `run` -> `runBatch` -> `io.ReadAll(br)` unbounded allocation.
- **Mechanism:** malicious or accidental huge stdin can exhaust memory.
- **Concrete scenario:** accidentally piping multi-GB artifact into fo; process OOM-killed.
- **Minimal fix:** use `io.LimitReader` with configurable cap; return explicit error when exceeded.
- **Test plan:** unit test with synthetic large reader exceeding configured cap.
- **Confidence:** Medium

#### 3) Silent dropping of unrecognized diagnostics in `wrap sarif` loses user data without signal
- **Severity:** Medium (Plausible reliability defect)
- **Evidence + boundary trace:** `runWrap` loop continues silently when `parseDiagLine` returns empty file.
- **Mechanism:** boundary parse failures are swallowed, resulting in incomplete SARIF without warning counters.
- **Concrete scenario:** mixed tool output format variation; large fraction of findings omitted, CI appears cleaner than reality.
- **Minimal fix:** count dropped lines and emit warning summary to stderr (or optional strict mode to fail).
- **Test plan:** mixed valid/invalid diagnostic input; assert warning count surfaced.
- **Confidence:** Medium
