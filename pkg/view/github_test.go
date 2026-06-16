package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

func renderGH(t *testing.T, r report.Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := RenderGitHub(&b, r); err != nil {
		t.Fatalf("RenderGitHub: %v", err)
	}
	return b.String()
}

func TestRenderGitHub_LevelsAndLocation(t *testing.T) {
	r := report.Report{Findings: []report.Finding{
		{Severity: report.SeverityError, File: "a.go", Line: 10, Col: 2, Message: "boom", RuleID: "E1"},
		{Severity: report.SeverityWarning, File: "b.go", Line: 5, Message: "meh"},
		{Severity: report.SeverityNote, File: "c.go", Message: "fyi"},
	}}
	got := renderGH(t, r)
	want := []string{
		"::error file=a.go,line=10,col=2,title=E1::boom",
		"::warning file=b.go,line=5::meh",
		"::notice file=c.go::fyi",
	}
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing annotation %q\n%s", w, got)
		}
	}
}

func TestRenderGitHub_EscapesMessageAndProps(t *testing.T) {
	r := report.Report{Findings: []report.Finding{
		{Severity: report.SeverityError, File: "weird,name:x.go", Line: 1, Message: "100% broken\nsecond line"},
	}}
	got := renderGH(t, r)
	if !strings.Contains(got, "file=weird%2Cname%3Ax.go") {
		t.Errorf("property not escaped: %s", got)
	}
	if !strings.Contains(got, "100%25 broken%0Asecond line") {
		t.Errorf("message not escaped: %s", got)
	}
	if strings.Count(got, "\n") != 1 { // exactly one trailing newline, no raw LF in message
		t.Errorf("message LF leaked into output: %q", got)
	}
}

func TestRenderGitHub_ScopedToNewWhenDiffPresent(t *testing.T) {
	r := report.Report{
		Findings: []report.Finding{
			{Severity: report.SeverityError, File: "new.go", Line: 1, Message: "new", Fingerprint: "fp-new"},
			{Severity: report.SeverityError, File: "old.go", Line: 1, Message: "persistent", Fingerprint: "fp-old"},
			{Severity: report.SeverityError, File: "reg.go", Line: 1, Message: "regressed", Fingerprint: "fp-reg"},
		},
		Diff: &report.DiffSummary{
			New:       []report.DiffItem{{Fingerprint: "fp-new"}},
			Regressed: []report.DiffItem{{Fingerprint: "fp-reg"}},
		},
	}
	got := renderGH(t, r)
	if !strings.Contains(got, "new.go") || !strings.Contains(got, "reg.go") {
		t.Errorf("new/regressed should be emitted: %s", got)
	}
	if strings.Contains(got, "old.go") {
		t.Errorf("persistent finding should be scoped out: %s", got)
	}
}

func TestRenderGitHub_NoDiffEmitsAll(t *testing.T) {
	r := report.Report{Findings: []report.Finding{
		{Severity: report.SeverityError, File: "a.go", Line: 1, Message: "x", Fingerprint: "fp1"},
		{Severity: report.SeverityError, File: "b.go", Line: 1, Message: "y", Fingerprint: "fp2"},
	}}
	if n := strings.Count(renderGH(t, r), "\n"); n != 2 {
		t.Errorf("no diff should emit all findings, got %d lines", n)
	}
}

func TestRenderGitHub_EmptyDiffEmitsNothing(t *testing.T) {
	r := report.Report{
		Findings: []report.Finding{{Severity: report.SeverityError, File: "a.go", Message: "x", Fingerprint: "fp1"}},
		Diff:     &report.DiffSummary{}, // present but empty New/Regressed
	}
	if got := renderGH(t, r); got != "" {
		t.Errorf("empty diff should emit nothing, got %q", got)
	}
}
