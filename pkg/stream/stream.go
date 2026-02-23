package stream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/testjson"
)

// LineKind identifies the type of output line for styling.
type LineKind int

const (
	KindPass LineKind = iota
	KindFail
	KindSkip
	KindPkgPass
	KindPkgFail
	KindOutput
	KindSeparator
)

// StyleFunc formats a line with colors/symbols.
// If nil, no styling is applied.
type StyleFunc func(kind LineKind, text string) string

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
	tw    *termWriter
	style StyleFunc

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

func newStreamer(tw *termWriter, style StyleFunc) *streamer {
	return &streamer{
		tw:        tw,
		style:     style,
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

// styleLine applies the style function if set, otherwise returns text unchanged.
func (s *streamer) styleLine(kind LineKind, text string) string {
	if s.style != nil {
		return s.style(kind, text)
	}
	return text
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
			s.handlePkgPass(e)
		}
	case "fail":
		if e.Test != "" {
			s.handleTestFail(e)
		} else {
			s.handlePkgFail(e)
		}
	case "skip":
		if e.Test != "" {
			s.handleTestSkip(e)
		}
	case "output":
		s.handleOutput(e)
	case "pause", "cont":
		// ignored
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
	if pkg, ok := s.active[e.Package]; ok {
		pkg.passed++
		pkg.finished++
	}
	line := fmt.Sprintf("  %-10s \u00b7 %-40s %5.2fs", shortPkg(e.Package), e.Test, e.Elapsed)
	s.tw.EraseFooter()
	s.tw.PrintLine(s.styleLine(KindPass, line))

	// Discard output buffer on pass
	delete(s.outputBuf, bufKey(e.Package, e.Test))
}

func (s *streamer) handleTestFail(e testjson.TestEvent) {
	if pkg, ok := s.active[e.Package]; ok {
		pkg.failed++
		pkg.finished++
	}
	s.hasFailed = true
	line := fmt.Sprintf("  %-10s \u2717 %-40s %5.2fs", shortPkg(e.Package), e.Test, e.Elapsed)
	s.tw.EraseFooter()
	s.tw.PrintLine(s.styleLine(KindFail, line))

	// Flush buffered output
	key := bufKey(e.Package, e.Test)
	if lines, ok := s.outputBuf[key]; ok {
		for _, l := range lines {
			if isBoilerplate(l) {
				continue
			}
			outLine := fmt.Sprintf("             %s", l)
			s.tw.PrintLine(s.styleLine(KindOutput, outLine))
		}
		delete(s.outputBuf, key)
	}
}

func (s *streamer) handleTestSkip(e testjson.TestEvent) {
	if pkg, ok := s.active[e.Package]; ok {
		pkg.skipped++
		pkg.finished++
	}
	line := fmt.Sprintf("  %-10s \u25cb %-40s", shortPkg(e.Package), e.Test)
	s.tw.EraseFooter()
	s.tw.PrintLine(s.styleLine(KindSkip, line))

	// Discard output buffer on skip
	delete(s.outputBuf, bufKey(e.Package, e.Test))
}

func (s *streamer) handlePkgPass(e testjson.TestEvent) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	total := pkg.passed + pkg.failed + pkg.skipped
	line := fmt.Sprintf("  \u2713 %-28s %d/%d  %.1fs", pkg.short, pkg.passed, total, e.Elapsed)
	s.tw.EraseFooter()
	s.tw.PrintLine(s.styleLine(KindPkgPass, line))

	s.totalPassed += pkg.passed
	s.totalFailed += pkg.failed
	s.totalSkipped += pkg.skipped
	s.totalPkgs++
	if e.Elapsed > s.maxDuration {
		s.maxDuration = e.Elapsed
	}

	delete(s.active, e.Package)
}

func (s *streamer) handlePkgFail(e testjson.TestEvent) {
	pkg, ok := s.active[e.Package]
	if !ok {
		return
	}
	s.hasFailed = true
	total := pkg.passed + pkg.failed + pkg.skipped
	line := fmt.Sprintf("  \u2717 %-28s %d/%d  %.1fs", pkg.short, pkg.passed, total, e.Elapsed)
	s.tw.EraseFooter()
	s.tw.PrintLine(s.styleLine(KindPkgFail, line))

	// Flush any remaining package-level output
	key := bufKey(e.Package, "")
	if lines, ok := s.outputBuf[key]; ok {
		for _, l := range lines {
			if isBoilerplate(l) {
				continue
			}
			outLine := fmt.Sprintf("             %s", l)
			s.tw.PrintLine(s.styleLine(KindOutput, outLine))
		}
		delete(s.outputBuf, key)
	}

	s.totalPassed += pkg.passed
	s.totalFailed += pkg.failed
	s.totalSkipped += pkg.skipped
	s.totalPkgs++
	if e.Elapsed > s.maxDuration {
		s.maxDuration = e.Elapsed
	}

	delete(s.active, e.Package)
}

func (s *streamer) handleOutput(e testjson.TestEvent) {
	output := strings.TrimRight(e.Output, "\n")
	if output == "" {
		return
	}

	key := bufKey(e.Package, e.Test)
	s.outputBuf[key] = append(s.outputBuf[key], output)

	// Package-level output: flush panic/goroutine lines immediately
	if e.Test == "" {
		if strings.Contains(output, "panic:") || strings.HasPrefix(output, "goroutine ") {
			s.tw.EraseFooter()
			s.tw.PrintLine(s.styleLine(KindOutput, "  "+output))
		}
	}
}

// isBoilerplate returns true for go test output lines that should be filtered.
func isBoilerplate(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "=== RUN") ||
		strings.HasPrefix(trimmed, "--- FAIL") ||
		strings.HasPrefix(trimmed, "--- PASS")
}

// redrawFooter rebuilds the active-packages footer.
func (s *streamer) redrawFooter() {
	if len(s.active) == 0 {
		return
	}

	var lines []string
	lines = append(lines, "  \u2500\u2500\u2500 active \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500")

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
	sep := "  \u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500\u2500"
	s.tw.PrintLine(s.styleLine(KindSeparator, sep))

	var summary string
	if s.hasFailed {
		summary = fmt.Sprintf("  FAIL (%.1fs) %d/%d tests, %d packages",
			s.maxDuration, s.totalFailed, totalTests, s.totalPkgs)
		s.tw.PrintLine(s.styleLine(KindFail, summary))
	} else {
		summary = fmt.Sprintf("  PASS (%.1fs) %d tests, %d packages",
			s.maxDuration, totalTests, s.totalPkgs)
		s.tw.PrintLine(s.styleLine(KindPass, summary))
	}
}

// Run reads go test -json events from r and renders them to out.
// Returns exit code: 0=all pass, 1=failures, 2=error.
func Run(ctx context.Context, r io.Reader, out io.Writer, width, height int, style StyleFunc) int {
	tw := newTermWriter(out, width, height)
	s := newStreamer(tw, style)

	err := testjson.Stream(ctx, r, func(e testjson.TestEvent) {
		s.handleEvent(e)
	})
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
