---
review_type: concurrency
review_date: 2026-01-19
reviewer: Claude (go-concurrency-reviewer)
codebase_root: .
focus_files: ["all"]
race_detector_run: true
race_detector_log: docs/feedback/race-detector.log
total_findings: 2
summary:
  critical: 0
  high: 2
  medium: 0
  info: 0
---

# Go Concurrency Review - 2026-01-19

## Executive Summary

| Severity | Count | Icon |
|----------|-------|------|
| 🔴 Critical - Deadlock/Race | 0 | 💀 |
| 🟠 High - Goroutine Leak | 2 | 💧 |
| 🟡 Medium - Contention | 0 | 🐢 |
| 🔵 Info - Non-Idiomatic | 0 | 🎨 |

**Hotspot Packages:** (Top 5 by goroutine count)
- `magefile.go` - 7 goroutine launches
- `pkg/dashboard/formatter_simple.go` - 6 goroutine launches
- `pkg/design/config.go` - 5 goroutine launches
- `pkg/dashboard/task.go` - 5 goroutine launches
- `fo/testjson.go` - 5 goroutine launches

## Top 5 Priority Fixes

### 1. Signal handler can block forever when `cmd.Start()` fails in capture mode

**File:** `fo/console.go:1683`
**Function:** `executeCaptureMode`
**Severity:** High
**Category:** deadlock
**Principle Violated:** GoroutineLifecycle

**Finding:**
If a signal arrives before `cmd.Start()` completes and `cmd.Start()` fails, `processStarted` is never closed. The signal handler waits on `<-processStarted` and can block indefinitely, causing `runContext()` to hang on `<-signalHandlerDone`.

**Analysis:**
`signalHandler` and `handleSignal` both block on `processStarted` to gate process cleanup. In the `cmd.Start()` error path, `processStarted` is left open, so a signal-triggered path can deadlock, leaving the command runner stuck and the spinner never stopping.

**Code Snippet:**
```go
// Line 1683-1694
if err := cmd.Start(); err != nil {
    errMsg := formatInternalError("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
    task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
    // ... close pipes ...
    close(cmdDone) // Signal that command has finished (failed to start)
    return getExitCode(err, c.cfg.Debug), err
}
```

**Race Detector Output:**
_Not observed (race detector clean in `docs/feedback/race-detector.log`)._

**Recommendation:**
Ensure `processStarted` is closed in all early-return paths of `executeCaptureMode`, including the `cmd.Start()` error path. A minimal fix is to `close(processStarted)` just before closing `cmdDone` on start failure.

**Repro Command:**
```bash
go test -run TestConsoleRunCapture ./fo
```

**Acceptance Criteria:**
- [ ] Signal handling completes even when `cmd.Start()` fails.
- [ ] `runContext()` never blocks waiting on `signalHandlerDone`.
- [ ] `go test -race` remains clean.

**Labels:** `P1`, `concurrency`, `deadlock`

---

### 2. Concurrent writes to `Task` fields without synchronization in dashboard tasks

**File:** `pkg/dashboard/task.go:72`
**Function:** `runTask`
**Severity:** High
**Category:** race
**Principle Violated:** CorrectLocking

**Finding:**
`runTask` directly mutates `Task.Status`, `Task.ExitCode`, and timestamps while the UI goroutine reads these fields for rendering without synchronization. This creates potential data races and inconsistent UI state under load.

**Analysis:**
`runTask` is a background goroutine; `tui.go` renders task status/duration concurrently. Since only `Task.Output` is protected by `mu`, the other fields are unguarded and can be read while being mutated. The race detector did not exercise this UI path, but this is a real risk in production.

**Code Snippet:**
```go
// Line 72-109
task.StartedAt = time.Now()
task.Status = TaskRunning
updates <- TaskUpdate{Index: index, Status: TaskRunning, StartedAt: task.StartedAt}
// ...
task.FinishedAt = time.Now()
if err != nil {
    task.Status = TaskFailed
    task.ExitCode = exitErr.ExitCode()
}
updates <- TaskUpdate{Index: index, Status: task.Status, ExitCode: task.ExitCode, FinishedAt: task.FinishedAt}
```

**Race Detector Output:**
_Not observed (race detector clean in `docs/feedback/race-detector.log`)._

**Recommendation:**
Avoid mutating shared `Task` state in background goroutines. Instead, treat `TaskUpdate` as the single source of truth and update `Task` state only in the UI goroutine (or guard all `Task` fields with a mutex/atomic accessors). Consider encapsulating state updates in methods that hold a lock for all fields, not just `Output`.

**Repro Command:**
```bash
go test -race -run TestDashboardSuite ./pkg/dashboard
```

**Acceptance Criteria:**
- [ ] No unsynchronized reads/writes of `Task.Status`, `ExitCode`, `StartedAt`, `FinishedAt`.
- [ ] UI renders consistent durations/statuses under concurrent load.
- [ ] `go test -race` remains clean.

**Labels:** `P1`, `concurrency`, `race`

---

## Additional Findings

_No additional findings met the threshold for reporting._

## Analysis Configuration

- **Review Date:** 2026-01-19
- **Code Root:** /workspace/fo
- **Focus Files:** all
- **Race Detector:** true (log: `docs/feedback/race-detector.log`)
- **Tools Used:** ripgrep, go test -race
- **Packages Analyzed:** all Go packages in repo
- **Hotspot Packages:**
  - `magefile.go` - 7 goroutine launches
  - `pkg/dashboard/formatter_simple.go` - 6 goroutine launches
  - `pkg/design/config.go` - 5 goroutine launches
  - `pkg/dashboard/task.go` - 5 goroutine launches
  - `fo/testjson.go` - 5 goroutine launches
