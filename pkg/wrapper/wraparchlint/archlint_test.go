package wraparchlint

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestArchlint_OutputFormat(t *testing.T) {
	w := newArchlint()
	if w.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", w.OutputFormat())
	}
}

func TestArchlint_Clean(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var buf bytes.Buffer
	if err := newArchlint().Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Runs[0].Tool.Driver.Name != "go-arch-lint" {
		t.Errorf("expected tool go-arch-lint, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results for clean output, got %d", len(doc.Runs[0].Results))
	}
}

func TestArchlint_WithViolations(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"search","FileRelativePath":"pkg/search/search.go","ResolvedImportName":"embedder"}
	],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[]}}`
	var buf bytes.Buffer
	if err := newArchlint().Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.RuleID != "dependency-violation" {
		t.Errorf("expected ruleId dependency-violation, got %s", r.RuleID)
	}
	if r.Level != "error" {
		t.Errorf("expected level error, got %s", r.Level)
	}
	if !strings.Contains(r.Message.Text, "search") || !strings.Contains(r.Message.Text, "embedder") {
		t.Errorf("expected message to reference search and embedder, got %q", r.Message.Text)
	}
	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "pkg/search/search.go" {
		t.Errorf("expected location pkg/search/search.go, got %s", loc.ArtifactLocation.URI)
	}
}

func TestArchlint_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	err := newArchlint().Convert(strings.NewReader("bad"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestArchlint_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := newArchlint().Convert(strings.NewReader(""), &buf)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestArchlint_FullImportPath(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"agentSupervisor","FileRelativePath":"/internal/agent/supervisor/supervisor.go","ResolvedImportName":"github.com/example/project/internal/agent/shell"}
	],"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var buf bytes.Buffer
	if err := newArchlint().Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if !strings.Contains(r.Message.Text, "github.com/example/project/internal/agent/shell") {
		t.Errorf("expected full import path in message, got %q", r.Message.Text)
	}
}

func TestParseResult(t *testing.T) {
	input := []byte(`{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"a","FileRelativePath":"a.go","ResolvedImportName":"b"},
		{"ComponentName":"c","FileRelativePath":"c.go","ResolvedImportName":"d"}
	],"Qualities":[{"ID":"q1","Used":true},{"ID":"q2","Used":false}]}}`)
	vs, err := parseResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Errorf("expected 2 violations, got %d", len(vs))
	}
}
