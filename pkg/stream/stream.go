package stream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/testjson"
)

// pkgProgress tracks state for one active package.
type pkgProgress struct {
	name        string // full package path
	short       string // last segment
	startTime   time.Time
	finished    int // tests completed
	passed      int
	failed      int
	skipped     int
	currentTest string // most recently run test name
}

// streamer is the core state machine for streaming go test -json output.
type streamer struct {
	tw *termWriter

	active map[string]*pkgProgress // active packages by full name
	order  []string                // package order for footer rendering

	outputBuf map[string][]string // per-test output buffer, keyed by "pkg\x00test"

	totalPassed  int
	totalFailed  int
	totalSkipped int
	totalPkgs    int
	maxDuration  float64
	hasFailed    bool // any test or package failed
}

func newStreamer(tw *termWriter) *streamer {
	return &streamer{
		tw:        tw,
		active:    make(map[string]*pkgProgress),
		outputBuf: make(map[string][]string),
	}
}

// shortPkg returns the last path segment of a package name.
func shortPkg(pkg string) string {
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		return pkg[i+1:]
	}
	return pkg
}

// bufKey returns the output buffer key for a package/test pair.
func bufKey(pkg, test string) string {
	return pkg + "\x00" + test
}

// handleEvent processes a single test event according to the event matrix.
func (s *streamer) handleEvent(e testjson.TestEvent) {
	switch e.Action {
	case "start":
		s.handleStart(e)
	case "run":
		s.handleRun(e)
	case "pass":
		if e.Test != "" {
			s.handleTestPass(e)
		} else {
			s.handlePkgDone(e, false)
		}
	case "fail":
		if e.Test != "" {
			s.handleTestFail(e)
		} else {
			s.handlePkgDone(e, true)
		}
	case "skip":
		if e.Test != "" {
			s.handleTestSkip(e)
		}
	case "output":
		if !s.handleOutput(e) {
			return
		}
	case "pause", "cont":
		// ignored
		return
	}

	s.redrawFooter()
}

func (s *streamer) handleStart(e testjson.TestEvent) {
	pkg := &pkgProgress{
		name:      e.Package,
		short:     shortPkg(e.Package),
		startTime: e.Time,
	}
	s.active[e.Package] = pkg
	s.order = append(s.order, e.Package)
}

func (s *streamer) handleRun(e testjson.TestEvent) {
	if pkg, ok := s.active[e.Package]; ok {
		pkg.currentTest = e.Test
	}
}

func (s *streamer) handleTestPass(e testjson.TestEvent) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	pkg.passed++
	pkg.finished++
	s.tw.EraseFooter()
	s.tw.PrintLine(fmt.Sprintf("  %-10s · %-40s %5.2fs", pkg.short, e.Test, e.Elapsed))
	delete(s.outputBuf, bufKey(e.Package, e.Test))
}

func (s *streamer) handleTestFail(e testjson.TestEvent) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	pkg.failed++
	pkg.finished++
	s.hasFailed = true
	s.tw.EraseFooter()
	s.tw.PrintLine(fmt.Sprintf("  %-10s ✗ %-40s %5.2fs", pkg.short, e.Test, e.Elapsed))
	s.flushOutputBuf(bufKey(e.Package, e.Test))
}

func (s *streamer) handleTestSkip(e testjson.TestEvent) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	pkg.skipped++
	pkg.finished++
	s.tw.EraseFooter()
	s.tw.PrintLine(fmt.Sprintf("  %-10s ○ %-40s", pkg.short, e.Test))
	delete(s.outputBuf, bufKey(e.Package, e.Test))
}

func (s *streamer) handlePkgDone(e testjson.TestEvent, failed bool) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	if failed {
		s.hasFailed = true
	}
	total := pkg.passed + pkg.failed + pkg.skipped
	sym := "✓"
	if failed {
		sym = "✗"
	}
	s.tw.EraseFooter()
	s.tw.PrintLine(fmt.Sprintf("  %s %-28s %d/%d  %.1fs", sym, pkg.short, pkg.passed, total, e.Elapsed))
	if failed {
		s.flushOutputBuf(bufKey(e.Package, ""))
	}
	s.recordPkg(pkg, e.Elapsed)
	delete(s.active, e.Package)
	delete(s.outputBuf, bufKey(e.Package, ""))
	s.removeOrder(e.Package)
}

