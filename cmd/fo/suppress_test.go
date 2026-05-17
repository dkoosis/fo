package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/report"
)

func TestApplySuppress_AbsentFileNoop(t *testing.T) {
	r := &report.Report{
		Findings: []report.Finding{
			{RuleID: "X", File: aDotGo, Severity: report.SeverityWarning},
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
		Findings: []report.Finding{{RuleID: "X", File: aDotGo}},
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

// TestLoadSuppressRuleset_StreamingPath covers the helper used by
// runStreamCtx to share a ruleset across snapshots (fo-2sk).
func TestLoadSuppressRuleset_StreamingPath(t *testing.T) {
	t.Run("absent file returns nil", func(t *testing.T) {
		var stderr bytes.Buffer
		got := loadSuppressRuleset(nil, filepath.Join(t.TempDir(), "x"), &stderr)
		if got != nil {
			t.Errorf("got %v, want nil for absent file", got)
		}
	})
	t.Run("valid file returns ruleset", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ignore")
		if err := os.WriteFile(path, []byte("SA1019\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		rs := loadSuppressRuleset(nil, path, &stderr)
		if rs == nil {
			t.Fatal("got nil, want a Ruleset")
		}
		// Apply to a finding to confirm it works.
		r := &report.Report{Findings: []report.Finding{
			{RuleID: "SA1019", File: aDotGo, Severity: report.SeverityWarning},
		}}
		report.ApplyFilter(r, rs, time.Now())
		if len(r.Findings) != 0 {
			t.Errorf("ruleset did not filter: %+v", r.Findings)
		}
	})
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
			{RuleID: "SA1019", File: aDotGo, Severity: report.SeverityWarning},
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
