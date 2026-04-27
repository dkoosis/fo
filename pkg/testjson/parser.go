package testjson

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ParseStream parses go test -json NDJSON from a reader, line by line.
// Returns the parsed results, the number of malformed lines skipped, and any error.
func ParseStream(r io.Reader) ([]TestPackageResult, int, error) {
	agg := newAggregator()
	scanner := bufio.NewScanner(r)
	// Allow large lines for verbose test output.
	// BUG: a line exceeding 1 MiB (e.g. huge panic trace) triggers
	// bufio.ErrTooLong, which is fatal — all remaining events are lost.
	// To fix: switch to bufio.Reader.ReadBytes('\n') or skip oversized
	// lines as malformed instead of aborting.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var malformed int
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event TestEvent
		if err := json.Unmarshal(line, &event); err != nil {
			malformed++
			continue
		}
		agg.processEvent(event)
	}
	if err := scanner.Err(); err != nil {
		return nil, malformed, fmt.Errorf("scanning test output: %w", err)
	}
	return agg.results(), malformed, nil
}

// ParseBytes is a convenience for parsing from a byte slice.
func ParseBytes(data []byte) ([]TestPackageResult, int, error) {
	return ParseStream(bytes.NewReader(data))
}

// scanResult carries a scanned line or terminal error from the scanner goroutine.
type scanResult struct {
	line []byte
	err  error
}

// Stream parses go test -json events line by line and calls fn for each one.
// Stops on EOF or when ctx is cancelled. Returns the number of malformed lines
// skipped and any error.
//
// Cancellation: the scanner runs in a background goroutine. On context cancel,
// Stream closes r (if it implements io.Closer) to unblock the scanner. If r
// does not implement io.Closer (e.g. *bufio.Reader), the caller must close the
// underlying reader externally to prevent a goroutine leak.
func Stream(ctx context.Context, r io.Reader, fn func(TestEvent)) (int, error) {
	lines := scanAsync(ctx, r)
	return drainLines(ctx, r, lines, fn)
}

// scanAsync runs a scanner in a background goroutine and sends each line (or
// terminal error) on the returned channel. The channel is closed on EOF.
func scanAsync(ctx context.Context, r io.Reader) <-chan scanResult {
	scanner := bufio.NewScanner(r)
	// Same 1 MiB limit as ParseStream — see BUG note there.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make(chan scanResult)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			cp := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- scanResult{line: cp}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case lines <- scanResult{err: err}:
			case <-ctx.Done():
			}
		}
	}()
	return lines
}

// drainLines reads from lines, dispatching parsed events to fn. Returns when
// the channel closes, the context is cancelled, or a scan error occurs.
func drainLines(ctx context.Context, r io.Reader, lines <-chan scanResult, fn func(TestEvent)) (int, error) {
	var malformed int
	for {
		select {
		case <-ctx.Done():
			cancelScan(r)
			return malformed, ctx.Err()
		case res, ok := <-lines:
			if !ok {
				return malformed, nil
			}
			if res.err != nil {
				return malformed, res.err
			}
			malformed += processLine(res.line, fn)
		}
	}
}

// cancelScan attempts to unblock a scanner goroutine by closing r if it
// implements io.Closer.
func cancelScan(r io.Reader) {
	if c, ok := r.(io.Closer); ok {
		_ = c.Close()
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
	} else {
		pkg.duration = time.Duration(e.Elapsed * float64(time.Second))
		// Check if this is a build error (failed with no tests run)
		if pkg.passed == 0 && pkg.failed == 0 && pkg.skipped == 0 {
			pkg.buildError = strings.Join(pkg.outputBuf[""], "\n")
		}
	}
}

func (*aggregator) handleOutput(pkg *pkgState, e TestEvent) {
	output := strings.TrimRight(e.Output, "\n")
	if output == "" {
		return
	}
	pkg.outputBuf[e.Test] = append(pkg.outputBuf[e.Test], output)

	if strings.Contains(output, "panic:") || strings.HasPrefix(output, "goroutine ") {
		pkg.panicked = true
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
