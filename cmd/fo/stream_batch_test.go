package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

// TestRunStreamBatch_LargeInput verifies that --stream allows piped callers
// to process go test -json input larger than the default 256 MiB boundread
// cap without buffering. Closes fo-frl: prior to this fix, non-TTY callers
// using --format=llm/json/human had no path to streaming and would hard-fail
// on multi-GB CI runs.
//
// We don't actually generate a multi-GB stream (too slow for unit tests);
// instead we verify the streaming path completes successfully on a stream
// large enough to exceed any small ad-hoc buffer (~5 MiB of synthetic
// events) and produces the expected per-package counts.
func TestRunStreamBatch_LargeInput(t *testing.T) {
	const numPkgs = 200
	const testsPerPkg = 50

	var buf bytes.Buffer
	for p := range numPkgs {
		pkg := fmt.Sprintf("example.com/synth/pkg%03d", p)
		for tIdx := range testsPerPkg {
			testName := fmt.Sprintf("TestSynth_%03d", tIdx)
			fmt.Fprintf(&buf,
				`{"Time":"2026-04-29T00:00:00Z","Action":"run","Package":%q,"Test":%q}`+"\n",
				pkg, testName)
			// A burst of output to inflate the stream size.
			for range 8 {
				fmt.Fprintf(&buf,
					`{"Time":"2026-04-29T00:00:00Z","Action":"output","Package":%q,"Test":%q,"Output":"some test log line padding xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n"}`+"\n",
					pkg, testName)
			}
			fmt.Fprintf(&buf,
				`{"Time":"2026-04-29T00:00:01Z","Action":"pass","Package":%q,"Test":%q,"Elapsed":0.01}`+"\n",
				pkg, testName)
		}
		fmt.Fprintf(&buf,
			`{"Time":"2026-04-29T00:00:01Z","Action":"pass","Package":%q,"Elapsed":1.0}`+"\n",
			pkg)
	}

	streamSize := buf.Len()
	if streamSize < 5<<20 {
		t.Fatalf("synthetic stream too small (%d bytes); test would not exercise streaming benefit", streamSize)
	}

	var stdout, stderr bytes.Buffer
	rc := exitCodeOnly(t, []string{"--no-state", "--stream", "--format=json"}, &buf, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("exit=%d, stderr=%q", rc, stderr.String())
	}

	var rep struct {
		Tests []struct {
			Package string `json:"package"`
			Outcome string `json:"outcome"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("decode JSON report: %v; stdout=%q", err, truncate(stdout.String(), 400))
	}
	// Clean runs collapse to one package-level entry per package.
	if len(rep.Tests) != numPkgs {
		t.Errorf("got %d package entries, want %d", len(rep.Tests), numPkgs)
	}
	for _, tr := range rep.Tests {
		if tr.Outcome != "pass" {
			t.Errorf("package %s outcome=%q, want pass", tr.Package, tr.Outcome)
			break
		}
	}
	if strings.Contains(stderr.String(), "input exceeds") || strings.Contains(stderr.String(), "too large") {
		t.Errorf("streaming path should not hit boundread cap; stderr=%q", stderr.String())
	}
}

// TestRunStreamBatch_FlagOnNonTestJSONIsIgnored verifies that --stream with
// non-go-test input (e.g. SARIF) falls through to the batch path rather
// than mis-routing.
func TestRunStreamBatch_FlagOnNonTestJSONIsIgnored(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"x"}},"results":[]}]}`
	var stdout, stderr bytes.Buffer
	rc := exitCodeOnly(t, []string{"--no-state", "--stream", "--format=json"}, strings.NewReader(sarif), &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("exit=%d stderr=%q", rc, stderr.String())
	}
	if stdout.Len() == 0 {
		t.Errorf("expected JSON output, got empty")
	}
}

func exitCodeOnly(t *testing.T, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	t.Helper()
	return run(args, stdin, stdout, stderr)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
