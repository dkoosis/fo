package wrapdiag

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
)

func diagConvert(t *testing.T, args []string, input string) (bytes.Buffer, error) {
	t.Helper()
	opts := parseDiagArgs(t, args)
	var buf bytes.Buffer
	err := Convert(strings.NewReader(input), &buf, opts)
	return buf, err
}

// parseDiagArgs walks --tool/--rule/--level/--version pairs.
func parseDiagArgs(t *testing.T, args []string) DiagOpts {
	t.Helper()
	opts := DiagOpts{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tool":
			i++
			opts.Tool = args[i]
		case "--rule":
			i++
			opts.Rule = args[i]
		case "--level":
			i++
			opts.Level = args[i]
		case "--version":
			i++
			opts.Version = args[i]
		}
	}
	return opts
}

func TestDiag_FileLineColMessage(t *testing.T) {
	input := "main.go:15:3: unreachable code after return\npkg/util.go:42: unused variable x\n"
	buf, err := diagConvert(t, []string{"--tool", "govet"}, input)
	if err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", doc.Version)
	}
	if doc.Runs[0].Tool.Driver.Name != "govet" {
		t.Errorf("expected tool govet, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.Locations[0].PhysicalLocation.Region.StartLine != 15 {
		t.Errorf("expected line 15, got %d", r.Locations[0].PhysicalLocation.Region.StartLine)
	}
}

func TestDiag_FileOnly(t *testing.T) {
	input := "pkg/handler.go\nmain.go\n"
	buf, err := diagConvert(t, []string{"--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"}, input)
	if err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(doc.Runs[0].Results))
	}
	if doc.Runs[0].Results[0].RuleID != "needs-formatting" {
		t.Errorf("expected rule needs-formatting, got %s", doc.Runs[0].Results[0].RuleID)
	}
}

func TestDiag_MissingToolFlag(t *testing.T) {
	var buf bytes.Buffer
	err := Convert(strings.NewReader("x.go:1: msg\n"), &buf, DiagOpts{})
	if err == nil {
		t.Error("expected error for missing --tool flag")
	}
}

func TestDiag_WindowsDriveLetter(t *testing.T) {
	input := "C:\\Users\\dev\\main.go:15:3: unreachable code\n"
	buf, err := diagConvert(t, []string{"--tool", "govet"}, input)
	if err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	uri := doc.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI
	if uri != "C:\\Users\\dev\\main.go" {
		t.Errorf("expected Windows path, got %q", uri)
	}
}

func TestDiag_FixCommand_GolangciLint(t *testing.T) {
	input := "main.go:15:3: unreachable code\n"
	buf, err := diagConvert(t, []string{"--tool", "golangci-lint", "--rule", "SA4006"}, input)
	if err != nil {
		t.Fatal(err)
	}
	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Runs[0].Results[0].FixCommand()
	want := "golangci-lint run --fix --enable-only=SA4006 main.go"
	if got != want {
		t.Errorf("FixCommand = %q, want %q", got, want)
	}
}

func TestDiag_FixCommand_Gofmt(t *testing.T) {
	input := "pkg/handler.go\n"
	buf, err := diagConvert(t, []string{"--tool", "gofmt"}, input)
	if err != nil {
		t.Fatal(err)
	}
	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Runs[0].Results[0].FixCommand()
	want := "gofmt -w pkg/handler.go"
	if got != want {
		t.Errorf("FixCommand = %q, want %q", got, want)
	}
}

func TestDiag_FixCommand_UnknownToolOmitted(t *testing.T) {
	input := "main.go:15:3: some finding\n"
	buf, err := diagConvert(t, []string{"--tool", "govulncheck"}, input)
	if err != nil {
		t.Fatal(err)
	}
	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if got := doc.Runs[0].Results[0].FixCommand(); got != "" {
		t.Errorf("expected empty FixCommand for unknown tool, got %q", got)
	}
	if len(doc.Runs[0].Results[0].Fixes) != 0 {
		t.Errorf("expected no Fixes attached for unknown tool, got %d", len(doc.Runs[0].Results[0].Fixes))
	}
}

func TestFixCommandFor(t *testing.T) {
	tests := []struct {
		tool, rule, file string
		want             string
	}{
		{"golangci-lint", "SA4006", "main.go", "golangci-lint run --fix --enable-only=SA4006 main.go"},
		{"golangci-lint", "finding", "main.go", "golangci-lint run --fix main.go"},
		{"golangci-lint", "", "main.go", "golangci-lint run --fix main.go"},
		{"gofmt", "x", "a.go", "gofmt -w a.go"},
		{"goimports", "x", "a.go", "goimports -w a.go"},
		{"govulncheck", "x", "a.go", ""},
		{"unknown", "x", "a.go", ""},
	}
	for _, tt := range tests {
		got := fixCommandFor(tt.tool, tt.rule, tt.file)
		if got != tt.want {
			t.Errorf("fixCommandFor(%q,%q,%q) = %q, want %q", tt.tool, tt.rule, tt.file, got, tt.want)
		}
	}
}

func TestParseDiagLine(t *testing.T) {
	tests := []struct {
		input             string
		wantFile          string
		wantLine, wantCol int
		wantMsg           string
	}{
		{"main.go:15:3: unreachable code", "main.go", 15, 3, "unreachable code"},
		{"pkg/util.go:42: unused variable x", "pkg/util.go", 42, 0, "unused variable x"},
		{"pkg/handler.go", "pkg/handler.go", 0, 0, "needs formatting"},
		{"C:\\Users\\dev\\main.go:15:3: unreachable code", "C:\\Users\\dev\\main.go", 15, 3, "unreachable code"},
		{"D:\\proj\\util.go:42: unused", "D:\\proj\\util.go", 42, 0, "unused"},
		{"not a diagnostic", "", 0, 0, ""},
		{"src/main.rs", "", 0, 0, ""},           // non-.go file-only path: not matched
		{"some/path/to/file.txt", "", 0, 0, ""}, // non-.go with slash: not matched
	}
	for _, tt := range tests {
		file, ln, col, msg := parseDiagLine(tt.input)
		if file != tt.wantFile || ln != tt.wantLine || col != tt.wantCol || msg != tt.wantMsg {
			t.Errorf("parseDiagLine(%q) = (%q,%d,%d,%q), want (%q,%d,%d,%q)",
				tt.input, file, ln, col, msg, tt.wantFile, tt.wantLine, tt.wantCol, tt.wantMsg)
		}
	}
}
