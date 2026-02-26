package mapper

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/report"
)

func TestFromReport_TextPassSection(t *testing.T) {
	sections := []report.Section{
		{Tool: "vuln", Format: "text", Status: "pass", Content: []byte("No vulnerabilities.")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) < 1 {
		t.Fatal("expected at least 1 pattern")
	}
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Metrics[0].Kind != "success" {
		t.Errorf("expected success kind, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_SARIFSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","$schema":"https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json","runs":[{"tool":{"driver":{"name":"govet","rules":[]}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) == 0 {
		t.Fatal("expected patterns")
	}
}

func TestFromReport_MalformedSectionReportsError(t *testing.T) {
	sections := []report.Section{
		{Tool: "lint", Format: "sarif", Content: []byte("not valid json{{{")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal("FromReport should not return top-level error for section failures")
	}
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Metrics[0].Kind != "error" {
		t.Errorf("malformed section should be marked error, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MultiSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","$schema":"...","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "arch", Format: "text", Status: "pass", Content: []byte("OK")},
	}
	patterns, err := FromReport(sections)
	if err != nil {
		t.Fatal(err)
	}
	sum := patterns[0].(*pattern.Summary)
	if len(sum.Metrics) != 2 {
		t.Errorf("expected 2 tool metrics, got %d", len(sum.Metrics))
	}
}

func TestFromReport_AllPassLabel(t *testing.T) {
	sections := []report.Section{
		{Tool: "vet", Format: "text", Status: "pass", Content: []byte("OK")},
		{Tool: "lint", Format: "text", Status: "pass", Content: []byte("OK")},
	}
	patterns, _ := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Label, "all pass") {
		t.Errorf("expected 'all pass' in label, got %q", sum.Label)
	}
}

func TestFromReport_FailLabel(t *testing.T) {
	sections := []report.Section{
		{Tool: "vet", Format: "text", Status: "pass", Content: []byte("OK")},
		{Tool: "lint", Format: "text", Status: "fail", Content: []byte("errors")},
	}
	patterns, _ := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Label, "1 fail") {
		t.Errorf("expected '1 fail' in label, got %q", sum.Label)
	}
}
