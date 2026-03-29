package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/detect"
	"github.com/dkoosis/fo/pkg/pattern"
	_ "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

// --- JTBD E2E Tests ---
// These exercise the full pipeline: stdin → detect → parse → map → render → stdout

func TestJTBD_RenderSARIFLintResults(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"golangci-lint","version":"1.55.0"}},"results":[
		{"ruleId":"ineffassign","level":"error","message":{"text":"assigned but not used: resp"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"internal/handler.go"},"region":{"startLine":12,"startColumn":5}}}]},
		{"ruleId":"errcheck","level":"error","message":{"text":"error return value not checked"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"internal/handler.go"},"region":{"startLine":45,"startColumn":12}}}]},
		{"ruleId":"govet","level":"warning","message":{"text":"printf format %s has arg of wrong type"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"pkg/api/client.go"},"region":{"startLine":23,"startColumn":8}}}]}
	]}]}`

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(sarif), &stdout, &stderr)

	output := stdout.String()

	// Exit code 1: errors present
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	// Version preamble
	if !strings.HasPrefix(output, "fo:llm:v1\n") {
		t.Error("missing fo:llm:v1 preamble")
	}

	// Triage line with error and warning counts
	if !strings.Contains(output, "2 ✗ 1 ⚠") {
		t.Errorf("missing triage counts; got:\n%s", output)
	}

	// Findings use severity symbols and flat file:line:col format (no ## headers)
	if !strings.Contains(output, "✗ internal/handler.go:12:5 ineffassign") {
		t.Errorf("missing error finding; got:\n%s", output)
	}
	if !strings.Contains(output, "⚠ pkg/api/client.go:23:8 govet") {
		t.Errorf("missing warning finding; got:\n%s", output)
	}

	// No ## file headers in standalone SARIF mode
	if strings.Contains(output, "## internal/handler.go") {
		t.Error("standalone SARIF should not have ## file headers")
	}

	// Zero ANSI codes in LLM mode
	if strings.Contains(output, "\033[") {
		t.Error("LLM output contains ANSI escape codes")
	}
}

func TestJTBD_RenderGoTestResults(t *testing.T) {
	testJSON := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"start","Package":"example.com/pkg/handler"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg/handler","Test":"TestCreateUser_Valid"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg/handler","Test":"TestCreateUser_Valid","Elapsed":0.1}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg/handler","Test":"TestCreateUser_InvalidEmail"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg/handler","Test":"TestCreateUser_InvalidEmail","Output":"    handler_test.go:45: expected error \"invalid email\", got nil\n"}`,
		`{"Time":"2024-01-01T00:00:01Z","Action":"fail","Package":"example.com/pkg/handler","Test":"TestCreateUser_InvalidEmail","Elapsed":0.3}`,
		`{"Time":"2024-01-01T00:00:01Z","Action":"fail","Package":"example.com/pkg/handler","Elapsed":1.2}`,
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(testJSON), &stdout, &stderr)

	output := stdout.String()

	// Exit code 1: test failures
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	// Triage line shows failure count
	if !strings.Contains(output, "1 ✗ /") {
		t.Errorf("expected triage line with failure count, got:\n%s", output)
	}

	// Failed test listed with ✗ symbol and package
	if !strings.Contains(output, "✗ pkg/handler TestCreateUser_InvalidEmail") {
		t.Errorf("missing failed test; got:\n%s", output)
	}
	if !strings.Contains(output, "handler_test.go:45") {
		t.Errorf("missing failure output; got:\n%s", output)
	}

	// Passing tests suppressed
	if strings.Contains(output, "TestCreateUser_Valid") {
		t.Errorf("passing test should be suppressed; got:\n%s", output)
	}
}

