package stream

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/testjson"
)

// runStream is a test helper that feeds events through a streamer and returns
// the raw output (including ANSI escapes from footer erase/draw cycles).
func runStream(t *testing.T, events []testjson.TestEvent, width, height int) string {
	t.Helper()
	var buf bytes.Buffer
	tw := newTermWriter(&buf, width, height)
	s := newStreamer(tw, nil) // nil style = no styling
	for _, e := range events {
		s.handleEvent(e)
	}
	s.finish()
	return buf.String()
}

// plainOutput strips ANSI escapes from runStream output so tests can check
// visible text without fighting cursor-movement sequences.
func plainOutput(raw string) string {
	return stripANSI(raw)
}

func TestStreamer_PassingTest_PrintsResultLine(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/foo/bar", Time: time.Now()},
		{Action: "run", Package: "example.com/foo/bar", Test: "TestHello"},
		{Action: "pass", Package: "example.com/foo/bar", Test: "TestHello", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/foo/bar", Elapsed: 0.5},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "bar") {
		t.Error("output missing package short name 'bar'")
	}
	if !strings.Contains(out, "TestHello") {
		t.Error("output missing test name 'TestHello'")
	}
	if !strings.Contains(out, "\u00b7") {
		t.Error("output missing pass symbol '\u00b7'")
	}
}

func TestStreamer_FailingTest_FlushesOutput(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/eval", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/eval", Test: "TestBar"},
		{Action: "output", Package: "example.com/pkg/eval", Test: "TestBar", Output: "    expected 1, got 2\n"},
		{Action: "fail", Package: "example.com/pkg/eval", Test: "TestBar", Elapsed: 0.02},
		{Action: "fail", Package: "example.com/pkg/eval", Elapsed: 1.0},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "\u2717") {
		t.Error("output missing fail symbol '\u2717'")
	}
	if !strings.Contains(out, "expected 1, got 2") {
		t.Error("output missing flushed failure output 'expected 1, got 2'")
	}
}

func TestStreamer_PassingTest_DiscardsOutput(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/eval", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/eval", Test: "TestOK"},
		{Action: "output", Package: "example.com/pkg/eval", Test: "TestOK", Output: "some debug noise\n"},
		{Action: "pass", Package: "example.com/pkg/eval", Test: "TestOK", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/pkg/eval", Elapsed: 0.5},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if strings.Contains(out, "some debug noise") {
		t.Error("passing test output should be discarded, but 'some debug noise' appeared")
	}
}

func TestStreamer_PackageSummary_ShowsCounts(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/embedder", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/embedder", Test: "TestA"},
		{Action: "pass", Package: "example.com/pkg/embedder", Test: "TestA", Elapsed: 0.01},
		{Action: "run", Package: "example.com/pkg/embedder", Test: "TestB"},
		{Action: "pass", Package: "example.com/pkg/embedder", Test: "TestB", Elapsed: 0.02},
		{Action: "run", Package: "example.com/pkg/embedder", Test: "TestC"},
		{Action: "pass", Package: "example.com/pkg/embedder", Test: "TestC", Elapsed: 0.03},
		{Action: "pass", Package: "example.com/pkg/embedder", Elapsed: 2.6},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "3/3") {
		t.Errorf("package summary should contain '3/3', got:\n%s", out)
	}
	if !strings.Contains(out, "2.6s") {
		t.Errorf("package summary should contain '2.6s', got:\n%s", out)
	}
}

