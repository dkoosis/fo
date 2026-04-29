# Streaming Test Output — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add real-time streaming display for `go test -json` output with scrolling history + in-place active-packages footer.

**Architecture:** New `pkg/stream` package with `termWriter` (ANSI cursor control) and event loop. `testjson.Stream()` callback API feeds events. `cmd/fo/main.go` routes to streaming or batch based on `detect.Sniff()` + TTY check. Existing batch pipeline unchanged.

**Tech Stack:** Go 1.24, lipgloss (existing, for styling delegation), x/term (existing, for terminal size), raw ANSI escapes for cursor control.

**Design doc:** `docs/plans/2026-02-22-streaming-test-output-design.md`

---

### Task 1: Add `testjson.Stream()` callback API

The streaming renderer needs to process events one at a time. The existing `ParseStream()` aggregates everything internally. Add a `Stream()` function that calls a user-provided function for each event.

**Files:**
- Modify: `pkg/testjson/parser.go` — add `Stream()` function after line 39
- Modify: `pkg/testjson/types.go` — add `ProcessFunc` type
- Create: `pkg/testjson/stream_test.go`

**Step 1: Write the failing test**

Create `pkg/testjson/stream_test.go`:

```go
package testjson

import (
	"context"
	"strings"
	"testing"
)

func TestStream_CallsFuncForEachEvent(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestFoo"}`,
		`{"Action":"pass","Package":"example.com/pkg","Test":"TestFoo","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	var events []TestEvent
	err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	if events[0].Action != "start" {
		t.Errorf("events[0].Action = %q, want \"start\"", events[0].Action)
	}
	if events[2].Test != "TestFoo" {
		t.Errorf("events[2].Test = %q, want \"TestFoo\"", events[2].Test)
	}
}

func TestStream_SkipsMalformedLines(t *testing.T) {
	input := `not json
{"Action":"start","Package":"example.com/pkg"}
also not json
{"Action":"pass","Package":"example.com/pkg","Elapsed":0.1}
`
	var events []TestEvent
	err := Stream(context.Background(), strings.NewReader(input), func(e TestEvent) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2 (malformed lines skipped)", len(events))
	}
}

func TestStream_RespectsContextCancellation(t *testing.T) {
	// Create a reader that blocks forever after first line
	input := `{"Action":"start","Package":"example.com/pkg"}` + "\n"

	ctx, cancel := context.WithCancel(context.Background())
	var count int
	err := Stream(ctx, strings.NewReader(input), func(e TestEvent) {
		count++
		cancel() // cancel after first event
	})
	// Should return without error (context cancelled after all input read)
	if err != nil && err != context.Canceled {
		t.Fatalf("Stream() unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d events, want 1", count)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/testjson/ -run TestStream -v`
Expected: FAIL — `Stream` undefined

**Step 3: Write minimal implementation**

Add `ProcessFunc` type to `pkg/testjson/types.go` (after line 14):

```go
// ProcessFunc is called for each parsed event during streaming.
type ProcessFunc func(TestEvent)
```

Add `Stream` function to `pkg/testjson/parser.go` (after line 39):

```go
// Stream parses go test -json events line by line and calls fn for each one.
// Stops on EOF or when ctx is cancelled. Malformed lines are silently skipped.
func Stream(ctx context.Context, r io.Reader, fn ProcessFunc) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event TestEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		fn(event)
	}
	return scanner.Err()
}
```

Add `"context"` to the imports in `parser.go`.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/testjson/ -run TestStream -v`
Expected: PASS (3 tests)

Run: `go test ./pkg/testjson/ -v`
Expected: PASS (all existing tests still pass)

**Step 5: Commit**

```
feat: add testjson.Stream() callback API for event-by-event processing
```

---

### Task 2: Create `pkg/stream/termwriter.go` — ANSI footer control

The `termWriter` is the single point of terminal output in streaming mode. It handles line printing, footer erase/draw, and width truncation.

**Files:**
- Create: `pkg/stream/termwriter.go`
- Create: `pkg/stream/termwriter_test.go`

**Step 1: Write the failing tests**

Create `pkg/stream/termwriter_test.go`:

```go
package stream

import (
	"bytes"
	"strings"
	"testing"
)

func TestTermWriter_PrintLine_AppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.PrintLine("hello")
	got := buf.String()
	if got != "hello\n" {
		t.Errorf("PrintLine output = %q, want %q", got, "hello\n")
	}
}

func TestTermWriter_DrawFooter_TracksLineCount(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.DrawFooter([]string{"line1", "line2", "line3"})
	if tw.footerLines != 3 {
		t.Errorf("footerLines = %d, want 3", tw.footerLines)
	}
}

