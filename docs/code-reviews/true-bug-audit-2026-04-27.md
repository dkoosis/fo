# True-Bug Audit (3 Passes) — 2026-04-27

This audit focuses on production-impacting defects (correctness/reliability/concurrency/boundaries), not style.

## PASS 1 — Correctness & Reliability

### System Map (concise)
- **Primary entrypoint**: `cmd/fo/main.go:main -> run`.
- **Input ingestion**:
  - Single-stream mode reads all stdin (`io.ReadAll`) then dispatches by sniffing SARIF/go-test JSON.
  - “Stream mode” is selected only for `--format auto` + TTY + go test JSON, but still buffers all stdin before rendering.
- **Parsing/mapping**:
  - SARIF: `pkg/sarif.ReadBytes -> ToReportWithMeta`.
  - go test JSON: `pkg/testjson.ParseBytes -> ToReportWithMeta`.
  - Multiplex protocol: `pkg/report.ParseSections` + per-section parse/merge.
- **State persistence**: `attachDiff -> state.Load/Classify/Save` writes `.fo/last-run.json`.
- **External boundaries**: stdin/stdout/stderr + local filesystem sidecar writes.

### Findings (ranked)

#### 1) Unbounded memory growth from whole-stdin buffering can OOM on large inputs
- **Severity**: High
- **Evidence**:
  - `run` buffers stdin fully: `input, err := io.ReadAll(br)`.
  - `runStream` also buffers stdin fully: `data, err := io.ReadAll(br)`.
  - Reachability: `main -> run`, with branch to both paths depending on format/TTY.
- **Mechanism**:
  - Large SARIF/go-test streams are fully materialized in memory; streaming mode currently does not stream incrementally.
- **Concrete failure scenario**:
  - Large CI run with verbose tests produces hundreds of MB/GB of JSON; process is OOM-killed and exits non-deterministically.
- **Minimal fix + tests**:
  - Replace `io.ReadAll` paths with incremental parse/render pipelines (`testjson.Stream`-style feeding aggregator/snapshots).
  - Add integration test with generated large NDJSON ensuring bounded RSS and successful completion.
- **Confidence**: High

#### 2) Valid go test streams with non-JSON prefix are rejected before tolerant parser runs
- **Severity**: Medium
- **Evidence**:
  - `parseToReport` hard-rejects when first non-space byte is not `{`.
  - Parser itself is tolerant of malformed lines (`ParseStream`/`processEventLine` counts and skips malformed lines).
  - Reachability: `run -> parseToReport` on non-stream path.
- **Mechanism**:
  - Dispatch gate requires JSON-first input; mixed prelude lines (from wrapped commands / merged stderr) prevent parsing of subsequent valid events.
- **Concrete failure scenario**:
  - Pipeline prepends banner/prolog lines before JSON events; fo exits usage error instead of rendering actionable failures.
- **Minimal fix + tests**:
  - Add fallback: if sniffing fails, attempt `testjson.ParseBytes`; accept if at least one valid event parsed.
  - Test with prelude + valid go test JSON lines.
- **Confidence**: Medium

#### 3) Sidecar atomic-write durability gap (missing parent-dir fsync)
- **Severity**: Medium
- **Evidence**:
  - `state.Save` fsyncs temp file then `os.Rename`, but does not fsync parent directory afterward.
  - Reachability: `run/runStream -> attachDiff -> state.Save`.
- **Mechanism**:
  - On crash/power loss after rename, directory metadata may not be durable; sidecar update may disappear.
- **Concrete failure scenario**:
  - fo reports classifications for run N, but on next invocation state appears rolled back/missing, causing incorrect diff/flaky classification.
- **Minimal fix + tests**:
  - Open parent directory and `Sync()` after rename (best-effort platform guard).
  - Add fault-injection test (where feasible) or documented durability contract test.
- **Confidence**: Medium

---

## PASS 2 — Concurrency & Lifecycle

### Concurrency Roots Inventory
- **Goroutine starts**:
  - `pkg/testjson.scanAsync` starts `go scanLoop(...)`.
  - No long-lived worker pools in CLI path.
- **Cancellation roots**:
  - `runStream` uses `signal.NotifyContext` and `context.AfterFunc` to close stdin when canceled.
  - `testjson.Stream` cancellation closes reader if it implements `io.Closer`.
- **Shared resources**:
  - channel handoff (`scanResult`), stdin readers, stdout renderer.

