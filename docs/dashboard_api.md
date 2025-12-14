# Dashboard API (Phase 2)

The `dashboard` package exposes a minimal, semver-stable API for orchestrating build suites from Go code. It replaces spawning multiple `fo` CLI processes with a single, coordinated dashboard run that works in both TTY (TUI) and non-TTY (CI) environments.

## Quick Start

```go
package main

import (
        "context"
        "log"

        "github.com/dkoosis/fo/dashboard"
)

func main() {
        dash := dashboard.New("ORCA BUILD SUITE")
        dash.AddTask("Build", "go build", "go", "build", "./...")
        dash.AddTask("Quality", "golangci-lint", "golangci-lint", "run")
        dash.AddTask("Quality", "staticcheck", "staticcheck", "./...")

        result, err := dash.Run(context.Background())
        if err != nil {
                log.Fatalf("suite failed: %v", err)
        }
        log.Printf("completed %d tasks", len(result.Tasks))
}
```

## Behavior

- **TTY detection:** Defaults to auto-detect based on stdout. When not a TTY (CI), output is streamed with a task prefix and a final summary table. Use `WithTTY` to force behavior.
- **Concurrency:** All tasks start concurrently and run to completion.
- **Output capture:** Stdout/stderr are merged and captured with line preservation. The last N lines (default 5000) are retained per task via `OutputTail`.
- **Results and errors:** `SuiteResult` aggregates per-task `TaskResult` structs. `Run` returns a non-nil error if any task fails and `AllowFailure` is false.

## Options

- `WithStdout(w io.Writer)`, `WithStderr(w io.Writer)`: Override writers.
- `WithTTY(force *bool)`: Force TTY on/off or allow auto-detect (nil).
- `WithMaxTailLines(n int)`: Adjust tail retention per task (default 5000).
- `WithOnEvent(fn func(Event))`: Receive lifecycle and output events.

## Caveats

- Output tails are truncated to the last N lines per task to bound memory use.
- Bubble Tea UI integration is encapsulated; callers only work with the stable `dashboard` API.
- Parser hooks for SARIF and other formats are planned but not implemented in this phase.
