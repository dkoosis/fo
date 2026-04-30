package testjson

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dkoosis/fo/internal/lineread"
)

// ParseStream parses go test -json NDJSON from a reader, line by line.
// Returns the parsed results, the number of malformed lines skipped, and any error.
//
// Oversize lines (>maxLineLen) are counted as malformed and skipped instead
// of aborting the parse — fo-gn0.
func ParseStream(r io.Reader) ([]TestPackageResult, int, error) {
	agg := newAggregator()
	br := bufio.NewReaderSize(r, 64*1024)

	var malformed int
	for {
		line, oversize, err := lineread.Read(br)
		if oversize {
			malformed++
		} else if len(line) > 0 {
			malformed += processEventLine(line, agg.processEvent)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return agg.results(), malformed, nil
			}
			return agg.results(), malformed, fmt.Errorf("reading test output: %w", err)
		}
	}
}

// processEventLine parses one JSON event line and dispatches to fn. Returns
// 1 when the line is non-empty but invalid JSON, 0 otherwise.
func processEventLine(line []byte, fn func(TestEvent)) int {
	if len(line) == 0 {
		return 0
	}
	var event TestEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return 1
	}
	fn(event)
	return 0
}

// ParseBytes is a convenience for parsing from a byte slice.
func ParseBytes(data []byte) ([]TestPackageResult, int, error) {
	return ParseStream(bytes.NewReader(data))
}

// scanResult carries a scanned line or terminal error from the scanner
// goroutine. oversize is set when the line exceeded maxLineLen and was
// discarded; consumers count it as malformed but do not abort.
type scanResult struct {
	line     []byte
	oversize bool
	err      error
}

// Stream parses go test -json events line by line and calls fn for each one.
// Stops on EOF or when ctx is cancelled. Returns the number of malformed lines
// skipped and any error.
//
// Cancellation: the scanner runs in a background goroutine. On context cancel,
// Stream calls r.Close() to unblock the scanner. r MUST be an io.ReadCloser
// whose Close actually interrupts in-flight Read calls — *bufio.Reader wrapped
// in io.NopCloser will leak the scanner goroutine on cancel (fo-u2w).
func Stream(ctx context.Context, r io.ReadCloser, fn func(TestEvent)) (int, error) {
	lines := scanAsync(ctx, r)
	return drainLines(ctx, r, lines, fn)
}

// scanAsync runs the line reader in a background goroutine and sends each
// line (or terminal error) on the returned channel. The channel is closed
// on EOF. Oversize lines are signaled via scanResult.oversize so the
// consumer can count them as malformed without aborting the stream
// (fo-gn0).
func scanAsync(ctx context.Context, r io.ReadCloser) <-chan scanResult {
	br := bufio.NewReaderSize(r, 64*1024)
	lines := make(chan scanResult)
	go scanLoop(ctx, br, lines)
	return lines
}

func scanLoop(ctx context.Context, br *bufio.Reader, lines chan<- scanResult) {
	defer close(lines)
	for {
		line, oversize, err := lineread.Read(br)
		if (oversize || len(line) > 0) && !sendResult(ctx, lines, scanResult{line: copyBytes(line), oversize: oversize}) {
			return
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			_ = sendResult(ctx, lines, scanResult{err: err})
		}
		return
	}
}

func sendResult(ctx context.Context, lines chan<- scanResult, res scanResult) bool {
	select {
	case lines <- res:
		return true
	case <-ctx.Done():
		return false
	}
}

func copyBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return append([]byte(nil), b...)
}

// drainLines reads from lines, dispatching parsed events to fn. Returns when
// the channel closes, the context is cancelled, or a scan error occurs.
func drainLines(ctx context.Context, r io.ReadCloser, lines <-chan scanResult, fn func(TestEvent)) (int, error) {
	var malformed int
	for {
		select {
		case <-ctx.Done():
			_ = r.Close()
			return malformed, ctx.Err()
		case res, ok := <-lines:
			if !ok {
				return malformed, nil
			}
			if res.err != nil {
				return malformed, res.err
			}
			if res.oversize {
				malformed++
				continue
			}
			malformed += processLine(res.line, fn)
		}
	}
}

// processLine parses one raw line and dispatches to fn.
// Returns 1 if the line is malformed (non-empty but invalid JSON), 0 otherwise.
func processLine(line []byte, fn func(TestEvent)) int {
	if len(line) == 0 {
		return 0
	}
	var event TestEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return 1
	}
	fn(event)
	return 0
}

// Aggregator accumulates TestEvents into per-package results. Exposed so
// streaming consumers can build incremental snapshots between events.
// Not safe for concurrent use; callers serialize event delivery.
type Aggregator = aggregator

// NewAggregator returns an empty Aggregator ready to consume events.
func NewAggregator() *Aggregator { return newAggregator() }

// ProcessEvent feeds one event into the aggregator. Mirrors the behavior
// used internally by ParseStream.
func (a *aggregator) ProcessEvent(e TestEvent) { a.processEvent(e) }

// Results returns the current accumulated package results in arrival
// order, skipping packages with no observed activity. Safe to call
// repeatedly between events to produce snapshots.
func (a *aggregator) Results() []TestPackageResult { return a.results() }

type aggregator struct {
	packages map[string]*pkgState
	order    []string
}

type pkgState struct {
	name        string
	passed      int
	failed      int
	skipped     int
	duration    time.Duration
	coverage    float64
	failedOrder []string // failed test names in run order
	buildError  string
	buildOutput []string
	panicked    bool
	panicOutput []string
	// Track output per test (empty test name = package-level output).
	// On failure, output is read directly from here via failedOrder keys.
	outputBuf map[string][]string
}