### Findings (ranked)

#### 1) `testjson.Stream` can leak scanner goroutine on cancel with non-closable reader
- **Severity**: Medium (Plausible for library consumers)
- **Evidence**:
  - `scanAsync` launches goroutine.
  - `drainLines` on cancel calls `cancelScan(r)`, which only closes when `r` is `io.Closer`.
  - Code comment explicitly notes leak risk for non-closer readers.
- **Mechanism**:
  - If caller passes `*bufio.Reader` (non-closer) and cancels context while `lineread.Read` blocks, goroutine may remain stuck.
- **Timeline failure scenario**:
  - service embeds parser, cancels contexts repeatedly under load; leaked goroutines accumulate and degrade process.
- **Minimal fix**:
  - Prefer context-aware scanner path (polling reads via interruptible source) or require `io.ReadCloser` in API.
- **Test strategy**:
  - Leak test with cancellation and non-closer wrapper around blocking reader.
- **Confidence**: Medium

#### 2) Ctrl-C responsiveness in CLI “stream mode” depends on reader closability during full-buffer read
- **Severity**: Medium
- **Evidence**:
  - `runStream` still does blocking `io.ReadAll(br)` before rendering.
  - Cancellation relies on closing `stdin` only if it is `io.Closer`.
- **Mechanism**:
  - For non-standard reader wiring (tests/embedders), cancel may not preempt promptly.
- **Timeline failure scenario**:
  - User interrupts long run; process does not terminate quickly until upstream closes.
- **Minimal fix**:
  - Replace full-buffer read with incremental consume that checks `ctx.Done()` regularly.
- **Test strategy**:
  - Integration test with slow producer + interrupt, assert bounded shutdown latency.
- **Confidence**: Medium

---

## PASS 3 — Persistence & Boundary

### Boundary Inventory
- **Filesystem writes**: `pkg/state.Save`, `pkg/state.Reset`.
- **Filesystem reads**: `pkg/state.Load`.
- **Input boundaries**: stdin parsers in `cmd/fo/main.go`, `pkg/testjson`, wrapper converters.
- **No DB/HTTP client layers** in this codebase.

### Findings (ranked)

#### 1) Sidecar rename durability issue can cause persisted-state loss
- **Severity**: Medium
- **Evidence + trace**:
  - boundary path: `run -> attachDiff -> state.Save`.
  - `Save` does tmp write + fsync + rename but no parent-dir fsync.
- **Mechanism**:
  - Crash window after rename can lose directory entry update.
- **Concrete failure scenario**:
  - Previously resolved findings reappear as “new” because the latest sidecar never became durable.
- **Minimal fix**:
  - Add parent directory `Sync()` after rename on platforms that support it.
- **Test plan**:
  - Unit test asserting directory sync path invoked (via small abstraction) + docs on fs semantics.
- **Confidence**: Medium

#### 2) Diagnostic wrapper silently drops oversize lines (boundary data loss)
- **Severity**: Medium
- **Evidence + trace**:
  - `diag.readAndAdd` reads `(line, oversize, err)` but ignores `oversize` and just calls `addLine`.
  - reachability: `runWrapDiag -> diag.Convert -> readAndAdd`.
- **Mechanism**:
  - Extremely long diagnostic lines are discarded without surfaced warning count.
- **Concrete failure scenario**:
  - Critical lint finding with huge message/path disappears from SARIF, yielding false-clean output.
- **Minimal fix**:
  - Count oversize drops and emit warning to stderr or synthetic SARIF note.
- **Test plan**:
  - Feed oversize diagnostic line and assert warning/marker emitted.
- **Confidence**: Medium

#### 3) Multiplex parser discards pre-delimiter content silently
- **Severity**: Low
- **Evidence + trace**:
  - `ParseSections` docs and behavior: “Lines before the first delimiter are silently discarded.”
  - boundary path: `parseToReport -> parseMultiplex -> report.ParseSections`.
- **Mechanism**:
  - Upstream wrapper mis-ordering or accidental prelude text can lose tool output without explicit error.
- **Concrete failure scenario**:
  - First tool emits bytes before delimiter due script banner; those findings never appear.
- **Minimal fix**:
  - If non-whitespace prelude exists before first delimiter, return parse warning/error.
- **Test plan**:
  - Input with prelude + valid sections should produce explicit warning finding.
- **Confidence**: Medium
