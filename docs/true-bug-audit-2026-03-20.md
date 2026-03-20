# True Bug Audit — 2026-03-20

## PASS 1 — Correctness & Reliability

### System Map (concise)
- **Entrypoint:** CLI `main()` → `run()` with stdin format sniffing and parse/render dispatch. (`cmd/fo/main.go`)
- **Input boundaries:** stdin parser paths for SARIF, `go test -json`, and multi-section report. (`cmd/fo/main.go`, `internal/detect/detect.go`, `internal/report/report.go`)
- **Streaming path:** TTY + go test JSON routes to live streamer `runStream()` → `stream.Run()` → `testjson.Stream()`. (`cmd/fo/main.go`, `pkg/stream/stream.go`, `pkg/testjson/parser.go`)
- **Batch path:** reads all stdin then parses via `sarif.ReadBytes`, `testjson.ParseBytes`, or `report.Parse`, then maps to patterns and renders.
- **Concurrency model:** single consumer loop + one scanner goroutine in `testjson.Stream()`.
- **Persistence/external I/O:** stdin/stdout only; no DB/network write paths in this repo.

### Findings

#### 1) Large single-line test output can hard-fail parsing (scanner token limit)
- **Severity:** High
- **Evidence:** Both batch and streaming testjson parsers cap scanner tokens at 1 MiB (`scanner.Buffer(..., 1024*1024)`), and return scanner errors as fatal.
  - `pkg/testjson/parser.go:20,35-37,62,76-79,97-99`
- **Snipe reachability:**
  - `snipe callers ParseBytes` → `cmd/fo/main.go:214` and `pkg/mapper/report.go:125`.
  - `snipe callers Stream` → `pkg/stream/stream.go:288`.
  - `snipe callers --id a24083d4e541f6b0` (stream.Run) → `cmd/fo/main.go:199`.
- **Mechanism:** `bufio.Scanner` returns `ErrTooLong` when a line exceeds max token size; code converts this to fatal parse/stream error.
- **Concrete failure scenario:** A panic stack trace or logged JSON blob in test output exceeds 1 MiB on a single line. `fo` exits with parse error (batch) or stream error (live), obscuring test results and causing CI incident noise.
- **Minimal fix + tests:**
  - Replace `bufio.Scanner` with `bufio.Reader.ReadBytes('\n')` or raise max token substantially with bounded fallback.
  - Add regression tests feeding >1 MiB NDJSON lines in both `ParseStream` and `Stream` paths.
- **Confidence:** High

#### 2) Report parser rejects CRLF-delimited headers (Windows-generated reports)
- **Severity:** Medium
- **Evidence:** parser splits on `\n` but does not strip trailing `\r`; delimiter regex is anchored and expects exact ` ---$` ending.
  - `internal/report/report.go:13-15,33-40`
- **Snipe reachability:**
  - `snipe callers --id dbf6e74048c3f64e` (internal/report.Parse) → `cmd/fo/main.go:224`.
  - `snipe callers Sniff` → `cmd/fo/main.go:126,144` (report detection gate before parse).
- **Mechanism:** delimiter lines like `--- tool:lint format:sarif ---\r` fail regex match, yielding `ErrNoSections`.
- **Concrete failure scenario:** A Windows wrapper emits CRLF in `--- tool:... ---` report delimiters; `fo` returns "no sections found" and exits 2, even though payload is valid.
- **Minimal fix + tests:**
  - Trim `\r` per line before delimiter matching, or normalize `\r\n` upfront.
  - Add tests for CRLF report input with multiple sections.
- **Confidence:** High

---

## PASS 2 — Concurrency & Lifecycle

### Concurrency Roots Inventory
- `testjson.Stream()` starts a scanner goroutine that feeds a channel to main loop. (`pkg/testjson/parser.go:64-82`)
- `stream.Run()` invokes `testjson.Stream()` and updates in-memory stream state from callback. (`pkg/stream/stream.go:288-290`)
- `runStream()` creates cancellable context with SIGINT and conditionally closes `stdin` on cancel. (`cmd/fo/main.go:190-197`)