func newAggregator() *aggregator {
	return &aggregator{
		packages: make(map[string]*pkgState),
	}
}

func (a *aggregator) getOrCreate(name string) *pkgState {
	if pkg, ok := a.packages[name]; ok {
		return pkg
	}
	pkg := &pkgState{
		name:      name,
		outputBuf: make(map[string][]string),
	}
	a.packages[name] = pkg
	a.order = append(a.order, name)
	return pkg
}

func (a *aggregator) processEvent(e TestEvent) {
	if e.Action == "build-output" || e.Action == "build-fail" {
		a.handleBuildEvent(e)
		return
	}
	pkg := a.getOrCreate(e.Package)

	switch e.Action {
	case "pass":
		a.handlePass(pkg, e)
	case "fail":
		a.handleFail(pkg, e)
	case "skip":
		if e.Test != "" {
			pkg.skipped++
			delete(pkg.outputBuf, e.Test)
		}
	case "output":
		a.handleOutput(pkg, e)
	}
}

// handleBuildEvent routes build-output / build-fail to the underlying
// package. ImportPath looks like "pkg" or "pkg [pkg.test]"; we strip the
// bracketed test-binary suffix so build output lands on the same package
// state as later test events.
func (a *aggregator) handleBuildEvent(e TestEvent) {
	name := e.ImportPath
	if i := strings.Index(name, " ["); i >= 0 {
		name = name[:i]
	}
	if name == "" {
		return
	}
	pkg := a.getOrCreate(name)
	switch e.Action {
	case "build-output":
		out := strings.TrimRight(e.Output, "\n")
		if out == "" || strings.HasPrefix(out, "# ") {
			return
		}
		pkg.buildOutput = append(pkg.buildOutput, out)
	case "build-fail":
		if pkg.buildError == "" {
			pkg.buildError = strings.Join(pkg.buildOutput, "\n")
		}
	}
}

func (*aggregator) handlePass(pkg *pkgState, e TestEvent) {
	if e.Test != "" {
		pkg.passed++
		delete(pkg.outputBuf, e.Test)
	} else {
		pkg.duration = time.Duration(e.Elapsed * float64(time.Second))
	}
}

func (*aggregator) handleFail(pkg *pkgState, e TestEvent) {
	if e.Test != "" {
		pkg.failed++
		pkg.failedOrder = append(pkg.failedOrder, e.Test)
		return
	}
	pkg.duration = time.Duration(e.Elapsed * float64(time.Second))
	// Check if this is a build error (failed with no tests run).
	// Prefer compiler output collected from build-output events; fall
	// back to package-level output for older streams that don't carry
	// the build-output action.
	if pkg.passed == 0 && pkg.failed == 0 && pkg.skipped == 0 && pkg.buildError == "" {
		if len(pkg.buildOutput) > 0 {
			pkg.buildError = strings.Join(pkg.buildOutput, "\n")
		} else {
			pkg.buildError = strings.Join(pkg.outputBuf[""], "\n")
		}
	}
}

// isPanicNoise returns true for test-runner noise that shouldn't be in
// the panic block (RUN/PASS/FAIL banners, package-level FAIL summary).
func isPanicNoise(s string) bool {
	t := strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(t, "=== RUN"),
		strings.HasPrefix(t, "=== PAUSE"),
		strings.HasPrefix(t, "=== CONT"),
		strings.HasPrefix(t, "--- PASS"),
		strings.HasPrefix(t, "--- FAIL"),
		strings.HasPrefix(t, "--- SKIP"),
		strings.HasPrefix(t, "PASS"),
		strings.HasPrefix(t, "FAIL\t"),
		strings.HasPrefix(t, "ok  \t"):
		return true
	}
	return false
}

func (*aggregator) handleOutput(pkg *pkgState, e TestEvent) {
	output := strings.TrimRight(e.Output, "\n")
	if output == "" {
		return
	}
	pkg.outputBuf[e.Test] = append(pkg.outputBuf[e.Test], output)

	if !pkg.panicked && (strings.Contains(output, "panic:") || strings.HasPrefix(output, "goroutine ")) {
		pkg.panicked = true
	}
	if pkg.panicked && !isPanicNoise(output) {
		pkg.panicOutput = append(pkg.panicOutput, output)
	}

	if strings.Contains(output, "coverage:") && strings.Contains(output, "% of statements") {
		var cov float64
		_, _ = fmt.Sscanf(output, "coverage: %f%% of statements", &cov)
		if cov > 0 {
			pkg.coverage = cov
		}
	}
}

func (a *aggregator) results() []TestPackageResult {
	results := make([]TestPackageResult, 0, len(a.order))
	for _, name := range a.order {
		pkg := a.packages[name]
		// Skip packages with no test activity
		if pkg.passed == 0 && pkg.failed == 0 && pkg.skipped == 0 && pkg.buildError == "" && !pkg.panicked {
			continue
		}

		r := TestPackageResult{
			Name:        pkg.name,
			Passed:      pkg.passed,
			Failed:      pkg.failed,
			Skipped:     pkg.skipped,
			Duration:    pkg.duration,
			Coverage:    pkg.coverage,
			BuildError:  pkg.buildError,
			Panicked:    pkg.panicked,
			PanicOutput: pkg.panicOutput,
		}

		// Build failed tests list in run order
		for _, testName := range pkg.failedOrder {
			r.FailedTests = append(r.FailedTests, FailedTest{
				Name:   testName,
				Output: pkg.outputBuf[testName],
			})
		}

		results = append(results, r)
	}
	return results
}
