---
review_type: concurrency
review_date: 2025-11-19
reviewer: Claude (go-concurrency-reviewer)
codebase_root: .
focus_files: all
race_detector_run: true
race_detector_log: docs/code-reviews/race-detector.log
total_findings: 2
summary:
  critical: 1
  high: 0
  medium: 0
  info: 1
---

# Go Concurrency Review - 2025-11-19

## Executive Summary

| Severity | Count | Icon |
|----------|-------|------|
| 🔴 Critical - Deadlock/Race | 1 | 💀 |
| 🟠 High - Goroutine Leak | 0 | 💧 |
| 🟡 Medium - Contention | 0 | 🐢 |
| 🔵 Info - Non-Idiomatic | 1 | 🎨 |

**Hotspot Packages:** (Top 5 by goroutine count)
- `mageconsole` - 6 goroutine launches
- `internal/design` - 5 goroutine launches
- `magefile.go` - 3 goroutine launches

## Top 5 Priority Fixes

### 1. Concurrent writes to shared buffer when merging stdout/stderr

**File:** `mageconsole/console.go:445-459`
**Function:** `executeCaptureMode`
**Severity:** Critical
**Category:** race
**Principle Violated:** ChannelSafety

**Finding:**
`outputBuffer` (a `bytes.Buffer`) is written from two goroutines concurrently while copying stdout and stderr into a combined buffer. `bytes.Buffer` is not safe for concurrent use, so simultaneous `ReadFrom` calls can corrupt the buffer or panic under the race detector.

**Analysis:**
In capture mode, two goroutines read from `stdoutBuffer` and `stderrBuffer`, each calling `outputBuffer.ReadFrom`. Without serialization, these writes race and the combined output ordering becomes nondeterministic. The race detector would flag this as a data race, and under heavy output the buffer could become corrupted, producing truncated or interleaved output and undermining downstream parsing/classification.

**Code Snippet:**
```go
var outputBuffer bytes.Buffer
stdoutDone := make(chan struct{})
stderrDone := make(chan struct{})

go func() {
    defer close(stdoutDone)
    limitedReader := io.LimitReader(&stdoutBuffer, c.cfg.MaxBufferSize)
    _, _ = outputBuffer.ReadFrom(limitedReader) // ← races with below
}()

go func() {
    defer close(stderrDone)
    limitedReader := io.LimitReader(&stderrBuffer, c.cfg.MaxBufferSize)
    _, _ = outputBuffer.ReadFrom(limitedReader) // ← races
}()
```

**Recommendation:**
Serialize the merges. E.g., perform the two `ReadFrom` calls sequentially after the copy goroutines finish, or protect the shared buffer with a mutex and keep ordering deterministic. Simpler: remove the goroutines and call `outputBuffer.ReadFrom` for stdout then stderr on the current goroutine after `wgRead.Wait()`.

**Repro Command:**
```bash
go test -race ./...
```

**Acceptance Criteria:**
- [ ] No data race reported by `go test -race` in `executeCaptureMode`
- [ ] Captured stdout/stderr are deterministically concatenated in order
- [ ] Capture mode still honors `MaxBufferSize`

**Labels:** `P0`, `concurrency`, `race`

## Additional Findings

### Informational

**File:** `mageconsole/console.go:115-191`
**Function:** `runContext` signal watcher
**Severity:** Info
**Category:** leak
**Principle Violated:** GoroutineLifecycle

**Finding:**
The signal-watcher goroutine blocks on `sigChan`/`ctx.Done()` but the `cmdDone` channel it selects on is only closed by the goroutine itself. Command completion does not notify this goroutine, so it relies solely on external cancellation to exit.

**Analysis:**
In the current `Run` path a deferred `cancel()` eventually stops the goroutine, so it does not leak. However, if `runContext` were reused elsewhere without a matching cancellation, the goroutine could remain blocked after a natural command exit. Consider either removing the unused `cmdDone` case or explicitly closing `cmdDone` when the command ends to make the lifecycle self-contained.

**Recommendation:**
Close `cmdDone` from the command execution path (after `cmd.Wait`) and keep the goroutine's select cases aligned with the actual signals it can receive. Alternatively, drop `cmdDone` from the select to avoid suggesting a completion signal that never arrives.

**Repro Command:**
```bash
go test ./mageconsole -run TestNonexistent # structural check only
```

**Acceptance Criteria:**
- [ ] Signal watcher goroutine terminates without relying on outer defers
- [ ] Channel usage reflects real ownership (no unused cases)

**Labels:** `P3`, `concurrency`, `cleanup`

## Analysis Configuration

- **Review Date:** 2025-11-19
- **Code Root:** .
- **Focus Files:** all
- **Race Detector:** run (see docs/code-reviews/race-detector.log)
- **Tools Used:** ripgrep, go test -race
- **Packages Analyzed:** 5
- **Hotspot Packages:** mageconsole, internal/design, magefile