func TestJTBD_AllTestsPass(t *testing.T) {
	testJSON := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.05}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestB"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestB","Elapsed":0.02}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.1}`,
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(testJSON), &stdout, &stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for all-pass, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "0 ✗ /") {
		t.Errorf("expected zero-failure triage line, got:\n%s", output)
	}
}

func TestJTBD_CleanSARIFExitZero(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(sarif), &stdout, &stderr)

	if code != 0 {
		t.Errorf("expected exit code 0 for clean SARIF, got %d", code)
	}
}

func TestJTBD_WrapSARIFConvertsLineDiagnostics(t *testing.T) {
	input := "main.go:15:3: unreachable code after return\npkg/util.go:42: unused variable x\n"

	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"diag", "--tool", "govet"}, strings.NewReader(input), &stdout, &stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	output := stdout.String()
	// Valid SARIF
	if !strings.Contains(output, `"version": "2.1.0"`) {
		t.Error("output is not valid SARIF")
	}
	if !strings.Contains(output, `"name": "govet"`) {
		t.Error("missing tool name")
	}
	if !strings.Contains(output, `"uri": "main.go"`) {
		t.Error("missing file URI")
	}
	if !strings.Contains(output, `"startLine": 15`) {
		t.Error("missing start line")
	}
	if !strings.Contains(output, `"startColumn": 3`) {
		t.Error("missing start column")
	}
}

func TestJTBD_WrapSARIFFileOnly(t *testing.T) {
	input := "pkg/handler.go\nmain.go\n"

	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"diag", "--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"},
		strings.NewReader(input), &stdout, &stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, `"uri": "pkg/handler.go"`) {
		t.Errorf("missing file-only entry; got:\n%s", output)
	}
}

func TestJTBD_JSONOutputFormat(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"test"}},"results":[{"ruleId":"r1","level":"warning","message":{"text":"msg"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"f.go"},"region":{"startLine":1}}}]}]}]}`

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json"}, strings.NewReader(sarif), &stdout, &stderr)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, `"version": "2.0"`) {
		t.Error("missing JSON version")
	}
	if !strings.Contains(output, `"patterns"`) {
		t.Error("missing patterns array")
	}
}

func TestJTBD_EmptyInputExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2 for empty input, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no input") {
		t.Errorf("expected 'no input' error, got: %s", stderr.String())
	}
}

func TestJTBD_UnrecognizedFormatExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, strings.NewReader("this is not json\n"), &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
}

func TestJTBD_WrapSARIFMissingToolFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"diag"}, strings.NewReader("x.go:1: msg\n"), &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
}

// --- Report format tests ---

func TestRun_ReportFormat(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n" +
		`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}` + "\n" +
		"--- tool:test format:sarif ---\n" +
		`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}` + "\n"
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	// New format: triage line with ✔ for passing tools, no REPORT: header
	if !strings.Contains(out, "0 ✗ 0 ⚠") {
		t.Errorf("expected triage counts, got:\n%s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Errorf("expected ✔ for passing tools, got:\n%s", out)
	}
}

// --- JTBD: Deterministic sort order (LLM spec) ---

func TestJTBD_LLMSortOrderSeverityThenFileThenLine(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"lint"}},"results":[
		{"ruleId":"warn1","level":"warning","message":{"text":"w1"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":10}}}]},
		{"ruleId":"err1","level":"error","message":{"text":"e1"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":5}}}]},
		{"ruleId":"err2","level":"error","message":{"text":"e2"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go"},"region":{"startLine":1}}}]}
	]}]}`

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(sarif), &stdout, &stderr)

	_ = code
	output := stdout.String()
	lines := strings.Split(output, "\n")

	// Find diagnostic lines (new format uses ✗ and ⚠ symbols)
	var diagLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "✗ ") || strings.HasPrefix(trimmed, "⚠ ") {
			diagLines = append(diagLines, trimmed)
		}
	}

	if len(diagLines) < 3 {
		t.Fatalf("expected 3 diag lines, got %d: %v\nfull output:\n%s", len(diagLines), diagLines, output)
	}

	// Errors (✗) should come before warnings (⚠)
	if !strings.HasPrefix(diagLines[0], "✗") {
		t.Errorf("first diag should be ✗, got: %s", diagLines[0])
	}
	if !strings.HasPrefix(diagLines[1], "✗") {
		t.Errorf("second diag should be ✗, got: %s", diagLines[1])
	}
	if !strings.HasPrefix(diagLines[2], "⚠") {
		t.Errorf("third diag should be ⚠, got: %s", diagLines[2])
	}

	// Within errors: a.go before b.go (file asc)
	if !strings.Contains(diagLines[0], "err2") {
		t.Errorf("first error should be in a.go (err2), got: %s", diagLines[0])
	}
}

// --- JTBD: Panics surface first in test output ---

func TestJTBD_PanicsSurfaceFirst(t *testing.T) {
	testJSON := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/a","Test":"TestOK"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/a","Test":"TestOK","Elapsed":0.1}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/a","Elapsed":0.1}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/b","Test":"TestBad","Output":"panic: runtime error: index out of range\n"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/b","Test":"TestBad","Elapsed":0.0}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/b","Elapsed":0.0}`,
	}, "\n") + "\n"

	var stdout, stderr bytes.Buffer
	_ = run([]string{"--format", "llm"}, strings.NewReader(testJSON), &stdout, &stderr)

	output := stdout.String()

	// Panic surfaces as a failure
	if !strings.Contains(output, "PANIC") {
		t.Fatalf("missing PANIC in output; got:\n%s", output)
	}
	if !strings.Contains(output, "panic: runtime error") {
		t.Fatalf("missing panic details; got:\n%s", output)
	}

	// Passing tests/packages suppressed in new format
	if strings.Contains(output, "Passing") {
		t.Errorf("passing packages should be suppressed; got:\n%s", output)
	}
	if strings.Contains(output, "TestOK") {
		t.Errorf("passing test should be suppressed; got:\n%s", output)
	}
}

