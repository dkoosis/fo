package testjson

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParseStream_Behavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		inputLines      []string
		wantMalformed   int
		wantPackageName string
		wantPassed      int
		wantFailed      int
		wantSkipped     int
		wantStatus      Status
		wantCoverage    float64
		wantPanicked    bool
		wantPackages    int
	}{
		{
			name: "pass/fail aggregation and package status",
			inputLines: []string{
				`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestB"}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/pkg","Test":"TestB","Elapsed":0.2}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantPassed:      1,
			wantFailed:      1,
			wantStatus:      StatusFail,
		},
		{
			name: "coverage is parsed from output",
			inputLines: []string{
				`{"Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
				`{"Action":"output","Package":"example.com/pkg","Output":"coverage: 85.3% of statements\n"}`,
				`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantPassed:      1,
			wantCoverage:    85.3,
			wantStatus:      StatusPass,
		},
		{
			name: "panic output marks package as panicked",
			inputLines: []string{
				`{"Action":"run","Package":"example.com/pkg","Test":"TestBad"}`,
				`{"Action":"output","Package":"example.com/pkg","Test":"TestBad","Output":"panic: runtime error: index out of range\n"}`,
				`{"Action":"fail","Package":"example.com/pkg","Test":"TestBad","Elapsed":0.0}`,
				`{"Action":"fail","Package":"example.com/pkg","Elapsed":0.0}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantFailed:      1,
			wantPanicked:    true,
			wantStatus:      StatusFail,
		},
		{
			name: "malformed lines are skipped and counted",
			inputLines: []string{
				`not json`,
				`{bad json`,
				`{"Action":"run","Package":"x","Test":"T"}`,
				`{"Action":"pass","Package":"x","Test":"T","Elapsed":0.1}`,
				`{"Action":"pass","Package":"x","Elapsed":0.1}`,
			},
			wantMalformed:   2,
			wantPackages:    1,
			wantPackageName: "x",
			wantPassed:      1,
			wantStatus:      StatusPass,
		},
		{
			name: "package with no test activity is skipped",
			inputLines: []string{
				`{"Action":"start","Package":"example.com/empty"}`,
			},
			wantMalformed: 0,
			wantPackages:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Join(tt.inputLines, "\n") + "\n"
			results, malformed, err := ParseStream(strings.NewReader(input))
			if err != nil {
				t.Fatalf("ParseStream() error = %v", err)
			}
			if malformed != tt.wantMalformed {
				t.Fatalf("malformed = %d, want %d", malformed, tt.wantMalformed)
			}
			if len(results) != tt.wantPackages {
				t.Fatalf("packages = %d, want %d", len(results), tt.wantPackages)
			}
			if tt.wantPackages == 0 {
				return
			}

			got := results[0]
			if got.Name != tt.wantPackageName {
				t.Fatalf("package name = %q, want %q", got.Name, tt.wantPackageName)
			}
			if got.Passed != tt.wantPassed {
				t.Fatalf("passed = %d, want %d", got.Passed, tt.wantPassed)
			}
			if got.Failed != tt.wantFailed {
				t.Fatalf("failed = %d, want %d", got.Failed, tt.wantFailed)
			}
			if got.Skipped != tt.wantSkipped {
				t.Fatalf("skipped = %d, want %d", got.Skipped, tt.wantSkipped)
			}
			if tt.wantStatus != "" && got.Status() != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status(), tt.wantStatus)
			}
			if tt.wantCoverage > 0 && (got.Coverage < tt.wantCoverage-0.01 || got.Coverage > tt.wantCoverage+0.01) {
				t.Fatalf("coverage = %.2f, want %.2f", got.Coverage, tt.wantCoverage)
			}
			if got.Panicked != tt.wantPanicked {
				t.Fatalf("panicked = %t, want %t", got.Panicked, tt.wantPanicked)
			}
		})
	}
}