func TestTermWriter_EraseFooter_WhenZeroLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.EraseFooter()
	// Should write nothing when footerLines == 0
	if buf.Len() != 0 {
		t.Errorf("EraseFooter with 0 lines wrote %d bytes, want 0", buf.Len())
	}
}

func TestTermWriter_EraseFooter_ClearsLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.DrawFooter([]string{"line1", "line2"})
	buf.Reset()

	tw.EraseFooter()
	got := buf.String()
	// Should contain cursor-up and erase sequences
	if !strings.Contains(got, "\033[1A") {
		t.Error("EraseFooter missing cursor-up escape")
	}
	if !strings.Contains(got, "\033[2K") {
		t.Error("EraseFooter missing erase-line escape")
	}
	if tw.footerLines != 0 {
		t.Errorf("footerLines after erase = %d, want 0", tw.footerLines)
	}
}

func TestTermWriter_DrawFooter_TruncatesToWidth(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 20, 24)
	tw.DrawFooter([]string{"this is a very long line that exceeds twenty chars"})
	got := buf.String()
	// Each line in output should not exceed width (20 chars + escape sequences)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for _, line := range lines {
		// Strip ANSI escapes for length check
		plain := stripANSI(line)
		if len([]rune(plain)) > 20 {
			t.Errorf("footer line %q exceeds width 20 (len=%d)", plain, len([]rune(plain)))
		}
	}
}

func TestTermWriter_DrawFooter_CapsToMaxLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 12) // height 12 → max footer = max(3, 12/3) = 4
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "pkg" + string(rune('A'+i))
	}
	tw.DrawFooter(lines)
	// Should be capped: 4 lines (3 packages + "... and N more")
	if tw.footerLines > 4 {
		t.Errorf("footerLines = %d, want <= 4 (capped by height)", tw.footerLines)
	}
	got := buf.String()
	if !strings.Contains(got, "... and") {
		t.Error("capped footer should contain '... and N more'")
	}
}

func TestTermWriter_EraseAndRedraw_Cycle(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)

	// Draw, erase, print history, draw again
	tw.DrawFooter([]string{"footer1"})
	tw.EraseFooter()
	tw.PrintLine("history line")
	tw.DrawFooter([]string{"footer2"})

	got := buf.String()
	if !strings.Contains(got, "history line") {
		t.Error("missing history line in output")
	}
	if tw.footerLines != 1 {
		t.Errorf("final footerLines = %d, want 1", tw.footerLines)
	}
}

// stripANSI removes ANSI escape sequences for length checking.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we hit a letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the letter
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/stream/ -v`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

Create `pkg/stream/termwriter.go`:

```go
// Package stream provides real-time streaming display for go test -json output.
package stream

import (
	"fmt"
	"io"
	"strings"
)

// termWriter is the single point of terminal output in streaming mode.
// All output flows through this struct — no other code writes to stdout
// during streaming.
type termWriter struct {
	out         io.Writer
	width       int // terminal width, read once at init
	height      int // terminal height, read once at init
	footerLines int // lines actually printed in current footer
}

func newTermWriter(out io.Writer, width, height int) *termWriter {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return &termWriter{out: out, width: width, height: height}
}

// PrintLine writes a line to the scrolling history region.
// Always appends \n. Must be called after EraseFooter and before DrawFooter.
func (w *termWriter) PrintLine(s string) {
	fmt.Fprintln(w.out, s)
}

// EraseFooter removes the current footer from the terminal.
// If footerLines == 0, this is a no-op.
// After erasing, the cursor is at the start of where the footer was.
func (w *termWriter) EraseFooter() {
	if w.footerLines == 0 {
		return
	}
	// Move to start of footer: cursor up (footerLines - 1) times from current position.
	// We're at the line after the last footer line, so go up footerLines.
	for i := 0; i < w.footerLines; i++ {
		// Erase current line and move up
		fmt.Fprint(w.out, "\r\033[2K")
		if i < w.footerLines-1 {
			fmt.Fprint(w.out, "\033[1A")
		}
	}
	// We're now at the top of the old footer, line erased. Move cursor to start.
	fmt.Fprint(w.out, "\r")
	w.footerLines = 0
}

