# Streaming Test Output Design

**Date:** 2026-02-22
**Status:** Ready

## Problem

fo reads all stdin before rendering. For a 12-second test suite (467 tests, 9 packages), the user sees nothing until everything finishes. The build output renderer should show test results as they arrive.

## Solution

Two-region terminal display for `go test -json` streams:

1. **Scrolling history** (top) — completed test results append and scroll naturally
2. **In-place footer** (bottom) — currently-running packages, redrawn on each event

```
  embedder  · CosineSimilarity_Identical   0.00s
  embedder  · CosineSimilarity_Orthogonal  0.00s
  embedder  · CosineSimilarity_Opposite    0.00s
  ✓ embedder                       26/26   2.6s

  ─── active ──────────────────────────────────
  eval   [34] TestRun_Timeout           6.2s
  index  [12] TestWatch_File...         4.1s
  mcp    [8]  TestHandler_List          3.0s
```

Output remains copy/paste friendly — history is real terminal scrollback, not alternate screen.

After EOF, the footer is erased and replaced with the final summary.

## Activation

Auto-detect, no flags:

```
go test -json ./... | fo          # TTY → stream
go test -json ./... | fo | tee    # piped → batch
cat results.json | fo             # TTY → stream (replays at full speed)
```

Rules:
- stdout is TTY + input is go test -json → streaming
- stdout not TTY → batch (current behavior, unchanged)
- Input is SARIF → always batch (not streamable)

Note: `cat results.json | fo` replays at full speed with no pacing. This is expected behavior — the streaming display still works, it just finishes instantly.

## Architecture

### Detection

Two separate decisions, evaluated in order:

1. **Format detection** — reuse `detect.Sniff()` on peeked bytes to determine SARIF vs go test -json. This is the existing logic, unchanged.
2. **Stream vs batch** — if format is go test -json AND stdout is a TTY, use streaming. Otherwise batch.

These are independent concerns. Format detection answers "what is this input?" Stream/batch answers "how should we consume it?"

Use a `bufio.Reader` wrapping stdin. Peek enough bytes for `detect.Sniff()` to work (it needs to see the opening structure). Peeked bytes are not consumed — the reader is passed to either the streaming or batch path.

SARIF is always batch — it's a single JSON document that must be fully parsed before rendering.

### Data flow

```
stdin (io.Reader)
  │
  ├── bufio.Reader: peek bytes for detect.Sniff()
  │     ├── go test -json + TTY stdout → streaming path
  │     ├── go test -json + piped stdout → batch path (existing)
  │     ├── SARIF → batch path (existing)
  │     └── unknown → batch path (existing)
  │
  streaming path:
  │
  ├── stream.Run(ctx, io.Reader, stdout, theme)
  │     │
  │     ├── reads events via testjson.Stream(ctx, r, fn)
  │     ├── maintains state:
  │     │     - active: map[package]*pkgProgress
  │     │     - completed: aggregated stats for final summary
  │     │     - tw: *termWriter (owns footer state)
  │     │     - outputBuf: map[pkg/test][]string (per-test output buffer)
  │     │
  │     ├── on test pass/skip:
  │     │     1. erase footer
  │     │     2. print result line: "  pkg  · TestName  0.02s"
  │     │     3. discard buffered output for this test
  │     │     4. redraw footer
  │     │
  │     ├── on test fail:
  │     │     1. erase footer
  │     │     2. print fail line: "  pkg  ✗ TestName  0.02s"
  │     │     3. flush buffered output (indented)
  │     │     4. redraw footer
  │     │
  │     ├── on output event:
  │     │     buffer into outputBuf[pkg/test] (do not print)
  │     │
  │     ├── on package pass/fail:
  │     │     1. erase footer
  │     │     2. print package summary: "  ✓ pkg  26/26  2.6s"
  │     │     3. remove from active set
  │     │     4. redraw footer
  │     │
  │     └── on EOF or ctx.Done():
  │           1. erase footer
  │           2. build final summary via existing mapper + terminal renderer
  │           3. print summary
  │
  └── exit code from accumulated state
```

### Event processing

Reuse the existing `testjson.TestEvent` type. Add a callback-style API with context support:

```go
// ProcessFunc is called for each parsed event.
type ProcessFunc func(TestEvent)

// Stream parses go test -json events and calls fn for each one.
// Stops on EOF or when ctx is cancelled.
func Stream(ctx context.Context, r io.Reader, fn ProcessFunc) error
```

Uses `bufio.Scanner` with explicit large buffer (already in existing code: `scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)`) to handle verbose test output and panic traces. The context enables clean SIGINT cancellation — the read loop checks `ctx.Err()` between events.

