package state

import (
	"path/filepath"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

func TestSnapshot_RoundTripAndLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "findings.json")
	r := &report.Report{
		Tool: "golangci",
		Findings: []report.Finding{
			{ID: "F-7a2", Fingerprint: "7a2ffff", RuleID: "SA1000", Message: "boom", File: "x.go", Line: 4},
			{Fingerprint: "noid000"}, // no ID → excluded from snapshot
		},
		Tests: []report.TestResult{
			{ID: "T-3f1", Fingerprint: "3f1aaaa", Package: "p", Test: "TestX", Outcome: report.OutcomeFail},
		},
	}
	if err := SaveSnapshot(path, SnapshotFromReport(r)); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	got, err := LoadSnapshot(path)
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("want 1 snapshot finding (ID-less excluded), got %d", len(got.Findings))
	}
	f, _, ok := got.Lookup("F-7a2")
	if !ok || f.Message != "boom" {
		t.Errorf("Lookup F-7a2: ok=%v finding=%+v", ok, f)
	}
	_, tr, ok := got.Lookup("T-3f1")
	if !ok || tr.Test != "TestX" {
		t.Errorf("Lookup T-3f1: ok=%v test=%+v", ok, tr)
	}
	if _, _, ok := got.Lookup("F-nope"); ok {
		t.Error("Lookup of unknown ID should miss")
	}
}

func TestSnapshot_PriorIDs(t *testing.T) {
	s := &Snapshot{
		Findings: []report.Finding{{ID: "F-7a2", Fingerprint: "7a2ffff"}},
		Tests:    []report.TestResult{{ID: "T-3f1", Fingerprint: "3f1aaaa"}},
	}
	p := s.PriorIDs()
	if p.Findings["7a2ffff"] != "F-7a2" {
		t.Errorf("finding prior map wrong: %v", p.Findings)
	}
	if p.Tests["3f1aaaa"] != "T-3f1" {
		t.Errorf("test prior map wrong: %v", p.Tests)
	}
}

func TestSnapshot_LoadMissingIsNil(t *testing.T) {
	got, err := LoadSnapshot(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || got != nil {
		t.Errorf("missing snapshot: want (nil,nil), got (%v,%v)", got, err)
	}
}

func TestSnapshot_NilPriorIDs(t *testing.T) {
	var s *Snapshot
	if s.PriorIDs() != nil {
		t.Error("nil snapshot should yield nil Prior")
	}
}