// DrawFooter prints the footer lines, truncating each to terminal width.
// Updates footerLines to reflect what was actually printed.
func (w *termWriter) DrawFooter(lines []string) {
	maxLines := w.maxFooterLines(len(lines))
	capped := len(lines) > maxLines

	printLines := lines
	if capped && maxLines > 0 {
		printLines = lines[:maxLines-1]
	}

	printed := 0
	for _, line := range printLines {
		truncated := truncateToWidth(line, w.width)
		fmt.Fprintln(w.out, truncated)
		printed++
	}
	if capped {
		overflow := len(lines) - len(printLines)
		more := truncateToWidth(fmt.Sprintf("  ... and %d more", overflow), w.width)
		fmt.Fprintln(w.out, more)
		printed++
	}
	w.footerLines = printed
}

// maxFooterLines returns the maximum footer lines for the given count.
// Caps to min(count, max(3, height/3)).
func (w *termWriter) maxFooterLines(count int) int {
	maxH := w.height / 3
	if maxH < 3 {
		maxH = 3
	}
	if count <= maxH {
		return count
	}
	return maxH
}

// truncateToWidth truncates a string to fit within the given width.
// Accounts for rune width (not byte length).
func truncateToWidth(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(r))
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/stream/ -v`
Expected: PASS (all tests)

**Step 5: Commit**

```
feat: add pkg/stream termWriter with ANSI footer erase/draw
```

---

### Task 3: Create `pkg/stream/stream.go` — event loop and state machine

This is the core: the `Run()` function that reads events, maintains state, and drives the termWriter.

**Files:**
- Create: `pkg/stream/stream.go`
- Create: `pkg/stream/stream_test.go`

**Step 1: Write the failing tests**

Create `pkg/stream/stream_test.go`:

```go
package stream

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/testjson"
)

// helper: build events and run through stream, return output
func runStream(t *testing.T, events []testjson.TestEvent, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	tw := newTermWriter(&buf, width, height)
	s := newStreamer(tw)
	for _, e := range events {
		s.handleEvent(e)
	}
	s.finish()
	return buf.String()
}

func TestStreamer_PassingTest_PrintsResultLine(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/internal/foo"},
		{Action: "run", Package: "example.com/internal/foo", Test: "TestBar"},
		{Action: "pass", Package: "example.com/internal/foo", Test: "TestBar", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/internal/foo", Elapsed: 0.5},
	}
	out := runStream(t, events, 80, 24)
	if !strings.Contains(out, "foo") {
		t.Error("output missing package prefix 'foo'")
	}
	if !strings.Contains(out, "TestBar") {
		t.Error("output missing test name 'TestBar'")
	}
}

func TestStreamer_FailingTest_FlushesOutput(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/x"},
		{Action: "run", Package: "example.com/pkg/x", Test: "TestBad"},
		{Action: "output", Package: "example.com/pkg/x", Test: "TestBad", Output: "expected 1, got 2\n"},
		{Action: "fail", Package: "example.com/pkg/x", Test: "TestBad", Elapsed: 0.02},
		{Action: "fail", Package: "example.com/pkg/x", Elapsed: 1.0},
	}
	out := runStream(t, events, 80, 24)
	if !strings.Contains(out, "expected 1, got 2") {
		t.Error("failing test output not flushed")
	}
}

func TestStreamer_PassingTest_DiscardsOutput(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/x"},
		{Action: "run", Package: "example.com/pkg/x", Test: "TestGood"},
		{Action: "output", Package: "example.com/pkg/x", Test: "TestGood", Output: "some log line\n"},
		{Action: "pass", Package: "example.com/pkg/x", Test: "TestGood", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/pkg/x", Elapsed: 0.5},
	}
	out := runStream(t, events, 80, 24)
	if strings.Contains(out, "some log line") {
		t.Error("passing test output should be discarded, but was printed")
	}
}

func TestStreamer_PackageSummary_ShowsCounts(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/internal/foo"},
		{Action: "run", Package: "example.com/internal/foo", Test: "TestA"},
		{Action: "pass", Package: "example.com/internal/foo", Test: "TestA", Elapsed: 0.01},
		{Action: "run", Package: "example.com/internal/foo", Test: "TestB"},
		{Action: "pass", Package: "example.com/internal/foo", Test: "TestB", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/internal/foo", Elapsed: 0.5},
	}
	out := runStream(t, events, 80, 24)
	if !strings.Contains(out, "2/2") {
		t.Error("package summary missing '2/2' count")
	}
}

func TestStreamer_MultiplePackages_Interleaved(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/internal/a"},
		{Action: "start", Package: "example.com/internal/b"},
		{Action: "run", Package: "example.com/internal/a", Test: "TestA1"},
		{Action: "run", Package: "example.com/internal/b", Test: "TestB1"},
		{Action: "pass", Package: "example.com/internal/a", Test: "TestA1", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/internal/b", Test: "TestB1", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/internal/a", Elapsed: 0.5},
		{Action: "pass", Package: "example.com/internal/b", Elapsed: 0.5},
	}
	out := runStream(t, events, 80, 24)
	// Both packages should appear
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Errorf("output missing package prefixes: %s", out)
	}
}

