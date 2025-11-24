---
review_type: concurrency
review_date: 2025-11-24
reviewer: Claude (go-concurrency-reviewer)
codebase_root: .
focus_files: "all"
race_detector_run: true
race_detector_log: docs/code-reviews/race-detector.log
total_findings: 1
summary:
  critical: 1
  high: 0
  medium: 0
  info: 0
---

# Go Concurrency Review - 2025-11-24

## Executive Summary

| Severity | Count | Icon |
|----------|-------|------|
| 🔴 Critical - Deadlock/Race | 1 | 💀 |
| 🟠 High - Goroutine Leak | 0 | 💧 |
| 🟡 Medium - Contention | 0 | 🐢 |
| 🔵 Info - Non-Idiomatic | 0 | 🎨 |

**Hotspot Packages:** (Top by goroutine count)
- `mageconsole` - goroutines for signal handling and stream capture
- `pkg/design` - spinner goroutine for inline progress

## Top 5 Priority Fixes

### 1. Global Title Caser is not concurrency-safe (confirmed data race)

**File:** `pkg/design/render.go:15`
**Function:** `RenderDirectMessage`
**Severity:** Critical
**Category:** race
**Principle Violated:** CorrectLocking

**Finding:**
A package-level `titler := cases.Title(language.English)` is shared by all callers. `cases.Title` is not safe for concurrent use, and `RenderDirectMessage` invokes `titler.String` from multiple goroutines during parallel tests, triggering races flagged by `go test -race`.

**Analysis:**
Shared mutable state inside the `cases.Caser` leaks across goroutines. Under load (e.g., rendering multiple messages concurrently), this can corrupt internal buffers, producing incorrect titles or panics.

**Code Snippet:**
```go
var titler = cases.Title(language.English)
...
styleKey = titler.String(lowerMessageType)
```

**Race Detector Output:**
```
WARNING: DATA RACE
Write at ... cases.(*titleCaser).Transform()
  github.com/dkoosis/fo/pkg/design.RenderDirectMessage()
      /workspace/fo/pkg/design/render.go:630 +0x198
```

**Recommendation:**
Avoid sharing the caser instance. Use a `sync.Pool` or create a new `cases.Title` per call, e.g. `cases.Title(language.English).String(...)`, or wrap access with a mutex if pooling.

**Repro Command:**
```bash
go test -race ./pkg/design -run TestRenderDirectMessage
```

**Acceptance Criteria:**
- [ ] `go test -race ./pkg/design` passes with no race warnings.
- [ ] Concurrent renders produce consistent title casing.

**Labels:** `P0`, `concurrency`, `race`

## Additional Findings

_No additional issues identified beyond the top priority item._

## Analysis Configuration

- **Review Date:** 2025-11-24
- **Code Root:** .
- **Focus Files:** all
- **Race Detector:** true (see docs/code-reviews/race-detector.log)
- **Tools Used:** ripgrep, go test -race
- **Packages Analyzed:** mageconsole, pkg/design
- **Hotspot Packages:** mageconsole, pkg/design