func TestProcessEvent_FreesOutputOnPassAndSkip(t *testing.T) {
	t.Parallel()

	// Generate a stream with many passing/skipped tests that produce output,
	// plus one failing test. Verify:
	// - counts are correct
	// - failed test output is preserved in results
	// - outputBuf entries for pass/skip are cleaned up (verified structurally
	//   by confirming correct results — the delete calls are the fix)
	const passingTests = 100
	lines := make([]string, 0, 4*passingTests+7)

	pkg := "example.com/leak"
	for i := range passingTests {
		name := fmt.Sprintf("TestPass%d", i)
		lines = append(lines,
			fmt.Sprintf(`{"Action":"run","Package":"%s","Test":"%s"}`, pkg, name),                                   //nolint:gocritic // sprintfQuotedString: building raw JSON, %s correct for string fields
			fmt.Sprintf(`{"Action":"output","Package":"%s","Test":"%s","Output":"log line %d\n"}`, pkg, name, i),    //nolint:gocritic // sprintfQuotedString: building raw JSON
			fmt.Sprintf(`{"Action":"output","Package":"%s","Test":"%s","Output":"more output %d\n"}`, pkg, name, i), //nolint:gocritic // sprintfQuotedString: building raw JSON
			fmt.Sprintf(`{"Action":"pass","Package":"%s","Test":"%s","Elapsed":0.01}`, pkg, name),                   //nolint:gocritic // sprintfQuotedString: building raw JSON
		)
	}
	// One skipped test with output
	lines = append(lines,
		fmt.Sprintf(`{"Action":"run","Package":"%s","Test":"TestSkipped"}`, pkg),                             //nolint:gocritic // sprintfQuotedString: building raw JSON
		fmt.Sprintf(`{"Action":"output","Package":"%s","Test":"TestSkipped","Output":"skip reason\n"}`, pkg), //nolint:gocritic // sprintfQuotedString: building raw JSON
		fmt.Sprintf(`{"Action":"skip","Package":"%s","Test":"TestSkipped","Elapsed":0.0}`, pkg),              //nolint:gocritic // sprintfQuotedString: building raw JSON
	)
	// One failing test with output (output must survive)
	lines = append(lines,
		fmt.Sprintf(`{"Action":"run","Package":"%s","Test":"TestFail"}`, pkg),                                  //nolint:gocritic // sprintfQuotedString: building raw JSON
		fmt.Sprintf(`{"Action":"output","Package":"%s","Test":"TestFail","Output":"expected X got Y\n"}`, pkg), //nolint:gocritic // sprintfQuotedString: building raw JSON
		fmt.Sprintf(`{"Action":"fail","Package":"%s","Test":"TestFail","Elapsed":0.1}`, pkg),                   //nolint:gocritic // sprintfQuotedString: building raw JSON
	)
	// Package-level fail (package contains a failing test)
	lines = append(lines,
		fmt.Sprintf(`{"Action":"fail","Package":"%s","Elapsed":1.0}`, pkg), //nolint:gocritic // sprintfQuotedString: building raw JSON
	)

	input := strings.Join(lines, "\n") + "\n"
	results, malformed, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseStream() error = %v", err)
	}
	if malformed != 0 {
		t.Fatalf("malformed = %d, want 0", malformed)
	}
	if len(results) != 1 {
		t.Fatalf("packages = %d, want 1", len(results))
	}

	got := results[0]
	if got.Passed != passingTests {
		t.Fatalf("passed = %d, want %d", got.Passed, passingTests)
	}
	if got.Skipped != 1 {
		t.Fatalf("skipped = %d, want 1", got.Skipped)
	}
	if got.Failed != 1 {
		t.Fatalf("failed = %d, want 1", got.Failed)
	}
	if len(got.FailedTests) != 1 {
		t.Fatalf("FailedTests = %d, want 1", len(got.FailedTests))
	}
	if got.FailedTests[0].Name != "TestFail" {
		t.Fatalf("FailedTests[0].Name = %q, want TestFail", got.FailedTests[0].Name)
	}
	if len(got.FailedTests[0].Output) == 0 {
		t.Fatal("FailedTests[0].Output is empty — failed test output was lost")
	}

	// Verify the aggregator freed pass/skip buffers by checking internal state.
	// We re-parse and inspect the aggregator directly.
	agg := newAggregator()
	for line := range strings.SplitSeq(strings.TrimSpace(input), "\n") {
		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		agg.processEvent(event)
	}
	pkgState := agg.packages[pkg]
	// Only package-level ("") output should remain — pass, skip, and now
	// fail (fo-n25.7) all evict their per-test entry from outputBuf on the
	// terminal event. The failed test's output survives via failedOutput,
	// asserted above through FailedTests[0].Output.
	for testName := range pkgState.outputBuf {
		if testName != "" {
			t.Errorf("outputBuf still contains %q — should have been freed on its terminal event", testName)
		}
	}
	for testName := range pkgState.outputBufBytes {
		if testName != "" {
			t.Errorf("outputBufBytes still contains %q — should have been freed on its terminal event", testName)
		}
	}
}