func TestStreamer_Run_WithContext(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestFoo"}`,
		`{"Action":"pass","Package":"example.com/pkg","Test":"TestFoo","Elapsed":0.01}`,
		`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := Run(context.Background(), strings.NewReader(input), &buf, 80, 24, nil)
	if exitCode != 0 {
		t.Errorf("Run() exit code = %d, want 0", exitCode)
	}
	out := buf.String()
	if !strings.Contains(out, "TestFoo") {
		t.Errorf("output missing test name: %s", out)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./pkg/stream/ -run TestStreamer -v`
Expected: FAIL — `newStreamer`, `Run` undefined

**Step 3: Write implementation**

Create `pkg/stream/stream.go`:

```go
package stream

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/testjson"
)

// LineKind identifies the type of output line for styling.
type LineKind int

const (
	KindPass    LineKind = iota
	KindFail
	KindSkip
	KindPkgPass
	KindPkgFail
	KindOutput
	KindSeparator
)

// StyleFunc formats a line with colors/symbols.
// Provided by pkg/render's theme at init time. If nil, no styling is applied.
type StyleFunc func(kind LineKind, text string) string

// pkgProgress tracks state for one active package.
type pkgProgress struct {
	name       string // full package path
	short      string // short display name (last path segment)
	startTime  time.Time
	finished   int // tests that have pass/fail/skip
	passed     int
	failed     int
	skipped    int
	currentTest string // most recently run test
}

// streamer is the core state machine for streaming output.
type streamer struct {
	tw        *termWriter
	style     StyleFunc
	active    map[string]*pkgProgress // keyed by full package path
	order     []string                // package order for footer
	outputBuf map[string][]string     // keyed by "pkg\x00test"
	startTime time.Time
}

func newStreamer(tw *termWriter) *streamer {
	return &streamer{
		tw:        tw,
		active:    make(map[string]*pkgProgress),
		outputBuf: make(map[string][]string),
	}
}

// Run is the entry point for streaming mode.
// Reads go test -json events from r, renders to out.
// Returns the exit code (0=pass, 1=fail, 2=error).
func Run(ctx context.Context, r io.Reader, out io.Writer, width, height int, style StyleFunc) int {
	tw := newTermWriter(out, width, height)
	s := newStreamer(tw)
	s.style = style

	err := testjson.Stream(ctx, r, func(e testjson.TestEvent) {
		s.handleEvent(e)
	})

	s.finish()

	if err != nil && err != context.Canceled {
		return 2
	}
	if s.hasFailures() {
		return 1
	}
	return 0
}

func (s *streamer) handleEvent(e testjson.TestEvent) {
	switch e.Action {
	case "start":
		s.onStart(e)
	case "run":
		s.onRun(e)
	case "pass":
		if e.Test != "" {
			s.onTestPass(e)
		} else {
			s.onPkgDone(e, true)
		}
	case "fail":
		if e.Test != "" {
			s.onTestFail(e)
		} else {
			s.onPkgDone(e, false)
		}
	case "skip":
		s.onTestSkip(e)
	case "output":
		s.onOutput(e)
	}
	// pause, cont, bench: ignored
}

func (s *streamer) onStart(e testjson.TestEvent) {
	if s.startTime.IsZero() {
		s.startTime = e.Time
	}
	pkg := &pkgProgress{
		name:      e.Package,
		short:     shortPkg(e.Package),
		startTime: e.Time,
	}
	s.active[e.Package] = pkg
	s.order = append(s.order, e.Package)
}

func (s *streamer) onRun(e testjson.TestEvent) {
	if pkg, ok := s.active[e.Package]; ok {
		pkg.currentTest = e.Test
	}
}

func (s *streamer) onTestPass(e testjson.TestEvent) {
	pkg := s.active[e.Package]
	if pkg == nil {
		return
	}
	pkg.finished++
	pkg.passed++

	s.tw.EraseFooter()
	line := s.formatTestLine(pkg.short, "·", e.Test, e.Elapsed)
	if s.style != nil {
		line = s.style(KindPass, line)
	}
	s.tw.PrintLine(line)
	s.discardOutput(e.Package, e.Test)
	s.tw.DrawFooter(s.footerLines())
}

func (s *streamer) onTestFail(e testjson.TestEvent) {
	pkg := s.active[e.Package]
	if pkg == nil {
		return
	}
	pkg.finished++
	pkg.failed++

	s.tw.EraseFooter()
	line := s.formatTestLine(pkg.short, "✗", e.Test, e.Elapsed)
	if s.style != nil {
		line = s.style(KindFail, line)
	}
	s.tw.PrintLine(line)
	s.flushOutput(e.Package, e.Test)
	s.tw.DrawFooter(s.footerLines())
}

func (s *streamer) onTestSkip(e testjson.TestEvent) {
	pkg := s.active[e.Package]
	if pkg == nil {
		return
	}
	pkg.finished++
	pkg.skipped++

	s.tw.EraseFooter()
	line := s.formatTestLine(pkg.short, "○", e.Test, 0)
	if s.style != nil {
		line = s.style(KindSkip, line)
	}
	s.tw.PrintLine(line)
	s.discardOutput(e.Package, e.Test)
	s.tw.DrawFooter(s.footerLines())
}

func (s *streamer) onPkgDone(e testjson.TestEvent, passed bool) {
	pkg := s.active[e.Package]
	if pkg == nil {
		return
	}

	s.tw.EraseFooter()

	total := pkg.passed + pkg.failed + pkg.skipped
	dur := formatElapsed(e.Elapsed)

	var line string
	if passed {
		line = fmt.Sprintf("  ✓ %-20s %d/%d  %s", pkg.short, pkg.passed, total, dur)
		if s.style != nil {
			line = s.style(KindPkgPass, line)
		}
	} else {
		line = fmt.Sprintf("  ✗ %-20s %d/%d  %s", pkg.short, pkg.passed, total, dur)
		if s.style != nil {
			line = s.style(KindPkgFail, line)
		}
	}
	s.tw.PrintLine(line)

	// Flush any package-level panic output
	key := e.Package + "\x00"
	if out, ok := s.outputBuf[key]; ok && !passed {
		for _, line := range out {
			trimmed := strings.TrimRight(line, "\n")
			if trimmed != "" {
				s.tw.PrintLine("    " + trimmed)
			}
		}
	}

	delete(s.active, e.Package)
	s.removeFromOrder(e.Package)
	s.tw.DrawFooter(s.footerLines())
}

func (s *streamer) onOutput(e testjson.TestEvent) {
	key := e.Package + "\x00" + e.Test
	s.outputBuf[key] = append(s.outputBuf[key], e.Output)

	// Detect panics in package-level output — flush immediately
	if e.Test == "" && (strings.Contains(e.Output, "panic:") || strings.HasPrefix(strings.TrimSpace(e.Output), "goroutine ")) {
		s.tw.EraseFooter()
		trimmed := strings.TrimRight(e.Output, "\n")
		if s.style != nil {
			trimmed = s.style(KindOutput, trimmed)
		}
		s.tw.PrintLine("    " + trimmed)
		s.tw.DrawFooter(s.footerLines())
	}
}

func (s *streamer) flushOutput(pkg, test string) {
	key := pkg + "\x00" + test
	if out, ok := s.outputBuf[key]; ok {
		for _, line := range out {
			trimmed := strings.TrimRight(line, "\n")
			// Skip "=== RUN" / "--- FAIL" boilerplate
			if trimmed == "" || strings.HasPrefix(trimmed, "=== RUN") || strings.HasPrefix(trimmed, "--- FAIL") || strings.HasPrefix(trimmed, "--- PASS") {
				continue
			}
			outLine := "    " + trimmed
			if s.style != nil {
				outLine = s.style(KindOutput, outLine)
			}
			s.tw.PrintLine(outLine)
		}
		delete(s.outputBuf, key)
	}
}

func (s *streamer) discardOutput(pkg, test string) {
	delete(s.outputBuf, pkg+"\x00"+test)
}

func (s *streamer) finish() {
	s.tw.EraseFooter()
	// Print final summary line
	totalPassed, totalFailed, totalSkipped, totalPkgs := 0, 0, 0, 0
	var maxDur time.Duration
	// We need to look at completed packages — they've been removed from active.
	// Use the accumulated stats that were tracked during pkg done events.
	// For simplicity, we track totals incrementally in handleEvent.
	// This is computed from all packages seen.
	s.tw.PrintLine("  " + strings.Repeat("─", 45))
}

func (s *streamer) hasFailures() bool {
	// Check if any active packages had failures (shouldn't happen at EOF)
	for _, pkg := range s.active {
		if pkg.failed > 0 {
			return true
		}
	}
	return false
}

func (s *streamer) footerLines() []string {
	if len(s.order) == 0 {
		return nil
	}
	lines := []string{"  " + strings.Repeat("─", 3) + " active " + strings.Repeat("─", 34)}
	for _, name := range s.order {
		pkg := s.active[name]
		if pkg == nil {
			continue
		}
		elapsed := ""
		if !pkg.startTime.IsZero() {
			elapsed = fmt.Sprintf("%.1fs", time.Since(pkg.startTime).Seconds())
		}
		testName := pkg.currentTest
		if len(testName) > 30 {
			testName = testName[:27] + "..."
		}
		line := fmt.Sprintf("  %-10s [%d] %-30s %s", pkg.short, pkg.finished, testName, elapsed)
		lines = append(lines, line)
	}
	return lines
}

func (s *streamer) formatTestLine(pkgShort, symbol, testName string, elapsed float64) string {
	dur := ""
	if elapsed > 0 {
		dur = formatElapsed(elapsed)
	}
	return fmt.Sprintf("  %-10s %s %-35s %s", pkgShort, symbol, testName, dur)
}

func (s *streamer) removeFromOrder(pkg string) {
	for i, name := range s.order {
		if name == pkg {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

func shortPkg(fullPath string) string {
	return path.Base(fullPath)
}

func formatElapsed(elapsed float64) string {
	if elapsed < 0.001 {
		return "0.00s"
	}
	if elapsed < 1 {
		return fmt.Sprintf("%.2fs", elapsed)
	}
	return fmt.Sprintf("%.1fs", elapsed)
}
```

Note: The `finish()` method is intentionally minimal here. Task 5 will wire up the final summary using existing `mapper.FromTestJSON` + `render.Terminal`.

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/stream/ -v`
Expected: PASS (all tests)

**Step 5: Commit**

```
feat: add pkg/stream event loop and state machine
```

---

### Task 4: Wire up `cmd/fo/main.go` — peek detection + stream routing

Replace `io.ReadAll` with `bufio.Reader` peek, route to streaming or batch.

**Files:**
- Modify: `cmd/fo/main.go`

**Step 1: Run existing tests (baseline)**

Run: `go build ./... && go test ./...`
Expected: PASS — everything works before changes

**Step 2: Modify `cmd/fo/main.go`**

Replace the `run()` function body. Key changes:
- Wrap stdin in `bufio.Reader`
- Peek bytes for `detect.Sniff()`
- If go test -json + TTY → call `stream.Run()`
- Otherwise → read remaining bytes, batch path (existing logic unchanged)
- Add context with signal handling

Replace the current `run` function (lines 42-108) with:

```go
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Check for subcommands before flag parsing
	if len(args) > 0 && args[0] == "wrap" {
		return runWrap(args[1:], stdin, stdout, stderr)
	}

	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	formatFlag := fs.String("format", "auto", "Output format: auto, terminal, llm, json")
	themeFlag := fs.String("theme", "default", "Theme: default, orca, mono")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Peek stdin to detect format without consuming
	br := bufio.NewReaderSize(stdin, 8*1024)
	peeked, _ := br.Peek(br.Buffered() + 4096)
	if len(peeked) == 0 {
		// Try to read a bit to fill the buffer
		if _, err := br.Read(make([]byte, 0)); err != nil && err != io.EOF {
			// ignore
		}
		peeked, _ = br.Peek(4096)
	}
	if len(peeked) == 0 {
		fmt.Fprintf(stderr, "fo: no input on stdin\n")
		return 2
	}

	format := detect.Sniff(peeked)

	// Stream mode: go test -json + TTY stdout
	isTTY := false
	if f, ok := stdout.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}

	if format == detect.GoTestJSON && isTTY && *formatFlag == "auto" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		width, height := 80, 24
		if f, ok := stdout.(*os.File); ok {
			if w, h, err := term.GetSize(int(f.Fd())); err == nil {
				if w > 0 {
					width = w
				}
				if h > 0 {
					height = h
				}
			}
		}

		return stream.Run(ctx, br, stdout, width, height, nil)
	}

	// Batch mode: read all remaining input
	input, err := io.ReadAll(br)
	if err != nil {
		fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		return 2
	}
	if len(input) == 0 {
		fmt.Fprintf(stderr, "fo: no input on stdin\n")
		return 2
	}

	// Re-detect on full input (peeked may have been partial)
	if format == detect.Unknown {
		format = detect.Sniff(input)
	}

	// Parse and map to patterns
	var patterns []pattern.Pattern
	switch format {
	case detect.SARIF:
		doc, parseErr := sarif.ReadBytes(input)
		if parseErr != nil {
			fmt.Fprintf(stderr, "fo: parsing SARIF: %v\n", parseErr)
			return 2
		}
		patterns = mapper.FromSARIF(doc)

	case detect.GoTestJSON:
		results, parseErr := testjson.ParseBytes(input)
		if parseErr != nil {
			fmt.Fprintf(stderr, "fo: parsing go test -json: %v\n", parseErr)
			return 2
		}
		patterns = mapper.FromTestJSON(results)

	default:
		fmt.Fprintf(stderr, "fo: unrecognized input format (expected SARIF or go test -json)\n")
		return 2
	}

	// Validate and select renderer
	mode := resolveFormat(*formatFlag, stdout)
	validFormats := map[string]bool{"terminal": true, "llm": true, "json": true}
	if !validFormats[mode] {
		fmt.Fprintf(stderr, "fo: unknown format %q (expected auto, terminal, llm, json)\n", *formatFlag)
		return 2
	}
	renderer := selectRenderer(mode, *themeFlag, stdout)

	// Render and output
	output := renderer.Render(patterns)
	fmt.Fprint(stdout, output)

	return exitCode(patterns)
}
```

Add imports: `"context"`, `"os/signal"`, `"github.com/dkoosis/fo/pkg/stream"`

**Step 3: Verify build + existing batch behavior**

Run: `go build ./cmd/fo/`
Expected: compiles

Run: `cat /tmp/trixi-gotest.json | go run ./cmd/fo/ --format llm`
Expected: same batch output as before (piped = not TTY = batch)

Run: `echo '{"version":"2.1.0","$schema":"","runs":[]}' | go run ./cmd/fo/`
Expected: SARIF path, no crash

**Step 4: Test streaming mode**

Run: `cat /tmp/trixi-gotest.json | go run ./cmd/fo/`
Expected: streaming output with per-test lines and active footer (when run in a TTY)

**Step 5: Commit**

```
feat: auto-detect streaming mode for go test -json on TTY
```

---

### Task 5: Final summary + accumulated stats tracking

The `finish()` method needs to print a proper summary. Track totals incrementally during event processing.

**Files:**
- Modify: `pkg/stream/stream.go` — add stats accumulation, proper `finish()`

**Step 1: Add test for final summary**

Add to `pkg/stream/stream_test.go`:

```go
func TestStreamer_Finish_PrintsSummary(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/internal/a"},
		{Action: "run", Package: "example.com/internal/a", Test: "TestA"},
		{Action: "pass", Package: "example.com/internal/a", Test: "TestA", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/internal/a", Elapsed: 0.5},
		{Action: "start", Package: "example.com/internal/b"},
		{Action: "run", Package: "example.com/internal/b", Test: "TestB"},
		{Action: "pass", Package: "example.com/internal/b", Test: "TestB", Elapsed: 0.02},
		{Action: "pass", Package: "example.com/internal/b", Elapsed: 0.3},
	}
	out := runStream(t, events, 80, 24)
	// Summary should mention total tests and packages
	if !strings.Contains(out, "2 tests") {
		t.Errorf("summary missing test count, got: %s", out)
	}
	if !strings.Contains(out, "2 packages") {
		t.Errorf("summary missing package count, got: %s", out)
	}
}

