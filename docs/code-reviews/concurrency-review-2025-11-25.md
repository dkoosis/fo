---
review_type: concurrency
review_date: 2025-11-25
reviewer: Claude (go-concurrency-reviewer)
codebase_root: .
focus_files: ["all"]
race_detector_run: true
race_detector_log: docs/code-reviews/race-detector.log
total_findings: 1
summary:
  critical: 1
  high: 0
  medium: 0
  info: 0
---

# Go Concurrency Review - 2025-11-25

## Executive Summary

| Severity | Count | Icon |
|----------|-------|------|
| 🔴 Critical - Deadlock/Race | 1 | 💀 |
| 🟠 High - Goroutine Leak | 0 | 💧 |
| 🟡 Medium - Contention | 0 | 🐢 |
| 🔵 Info - Non-Idiomatic | 0 | 🎨 |

**Hotspot Packages:** (Top 5 by goroutine/count keywords)
- `pkg/design` - 20 matches
- `fo` - 8 matches
- `internal/magetasks` - 6 matches
- `examples/mage` - 5 matches
- `pkg/adapter` - 4 matches

## Top 5 Priority Fixes

### 1. cmdDone not closed on early pipe errors can deadlock runContext

**File:** `fo/console.go:1343-1380`
**Function:** `executeCaptureMode`
**Severity:** Critical
**Category:** deadlock
**Principle Violated:** ChannelSafety | GoroutineLifecycle

**Finding:**
Early failures creating stdout/stderr pipes return without closing `cmdDone`. The `runContext` caller waits on `signalHandlerDone`, but the signal handler goroutine waits on `cmdDone` or cancellation. When pipe setup fails, `cmdDone` is never closed and the context is never canceled, so both goroutines block forever and `Run` hangs on initialization errors.

**Analysis:**
`runContext` launches the signal handler and then blocks on `<-signalHandlerDone>`. The handler’s select listens for `sigChan`, `ctx.Done()`, or `cmdDone`. In the stdout/stderr pipe error paths (lines 1350-1366), `executeCaptureMode` returns without closing `cmdDone` or canceling `ctx`. With no signal and no cancellation, the handler never exits, producing a deadlock whenever pipe creation fails (e.g., descriptor exhaustion or permission errors).

**Code Snippet:**
```go
stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    ...
    return 1, err // ← cmdDone not closed; signal handler blocks
}

stderrPipe, err := cmd.StderrPipe()
if err != nil {
    ...
    return 1, err // ← cmdDone not closed; ctx not canceled
}
```

**Recommendation:**
Ensure `cmdDone` is closed (and optionally cancel `ctx`) on all early-return paths before the goroutine waits on it. Example fix:
```go
defer func() { close(cmdDone) }() // only if not already closed later
stdoutPipe, err := cmd.StdoutPipe()
if err != nil {
    ...
    return 1, err
}
// similarly close cmdDone before returning on stderr pipe error
```
Or explicitly close `cmdDone` in each error branch prior to return.

**Repro Command:**
```bash
# Simulate pipe creation failure by injecting a stub or forcing fd exhaustion,
# then run the failing call path
FO_DEBUG=1 go test -run TestExecuteCaptureModePipeError ./fo
```

**Acceptance Criteria:**
- [ ] `executeCaptureMode` closes `cmdDone` on every return path before `runContext` waits.
- [ ] `Run` no longer hangs when pipe creation fails (add regression test triggering pipe error).
- [ ] `go test -race ./...` passes.

**Labels:** `P0`, `concurrency`, `deadlock`

## Additional Findings

### Critical
- None

### High
- None

### Medium
- None

### Informational
- None

## Analysis Configuration

- **Review Date:** 2025-11-25
- **Code Root:** .
- **Focus Files:** all
- **Race Detector:** run (see docs/code-reviews/race-detector.log)
- **Tools Used:** go test -race, ripgrep
- **Packages Analyzed:** all
- **Hotspot Packages:** pkg/design, fo, internal/magetasks, examples/mage, pkg/adapter