// TestFail_EvictsOutputBufPerTest is a regression for fo-n25.7: a stream
// with a large, attacker-influenced number of distinct FAILING test names
// used to leave one permanent entry per name in outputBuf/outputBufBytes
// (handleFail was the only terminal action that didn't evict), so
// distinct-name cardinality drove unbounded map growth. Verifies that after
// many sequential fail-terminated tests, outputBuf/outputBufBytes hold no
// per-test entries — matching the eviction pass/skip already did — while
// each failed test's output and count are still correctly reported.
func TestFail_EvictsOutputBufPerTest(t *testing.T) {
	t.Parallel()

	const numFailingTests = 5000
	pkg := "example.com/manyfail"

	var b strings.Builder
	for i := range numFailingTests {
		name := fmt.Sprintf("TestFail%d", i)
		fmt.Fprintf(&b, `{"Action":"run","Package":%q,"Test":%q}`+"\n", pkg, name)
		fmt.Fprintf(&b, `{"Action":"output","Package":%q,"Test":%q,"Output":"boom %d\n"}`+"\n", pkg, name, i)
		fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Test":%q,"Elapsed":0.01}`+"\n", pkg, name)
	}
	fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Elapsed":1.0}`+"\n", pkg)

	results, malformed, err := ParseStream(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ParseStream() error = %v", err)
	}
	if malformed != 0 {
		t.Fatalf("malformed = %d, want 0", malformed)
	}
	if len(results) != 1 {
		t.Fatalf("packages = %d, want 1", len(results))
	}
	got := results[0]
	if got.Failed != numFailingTests {
		t.Fatalf("failed = %d, want %d", got.Failed, numFailingTests)
	}
	if len(got.FailedTests) != numFailingTests {
		t.Fatalf("FailedTests = %d, want %d", len(got.FailedTests), numFailingTests)
	}
	for i, ft := range got.FailedTests {
		if len(ft.Output) == 0 {
			t.Fatalf("FailedTests[%d] (%s) has no output — captured output was lost", i, ft.Name)
		}
	}

	// Inspect the aggregator directly: outputBuf/outputBufBytes must not
	// have accumulated one entry per failed test name — each was evicted
	// at its own fail event (handleFail), same as pass/skip already do.
	agg := newAggregator()
	for line := range strings.SplitSeq(strings.TrimSpace(b.String()), "\n") {
		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		agg.processEvent(event)
	}
	pkgState := agg.packages[pkg]
	if n := len(pkgState.outputBuf); n > 1 { // at most the "" package-level key
		t.Errorf("outputBuf has %d entries after %d distinct failed tests, want ≤1 (unbounded growth)", n, numFailingTests)
	}
	if n := len(pkgState.outputBufBytes); n > 1 {
		t.Errorf("outputBufBytes has %d entries after %d distinct failed tests, want ≤1 (unbounded growth)", n, numFailingTests)
	}
}