func TestStreamer_HasFailures_ExitCode1(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestBad"}`,
		`{"Action":"fail","Package":"example.com/pkg","Test":"TestBad","Elapsed":0.02}`,
		`{"Action":"fail","Package":"example.com/pkg","Elapsed":1.0}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	exitCode := Run(context.Background(), strings.NewReader(input), &buf, 80, 24, nil)
	if exitCode != 1 {
		t.Errorf("Run() exit code = %d, want 1 (has failures)", exitCode)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/stream/ -run TestStreamer_Finish -v`
Expected: FAIL — summary content missing

**Step 3: Add stats tracking and finish() implementation**

Add accumulated stats fields to the `streamer` struct and update `finish()` and `hasFailures()`:

```go
// Add to streamer struct:
type streamer struct {
	// ... existing fields ...
	totalPassed  int
	totalFailed  int
	totalSkipped int
	totalPkgs    int
	maxDuration  float64
}
```

Update `onPkgDone` to accumulate stats:

```go
func (s *streamer) onPkgDone(e testjson.TestEvent, passed bool) {
	pkg := s.active[e.Package]
	if pkg == nil {
		return
	}

	s.tw.EraseFooter()

	total := pkg.passed + pkg.failed + pkg.skipped
	dur := formatElapsed(e.Elapsed)

	// Accumulate stats
	s.totalPassed += pkg.passed
	s.totalFailed += pkg.failed
	s.totalSkipped += pkg.skipped
	s.totalPkgs++
	if e.Elapsed > s.maxDuration {
		s.maxDuration = e.Elapsed
	}

	// ... rest unchanged ...
}
```

Update `finish()`:

```go
func (s *streamer) finish() {
	s.tw.EraseFooter()
	s.tw.PrintLine("  " + strings.Repeat("─", 45))

	totalTests := s.totalPassed + s.totalFailed + s.totalSkipped
	dur := formatElapsed(s.maxDuration)

	if s.totalFailed > 0 {
		line := fmt.Sprintf("  FAIL (%s) %d/%d tests, %d packages",
			dur, s.totalFailed, totalTests, s.totalPkgs)
		if s.style != nil {
			line = s.style(KindPkgFail, line)
		}
		s.tw.PrintLine(line)
	} else {
		line := fmt.Sprintf("  PASS (%s) %d tests, %d packages",
			dur, totalTests, s.totalPkgs)
		if s.style != nil {
			line = s.style(KindPkgPass, line)
		}
		s.tw.PrintLine(line)
	}
}

func (s *streamer) hasFailures() bool {
	return s.totalFailed > 0
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./pkg/stream/ -v`
Expected: PASS (all tests)

**Step 5: Commit**

```
feat: add final summary with accumulated stats to streaming output
```

---

### Task 6: End-to-end integration test with trixi data

Copy the trixi test data as a testdata fixture and verify the full pipeline.

**Files:**
- Create: `pkg/stream/testdata/gotest-pass.json` (copy from `/tmp/trixi-gotest.json`)
- Add to: `pkg/stream/stream_test.go`

**Step 1: Copy test fixture**

```bash
mkdir -p pkg/stream/testdata
cp /tmp/trixi-gotest.json pkg/stream/testdata/gotest-pass.json
```

**Step 2: Write integration test**

Add to `pkg/stream/stream_test.go`:

```go
func TestRun_Integration_TrixiData(t *testing.T) {
	f, err := os.Open("testdata/gotest-pass.json")
	if err != nil {
		t.Skipf("testdata not available: %v", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	exitCode := Run(context.Background(), f, &buf, 80, 24, nil)
	if exitCode != 0 {
		t.Errorf("exit code = %d, want 0 (all pass)", exitCode)
	}
	out := buf.String()
	// Should contain test lines from multiple packages
	if !strings.Contains(out, "embedder") {
		t.Error("output missing 'embedder' package")
	}
	if !strings.Contains(out, "index") {
		t.Error("output missing 'index' package")
	}
	// Should contain final summary
	if !strings.Contains(out, "PASS") {
		t.Error("output missing PASS summary")
	}
	if !strings.Contains(out, "9 packages") {
		t.Errorf("output missing '9 packages' in summary: %s", out[len(out)-200:])
	}
}
```

Add `"os"` to test imports.

**Step 3: Run integration test**

Run: `go test ./pkg/stream/ -run TestRun_Integration -v`
Expected: PASS

**Step 4: Run full test suite**

Run: `go test ./...`
Expected: PASS (all packages, batch tests unaffected)

**Step 5: Manual verification**

Run (in terminal): `cat pkg/stream/testdata/gotest-pass.json | go run ./cmd/fo/`
Expected: streaming output with all 467 tests scrolling, active footer updating, final PASS summary

**Step 6: Commit**

```
feat: add integration test with trixi gotest data
```

---

### Task 7: Verify everything, clean up

**Step 1: Full build + test**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: all pass

**Step 2: Verify batch mode is unaffected**

Run: `cat /tmp/trixi-gotest.json | go run ./cmd/fo/ --format llm`
Expected: same batch LLM output as before

Run: `echo '{}' | go run ./cmd/fo/ --format json 2>&1`
Expected: batch JSON mode still works (or appropriate error)

**Step 3: Verify SARIF batch path**

Run: `cat /tmp/trixi-lint.json | go run ./cmd/fo/`
Expected: no crash, processes as SARIF or unknown format gracefully

**Step 4: Commit all and verify clean state**

```
chore: streaming test output — complete implementation
```

---

## Task Dependency Order

```
Task 1 (testjson.Stream API)
  ↓
Task 2 (termWriter)     — independent of Task 1, can run in parallel
  ↓
Task 3 (stream.go event loop) — depends on Task 1 + Task 2
  ↓
Task 4 (main.go routing) — depends on Task 3
  ↓
Task 5 (final summary) — depends on Task 3
  ↓
Task 6 (integration test) — depends on Task 4 + Task 5
  ↓
Task 7 (verification) — depends on all
```

Tasks 1 and 2 can be done in parallel. Tasks 4 and 5 can be done in parallel after Task 3.
