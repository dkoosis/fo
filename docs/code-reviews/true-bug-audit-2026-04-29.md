# True Bug Audit — 2026-04-29

## PASS 1 — Correctness & Reliability

### System Map
- **Entry points:** `main()` → `run()`; `run()` dispatches to `runWrap`, `runState`, or parse/render path. Streaming path is selected only when `--format=auto`, stdout is TTY, and input sniff matches go test JSON. Reachability: `main -> run -> runStream/runStreamCtx|parseToReport`. 
- **Concurrency model:** non-stream path is single-threaded; stream path has one parser goroutine sending snapshots to renderer via buffered channel. Cancellation propagates by `context.AfterFunc` closing stdin if closable.
- **Persistence:** sidecar state file at `.fo/last-run.json` via `attachDiff()` calling `state.Load`, `state.Classify`, `state.Append`, `state.Save`.
- **Boundaries:** stdin parsing (`sarif.ReadBytes`, `testjson.ParseBytes`/`Stream`), stdout/stderr rendering, filesystem sidecar read/write.

### Findings

1) **Silent state persistence failure causes repeated false “new”/“resolved” churn**  
**Severity:** Medium  
**Evidence:** `attachDiff` logs `state.Save` errors but continues normal success path; call chain is `run -> attachDiff -> state.Save` on every non-`--no-state` run.  
**Mechanism:** if disk is full/readonly/permission denied, sidecar is never updated; next run compares against stale historical state and produces incorrect diff classifications.  
**Concrete scenario:** CI job mounted with intermittent read-only workspace marks findings as “new” every run, hiding real regressions in noise.  
**Minimal fix:** escalate save failure into report metadata/finding and optionally return non-zero in strict mode (e.g., `--state-strict`). Add regression test for Save error path classification behavior.
**Confidence:** High.

2) **Huge go test input can OOM in non-stream modes due to full buffering**  
**Severity:** High  
**Evidence:** non-stream path calls `boundread.All(br, 0)` which falls back to `DefaultMax = 256 MiB` (`internal/boundread/boundread.go:13`); `cmd/fo/main.go:148-149` already returns a stream-mode hint on `ErrInputTooLarge`. Stream mode itself is gated on `--format=auto` + TTY + go-test sniff (`cmd/fo/main.go:142`), so piped CI with explicit `--format llm/json/human` cannot use the streaming path even when input is large.  
**Mechanism:** non-TTY/non-auto callers buffer up to 256 MiB before parsing, then hard-fail rather than degrade to streaming; large `go test -json` runs in CI hit the cap and the user has no way to opt into streaming.  
**Concrete scenario:** monorepo integration run pipes multi-GB JSON to `fo --format llm`; fo aborts at 256 MiB with the "use stream mode" hint, but stream mode is unreachable from this invocation.  
**Minimal fix:** allow streaming parser for go test regardless of TTY when format is llm/json/human (with snapshot throttling), or expose an explicit `--stream` opt-in. Add stress test with large synthetic input.
**Confidence:** High.

3) **Parse errors are collapsed into generic “unrecognized input”, losing operational diagnosability**  
**Severity:** Low  
**Evidence:** `parseTestJSONTolerant` converts any `testjson.ParseBytes` error into `errUnrecognizedInput` when `err != nil || len(results)==0`. Called from `parseToReport` fallback path.  
**Mechanism:** real parser failures (e.g., truncated JSON, scanner IO issues) become indistinguishable from wrong tool input; operators cannot differentiate producer bugs from misrouting.  
**Concrete scenario:** wrapper truncates stream on timeout; user sees generic input error and reroutes tools instead of fixing upstream truncation, prolonging incident.  
**Minimal fix:** preserve original parse error when at least one JSON-looking line exists or when parser returned non-EOF failure; keep unrecognized only for true no-signal input. Add tests for truncated-stream diagnostics.
**Confidence:** Medium.

## PASS 2 — Concurrency & Lifecycle

### Concurrency Roots Inventory
- `runStreamCtx` starts parser goroutine; owns `snapshots`, `finalCh`, `parseErrCh` and depends on renderer drain behavior.
- `testjson.Stream` starts internal cancellation watcher goroutine and scanner loop.
- Cancellation source: signal context in `runStream` (SIGINT) or caller context in tests.

### Findings

1) **Potential head-of-line stall from bounded snapshot channel under slow renderer**  
**Severity:** Medium  
**Evidence:** `snapshots := make(chan report.Report, 8)`; parser goroutine sends snapshots for each package-finish event. If renderer is slow and packages finish rapidly, producer blocks in send, delaying parse progress and cancellation responsiveness to next select.  
**Mechanism:** fixed buffer can backpressure parser hard; large package fan-out may freeze perceived progress in long runs.  
**Timeline scenario:** 1k packages emit terminal pass events quickly, renderer spends time painting wide output; parser blocks on channel send and appears hung until renderer catches up.  
**Minimal fix:** non-blocking coalescing send (drop intermediate snapshots, keep latest) or dedicated latest-value atomic snapshot with periodic render tick.  
**Test strategy:** benchmark-style test with synthetic rapid package completions + slow writer, asserting bounded completion latency.  
**Confidence:** Medium.

## PASS 3 — Persistence & Boundary

### Boundary Inventory
- Filesystem: `state.Load/Save/Reset`.
- JSON boundaries: `sarif.ReadBytes`, `testjson.ParseBytes`, `testjson.Stream`.
- CLI IO: stdin reader with sniff/peek and possible close-on-cancel.

### Findings

1) **State durability acknowledged but not surfaced when directory fsync fails**  
**Severity:** Low  
**Evidence:** `state.Save` ignores `syncDir` error intentionally; caller cannot detect rename durability degradation.  
**Mechanism:** after crash/power loss, directory entry update may be lost; previous state reappears, skewing diff classification.  
**Concrete failure scenario:** NFS/virtualized FS intermittently fails directory sync; process reports success, but after node reboot state file reverts and findings are mislabeled as newly introduced.  
**Minimal fix:** return warning channel/flag when dir sync fails and emit explicit stderr warning (once) so operators understand durability risk.  
**Test plan:** inject failing `syncDir` and assert warning propagation path.
**Confidence:** Medium.
