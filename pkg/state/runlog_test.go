package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/report"
)

func TestRunLogEntryFromReport_Counts(t *testing.T) {
	r := &report.Report{
		Tool:        "staticcheck",
		GeneratedAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		Findings: []report.Finding{
			{RuleID: "SA1000", Severity: report.SeverityError},
			{RuleID: "SA1000", Severity: report.SeverityWarning},
			{RuleID: "ST1003", Severity: report.SeverityNote},
		},
		Tests: []report.TestResult{
			{Outcome: report.OutcomeFail},
			{Outcome: report.OutcomePass},
			{Outcome: report.OutcomeSkip},
		},
	}
	e := RunLogEntryFromReport(r)
	if e.RuleCounts["SA1000"] != 2 || e.RuleCounts["ST1003"] != 1 {
		t.Errorf("rule counts wrong: %v", e.RuleCounts)
	}
	if e.Errors != 1 || e.Warnings != 1 || e.Notes != 1 {
		t.Errorf("severity counts wrong: e=%d w=%d n=%d", e.Errors, e.Warnings, e.Notes)
	}
	if e.TestsFailed != 1 || e.TestsPassed != 1 {
		t.Errorf("test counts wrong: fail=%d pass=%d", e.TestsFailed, e.TestsPassed)
	}
}

func TestAppendRunLog_TrimsToMax(t *testing.T) {
	var rl *RunLog
	for i := range MaxRunLog + 10 {
		rl = AppendRunLog(rl, RunLogEntry{Errors: i})
	}
	if len(rl.Entries) != MaxRunLog {
		t.Fatalf("want %d entries, got %d", MaxRunLog, len(rl.Entries))
	}
	// Oldest dropped: first retained entry is #10, last is the newest.
	if rl.Entries[0].Errors != 10 {
		t.Errorf("oldest not trimmed: first entry Errors=%d want 10", rl.Entries[0].Errors)
	}
	if rl.Entries[len(rl.Entries)-1].Errors != MaxRunLog+9 {
		t.Errorf("newest wrong: %d", rl.Entries[len(rl.Entries)-1].Errors)
	}
}

func TestRunLog_RoundTripAndSeries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run-log.json")
	rl := AppendRunLog(nil, RunLogEntry{RuleCounts: map[string]int{"SA1000": 1}})
	rl = AppendRunLog(rl, RunLogEntry{RuleCounts: map[string]int{"SA1000": 3}})
	rl = AppendRunLog(rl, RunLogEntry{RuleCounts: map[string]int{"OTHER": 5}}) // SA1000 absent → 0
	if err := SaveRunLog(path, rl); err != nil {
		t.Fatalf("SaveRunLog: %v", err)
	}
	got, err := LoadRunLog(path)
	if err != nil {
		t.Fatalf("LoadRunLog: %v", err)
	}
	series := got.RuleSeries("SA1000")
	want := []int{1, 3, 0}
	if len(series) != len(want) {
		t.Fatalf("series len %d want %d", len(series), len(want))
	}
	for i := range want {
		if series[i] != want[i] {
			t.Errorf("series[%d]=%d want %d", i, series[i], want[i])
		}
	}
}

func TestLoadRunLog_MissingIsNil(t *testing.T) {
	got, err := LoadRunLog(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || got != nil {
		t.Errorf("missing run log: want (nil,nil), got (%v,%v)", got, err)
	}
}

func TestRuleSeries_NilReceiver(t *testing.T) {
	var rl *RunLog
	if rl.RuleSeries("x") != nil {
		t.Error("nil run log should yield nil series")
	}
}
