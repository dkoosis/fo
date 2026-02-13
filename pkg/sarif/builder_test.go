package sarif

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestBuilder_BasicOutput(t *testing.T) {
	b := NewBuilder("govet", "1.0")
	b.AddResult("printf", "warning", "wrong arg type", "main.go", 15, 3)

	var buf bytes.Buffer
	if _, err := b.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Parse output as SARIF
	var doc Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if doc.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", doc.Version)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(doc.Runs))
	}
	if doc.Runs[0].Tool.Driver.Name != "govet" {
		t.Errorf("expected tool govet, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.RuleID != "printf" {
		t.Errorf("expected ruleId printf, got %s", r.RuleID)
	}
	if r.Level != "warning" {
		t.Errorf("expected level warning, got %s", r.Level)
	}
	if r.Locations[0].PhysicalLocation.Region.StartLine != 15 {
		t.Errorf("expected line 15, got %d", r.Locations[0].PhysicalLocation.Region.StartLine)
	}
}

func TestBuilder_MultipleResults(t *testing.T) {
	b := NewBuilder("lint", "")
	b.AddResult("r1", "error", "msg1", "a.go", 1, 0)
	b.AddResult("r2", "warning", "msg2", "b.go", 2, 5)

	doc := b.Document()
	if len(doc.Runs[0].Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(doc.Runs[0].Results))
	}
}

func TestBuilder_Chaining(t *testing.T) {
	b := NewBuilder("tool", "1.0").
		AddResult("r1", "error", "m1", "a.go", 1, 0).
		AddResult("r2", "warning", "m2", "b.go", 2, 0)

	doc := b.Document()
	if len(doc.Runs[0].Results) != 2 {
		t.Errorf("expected 2 results from chained calls, got %d", len(doc.Runs[0].Results))
	}
}

func TestIsSARIF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", `{"version":"2.1.0","runs":[]}`, true},
		{"empty", ``, false},
		{"not json", `hello`, false},
		{"json but not sarif", `{"name":"test"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSARIF([]byte(tt.input))
			if got != tt.want {
				t.Errorf("IsSARIF(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"file:///abs/path.go", "/abs/path.go"},
		{"file://relative/path.go", "relative/path.go"},
		{"relative/path.go", "relative/path.go"},
		{"path.go", "path.go"},
	}
	for _, tt := range tests {
		got := NormalizePath(tt.input)
		if got != tt.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
