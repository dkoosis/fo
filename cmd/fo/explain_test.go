package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/state"
)

// seedSnapshot writes a findings snapshot into a temp FO_STATE_DIR and
// returns nothing — callers read it back through runExplain.
func seedSnapshot(t *testing.T, r *report.Report) {
	t.Helper()
	t.Setenv("FO_STATE_DIR", t.TempDir())
	report.AssignShortIDs(r)
	if err := state.SaveSnapshot(state.SnapshotPath(), state.SnapshotFromReport(r)); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
}

func TestRunExplain_ResolvesFinding(t *testing.T) {
	r := &report.Report{Findings: []report.Finding{
		{Fingerprint: "aaaa1111", RuleID: "SA1000", Severity: report.SeverityError, Message: "boom", File: "x.go", Line: 4},
	}}
	seedSnapshot(t, r)
	id := r.Findings[0].ID

	var out, errBuf bytes.Buffer
	if code := runExplain([]string{id}, &out, &errBuf); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{id, "SA1000", "boom", "x.go:4", "staticcheck.dev"} {
		if !strings.Contains(got, want) {
			t.Errorf("explain output missing %q\n%s", want, got)
		}
	}
}

func TestRunExplain_ResolvesTest(t *testing.T) {
	r := &report.Report{Tests: []report.TestResult{
		{Fingerprint: "bbbb2222", Package: "p", Test: "TestX", Outcome: report.OutcomeFail, Output: "want 1 got 2"},
	}}
	seedSnapshot(t, r)
	id := r.Tests[0].ID

	var out, errBuf bytes.Buffer
	if code := runExplain([]string{id}, &out, &errBuf); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errBuf.String())
	}
	got := out.String()
	for _, want := range []string{id, "TestX", "want 1 got 2"} {
		if !strings.Contains(got, want) {
			t.Errorf("explain output missing %q\n%s", want, got)
		}
	}
}

func TestRunExplain_UnknownID(t *testing.T) {
	seedSnapshot(t, &report.Report{Findings: []report.Finding{{Fingerprint: "aaaa1111", Message: "x"}}})
	var out, errBuf bytes.Buffer
	if code := runExplain([]string{"F-nope"}, &out, &errBuf); code != 2 {
		t.Errorf("unknown id: want exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "not found") {
		t.Errorf("want 'not found' message, got %q", errBuf.String())
	}
}

func TestRunExplain_NoSnapshot(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir()) // empty dir, no snapshot
	var out, errBuf bytes.Buffer
	if code := runExplain([]string{"F-abc"}, &out, &errBuf); code != 2 {
		t.Errorf("no snapshot: want exit 2, got %d", code)
	}
	if !strings.Contains(errBuf.String(), "no prior run") {
		t.Errorf("want 'no prior run' hint, got %q", errBuf.String())
	}
}

func TestRunExplain_MissingArg(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := runExplain(nil, &out, &errBuf); code != 2 {
		t.Errorf("missing id: want exit 2, got %d", code)
	}
}