### Findings

#### 1) Scanner goroutine leak when cancelled with non-closable reader
- **Severity:** Medium (Plausible)
- **Evidence:** `Stream()` documents leak risk if `r` is not `io.Closer`; cancellation branch can only close when `r` implements closer.
  - `pkg/testjson/parser.go:56-60,87-92`
- **Snipe lifecycle trace:**
  - `snipe callers Stream` → `pkg/stream/stream.go:288`
  - `snipe callers --id a24083d4e541f6b0` → `cmd/fo/main.go:199` (normal CLI path passes `*bufio.Reader` but adds separate `stdin` close hook only when possible).
- **Mechanism:** on context cancel, scanner goroutine may stay blocked on read forever if underlying reader cannot be closed.
- **Timeline failure scenario:** Embedded use of `stream.Run` with a pipe-like reader wrapper lacking `Close`; repeated cancellations accumulate blocked goroutines.
- **Minimal fix:**
  - Accept `io.ReadCloser` in `Stream`/`Run`, or require a dedicated cancellation-aware reader contract and enforce it.
  - Optionally move scanner loop to synchronous non-goroutine approach with context-aware reads.
- **Test strategy:**
  - Leak test with synthetic non-closable blocking reader + cancellation; assert goroutine count stabilizes after teardown.
- **Confidence:** Medium (Plausible; depends on non-CLI integration choices)

_No additional proven races/deadlocks found in current single-threaded state mutations._

---

## PASS 3 — Persistence & Boundary Audit

### Boundary Inventory
- **stdin parsing boundaries:** `detect.Sniff`, `sarif.ReadBytes`, `testjson.ParseBytes`, `report.Parse`. (`cmd/fo/main.go`)
- **streaming boundary:** incremental stdin reader via `testjson.Stream`. (`pkg/testjson/parser.go`)
- **output boundary:** stdout/stderr writes via renderers and stream term writer.

### Findings

#### 1) Non-atomic boundary failure on oversized `go test -json` lines
- **Severity:** High
- **Evidence + boundary trace:**
  - Entry: `run()` → `parseInput(...detect.GoTestJSON...)` → `testjson.ParseBytes` → `ParseStream` scanner fatal on `ErrTooLong`. (`cmd/fo/main.go:214-218`, `pkg/testjson/parser.go:20,35-37`)
  - Stream path: `runStream()` → `stream.Run()` → `testjson.Stream()` same scanner limit. (`cmd/fo/main.go:199`, `pkg/stream/stream.go:288-300`, `pkg/testjson/parser.go:62,76-79`)
- **Mechanism:** boundary reader enforces hard token cap, causing abrupt parse abort instead of degrading to malformed-line skip.
- **Concrete failure scenario:** one pathological output line causes total report loss (exit 2) despite all other test events being valid.
- **Minimal fix:** stream by reader chunks/lines without scanner cap; treat oversized lines as malformed records and continue.
- **Test plan:** fuzz + regression with mixed valid lines and one >1 MiB line; assert remaining events still processed.
- **Confidence:** High

#### 2) Report boundary parsing is line-ending fragile (CRLF incompatibility)
- **Severity:** Medium
- **Evidence + boundary trace:**
  - Entry: `run()` → `parseInput(detect.Report)` → `report.Parse()`. (`cmd/fo/main.go:224-229`)
  - Parse logic fails exact regex match for CRLF delimiters. (`internal/report/report.go:13-15,35-41`)
- **Mechanism:** strict delimiter matching without CR normalization breaks cross-platform producer compatibility.
- **Concrete failure scenario:** CI step on Windows emits CRLF-delimited sections; Linux consumer `fo` fails to detect any section and exits 2.
- **Minimal fix:** normalize `\r\n` to `\n` before split or trim trailing `\r` per line.
- **Test plan:** add CRLF fixture with 2+ sections and assert parse success and content integrity.
- **Confidence:** High

