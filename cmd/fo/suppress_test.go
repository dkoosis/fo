package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

func TestApplySuppress_AbsentFileNoop(t *testing.T) {
	r := &report.Report{
		Findings: []report.Finding{
			{RuleID: "X", File: "a.go", Severity: report.SeverityWarning},
		},
	}
	var stderr bytes.Buffer
	applySuppress(r, filepath.Join(t.TempDir(), "does-not-exist"), &stderr)
	if len(r.Findings) != 1 {
		t.Errorf("absent file should be no-op, got %d findings", len(r.Findings))
	}
	if r.Suppressed != 0 {
		t.Errorf("Suppressed = %d, want 0", r.Suppressed)
	}
	if len(r.Notices) != 0 {
		t.Errorf("absent file should not add notices: %v", r.Notices)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
}

func TestApplySuppress_ParseErrorAddsNotice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	if err := os.WriteFile(path, []byte("=bogus garbage line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &report.Report{
		Findings: []report.Finding{{RuleID: "X", File: "a.go"}},
	}
	var stderr bytes.Buffer
	applySuppress(r, path, &stderr)
	if len(r.Findings) != 1 {
		t.Errorf("parse failure should not drop findings")
	}
	if len(r.Notices) != 1 || !strings.Contains(r.Notices[0], "suppress") {
		t.Errorf("expected suppress notice, got %v", r.Notices)
	}
}

func TestApplySuppress_ActiveRuleSuppresses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	body := "SA1019 reason=\"upstream\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &report.Report{
		Findings: []report.Finding{
			{RuleID: "SA1019", File: "a.go", Severity: report.SeverityWarning},
			{RuleID: "OTHER", File: "b.go", Severity: report.SeverityError},
		},
	}
	var stderr bytes.Buffer
	applySuppress(r, path, &stderr)
	if len(r.Findings) != 1 || r.Findings[0].RuleID != "OTHER" {
		t.Errorf("findings after suppress: %+v", r.Findings)
	}
	if r.Suppressed != 1 {
		t.Errorf("Suppressed = %d, want 1", r.Suppressed)
	}
}