The aggregator logic stays — `stream.Run` maintains its own lightweight state for display, and also feeds events into an aggregator for the final summary.

### Terminal output discipline

**Hard rule: only one component writes to stdout in streaming mode.**

All terminal output flows through a single `termWriter` owned by `pkg/stream`. No direct `fmt.Fprint(stdout, ...)` from any other code path during streaming. This prevents concurrent writes corrupting cursor position, lipgloss emitting unexpected output, or debug prints breaking footer state.

`termWriter` is a concrete struct (not an interface) with these methods:

```go
type termWriter struct {
    out         io.Writer
    width       int           // read once at init via x/term
    footerLines int           // lines actually printed in current footer
    style       lineStyler    // delegates to pkg/render theme for styling
}

func (w *termWriter) PrintLine(s string)           // write s + \n, scrolls into history
func (w *termWriter) EraseFooter()                  // cursor-up + erase for each footer line
func (w *termWriter) DrawFooter(lines []string)     // truncate to width, print, update footerLines
```

**Styling delegation:** `termWriter` does not import lipgloss directly. It takes a `lineStyler` function (provided at init from `pkg/render`'s theme) that accepts a raw line string and returns a styled string. This keeps `pkg/stream` free of lipgloss and preserves `pkg/render`'s ownership of visual presentation.

```go
// lineStyler formats a raw result line with colors/symbols.
// Provided by pkg/render's theme at stream init time.
type lineStyler func(kind LineKind, text string) string
```

Terminal width is read once at startup. No resize handling mid-stream (documented in out-of-scope).

### ANSI terminal control

No raw mode, no alternate screen. Normal terminal with cursor manipulation. Compatible with common terminals (iTerm, Terminal.app, VS Code terminal, tmux). Degraded behavior on exotic terminals is acceptable.

| Escape | Effect |
|--------|--------|
| `\033[nA` | Cursor up n lines |
| `\033[2K` | Erase entire line |
| `\r` | Carriage return (start of line) |

**Footer redraw invariants:**

1. Every history line ends with `\n` — no partial lines, ever.
2. Every footer line is truncated to terminal width before printing — no wrapping.
3. `footerLines` tracks what was actually printed (after truncation), not what was intended.
4. If `footerLines == 0`, skip the erase step entirely (no `\033[0A` weirdness).
5. Erase algorithm: for each footer line, `\r\033[2K` clears current line, then `\033[1A` moves up. After erasing all footer lines, cursor is back at the start of the footer region.

**Footer size cap:**
- Read terminal height once at start via `term.GetSize()`.
- Cap footer to `min(activeCount+1, max(3, height/3))`.
- If capped, last line shows `  ... and N more`.

**Signal handling:** Register SIGINT handler via context cancellation. Handler erases the footer before exit so the terminal isn't left dirty. Exit with code 130 (standard SIGINT convention).

### Display format

**Test result line (scrolling):**

Every line carries a short package prefix so results are attributable regardless of interleaving. The prefix is the last path segment of the package, right-padded to align within the stream.

```
  embedder  · CosineSimilarity_Identical   0.00s
  embedder  · CosineSimilarity_Orthogonal  0.00s
  eval      ✗ TestRun_Timeout              6.20s
              expected nil, got context.DeadlineExceeded
  index     · TestWatch_FileChange         0.03s
  embedder  ○ TestGPU_Unavailable
```

No package headers. Each line is self-describing.

Symbols: `·` pass, `✗` fail, `○` skip. Failure output is flushed from the per-test buffer, indented under the fail line.

**Package summary (when package completes):**
```
  ✓ embedder                    26/26  2.6s
  ✗ eval                        55/56  8.4s
```

Package summary shows `passed/total` because at package completion time, all tests have reported. This is the only place totals appear — they are known at this point.

**Active footer:**
```
  ─── active ──────────────────────────────────
  eval   [34] TestRun_Timeout           6.2s
  index  [12] TestWatch_File...         4.1s
```

Footer shows `[finished]` count per package — **not** `[finished/total]`. go test -json does not announce total test count up front. Showing `[34/56]` would require knowing 56 before all tests have reported, which is impossible without pre-scanning. The finished count alone is honest and sufficient.

The test name shown is the most recently started test in that package. Duration is elapsed since the first `start` event for the package.

**Final summary (after EOF, replaces footer):**
```
  ─────────────────────────────────────────────
  PASS (12.3s) 467 tests, 9 packages
```

### Event handling matrix

| Action | Test field | Behavior |
|--------|-----------|----------|
| `start` | empty | Record package start time, add to active set, compute pkg prefix |
| `run` | present | Update active package's current test name in footer |
| `pass` | present | Print test result line, discard output buffer |
| `fail` | present | Print test fail line, flush output buffer |
| `skip` | present | Print test skip line |
| `pass` | empty | Print package summary, remove from active |
| `fail` | empty | Print package summary (with failure indicator), remove from active |
| `output` | present | Buffer into `outputBuf[pkg/test]` — do not print |
| `output` | empty | Buffer as package-level output (for build errors, panic traces) |
| `pause` | present | Ignore (test paused for t.Parallel) |
| `cont` | present | Ignore (test resumed) |

Output events are never printed directly during streaming. They are buffered per-test and flushed only on test failure. On test pass/skip, the buffer is discarded. This keeps the history clean — only results and failures are visible.

Exception: package-level output containing `panic:` is flushed immediately as failure output.

### Exit codes

| Condition | Code |
|-----------|------|
| All tests pass | 0 |
| Any test or package fails | 1 |
| fo internal error (parse failure, I/O error) | 2 |
| SIGINT (after cleanup) | 130 |

If a parse error occurs mid-stream (malformed JSON line), skip the line and continue. Only exit 2 for unrecoverable errors (I/O failure on stdout).

## Scope

### In scope
- Streaming `go test -json` to TTY
- Per-test result lines + package summaries
- In-place active-packages footer with size cap
- Failure output inline (buffered, flushed on fail)
- Final summary at EOF
- SIGINT cleanup with context cancellation
- Cancellation-safe event loop

### Unaffected
- `fo wrap sarif` — subcommand produces SARIF output, not consumed by streaming path. No changes needed.

### Out of scope
- Streaming SARIF (not possible — single JSON document)
- Composite `make-all.json` format (section markers)
- Terminal resize handling mid-stream (footer redraws at initial size)
- Progress bars or percentage indicators within the footer
- govulncheck format
- Replay pacing (`--replay-ms` or similar) — future work

## Files

### New
- `pkg/stream/stream.go` — `Run()` entry point, event loop, state machine
- `pkg/stream/termwriter.go` — `termWriter`: ANSI footer erase/draw, line output, width tracking
- `pkg/stream/stream_test.go` — Logic tests with `writeTracker`, ANSI integration tests

### Modified
- `cmd/fo/main.go` — Peek + `detect.Sniff()`, stream vs batch routing, context/signal setup
- `pkg/testjson/parser.go` — Add `Stream(ctx, io.Reader, ProcessFunc)` callback API
- `pkg/render/terminal.go` — Export `lineStyler` function for `pkg/stream` to consume

### Unchanged
- `pkg/pattern/` — Pure data, no changes
- `pkg/mapper/` — Reused for final summary mapping
- `pkg/sarif/` — Batch only, unchanged
- `internal/detect/` — Reused via `Sniff()` on peeked bytes

## Testing

**Approach: testable by design through layered abstraction.**

The key difficulty in testing streaming terminal output is ANSI cursor manipulation. A `bytes.Buffer` captures the raw escape sequences but can't verify that the footer actually erases correctly on a real terminal. The design addresses this by testing at two levels:

### Level 1: Logic tests (no ANSI)

`termWriter` accepts an `io.Writer`. In tests, pass a `bytes.Buffer` wrapped in a `writeTracker` that records each call semantically:

```go
type writeTracker struct {
    calls []writeCall  // e.g., {kind: "printLine", text: "embedder  · TestFoo  0.01s"}
}
```

The `lineStyler` in tests is a no-op (returns the string unstyled). This lets tests assert:

- History lines appear in event order
- Footer line count never exceeds the cap
- A test failure flushes its buffered output immediately after the fail line
- Package summary shows correct pass/total counts
- Package prefix appears on every test line
- Footer is erased before every history write and redrawn after

These are the bulk of the tests. They exercise the event loop, state machine, and output decisions without touching ANSI.

### Level 2: ANSI integration tests (2-3 max)

Feed synthetic events through the full `stream.Run` path, capture raw bytes, and assert exact escape sequences. These catch regressions in:

- `\033[nA` / `\033[2K` footer erase math
- Line truncation to terminal width
- Final summary replacing footer cleanly

Keep these minimal — they're fragile by nature and exist only to verify the escape sequence math.

### Level 3: Replay tests

- `cat testdata/gotest-stream.json | fo` — full-speed replay, assert exit code + no panics
- Existing batch tests unchanged and must continue to pass
