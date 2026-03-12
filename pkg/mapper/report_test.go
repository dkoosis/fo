package mapper

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/pattern"
)

func TestFromReport_TextPassSection(t *testing.T) {
	sections := []report.Section{
		{Tool: "vuln", Format: "text", Status: "pass", Content: []byte("No vulnerabilities.")},
	}
	patterns := FromReport(sections)
	if len(patterns) < 1 {
		t.Fatal("expected at least 1 pattern")
	}
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("expected Summary, got %T", patterns[0])
	}
	if sum.Kind != pattern.SummaryKindReport {
		t.Errorf("expected report kind, got %q", sum.Kind)
	}
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("expected success kind, got %q", sum.Metrics[0].Kind)
	}
}

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
		{Tool: "arch", Format: "text", Status: "pass", Content: []byte("OK")},
	}
	patterns := FromReport(sections)
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
	patterns := FromReport(sections)
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
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Label, "1 fail") {
		t.Errorf("expected '1 fail' in label, got %q", sum.Label)
	}
}

func TestFromReport_MetricsPassSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"eval","status":"pass","metrics":[{"name":"MRR","value":0.983,"threshold":0.95,"direction":"higher_is_better"},{"name":"FPR","value":0.0,"threshold":0.05,"unit":"%","direction":"lower_is_better"}],"summary":"86 queries"}`
	sections := []report.Section{
		{Tool: "eval", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("expected success kind for passing metrics, got %q", sum.Metrics[0].Kind)
	}
	if !strings.Contains(sum.Metrics[0].Value, "MRR=0.983") {
		t.Errorf("expected MRR in label, got %q", sum.Metrics[0].Value)
	}
}

func TestFromReport_MetricsFailSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"eval","status":"fail","metrics":[{"name":"MRR","value":0.800,"threshold":0.95,"direction":"higher_is_better"}],"summary":"regression","details":[{"message":"MRR dropped","severity":"error"}]}`
	sections := []report.Section{
		{Tool: "eval", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("expected error kind for failing metrics, got %q", sum.Metrics[0].Kind)
	}
	var foundTable bool
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			foundTable = true
			if len(tt.Results) != 1 {
				t.Errorf("expected 1 detail item, got %d", len(tt.Results))
			}
			if tt.Results[0].Status != "fail" {
				t.Errorf("severity error should map to fail, got %q", tt.Results[0].Status)
			}
		}
	}
	if !foundTable {
		t.Error("expected TestTable from non-empty details")
	}
}

func TestFromReport_MetricsWarnSection(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"dupl","status":"warn","metrics":[{"name":"clones","value":5}],"details":[{"message":"clone pair","severity":"warn"}]}`
	sections := []report.Section{
		{Tool: "dupl", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindWarning {
		t.Errorf("warn status should map to KindWarning, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsFailNoDetails(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"check","status":"fail","metrics":[{"name":"score","value":0.5}]}`
	sections := []report.Section{
		{Tool: "check", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("fail status should map to KindError, got %q", sum.Metrics[0].Kind)
	}
	var hasFailItem bool
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			for _, r := range tt.Results {
				if r.Status == "fail" {
					hasFailItem = true
				}
			}
		}
	}
	if !hasFailItem {
		t.Error("status:fail with no details must emit a synthetic fail item for exit code 1")
	}
}

func TestFromReport_MetricsEmptyMetrics(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"check","status":"pass","metrics":[]}`
	sections := []report.Section{
		{Tool: "check", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindSuccess {
		t.Errorf("expected success, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsWithUnit(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"perf","status":"pass","metrics":[{"name":"latency","value":42.5,"unit":"ms"}]}`
	sections := []report.Section{
		{Tool: "perf", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Metrics[0].Value, "ms") {
		t.Errorf("expected unit suffix in label, got %q", sum.Metrics[0].Value)
	}
}

func TestFromReport_MetricsInvalidJSON(t *testing.T) {
	sections := []report.Section{
		{Tool: "bad", Format: "metrics", Content: []byte(`not json`)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if sum.Metrics[0].Kind != pattern.KindError {
		t.Errorf("invalid JSON should produce error kind, got %q", sum.Metrics[0].Kind)
	}
}

func TestFromReport_MetricsDetailCategories(t *testing.T) {
	content := `{"schema":"fo-metrics/v1","tool":"arch","status":"fail","metrics":[],"details":[
		{"message":"a->b","category":"deps","severity":"error"},
		{"message":"c->d","category":"deps","severity":"error"},
		{"message":"unused file","category":"hygiene"}
	]}`
	sections := []report.Section{
		{Tool: "arch", Format: "metrics", Content: []byte(content)},
	}
	patterns := FromReport(sections)
	var table *pattern.TestTable
	for _, p := range patterns[1:] {
		if tt, ok := p.(*pattern.TestTable); ok {
			table = tt
		}
	}
	if table == nil {
		t.Fatal("expected TestTable from details")
	}
	if len(table.Results) != 3 {
		t.Errorf("expected 3 detail items, got %d", len(table.Results))
	}
}
