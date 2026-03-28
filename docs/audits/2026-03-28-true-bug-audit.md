# Go Codebase True-Bug Audit — 2026-03-28

This audit follows the requested 3-pass workflow and focuses only on production defects with plausible triggers.

## PASS 1 — True Bugs Audit (Correctness & Reliability)

### System Map (concise)

- **Entrypoint / control plane**: `cmd/fo/main.go` (`main` → `run`), with `wrap` subcommand and standard render path for SARIF / go test JSON / report inputs.
- **Input detection boundary**: `internal/detect.Sniff` differentiates SARIF, go test JSON, and report delimiters.
- **Core parsing/mapping**:
  - SARIF: `pkg/sarif.ReadBytes` → `pkg/mapper.FromSARIF`
  - go test JSON batch: `pkg/testjson.ParseBytes` → `pkg/mapper.FromTestJSON`
  - report: `internal/report.Parse` → `pkg/mapper.FromReport`
- **Streaming path**: `run` enters `runStream` only for go test JSON + TTY + `--format auto`; `runStream` invokes `pkg/stream.Run`, which consumes `pkg/testjson.Stream`.
- **Concurrency model**: streaming parser creates one scanner goroutine and one consumer loop; coordinated by context cancellation.
- **Persistence**: no DB/storage layer; primary boundaries are stdin/stdout streams and JSON decoding.
- **Error conventions**: parse/usage errors generally return process exit code `2`; lint/test failures return `1`; success `0`.

### Findings (ranked)

#### 1) Unbounded memory growth in batch test-json parsing due retained output buffers
- **Severity**: High
- **Evidence (code + reachability)**:
  - `pkg/testjson.aggregator.processEvent` appends every `output` line into `pkg.outputBuf[e.Test]`.
  - On `pass` / `skip`, counters are incremented, but that test’s output buffer is never cleared.
  - Reachability: `cmd/fo.run` → `parseInput(detect.GoTestJSON)` → `testjson.ParseBytes` → `ParseStream` → `aggregator.processEvent`.
- **Mechanism**:
  - For large suites with verbose logs, passing tests accumulate output in memory for entire run.
  - Unlike stream mode (`pkg/stream`), batch aggregator does not reclaim output for passing/skipped tests.
- **Concrete failure scenario**:
  - CI runs `go test -json ./...` with verbose subtests across many packages.
  - Process RSS grows with every passing test log line and can OOM or trigger container eviction.
- **Minimal fix + tests**:
  - In `processEvent`, delete `pkg.outputBuf[e.Test]` on terminal test actions (`pass`, `skip`, and after copying on `fail` if no further lines are needed).
  - Add stress test generating many passing tests with output; assert bounded heap growth or at least bounded map size after package completion.
- **Confidence**: High

#### 2) Windows CRLF report delimiters are not parsed, causing false “no sections” failures
- **Severity**: Medium
- **Evidence (code + reachability)**:
  - `internal/report.Parse` splits only on `\n`; each line from CRLF input retains trailing `\r`.
  - Delimiter regex is anchored (`^...$`) and does not tolerate `\r`, so delimiter lines fail to match.
  - Reachability: `cmd/fo.run` detects report input and calls `parseInput(detect.Report)` → `report.Parse`.
- **Mechanism**:
  - Report parser fails to detect section boundaries for CRLF-delimited files/streams.
- **Concrete failure scenario**:
  - A report generated on Windows (`\r\n`) is piped into `fo`; parser returns `ErrNoSections`, exiting with code `2` despite valid content.
- **Minimal fix + tests**:
  - Normalize line endings (`\r\n` → `\n`) before parsing or trim terminal `\r` per line before regex match.
  - Add test with CRLF delimiters and payload to verify successful section parsing.
- **Confidence**: High

#### 3) Report mode can silently mask malformed go test JSON lines (false-clean risk)
- **Severity**: Medium
- **Evidence (code + reachability)**:
  - In `pkg/mapper.mapTestJSONSection`, call is `results, _, err := testjson.ParseBytes(sec.Content)`; malformed line count is discarded.
  - In contrast, top-level go test path warns when malformed lines are skipped (`cmd/fo.parseInput`), but report section path does not.
  - Reachability: `cmd/fo.run` → `parseInput(detect.Report)` → `mapper.FromReport` → `mapTestJSONSection`.
- **Mechanism**:
  - Corrupted JSON lines in a report test section are ignored without surfacing warnings/errors.
  - If dropped lines include failing events, report status can appear cleaner than reality.
- **Concrete failure scenario**:
  - Aggregated report has truncated test-json lines due transport/log splitting.
  - Section parser skips malformed failures and emits mostly passing summary/tables.
- **Minimal fix + tests**:
  - Surface malformed count as a visible `pattern.Error` or warning metric in report section summary.
  - Add tests with mixed valid/malformed section lines and assert warning/error presence.
- **Confidence**: Medium-High

---

## PASS 2 — Concurrency & Lifecycle Audit

### Concurrency Roots Inventory

- `pkg/testjson.Stream`: starts scanner goroutine producing `scanResult` into channel; consumer loop in caller goroutine.
- `cmd/fo.runStream`: installs signal-aware context and registers `context.AfterFunc` to close stdin on cancellation.
- `pkg/stream.Run`: drives stream state machine in single goroutine (event callback from `testjson.Stream` consumer path).

### Findings (ranked)

