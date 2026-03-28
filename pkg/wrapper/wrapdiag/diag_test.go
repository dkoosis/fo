package wrapdiag

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestDiag_OutputFormat(t *testing.T) {
	d := New()
	if d.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", d.OutputFormat())
	}
}

func TestDiag_FileLineColMessage(t *testing.T) {
	input := "main.go:15:3: unreachable code after return\npkg/util.go:42: unused variable x\n"
	var buf bytes.Buffer
	if err := New().Wrap([]string{"--tool", "govet"}, strings.NewReader(input), &buf); err != nil {
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
	var buf bytes.Buffer
	err := New().Wrap([]string{"--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"}, strings.NewReader(input), &buf)
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
	err := New().Wrap([]string{}, strings.NewReader("x.go:1: msg\n"), &buf)
	if err == nil {
		t.Error("expected error for missing --tool flag")
	}
}

func TestDiag_WindowsDriveLetter(t *testing.T) {
	input := "C:\\Users\\dev\\main.go:15:3: unreachable code\n"
	var buf bytes.Buffer
	if err := New().Wrap([]string{"--tool", "govet"}, strings.NewReader(input), &buf); err != nil {
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
	}
	for _, tt := range tests {
		file, ln, col, msg := parseDiagLine(tt.input)
		if file != tt.wantFile || ln != tt.wantLine || col != tt.wantCol || msg != tt.wantMsg {
			t.Errorf("parseDiagLine(%q) = (%q,%d,%d,%q), want (%q,%d,%d,%q)",
				tt.input, file, ln, col, msg, tt.wantFile, tt.wantLine, tt.wantCol, tt.wantMsg)
		}
	}
}
