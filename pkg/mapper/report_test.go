package mapper

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/pattern"
)

func TestFromReport_SARIFSection(t *testing.T) {
	// Minimal valid SARIF — tests mapper logic, not SARIF parser edge cases
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	// Should produce a Summary (report header) + whatever SARIF patterns
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Metrics[0].Label != "vet" {
		t.Errorf("expected tool label 'vet', got %q", sum.Metrics[0].Label)
	}
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("clean SARIF should be success, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MalformedSectionEmitsError(t *testing.T) {
	sections := []report.Section{
		{Tool: "lint", Format: "sarif", Content: []byte("not valid json{{{")},
	}
	patterns := FromReport(sections)
	// Summary should mark the section as error
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("malformed section should be marked error, got %q", sum.Metrics[0].Kind)
	}
	// Should contain an Error pattern (not a TestTable)
	var foundError bool
	for _, p := range patterns {
		if e, ok := p.(*pattern.Error); ok {
			foundError = true
			if e.Source != "lint" {
				t.Errorf("error source = %q, want 'lint'", e.Source)
			}
		}
	}
	if !foundError {
		t.Error("expected Error pattern for malformed section")
	}
}

func TestFromReport_MultiSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "lint", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] is %T, want *pattern.Summary", patterns[0])
	}
	if len(sum.Metrics) != 2 {
		t.Errorf("expected 2 tool metrics, got %d", len(sum.Metrics))
	}
}

func TestFromReport_AllPassLabel(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"tool"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "lint", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] is %T, want *pattern.Summary", patterns[0])
	}
	if !strings.Contains(sum.Label, "all pass") {
		t.Errorf("expected 'all pass' in label, got %q", sum.Label)
	}
}

func TestFromReport_TestJSONMalformedLinesSurfaced(t *testing.T) {
	// Mix of valid go test -json events and malformed lines
	content := strings.Join([]string{
		`{"Action":"start","Package":"example.com/pkg"}`,
		`{"Action":"run","Package":"example.com/pkg","Test":"TestOne"}`,
		`{"Action":"pass","Package":"example.com/pkg","Test":"TestOne","Elapsed":0.01}`,
		`not valid json at all`,
		`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.02}`,
		`another bad line`,
	}, "\n") + "\n"

	sections := []report.Section{
		{Tool: "gotest", Format: "testjson", Content: []byte(content)},
	}
	patterns := FromReport(sections)

	// Summary should still show the section
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] is %T, want *pattern.Summary", patterns[0])
	}
	if !strings.Contains(sum.Metrics[0].Value, "malformed") {
		t.Errorf("expected 'malformed' in scope label, got %q", sum.Metrics[0].Value)
	}

	// Should contain an Error pattern warning about malformed lines
	var foundWarning bool
	for _, p := range patterns {
		if e, ok := p.(*pattern.Error); ok && strings.Contains(e.Message, "malformed") {
			foundWarning = true
			if e.Source != "gotest" {
				t.Errorf("error source = %q, want 'gotest'", e.Source)
			}
			if !strings.Contains(e.Message, "2") {
				t.Errorf("expected malformed count of 2 in message, got %q", e.Message)
			}
		}
	}
	if !foundWarning {
		t.Error("expected Error pattern warning about malformed lines")
	}
}

func TestFromReport_FailLabel(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"tool"}},"results":[{"ruleId":"E001","level":"error","message":{"text":"fail"}}]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`)},
		{Tool: "lint", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] is %T, want *pattern.Summary", patterns[0])
	}
	if !strings.Contains(sum.Label, "1 fail") {
		t.Errorf("expected '1 fail' in label, got %q", sum.Label)
	}
}
