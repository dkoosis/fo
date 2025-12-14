# Dashboard mode: unified TUI for parallel command execution

## Problem

When multiple commands run in parallel (e.g., mage orchestrating 13 linters), each `fo` invocation is independent. They all write to the same terminal, resulting in:

- Interleaved "Running..." status boxes that pile up (lines 4-65 of captured output are noise)
- No unified view of overall progress
- No way to inspect individual command output without scrolling through chaos

**Real-world example:** Orca's `mage qa` runs 13 parallel checks. The current output is unusable for understanding what's happening.

## Proposed Solution

Add a **dashboard mode** to fo that coordinates multiple commands in a single bubbletea TUI.

### Two-View Architecture

**List View** (default)
```
┌─────────────────────────────────────────────────────────┐
│ ORCA BUILD SUITE                              12/13 ✓  │
├─────────────────────────────────────────────────────────┤
│ BUILD                                                   │
│   ✓ go build                                     4.2s  │
├─────────────────────────────────────────────────────────┤
│ QUALITY                                         8/13   │
│   ✓ filesize                                     0.2s  │
│   ✓ lintkit                                      6.7s  │
│   ✓ go-arch-lint                                 7.6s  │
│ → ● golangci-lint                    ━━━━━━░░░░  34s  │
│   ● staticcheck                      ━━━░░░░░░░  12s  │
│   ● nilaway                          ━━░░░░░░░░   8s  │
│   ○ govulncheck                           pending      │
│   ✗ go-vet                              1 error  7.0s  │
│                                                        │
│                                          ↑↓ navigate   │
│                                          ⏎ view detail │
│                                          q quit        │
└─────────────────────────────────────────────────────────┘
```

- Compact one-line-per-task display
- Real-time status updates (spinner/progress for running, icon for complete)
- Arrow keys navigate, highlighted row shown with `→`
- Enter expands to detail view
- Overall progress in header

**Detail View** (on selection)
```
┌─────────────────────────────────────────────────────────┐
│ ● golangci-lint                              running   │
├─────────────────────────────────────────────────────────┤
│ > golangci-lint run --output.sarif.path=stdout         │
├─────────────────────────────────────────────────────────┤
│ internal/kg/importnugs/import.go                       │
│   ✗ typecheck:1:1                                      │
│                                                        │
│ internal/mcp/server/go_symbol_handler.go               │
│   △ U1000: var goSymbolInputSchema is unused           │
│   △ U1000: var goSymbolOutputSchema is unused          │
│                                                        │
│ ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ │
│                                                        │
│                                          esc back      │
│                                          ↑↓ scroll     │
└─────────────────────────────────────────────────────────┘
```

- Full scrollable viewport of task output
- Streaming updates as command runs
- Shows parsed/rendered output (SARIF, test JSON, etc.)
- Esc returns to list view

### Invocation

**Option A: Manifest from stdin**
```bash
fo --dashboard << EOF
build: go build ./...
test: go test -json ./...
lint: golangci-lint run
staticcheck: staticcheck ./...
EOF
```

**Option B: Multiple command args**
```bash
fo --dashboard \
  --task "build:go build ./..." \
  --task "lint:golangci-lint run" \
  --task "test:go test -json ./..."
```

**Option C: Config file**
```bash
fo --dashboard --config .fo-dashboard.yaml
```

**Option D: Library API for mage integration**
```go
func Qa() error {
    dash := fo.NewDashboard("Orca Build Suite")

    dash.AddGroup("Build",
        fo.Task("go build", "go", "build", "./..."))

    dash.AddGroup("Quality",
        fo.Task("golangci-lint", "golangci-lint", "run"),
        fo.Task("staticcheck", "staticcheck", "./..."),
        fo.Task("go-vet", "go", "vet", "./..."),
        // ... more tasks
    )

    return dash.Run() // blocks until all complete, returns aggregated error
}
```

## Technical Design

### Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - viewport, spinner components
- Already using `lipgloss` for styling

### Architecture
```
dashboard/
  model.go      - main bubbletea Model (list + detail states)
  list.go       - list view component
  detail.go     - detail view with viewport
  task.go       - task runner, captures output, sends messages
  group.go      - task grouping for visual organization
  keys.go       - keybindings
```

### Message Flow
1. Dashboard spawns goroutines for each task
2. Tasks send status messages: `TaskStarted`, `TaskOutput`, `TaskCompleted`
3. Model updates state, triggers re-render
4. bubbletea handles screen updates efficiently

### Non-TTY Fallback
When stdout is not a TTY (CI, piped output):
- Skip interactive TUI
- Fall back to streaming output with clear task prefixes
- Or render final summary only

## Test Project: Orca

Orca's `magefile.go` currently spawns 13 parallel `fo` processes. We'll:

1. Implement dashboard mode in fo
2. Update orca's magefile to use the library API
3. Compare output quality before/after
4. Validate with captured output in `fo/visual_test_outputs/`

### Orca's Current Tasks
```
Build:
- go build

Quality (parallel):
- test, test-race, test-arch
- golangci-lint, staticcheck, go-vet
- govulncheck, go-arch-lint, nilaway
- gofmt, filesize, docsprawl, nobackups
```

## Acceptance Criteria

- [ ] `fo --dashboard` launches interactive TUI
- [ ] List view shows all tasks with real-time status
- [ ] Navigation works (arrow keys, enter, esc)
- [ ] Detail view shows scrollable task output
- [ ] Running tasks show spinner/progress
- [ ] Completed tasks show duration and status icon
- [ ] Failed tasks highlighted, errors visible in list
- [ ] Non-TTY falls back gracefully
- [ ] Library API available for Go integration
- [ ] Orca magefile updated as reference implementation
- [ ] Visual test outputs captured for regression testing

## Out of Scope (Future)

- Task dependency graphs (run B after A)
- Persistent task history
- Remote/distributed execution
- Custom keybindings config
