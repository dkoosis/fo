package fometrics

import (
	"strings"
	"testing"
)

func TestParse_ValidPass(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "eval",
		"status": "pass",
		"metrics": [
			{"name": "MRR", "value": 0.983, "threshold": 0.950, "direction": "higher_is_better"},
			{"name": "FPR", "value": 0.000, "threshold": 0.050, "unit": "%", "direction": "lower_is_better"}
		],
		"summary": "86 queries, 0 regressions",
		"details": []
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Schema != "fo-metrics/v1" {
		t.Errorf("schema = %q, want fo-metrics/v1", doc.Schema)
	}
	if doc.Tool != "eval" {
		t.Errorf("tool = %q, want eval", doc.Tool)
	}
	if doc.Status != "pass" {
		t.Errorf("status = %q, want pass", doc.Status)
	}
	if len(doc.Metrics) != 2 {
		t.Fatalf("metrics len = %d, want 2", len(doc.Metrics))
	}
	if doc.Metrics[0].Name != "MRR" || doc.Metrics[0].Value != 0.983 {
		t.Errorf("metric[0] = %+v", doc.Metrics[0])
	}
	if doc.Metrics[0].Threshold == nil || *doc.Metrics[0].Threshold != 0.950 {
		t.Error("metric[0] threshold should be 0.950")
	}
	if doc.Metrics[1].Unit != "%" {
		t.Errorf("metric[1] unit = %q, want %%", doc.Metrics[1].Unit)
	}
	if doc.Metrics[1].Direction != "lower_is_better" {
		t.Errorf("metric[1] direction = %q, want lower_is_better", doc.Metrics[1].Direction)
	}
	if doc.Summary != "86 queries, 0 regressions" {
		t.Errorf("summary = %q", doc.Summary)
	}
}

func TestParse_Defaults(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "dupl",
		"status": "pass",
		"metrics": [{"name": "clones", "value": 12}]
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m := doc.Metrics[0]
	if m.Threshold != nil {
		t.Error("threshold should be nil by default")
	}
	if m.Direction != "higher_is_better" {
		t.Errorf("direction should default to higher_is_better, got %q", m.Direction)
	}
	if m.Unit != "" {
		t.Errorf("unit should default to empty, got %q", m.Unit)
	}
	if doc.Summary != "" {
		t.Errorf("summary should default to empty, got %q", doc.Summary)
	}
	if len(doc.Details) != 0 {
		t.Errorf("details should default to empty, got %d", len(doc.Details))
	}
}

func TestParse_WithDetails(t *testing.T) {
	input := []byte(`{
		"schema": "fo-metrics/v1",
		"tool": "dupl",
		"status": "warn",
		"metrics": [{"name": "clones", "value": 3}],
		"details": [
			{"message": "clone: a.go:1-10 <> b.go:1-10", "file": "a.go", "line": 1, "severity": "warn"},
			{"message": "dep violation", "category": "arch"}
		]
	}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Details) != 2 {
		t.Fatalf("details len = %d, want 2", len(doc.Details))
	}
	d := doc.Details[0]
	if d.Message != "clone: a.go:1-10 <> b.go:1-10" {
		t.Errorf("detail[0] message = %q", d.Message)
	}
	if d.File != "a.go" || d.Line != 1 || d.Severity != "warn" {
		t.Errorf("detail[0] = %+v", d)
	}
	if doc.Details[1].Category != "arch" {
		t.Errorf("detail[1] category = %q", doc.Details[1].Category)
	}
}

func TestParse_RejectsV2(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v2", "tool": "x", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for v2 schema")
	}
	if !strings.Contains(err.Error(), "unsupported schema") {
		t.Errorf("error = %q, want 'unsupported schema'", err)
	}
}

func TestParse_RejectsMissingSchema(t *testing.T) {
	input := []byte(`{"tool": "x", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing schema")
	}
}

func TestParse_RejectsInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParse_AcceptsV1Minor(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1.2", "tool": "x", "status": "pass", "metrics": []}`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatalf("v1.2 should be accepted: %v", err)
	}
	if doc.Tool != "x" {
		t.Errorf("tool = %q", doc.Tool)
	}
}

func TestParse_RejectsMissingTool(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1", "status": "pass", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestParse_RejectsInvalidStatus(t *testing.T) {
	input := []byte(`{"schema": "fo-metrics/v1", "tool": "x", "status": "unknown", "metrics": []}`)
	_, err := Parse(input)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}
