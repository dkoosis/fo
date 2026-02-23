package mapper

import (
	"testing"

	"github.com/dkoosis/fo/pkg/jtbd"
	"github.com/dkoosis/fo/pkg/pattern"
)

func TestFromJTBD_ProducesPatterns(t *testing.T) {
	report := &jtbd.Report{
		Total: 3, Running: 1, Broken: 1, WIP: 1,
		Jobs: []jtbd.JobResult{
			{
				Job:    jtbd.Job{ID: "KG-P1", Layer: "Plumbing", Statement: "save"},
				Status: "running", Passed: 2, Total: 2,
				Tests: []jtbd.TestOutcome{
					{FuncName: "TestSave", Status: "pass"},
					{FuncName: "TestGet", Status: "pass"},
				},
			},
			{
				Job:    jtbd.Job{ID: "KG-P2", Layer: "Plumbing", Statement: "search"},
				Status: "broken", Passed: 1, Failed: 1, Total: 2,
				Tests: []jtbd.TestOutcome{
					{FuncName: "TestSearch", Status: "pass"},
					{FuncName: "TestPrefix", Status: "fail"},
				},
			},
			{
				Job:    jtbd.Job{ID: "CORE-5", Layer: "Insight", Statement: "patterns"},
				Status: "wip", Total: 0,
			},
		},
	}

	patterns := FromJTBD(report)

	// Summary + 2 layer tables (Plumbing, Insight)
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %d", len(patterns))
	}

	summary, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatal("first pattern should be Summary")
	}
	if len(summary.Metrics) != 3 {
		t.Errorf("expected 3 metrics, got %d", len(summary.Metrics))
	}

	plumbing, ok := patterns[1].(*pattern.TestTable)
	if !ok {
		t.Fatal("second pattern should be TestTable")
	}
	if len(plumbing.Results) != 2 {
		t.Errorf("expected 2 plumbing items, got %d", len(plumbing.Results))
	}
	if plumbing.Results[0].Status != "pass" {
		t.Errorf("KG-P1 should be pass, got %s", plumbing.Results[0].Status)
	}
	if plumbing.Results[1].Status != "fail" {
		t.Errorf("KG-P2 should be fail, got %s", plumbing.Results[1].Status)
	}

	insight, ok := patterns[2].(*pattern.TestTable)
	if !ok {
		t.Fatal("third pattern should be TestTable")
	}
	if insight.Results[0].Status != "wip" {
		t.Errorf("CORE-5 should be wip, got %s", insight.Results[0].Status)
	}
}