// TestStreamMode_LargePerTestOutputBounded verifies that a single failing
// test emitting many MB of output does not balloon outputBuf — the parser
// caps per-test buffering and emits a truncation sentinel.
// Regression for #257 (fo-1f4): aggregator.outputBuf was unbounded.
func TestStreamMode_LargePerTestOutputBounded(t *testing.T) {
	t.Parallel()

	const targetBytes = 50 * 1024 * 1024 // 50 MiB
	const lineSize = 1024                // 1 KiB per output line
	const numLines = targetBytes / lineSize

	pkg := "example.com/big"
	payload := strings.Repeat("x", lineSize-1) // -1 leaves room for \n
	var b strings.Builder
	b.Grow(targetBytes + 1024)
	fmt.Fprintf(&b, `{"Action":"run","Package":%q,"Test":"TestBig"}`+"\n", pkg)
	for range numLines {
		fmt.Fprintf(&b, `{"Action":"output","Package":%q,"Test":"TestBig","Output":%q}`+"\n", pkg, payload+"\n")
	}
	fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Test":"TestBig","Elapsed":0.1}`+"\n", pkg)
	fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Elapsed":1.0}`+"\n", pkg)

	results, _, err := ParseStream(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ParseStream() error = %v", err)
	}
	if len(results) != 1 || len(results[0].FailedTests) != 1 {
		t.Fatalf("expected 1 package with 1 failed test, got %+v", results)
	}

	out := results[0].FailedTests[0].Output
	var total int
	for _, ln := range out {
		total += len(ln) + 1
	}
	if total > 2*maxPerTestOutputBytes {
		t.Fatalf("captured %d bytes, want ≤ %d (cap=%d)", total, 2*maxPerTestOutputBytes, maxPerTestOutputBytes)
	}

	var hasSentinel bool
	for _, ln := range out {
		if strings.Contains(ln, truncationSentinel) {
			hasSentinel = true
			break
		}
	}
	if !hasSentinel {
		t.Fatalf("expected truncation sentinel %q in output, got last line: %q",
			truncationSentinel, out[len(out)-1])
	}
}

// TestPanicOutput_Bounded verifies that panicOutput is also capped.
func TestPanicOutput_Bounded(t *testing.T) {
	t.Parallel()

	const numLines = 60 * 1024 // ~60 MiB at 1KiB/line
	const lineSize = 1024

	pkg := "example.com/panicker"
	payload := strings.Repeat("p", lineSize-1)
	var b strings.Builder
	fmt.Fprintf(&b, `{"Action":"run","Package":%q,"Test":"TestPanic"}`+"\n", pkg)
	fmt.Fprintf(&b, `{"Action":"output","Package":%q,"Test":"TestPanic","Output":"panic: boom\n"}`+"\n", pkg)
	for range numLines {
		fmt.Fprintf(&b, `{"Action":"output","Package":%q,"Test":"TestPanic","Output":%q}`+"\n", pkg, payload+"\n")
	}
	fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Test":"TestPanic","Elapsed":0.1}`+"\n", pkg)
	fmt.Fprintf(&b, `{"Action":"fail","Package":%q,"Elapsed":1.0}`+"\n", pkg)

	results, _, err := ParseStream(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ParseStream() error = %v", err)
	}
	if len(results) != 1 || !results[0].Panicked {
		t.Fatalf("expected panicked package, got %+v", results)
	}
	var total int
	for _, ln := range results[0].PanicOutput {
		total += len(ln) + 1
	}
	if total > 2*maxPerTestOutputBytes {
		t.Fatalf("panicOutput captured %d bytes, want ≤ %d", total, 2*maxPerTestOutputBytes)
	}
}

