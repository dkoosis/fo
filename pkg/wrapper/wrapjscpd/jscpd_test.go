package wrapjscpd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestJscpd_OutputFormat(t *testing.T) {
	w := newJscpd()
	if w.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", w.OutputFormat())
	}
}

func TestJscpd_EmptyDuplicates(t *testing.T) {
	input := `{"duplicates":[],"statistics":{}}`
	var buf bytes.Buffer
	if err := newJscpd().Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", doc.Version)
	}
	if doc.Runs[0].Tool.Driver.Name != "jscpd" {
		t.Errorf("expected tool jscpd, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results for empty duplicates, got %d", len(doc.Runs[0].Results))
	}
}

func TestJscpd_WithClones(t *testing.T) {
	input := `{"duplicates":[{
		"format":"go",
		"lines":22,
		"firstFile":{"name":"a.go","startLoc":{"line":1},"endLoc":{"line":22}},
		"secondFile":{"name":"b.go","startLoc":{"line":10},"endLoc":{"line":31}}
	}],"statistics":{}}`
	var buf bytes.Buffer
	if err := newJscpd().Convert(strings.NewReader(input), &buf); err != nil {
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
	if r.RuleID != "code-clone" {
		t.Errorf("expected ruleId code-clone, got %s", r.RuleID)
	}
	if r.Level != "warning" {
		t.Errorf("expected level warning, got %s", r.Level)
	}
	if !strings.Contains(r.Message.Text, "b.go") {
		t.Errorf("expected message to reference b.go, got %q", r.Message.Text)
	}
	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "a.go" {
		t.Errorf("expected location a.go, got %s", loc.ArtifactLocation.URI)
	}
	if loc.Region.StartLine != 1 {
		t.Errorf("expected start line 1, got %d", loc.Region.StartLine)
	}
}

func TestJscpd_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	err := newJscpd().Convert(strings.NewReader("not json"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJscpd_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := newJscpd().Convert(strings.NewReader(""), &buf)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseClones(t *testing.T) {
	input := []byte(`{"duplicates":[{
		"format":"go","lines":10,
		"firstFile":{"name":"x.go","startLoc":{"line":5},"endLoc":{"line":14}},
		"secondFile":{"name":"y.go","startLoc":{"line":20},"endLoc":{"line":29}}
	}],"statistics":{}}`)
	clones, err := parseClones(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(clones) != 1 {
		t.Fatalf("expected 1 clone, got %d", len(clones))
	}
	c := clones[0]
	if c.FileA != "x.go" || c.FileB != "y.go" || c.Lines != 10 {
		t.Errorf("unexpected clone: %+v", c)
	}
}
