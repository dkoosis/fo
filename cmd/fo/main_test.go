package main

import (
	"bytes"
	"strings"
	"testing"
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

	// SCOPE line present
	if !strings.Contains(output, "SCOPE:") {
		t.Error("missing SCOPE line")
	}

	// Files grouped
	if !strings.Contains(output, "## internal/handler.go") {
		t.Error("missing file group for internal/handler.go")
	}
	if !strings.Contains(output, "## pkg/api/client.go") {
		t.Error("missing file group for pkg/api/client.go")
	}

	// Diagnostics present with file:line:col format
	if !strings.Contains(output, "ERR ineffassign:12:5") {
		t.Errorf("missing diagnostic; got:\n%s", output)
	}
	if !strings.Contains(output, "WARN govet:23:8") {
		t.Errorf("missing warning diagnostic; got:\n%s", output)
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

	// SCOPE line indicates failure
	if !strings.Contains(output, "SCOPE: FAIL") {
		t.Errorf("expected SCOPE: FAIL, got:\n%s", output)
	}

	// Failed test listed with output
	if !strings.Contains(output, "FAIL TestCreateUser_InvalidEmail") {
		t.Errorf("missing failed test; got:\n%s", output)
	}
	if !strings.Contains(output, "handler_test.go:45") {
		t.Errorf("missing failure output; got:\n%s", output)
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
	if !strings.Contains(output, "SCOPE: PASS") {
		t.Errorf("expected SCOPE: PASS, got:\n%s", output)
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
	code := runWrap([]string{"sarif", "--tool", "govet"}, strings.NewReader(input), &stdout, &stderr)

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
	code := runWrap([]string{"sarif", "--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"},
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
	code := runWrap([]string{"sarif"}, strings.NewReader("x.go:1: msg\n"), &stdout, &stderr)

	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
}

// --- Report format tests ---

func TestRun_ReportFormat(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n" +
		`{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}` + "\n" +
		"--- tool:arch format:text status:pass ---\nAll checks passed.\n"
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "REPORT:") {
		t.Errorf("output should contain REPORT header, got:\n%s", stdout.String())
	}
}

// --- Unit: parseDiagLine ---

func TestParseDiagLine(t *testing.T) {
	tests := []struct {
		input            string
		wantFile         string
		wantLine, wantCol int
		wantMsg          string
	}{
		{"main.go:15:3: unreachable code", "main.go", 15, 3, "unreachable code"},
		{"pkg/util.go:42: unused variable x", "pkg/util.go", 42, 0, "unused variable x"},
		{"pkg/handler.go", "pkg/handler.go", 0, 0, "needs formatting"},
		{`C:\Users\dev\main.go:15:3: unreachable code`, `C:\Users\dev\main.go`, 15, 3, "unreachable code"},
		{`D:\proj\util.go:42: unused`, `D:\proj\util.go`, 42, 0, "unused"},
		{"not a diagnostic", "", 0, 0, ""},
	}
	for _, tt := range tests {
		file, ln, col, msg := parseDiagLine(tt.input)
		if file != tt.wantFile || ln != tt.wantLine || col != tt.wantCol || msg != tt.wantMsg {
			t.Errorf("parseDiagLine(%q) = (%q,%d,%d,%q), want (%q,%d,%d,%q)",
				tt.input, file, ln, col, msg, tt.wantFile, tt.wantLine, tt.wantCol, tt.wantMsg)
		}
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

	// Find diagnostic lines
	var diagLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ERR ") || strings.HasPrefix(trimmed, "WARN ") {
			diagLines = append(diagLines, trimmed)
		}
	}

	if len(diagLines) < 3 {
		t.Fatalf("expected 3 diag lines, got %d: %v", len(diagLines), diagLines)
	}

	// Errors should come before warnings
	if !strings.HasPrefix(diagLines[0], "ERR") {
		t.Errorf("first diag should be ERR, got: %s", diagLines[0])
	}
	if !strings.HasPrefix(diagLines[1], "ERR") {
		t.Errorf("second diag should be ERR, got: %s", diagLines[1])
	}
	if !strings.HasPrefix(diagLines[2], "WARN") {
		t.Errorf("third diag should be WARN, got: %s", diagLines[2])
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
	panicIdx := strings.Index(output, "PANIC")
	passIdx := strings.Index(output, "Passing")

	if panicIdx == -1 {
		t.Fatalf("missing PANIC section; got:\n%s", output)
	}
	if passIdx == -1 {
		t.Fatalf("missing Passing section; got:\n%s", output)
	}
	if panicIdx > passIdx {
		t.Error("PANIC should appear before Passing packages")
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