// removeOrder removes a package from the render-order slice.
func (s *streamer) removeOrder(pkg string) {
	for i, name := range s.order {
		if name == pkg {
			s.order = append(s.order[:i], s.order[i+1:]...)
			return
		}
	}
}

// flushOutputBuf writes buffered output lines for key, filtering boilerplate.
func (s *streamer) flushOutputBuf(key string) {
	lines, ok := s.outputBuf[key]
	if !ok {
		return
	}
	for _, l := range lines {
		if !isBoilerplate(l) {
			s.tw.PrintLine(fmt.Sprintf("             %s", l))
		}
	}
	delete(s.outputBuf, key)
}

// recordPkg accumulates totals from a finished package.
func (s *streamer) recordPkg(pkg *pkgProgress, elapsed float64) {
	s.totalPassed += pkg.passed
	s.totalFailed += pkg.failed
	s.totalSkipped += pkg.skipped
	s.totalPkgs++
	if elapsed > s.maxDuration {
		s.maxDuration = elapsed
	}
}

// handleOutput buffers test output. Returns true if the footer was disturbed
// and needs redrawing (panic/goroutine lines are flushed immediately).
func (s *streamer) handleOutput(e testjson.TestEvent) bool {
	output := strings.TrimRight(e.Output, "\n")
	if output == "" {
		return false
	}

	key := bufKey(e.Package, e.Test)
	s.outputBuf[key] = append(s.outputBuf[key], output)

	// Package-level output: flush panic/goroutine lines immediately
	if e.Test == "" {
		if strings.Contains(output, "panic:") || strings.HasPrefix(output, "goroutine ") {
			s.tw.EraseFooter()
			s.tw.PrintLine("  " + output)
			return true
		}
	}
	return false
}

// isBoilerplate returns true for go test output lines that should be filtered.
func isBoilerplate(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "=== RUN") ||
		strings.HasPrefix(trimmed, "--- FAIL") ||
		strings.HasPrefix(trimmed, "--- PASS") ||
		strings.HasPrefix(trimmed, "--- SKIP")
}

// redrawFooter rebuilds the active-packages footer.
func (s *streamer) redrawFooter() {
	if len(s.active) == 0 {
		return
	}

	var lines []string
	lines = append(lines, "  ─── active ─────────────────────────────────────")

	now := time.Now()
	for _, name := range s.order {
		pkg, ok := s.active[name]
		if !ok {
			continue
		}
		elapsed := now.Sub(pkg.startTime).Seconds()
		testName := pkg.currentTest
		if len(testName) > 25 {
			testName = testName[:22] + "..."
		}
		line := fmt.Sprintf("  %-7s [%d] %-25s %5.1fs", pkg.short, pkg.finished, testName, elapsed)
		lines = append(lines, line)
	}

	s.tw.DrawFooter(lines)
}

// finish erases the footer and prints the final summary line.
func (s *streamer) finish() {
	s.tw.EraseFooter()

	totalTests := s.totalPassed + s.totalFailed + s.totalSkipped
	s.tw.PrintLine("  ─────────────────────────────────────────────────")

	if s.hasFailed {
		s.tw.PrintLine(fmt.Sprintf("  FAIL (%.1fs) %d/%d tests, %d packages",
			s.maxDuration, s.totalFailed, totalTests, s.totalPkgs))
	} else {
		s.tw.PrintLine(fmt.Sprintf("  PASS (%.1fs) %d tests, %d packages",
			s.maxDuration, totalTests, s.totalPkgs))
	}
}

// Run reads go test -json events from r and renders them to out.
// Returns exit code: 0=all pass, 1=failures, 2=error.
func Run(ctx context.Context, r io.Reader, out io.Writer, width, height int) int {
	tw := newTermWriter(out, width, height)
	s := newStreamer(tw)

	malformed, err := testjson.Stream(ctx, r, func(e testjson.TestEvent) {
		s.handleEvent(e)
	})
	if malformed > 0 {
		fmt.Fprintf(out, "fo: warning: %d malformed line(s) skipped\n", malformed)
	}
	if err != nil {
		s.finish()
		if ctx.Err() != nil {
			return 130
		}
		return 2
	}

	s.finish()
	if s.hasFailed {
		return 1
	}
	return 0
}
