package report

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// TestSchemaIsValidJSON verifies the embedded schema parses cleanly.
func TestSchemaIsValidJSON(t *testing.T) {
	t.Parallel()
	var v map[string]any
	if err := json.Unmarshal([]byte(Schema()), &v); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if got := v["title"]; got != "Report" {
		t.Errorf("schema title = %v, want Report", got)
	}
}

// TestSchemaCoversReportFields catches drift: every JSON field on Report,
// Finding, TestResult, DiffSummary, and DiffItem must appear under the
// matching $defs/properties block in the schema.
func TestSchemaCoversReportFields(t *testing.T) {
	t.Parallel()
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(Schema()), &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	checks := []struct {
		typ    reflect.Type
		bucket map[string]json.RawMessage
		name   string
	}{
		{reflect.TypeFor[Report](), doc.Properties, "Report"},
		{reflect.TypeFor[Finding](), doc.Defs["Finding"].Properties, "Finding"},
		{reflect.TypeFor[TestResult](), doc.Defs["TestResult"].Properties, "TestResult"},
		{reflect.TypeFor[DiffSummary](), doc.Defs["DiffSummary"].Properties, "DiffSummary"},
		{reflect.TypeFor[DiffItem](), doc.Defs["DiffItem"].Properties, "DiffItem"},
	}
	for _, c := range checks {
		for i := range c.typ.NumField() {
			tag := c.typ.Field(i).Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			name := strings.SplitN(tag, ",", 2)[0]
			if name == "" {
				continue
			}
			if _, ok := c.bucket[name]; !ok {
				t.Errorf("schema missing %s.%s", c.name, name)
			}
		}
	}
}

// TestSchemaRoundtripSampleReport ensures a realistic Report serializes to
// JSON whose top-level keys are all declared in the schema. Cheap drift
// detector that doesn't pull in a full JSON Schema validator.
func TestSchemaRoundtripSampleReport(t *testing.T) {
	t.Parallel()
	r := Report{
		Tool:        "test",
		GeneratedAt: time.Unix(0, 0).UTC(),
		Findings: []Finding{{
			RuleID: "x", Severity: SeverityError, Message: "m",
		}},
		Tests: []TestResult{{Package: "p", Outcome: OutcomePass}},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(Schema()), &doc); err != nil {
		t.Fatalf("schema: %v", err)
	}
	for k := range got {
		if _, ok := doc.Properties[k]; !ok {
			t.Errorf("Report JSON contains key %q not declared in schema", k)
		}
	}
}