func TestJTBD_WrapSARIFLongLine(t *testing.T) {
	// A diagnostic message exceeding the default 64KiB scanner limit must not
	// cause an error. This reproduces GitHub issue #220.
	longMsg := strings.Repeat("x", 70_000)
	input := "main.go:1:1: " + longMsg + "\n"

	var stdout, stderr bytes.Buffer
	code := runWrap([]string{"diag", "--tool", "big"}, strings.NewReader(input), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, `"uri": "main.go"`) {
		t.Errorf("missing file URI in output:\n%.200s…", output)
	}
	if !strings.Contains(output, longMsg[:100]) {
		t.Errorf("long message truncated or missing from output")
	}
}

// --- Report format integration tests ---

func TestRun_ReportClean(t *testing.T) {
	input, err := os.ReadFile("testdata/clean.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("clean report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	// Clean report: zero triage counts and all tools passing
	if !strings.Contains(out, "0 ✗ 0 ⚠") {
		t.Errorf("expected zero triage counts, got:\n%s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Errorf("expected ✔ for passing tools, got:\n%s", out)
	}
	// No sections for clean tools
	if strings.Contains(out, "##") {
		t.Errorf("clean report should have no sections, got:\n%s", out)
	}
}

func TestRun_ReportFailing(t *testing.T) {
	input, err := os.ReadFile("testdata/failing.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("failing report exit code = %d, want 1; stderr: %s", code, stderr.String())
	}

	out := stdout.String()

	// SARIF diagnostics from the lint section must surface with file+rule detail
	if !strings.Contains(out, "internal/store/store.go") {
		t.Errorf("expected SARIF file path in output:\n%s", out)
	}
	if !strings.Contains(out, "errcheck") {
		t.Errorf("expected SARIF rule ID in output:\n%s", out)
	}

	// TestJSON failures from the test section must surface
	if !strings.Contains(out, "TestParser") {
		t.Errorf("expected failed test name in output:\n%s", out)
	}
	if !strings.Contains(out, "parser_test.go:20") {
		t.Errorf("expected test failure location in output:\n%s", out)
	}
}

func TestRun_ReportJSON(t *testing.T) {
	input, err := os.ReadFile("testdata/clean.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("JSON report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Errorf("expected JSON output, got:\n%s", stdout.String())
	}
}

func TestRun_ReportBrokenSection(t *testing.T) {
	input, err := os.ReadFile("testdata/broken-section.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 1 {
		t.Errorf("broken section report exit code = %d, want 1; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	// Broken lint section should surface with ✗ symbol
	if !strings.Contains(out, "## lint") {
		t.Errorf("expected ## lint section for broken tool, got:\n%s", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected ✗ for error, got:\n%s", out)
	}
	// Valid passing test section should be suppressed (no failures)
	if strings.Contains(out, "## test") {
		t.Errorf("passing test section should be suppressed, got:\n%s", out)
	}
	// Passing tools still listed in triage line
	if !strings.Contains(out, "test ✔") || !strings.Contains(out, "vet") {
		t.Errorf("expected passing tools in triage line, got:\n%s", out)
	}
}

func TestRun_ReportFullFormats(t *testing.T) {
	input, err := os.ReadFile("testdata/full.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("full report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	// All tools pass: zero counts, all tools in passing group
	if !strings.Contains(out, "0 ✗ 0 ⚠") {
		t.Errorf("expected zero triage counts, got:\n%s", out)
	}
	if !strings.Contains(out, "✔") {
		t.Errorf("expected ✔ for passing tools, got:\n%s", out)
	}
	// All four tool names appear in triage line
	for _, tool := range []string{"vet", "lint", "test", "vuln"} {
		if !strings.Contains(out, tool) {
			t.Errorf("expected tool %q in output:\n%s", tool, out)
		}
	}
	// No sections for all-clean report
	if strings.Contains(out, "##") {
		t.Errorf("all-pass report should have no sections, got:\n%s", out)
	}
}

// --- JTBD: Multi-run SARIF (multiple tools in one document) ---

func TestJTBD_MultiRunSARIF(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[
		{"tool":{"driver":{"name":"golangci-lint"}},"results":[
			{"ruleId":"errcheck","level":"error","message":{"text":"unchecked"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.go"},"region":{"startLine":1}}}]}
		]},
		{"tool":{"driver":{"name":"gosec"}},"results":[
			{"ruleId":"G104","level":"warning","message":{"text":"unhandled error"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"b.go"},"region":{"startLine":5}}}]}
		]}
	]}`

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(sarif), &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	output := stdout.String()
	// Both files should appear
	if !strings.Contains(output, "a.go") {
		t.Error("missing results from first run")
	}
	if !strings.Contains(output, "b.go") {
		t.Error("missing results from second run")
	}
}

func TestParseInput_ReturnsError_When_InputFormatUnknown(t *testing.T) {
	var stderr bytes.Buffer
	patterns, err := parseInput(detect.Unknown, []byte("not-json"), &stderr)
	if err == nil {
		t.Fatal("expected parseInput to fail for unknown format")
	}
	if patterns != nil {
		t.Fatalf("expected nil patterns on error, got %#v", patterns)
	}
	if !strings.Contains(err.Error(), "unrecognized input format") {
		t.Fatalf("expected unrecognized format error, got %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no warnings on unknown format, got %q", stderr.String())
	}
}

func TestParseInput_WritesWarning_When_GoTestJSONContainsMalformedLines(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
		`{"this":"is malformed"`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.01}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.02}`,
	}, "\n")

	var stderr bytes.Buffer
	patterns, err := parseInput(detect.GoTestJSON, []byte(input), &stderr)
	if err != nil {
		t.Fatalf("expected parseInput success, got %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected mapped patterns for valid events")
	}
	if !strings.Contains(stderr.String(), "malformed line(s) skipped") {
		t.Fatalf("expected malformed-line warning, got %q", stderr.String())
	}
}

func TestParseInput_ReturnsWrappedError_When_ReportSectionInvalid(t *testing.T) {
	input := []byte("--- tool:lint format:sarif ---\n{ definitely not json }\n")
	var stderr bytes.Buffer

	patterns, err := parseInput(detect.Report, input, &stderr)
	if err != nil {
		t.Fatalf("expected parseInput to degrade gracefully for bad section, got %v", err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected a mapped error pattern for invalid report section")
	}
	foundError := false
	for _, p := range patterns {
		if _, ok := p.(*pattern.Error); ok {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Fatalf("expected at least one pattern.Error, got %#v", patterns)
	}
}

func TestExitCode_ReturnsOne_When_FailurePatternPresent(t *testing.T) {
	cases := []struct {
		name     string
		patterns []pattern.Pattern
		wantCode int
	}{
		{
			name: "test table has failing row",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Results: []pattern.TestTableItem{
						{Status: pattern.StatusPass},
						{Status: pattern.StatusFail},
					},
				},
			},
			wantCode: 1,
		},
		{
			name: "error pattern present",
			patterns: []pattern.Pattern{
				&pattern.Error{Message: "bad input"},
			},
			wantCode: 1,
		},
		{
			name: "only passing test rows",
			patterns: []pattern.Pattern{
				&pattern.TestTable{
					Results: []pattern.TestTableItem{
						{Status: pattern.StatusPass},
					},
				},
			},
			wantCode: 0,
		},
		{
			name:     "empty pattern list",
			patterns: nil,
			wantCode: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := exitCode(tc.patterns)
			if got != tc.wantCode {
				t.Fatalf("exitCode() = %d, want %d", got, tc.wantCode)
			}
		})
	}
}

func TestRunWrap_ReturnsZero_When_HelpRequested(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "short help flag", args: []string{"-h"}},
		{name: "long help flag", args: []string{"--help"}},
		{name: "help command", args: []string{"help"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runWrap(tc.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runWrap() code = %d, want 0; stderr: %s", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "archlint") ||
				!strings.Contains(stderr.String(), "diag") ||
				!strings.Contains(stderr.String(), "jscpd") {
				t.Fatalf("expected help output to include wrappers, got: %q", stderr.String())
			}
		})
	}
}

func TestRun_ReturnsUsageError_When_FormatFlagInvalid(t *testing.T) {
	sarif := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}`
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "markdown"}, strings.NewReader(sarif), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown format "markdown"`) {
		t.Fatalf("expected unknown format error, got: %q", stderr.String())
	}
}

func TestRun_ReturnsUsageError_When_FlagParsingFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--not-a-real-flag"}, strings.NewReader(`{"version":"2.1.0","runs":[]}`), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Fatalf("expected flag parsing failure, got: %q", stderr.String())
	}
}
