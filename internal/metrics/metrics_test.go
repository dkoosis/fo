package metrics

import "testing"

func TestParse(t *testing.T) {
	input := []byte(`{
		"scope": "86 queries · 51 nugs",
		"columns": ["MRR", "P@5", "P@10", "NDCG5"],
		"rows": [
			{"name": "Overall", "values": [0.983, 0.227, 0.119, 0.961], "n": 86},
			{"name": "entity", "values": [0.938, 0.200, 0.100, 0.954], "n": 8}
		],
		"regressions": []
	}`)
	report, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scope != "86 queries · 51 nugs" {
		t.Errorf("scope = %q", report.Scope)
	}
	if len(report.Rows) != 2 {
		t.Errorf("got %d rows, want 2", len(report.Rows))
	}
	if len(report.Columns) != 4 {
		t.Errorf("got %d columns, want 4", len(report.Columns))
	}
}

func TestParse_WithRegressions(t *testing.T) {
	input := []byte(`{
		"scope": "86 queries",
		"columns": ["MRR"],
		"rows": [{"name": "Overall", "values": [0.900], "n": 86}],
		"regressions": [{"group": "entity", "metric": "MRR", "from": 0.938, "to": 0.875}]
	}`)
	report, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Regressions) != 1 {
		t.Fatalf("got %d regressions, want 1", len(report.Regressions))
	}
	if report.Regressions[0].Metric != "MRR" {
		t.Errorf("regression metric = %q, want MRR", report.Regressions[0].Metric)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