#### 1) Potential goroutine leak if `testjson.Stream` is canceled with non-closable blocked reader
- **Severity**: Medium (Plausible)
- **Evidence + lifecycle trace**:
  - `testjson.Stream` docs explicitly state leak risk when `r` does not implement `io.Closer` and blocks.
  - Cancellation path only attempts `r.(io.Closer).Close()`.
  - Lifecycle trace: caller starts `Stream` → scanner goroutine blocks in `scanner.Scan()` → ctx canceled → consumer returns; scanner may remain blocked if reader cannot be closed.
- **Mechanism**:
  - Producer goroutine can outlive consumer when blocked on read and no close mechanism exists.
- **Timeline failure scenario**:
  - Integrator reuses `testjson.Stream` with custom non-closable network/proxy reader; cancellation under timeout leaves leaked goroutine per invocation.
- **Minimal fix**:
  - Require a closer-capable reader for streaming API (or wrap reader with explicit close signal), or perform scanning in caller goroutine and push parse work to worker instead.
- **Test strategy**:
  - Add leak-oriented test with a blocking non-closer reader and cancellation; verify goroutine count stability with timeout harness.
- **Confidence**: Medium (Plausible; depends on non-closable blocked reader usage)

#### 2) Streaming callback can block the scanner pipeline without backpressure controls
- **Severity**: Low-Medium (Plausible)
- **Evidence + lifecycle trace**:
  - `testjson.Stream` channel is unbuffered; scanner goroutine sends each line and waits.
  - Consumer decodes JSON then calls `fn(event)` inline; slow callback stalls all reads.
  - Current callback (`pkg/stream.Run` → `s.handleEvent`) is lightweight, but API allows arbitrary heavy callbacks.
- **Mechanism**:
  - Any expensive callback (I/O, locks, slow rendering) can create head-of-line blocking and apparent hangs under high event throughput.
- **Timeline failure scenario**:
  - Consumer callback writes to slow sink; upstream go test process blocks on pipe due stalled reads, elongating/derailing CI timings.
- **Minimal fix**:
  - Document callback budget strictly and/or add bounded buffering with cancellation-aware dropping policy.
- **Test strategy**:
  - Simulate slow callback and high-rate input; assert throughput/latency behavior within expected bounds.
- **Confidence**: Medium-Low (Plausible; depends on third-party callback usage)

---

## PASS 3 — Persistence & Boundary Audit

### Boundary Inventory

- **Stdin/stdout boundaries**: CLI reads all stdin in batch mode; streaming mode reads incrementally.
- **JSON boundaries**:
  - SARIF decode via `encoding/json` decoder (`pkg/sarif`, `internal/detect`).
  - go test NDJSON decode line-by-line (`pkg/testjson`).
  - Wrapper-specific JSON (`wraparchlint`, `wrapjscpd`) via full `ReadAll` then unmarshal.
- **Report boundary**: delimiter-based framing in `internal/report.Parse`.
- **External HTTP/DB/filesystem mutation**: none in core runtime path.

### Findings (ranked)

#### 1) CRLF delimiter handling bug at report input boundary (duplicate of Pass-1 #2, boundary-critical)
- **Severity**: Medium
- **Evidence + boundary trace**:
  - Boundary trace: stdin report bytes → `detect.Sniff` identifies report → `report.Parse` fails delimiter match on `\r`-terminated lines.
  - Anchored regex rejects valid delimiter text plus carriage return.
- **Mechanism**:
  - Cross-platform newline mismatch at parser boundary.
- **Concrete failure scenario**:
  - Windows-produced report fails in Linux CI ingestion step, causing non-actionable parse failure.
- **Minimal fix**:
  - Normalize CRLF or trim `\r` before delimiter matching.
- **Test plan**:
  - Add parser and end-to-end CLI fixture test for CRLF report input.
- **Confidence**: High

#### 2) Full-read wrappers can exhaust memory on unusually large tool outputs
- **Severity**: Medium (Plausible)
- **Evidence + boundary trace**:
  - `wraparchlint.Wrap` and `wrapjscpd.Wrap` call `io.ReadAll(r)` before parsing.
  - Boundary trace: `fo wrap <name>` stdin → wrapper `ReadAll` → unmarshal.
- **Mechanism**:
  - Unbounded in-memory buffering at input boundary; large or malicious input can cause OOM.
- **Concrete failure scenario**:
  - Large monorepo duplication report (or malformed stream without bounds) causes wrapper process to exceed memory limits.
- **Minimal fix**:
  - Use streaming decoder (`json.Decoder`) and enforce configurable max input size.
- **Test plan**:
  - Fuzz/benchmark with large payloads; verify graceful size-limit error instead of OOM.
- **Confidence**: Medium (Plausible)

#### 3) Report test-json section loses malformed-line visibility at boundary (duplicate of Pass-1 #3)
- **Severity**: Medium
- **Evidence + boundary trace**:
  - Boundary trace: report section bytes → `testjson.ParseBytes` returns `(results, malformed, err)` → malformed discarded in `mapTestJSONSection`.
- **Mechanism**:
  - Boundary corruption is silently tolerated with no user signal.
- **Concrete failure scenario**:
  - Corrupted transport/log framing drops failing test events, producing misleadingly clean output.
- **Minimal fix**:
  - propagate malformed count to section-level warning/error pattern.
- **Test plan**:
  - golden report fixture with malformed test-json lines; assert surfaced warning.
- **Confidence**: Medium-High