// TestAggregatorResults_MemoizesStructuralDiff is the fo-d84 review fix
// for the streaming path: detectStructuralDiff used to run inside
// ToReport, and cmd/fo's stream pipeline calls ToReport(agg.Results())
// fresh on every package-finish tick over the WHOLE accumulated result
// set — so an already-processed failure got its (non-trivial: string
// splits, multiple linear scans, a parse pass) diff detection redone on
// every later tick. Detection now happens once, in the aggregator, at the
// test's terminal fail event (handleFail), and every later results() call
// carries the cached pointer through unchanged.
//
// detectStructuralDiff allocates a new *report.StructuralDiff on every
// call (`return &report.StructuralDiff{...}`), so pointer identity across
// two results() snapshots is a direct, non-invasive spy: if detection had
// been redone, the second snapshot would carry a different pointer.
func TestAggregatorResults_MemoizesStructuralDiff(t *testing.T) {
	t.Parallel()

	agg := NewAggregator()
	const pkg = "example.com/pkg"
	events := []TestEvent{
		{Action: ActionRun, Package: pkg, Test: "TestFoo"},
		{Action: ActionOutput, Package: pkg, Test: "TestFoo", Output: "  pkg.MyStruct{\n"},
		{Action: ActionOutput, Package: pkg, Test: "TestFoo", Output: "- \tField: 1,\n"},
		{Action: ActionOutput, Package: pkg, Test: "TestFoo", Output: "+ \tField: 2,\n"},
		{Action: ActionOutput, Package: pkg, Test: "TestFoo", Output: "  }\n"},
		{Action: ActionFail, Package: pkg, Test: "TestFoo", Elapsed: 0.1},
		{Action: ActionFail, Package: pkg, Elapsed: 0.2}, // package-terminal event
	}
	for _, e := range events {
		agg.ProcessEvent(e)
	}

	// Simulate two later streaming ticks: cmd/fo/stream.go's pipeline
	// re-derives a Report from agg.Results() on every package-finish
	// event, including ticks after this failure was already processed.
	first := agg.Results()
	if len(first) != 1 || len(first[0].FailedTests) != 1 {
		t.Fatalf("unexpected results shape: %+v", first)
	}
	sd1 := first[0].FailedTests[0].StructuralDiff
	if sd1 == nil {
		t.Fatal("StructuralDiff = nil, want populated (go-cmp shape in Output)")
	}

	second := agg.Results()
	sd2 := second[0].FailedTests[0].StructuralDiff
	if sd2 == nil {
		t.Fatal("second results() call: StructuralDiff = nil, want populated")
	}
	if sd1 != sd2 {
		t.Errorf("StructuralDiff pointer changed across results() calls (%p vs %p) — detection was redone instead of reused from the aggregator's cache", sd1, sd2)
	}

	// And through ToReport, which is what the streaming pipeline actually
	// renders from each tick.
	r1 := ToReport(first)
	r2 := ToReport(second)
	if len(r1.Tests) != 1 || len(r2.Tests) != 1 {
		t.Fatalf("unexpected report shape: r1=%+v r2=%+v", r1.Tests, r2.Tests)
	}
	if r1.Tests[0].StructuralDiff != r2.Tests[0].StructuralDiff {
		t.Errorf("ToReport's StructuralDiff pointer changed across ticks (%p vs %p) — detection was redone", r1.Tests[0].StructuralDiff, r2.Tests[0].StructuralDiff)
	}
}

func FuzzParseStream(f *testing.F) {
	f.Add(`{"Action":"run","Package":"x","Test":"T"}` + "\n" + `{"Action":"pass","Package":"x","Test":"T","Elapsed":0.1}` + "\n")
	f.Add(`not-json` + "\n" + `{"Action":"output","Package":"x","Output":"coverage: 80.0% of statements\n"}` + "\n")
	f.Add(`{"Action":"output","Package":"x","Output":"panic: boom\n"}` + "\n" + `{"Action":"fail","Package":"x","Elapsed":0.0}` + "\n")

	f.Fuzz(func(t *testing.T, input string) {
		results, malformed, err := ParseStream(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStream should not fail for arbitrary input: %v", err)
		}
		if malformed < 0 {
			t.Fatalf("malformed should never be negative: %d", malformed)
		}
		_ = results
	})
}
