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
	// Allow large lines for verbose test output
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
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lines := make(chan scanResult)
	go func() {
		defer close(lines)
		for scanner.Scan() {
			// Copy bytes — scanner reuses the buffer.
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

	var malformed int
	for {
		select {
		case <-ctx.Done():
			// Attempt to unblock the scanner goroutine.
			if c, ok := r.(io.Closer); ok {
				_ = c.Close()
			}
			return malformed, ctx.Err()
		case res, ok := <-lines:
			if !ok {
				return malformed, nil
			}
			if res.err != nil {
				return malformed, res.err
			}
			if len(res.line) == 0 {
				continue
			}
			var event TestEvent
			if err := json.Unmarshal(res.line, &event); err != nil {
				malformed++
				continue
			}
			fn(event)
		}
	}
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
	failedTests map[string][]string // test name → output lines
	failedOrder []string            // failed test names in run order
	buildError  string
	panicked    bool
	panicOutput []string
	// Track output for tests in progress
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
		name:        name,
		failedTests: make(map[string][]string),
		outputBuf:   make(map[string][]string),
	}
	a.packages[name] = pkg
	a.order = append(a.order, name)
	return pkg
}

func (a *aggregator) processEvent(e TestEvent) {
	pkg := a.getOrCreate(e.Package)

	switch e.Action {
	case "pass":
		if e.Test != "" {
			pkg.passed++
		} else {
			pkg.duration = time.Duration(e.Elapsed * float64(time.Second))
		}

	case "fail":
		if e.Test != "" {
			pkg.failed++
			pkg.failedTests[e.Test] = pkg.outputBuf[e.Test]
			pkg.failedOrder = append(pkg.failedOrder, e.Test)
		} else {
			pkg.duration = time.Duration(e.Elapsed * float64(time.Second))
			// Check if this is a build error (failed with no tests run)
			if pkg.passed == 0 && pkg.failed == 0 && pkg.skipped == 0 {
				pkg.buildError = strings.Join(pkg.outputBuf[""], "\n")
			}
		}

	case "skip":
		if e.Test != "" {
			pkg.skipped++
		}

	case "output":
		output := strings.TrimRight(e.Output, "\n")
		if output == "" {
			return
		}
		// Track output per test (empty test name = package-level output)
		pkg.outputBuf[e.Test] = append(pkg.outputBuf[e.Test], output)

		// Detect panics
		if strings.Contains(output, "panic:") || strings.HasPrefix(output, "goroutine ") {
			pkg.panicked = true
			pkg.panicOutput = append(pkg.panicOutput, output)
		}

		// Parse coverage
		if strings.Contains(output, "coverage:") && strings.Contains(output, "% of statements") {
			var cov float64
			_, _ = fmt.Sscanf(output, "coverage: %f%% of statements", &cov)
			if cov > 0 {
				pkg.coverage = cov
			}
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
			Name:       pkg.name,
			Passed:     pkg.passed,
			Failed:     pkg.failed,
			Skipped:    pkg.skipped,
			Duration:   pkg.duration,
			Coverage:   pkg.coverage,
			BuildError: pkg.buildError,
			Panicked:   pkg.panicked,
		}

		if pkg.panicked {
			r.PanicOutput = pkg.panicOutput
		}

		// Build failed tests list in run order
		for _, testName := range pkg.failedOrder {
			r.FailedTests = append(r.FailedTests, FailedTest{
				Name:   testName,
				Output: pkg.failedTests[testName],
			})
		}

		results = append(results, r)
	}
	return results
}
