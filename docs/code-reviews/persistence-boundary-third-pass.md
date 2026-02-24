# Third Pass: Persistence + Boundary Failure Bug Hunt

## Boundary Inventory

> `snipe` was not available in this container (`snipe: command not found`), so this inventory and call tracing was performed with direct code navigation and symbol tracing via `rg` + file inspection.

### 1) Persistence (DB/transaction surfaces)

- No `database/sql`, ORM, KV store, transaction helper, or repository layer is present in this repository.
- No write roots calling `Exec/Query/Begin/Commit/Rollback` exist.

### 2) External calls (HTTP/RPC/queues)

- No outbound HTTP client / RPC client / webhook / queue producer code paths were found.
- No retry/backoff wrappers were found.

### 3) File / stream boundaries (active reliability surfaces)

Primary boundary packages and top paths:

- `cmd/fo/main.go`
  - Batch path: `main -> run -> runBatch -> parseInput -> (sarif.ReadBytes | testjson.ParseBytes)`.
  - Stream path: `main -> run -> runStream -> stream.Run -> testjson.Stream`.
  - Wrap path: `main -> run -> runWrap -> scanner.Scan + parseDiagLine -> sarif.Builder.WriteTo`.
- `pkg/testjson/parser.go`
  - NDJSON ingestion in `ParseStream` and `Stream` using `bufio.Scanner` + per-line `json.Unmarshal`.
- `pkg/sarif/reader.go`
  - SARIF JSON ingestion in `Read` via `json.Decoder.Decode`.
- `pkg/sarif/builder.go`
  - SARIF emission in `WriteTo` via `json.MarshalIndent` + single `Writer.Write`.

---

## Ranked Findings

## 1) Silent event drops in go test NDJSON parser cause incorrect test results (data loss)

- **Severity:** High
- **Evidence:**
  - `ParseStream` and `Stream` both skip malformed JSON lines via `continue` with no error propagation or accounting.
  - Reachability:
    - Batch: `cmd/fo/main.go` `runBatch -> parseInput -> testjson.ParseBytes -> ParseStream`.
    - Streaming: `cmd/fo/main.go` `runStream -> stream.Run -> testjson.Stream`.
- **Mechanism:**
  - Any line-level decode failure (truncated pipe chunk, mixed stdout noise, partial write from producer) is silently discarded. The aggregator then computes pass/fail/skip counts from incomplete event history.
- **Concrete failure scenario:**
  1. `go test -json` output is piped through a flaky wrapper that occasionally emits partial JSON lines.
  2. A `{"Action":"fail",...}` line fails `json.Unmarshal` and gets dropped.
  3. Package summary is computed without that fail event.
  4. `fo` reports fewer/no failures; automation or users accept false-clean state.
- **Minimal fix:**
  - Track malformed-line count and return an error once non-zero (strict mode), or surface warnings and mark output as degraded.
  - At minimum, attach line number + sample parse error to stderr/log so loss is detectable.
- **Test plan:**
  - Unit: feed NDJSON with one malformed fail-event line; assert parser returns non-nil error (or degraded flag).
  - Integration: pipe mixed valid + broken events through `runBatch` and assert exit code reflects parse degradation.
  - Idempotency: replay same stream twice; ensure malformed accounting is deterministic.
- **Confidence:** Certain

## 2) Cancellation does not interrupt blocked stream reads, leading to stuck interactive sessions

- **Severity:** High
- **Evidence:**
  - `runStream` derives cancelable context from `signal.NotifyContext`.
  - `testjson.Stream` checks `ctx.Err()` only *inside* the `for scanner.Scan()` loop body.
  - `scanner.Scan()` is blocking; cancellation is not observed until a new token is read or EOF/error occurs.
- **Mechanism:**
  - If input stalls without newline/EOF (common with long-running or wedged test subprocess pipelines), SIGINT cancels context but the scanner stays blocked on read. `fo` appears unresponsive and can require hard kill.
- **Concrete failure scenario:**
  1. User runs `go test -json ./... | fo`.
  2. Upstream process hangs mid-line (or pipe deadlocks).
  3. User presses Ctrl-C; context is canceled in `runStream`.
  4. `testjson.Stream` never re-enters loop body to observe `ctx.Err()`.
  5. Process does not exit promptly (reliability incident in interactive boundary behavior).
- **Minimal fix:**
  - Replace scanner loop with a decoder/reader architecture that can be interrupted by context (e.g., goroutine read pump + select on `ctx.Done()`), or close underlying reader on cancel where feasible.
- **Test plan:**
  - Unit: custom reader that blocks forever in `Read`; cancel context and assert `Stream` returns within timeout.
  - Integration: spawn subprocess feeding partial line then stall; send SIGINT and verify timely exit with code 130.
- **Confidence:** Likely

## 3) SARIF reader accepts trailing garbage after first JSON value (incorrect read acceptance)

- **Severity:** Medium
- **Evidence:**
  - `pkg/sarif/reader.go` `Read` performs a single `json.NewDecoder(r).Decode(&doc)` then validates document, with no EOF/extra-token check.
- **Mechanism:**
  - `Decode` of one value succeeds even when additional non-whitespace bytes follow. Corrupted concatenated input can be treated as valid SARIF, masking upstream transport/file corruption.
- **Concrete failure scenario:**
  1. A pipeline concatenates `sarif.json` with unrelated log bytes due to boundary bug.
  2. First JSON object decodes successfully.
  3. `fo` accepts document as valid and ignores trailing corruption.
  4. Data integrity checks miss that input artifact is malformed/contaminated.
- **Minimal fix:**
  - After first decode, perform a second decode into `struct{}` and require `io.EOF`.
- **Test plan:**
  - Unit: `ReadBytes([]byte(validSARIF + "\nGARBAGE"))` should fail.
  - Integration: file-based read with concatenated payload should return parse error.
- **Confidence:** Certain

## 4) `fo wrap sarif` can drop diagnostics due to scanner token limit and format fallthrough

- **Severity:** Medium
- **Evidence:**
  - `runWrap` uses default `bufio.Scanner` settings (64 KiB token limit) and does not call `scanner.Buffer(...)`.
  - `runWrap` silently drops unrecognized lines (`if file == "" { continue }`).
- **Mechanism:**
  - Very long diagnostic lines (common with generated code snippets or analyzer payloads) can trigger `ErrTooLong`, aborting conversion before output write.
  - Even within limit, parse mismatches are silently discarded, causing unnoticed loss of findings.
- **Concrete failure scenario:**
  1. Tool emits a diagnostic line >64 KiB.
  2. Scanner returns `ErrTooLong`; `runWrap` exits non-zero and emits no SARIF.
  3. CI step relying on SARIF artifact misses all findings (data loss at conversion boundary).
  4. Separately, non-matching but important diagnostics are ignored without visibility.
- **Minimal fix:**
  - Increase scanner buffer (e.g., 1–10 MiB, as already done in testjson parser).
  - Count/report dropped lines; optionally include an `artifacts/misc` fallback result instead of silent drop.
- **Test plan:**
  - Unit: pass line >64 KiB and assert wrap handles it after buffer bump.
  - Integration: mixed parseable/unparseable diagnostics; assert dropped-line count surfaced.
- **Confidence:** Certain
