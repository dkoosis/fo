package sarif_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/sarif"
)

func TestToReport_GolangciLintFixture(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/golangci-lint-113-post-cleanup.sarif")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := sarif.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := sarif.ToReport(doc)

	if r.Tool != "golangci-lint" {
		t.Errorf("Tool = %q, want golangci-lint", r.Tool)
	}
	if len(r.Findings) == 0 {
		t.Fatal("expected findings, got 0")
	}
	if len(r.Tests) != 0 {
		t.Errorf("Tests = %d, want 0 (SARIF has no tests)", len(r.Tests))
	}

	for i := 1; i < len(r.Findings); i++ {
		if r.Findings[i].Score > r.Findings[i-1].Score {
			t.Fatalf("findings not sorted by Score desc: idx %d (%v) > idx %d (%v)",
				i, r.Findings[i].Score, i-1, r.Findings[i-1].Score)
		}
	}

	for _, f := range r.Findings {
		switch f.Severity {
		case report.SeverityError, report.SeverityWarning, report.SeverityNote:
		default:
			t.Errorf("finding %q: Severity = %q, want one of error/warning/note",
				f.RuleID, f.Severity)
		}
		if f.Fingerprint == "" {
			t.Errorf("finding %q: empty Fingerprint", f.RuleID)
		}
		if len(f.Fingerprint) != 64 {
			t.Errorf("finding %q: Fingerprint len = %d, want 64", f.RuleID, len(f.Fingerprint))
		}
	}
}

func TestToReport_DeterministicFingerprint(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/golangci-lint-113-post-cleanup.sarif")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := sarif.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	a := sarif.ToReport(doc)
	b := sarif.ToReport(doc)

	if len(a.Findings) != len(b.Findings) {
		t.Fatalf("finding counts diverged: %d vs %d", len(a.Findings), len(b.Findings))
	}
	for i := range a.Findings {
		if a.Findings[i].Fingerprint != b.Findings[i].Fingerprint {
			t.Fatalf("finding %d: fingerprint diverged: %s vs %s",
				i, a.Findings[i].Fingerprint, b.Findings[i].Fingerprint)
		}
	}
}

func TestToReportWithMeta_StampsDataHash(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/golangci-lint-113-post-cleanup.sarif")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	doc, err := sarif.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	r := sarif.ToReportWithMeta(doc, data)
	if len(r.DataHash) != 64 {
		t.Errorf("DataHash len = %d, want 64", len(r.DataHash))
	}
}

func TestToReport_EmptyDocument(t *testing.T) {
	t.Parallel()

	r := sarif.ToReport(&sarif.Document{})
	if r == nil {
		t.Fatal("ToReport returned nil for empty doc")
	}
	if len(r.Findings) != 0 {
		t.Errorf("Findings = %d, want 0", len(r.Findings))
	}
	if r.Tool != "" {
		t.Errorf("Tool = %q, want empty", r.Tool)
	}
}

func TestToReport_FixCommandPassthrough(t *testing.T) {
	t.Parallel()

	const wantCmd = "gofmt -w bad.go"
	b := sarif.NewBuilder("gofmt", "")
	b.AddResultWithFix("needs-formatting", "warning", "needs formatting", "bad.go", 0, 0, wantCmd)

	r := sarif.ToReport(b.Document())
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Findings))
	}
	if got := r.Findings[0].FixCommand; got != wantCmd {
		t.Errorf("Finding.FixCommand = %q, want %q", got, wantCmd)
	}
}

func TestToReport_NoFix_EmptyFixCommand(t *testing.T) {
	t.Parallel()

	b := sarif.NewBuilder("govet", "")
	b.AddResult("printf", "warning", "wrong arg", "main.go", 5, 0)

	r := sarif.ToReport(b.Document())
	if len(r.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(r.Findings))
	}
	if got := r.Findings[0].FixCommand; got != "" {
		t.Errorf("Finding.FixCommand = %q, want empty for result without fix", got)
	}
}