func TestStreamer_MultiplePackages_Interleaved(t *testing.T) {
	now := time.Now()
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/alpha", Time: now},
		{Action: "start", Package: "example.com/beta", Time: now},
		{Action: "run", Package: "example.com/alpha", Test: "TestA1"},
		{Action: "run", Package: "example.com/beta", Test: "TestB1"},
		{Action: "pass", Package: "example.com/alpha", Test: "TestA1", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/beta", Test: "TestB1", Elapsed: 0.02},
		{Action: "pass", Package: "example.com/alpha", Elapsed: 0.5},
		{Action: "pass", Package: "example.com/beta", Elapsed: 0.6},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "alpha") {
		t.Error("output missing package prefix 'alpha'")
	}
	if !strings.Contains(out, "beta") {
		t.Error("output missing package prefix 'beta'")
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
	code := Run(context.Background(), strings.NewReader(input), &buf, 120, 24, nil)
	if code != 0 {
		t.Errorf("Run() = %d, want 0 for all-pass", code)
	}
}

func TestStreamer_HasFailures_ExitCode1(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestBad"}`,
		`{"Action":"output","Package":"example.com/pkg","Test":"TestBad","Output":"    want true\n"}`,
		`{"Action":"fail","Package":"example.com/pkg","Test":"TestBad","Elapsed":0.05}`,
		`{"Action":"fail","Package":"example.com/pkg","Elapsed":1.0}`,
	}, "\n") + "\n"

	var buf bytes.Buffer
	code := Run(context.Background(), strings.NewReader(input), &buf, 120, 24, nil)
	if code != 1 {
		t.Errorf("Run() = %d, want 1 for failures", code)
	}
}

func TestStreamer_Finish_PrintsSummary(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/a", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/a", Test: "TestX"},
		{Action: "pass", Package: "example.com/pkg/a", Test: "TestX", Elapsed: 0.01},
		{Action: "pass", Package: "example.com/pkg/a", Elapsed: 1.0},
		{Action: "start", Package: "example.com/pkg/b", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/b", Test: "TestY"},
		{Action: "pass", Package: "example.com/pkg/b", Test: "TestY", Elapsed: 0.02},
		{Action: "pass", Package: "example.com/pkg/b", Elapsed: 2.0},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "PASS") {
		t.Error("summary missing 'PASS'")
	}
	if !strings.Contains(out, "2 tests") {
		t.Errorf("summary should contain '2 tests', got:\n%s", out)
	}
	if !strings.Contains(out, "2 packages") {
		t.Errorf("summary should contain '2 packages', got:\n%s", out)
	}
}

func TestStreamer_FailSummary_ShowsFailCount(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg/x", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg/x", Test: "TestGood"},
		{Action: "pass", Package: "example.com/pkg/x", Test: "TestGood", Elapsed: 0.01},
		{Action: "run", Package: "example.com/pkg/x", Test: "TestBad"},
		{Action: "fail", Package: "example.com/pkg/x", Test: "TestBad", Elapsed: 0.02},
		{Action: "fail", Package: "example.com/pkg/x", Elapsed: 1.0},
	}
	out := plainOutput(runStream(t, events, 80, 24))

	if !strings.Contains(out, "FAIL") {
		t.Errorf("summary missing FAIL, got: %s", out)
	}
	if !strings.Contains(out, "1/2") {
		t.Errorf("summary missing fail count '1/2', got: %s", out)
	}
}

func TestStreamer_BoilerplateFiltered(t *testing.T) {
	events := []testjson.TestEvent{
		{Action: "start", Package: "example.com/pkg", Time: time.Now()},
		{Action: "run", Package: "example.com/pkg", Test: "TestFail"},
		{Action: "output", Package: "example.com/pkg", Test: "TestFail", Output: "=== RUN   TestFail\n"},
		{Action: "output", Package: "example.com/pkg", Test: "TestFail", Output: "--- FAIL: TestFail (0.00s)\n"},
		{Action: "output", Package: "example.com/pkg", Test: "TestFail", Output: "    actual error message\n"},
		{Action: "fail", Package: "example.com/pkg", Test: "TestFail", Elapsed: 0.01},
		{Action: "fail", Package: "example.com/pkg", Elapsed: 0.5},
	}
	out := plainOutput(runStream(t, events, 120, 24))

	if !strings.Contains(out, "actual error message") {
		t.Error("real error message should be present")
	}
	// Boilerplate lines should be filtered
	if strings.Contains(out, "=== RUN") {
		t.Error("'=== RUN' boilerplate should be filtered from flushed output")
	}
	if strings.Contains(out, "--- FAIL: TestFail") {
		t.Error("'--- FAIL:' boilerplate should be filtered from flushed output")
	}
}

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
		t.Errorf("output missing '9 packages' in summary")
	}
}
