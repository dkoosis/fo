package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

func TestStateReset_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-run.json")
	if err := os.WriteFile(p, []byte(`{"version":1,"runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"state", "reset", "--state-file", p}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("state file should be gone, err=%v", err)
	}
}

func TestStateReset_MissingFileIsOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "absent.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"state", "reset", "--state-file", p}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
}

func TestState_RequiresSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"state"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
}

func TestWriteDiffDetail_NilReport(t *testing.T) {
	var buf bytes.Buffer
	writeDiffDetail(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil report, got %q", buf.String())
	}
}

func TestWriteDiffDetail_NilDiff(t *testing.T) {
	var buf bytes.Buffer
	writeDiffDetail(&buf, &report.Report{})
	if buf.Len() != 0 {
		t.Fatalf("expected no output for nil diff, got %q", buf.String())
	}
}

func TestWriteDiffDetail_EmptyDiff(t *testing.T) {
	var buf bytes.Buffer
	writeDiffDetail(&buf, &report.Report{
		Diff: &report.DiffSummary{},
	})
	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty diff, got %q", buf.String())
	}
}

func TestWriteDiffDetail_NewItems(t *testing.T) {
	r := &report.Report{
		Findings: []report.Finding{
			{Fingerprint: "fp1", RuleID: "errcheck", File: "pkg/foo.go", Line: 42, Message: "error return ignored"},
		},
		Diff: &report.DiffSummary{
			New: []report.DiffItem{{Fingerprint: "fp1", RuleID: "errcheck", File: "pkg/foo.go"}},
		},
	}
	var buf bytes.Buffer
	writeDiffDetail(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "NEW (1)") {
		t.Errorf("missing NEW header: %q", out)
	}
	if !strings.Contains(out, "pkg/foo.go:42") {
		t.Errorf("missing file:line: %q", out)
	}
	if !strings.Contains(out, "errcheck") {
		t.Errorf("missing rule ID: %q", out)
	}
	if !strings.Contains(out, "error return ignored") {
		t.Errorf("missing message: %q", out)
	}
	if strings.Contains(out, "REGRESSED") {
		t.Errorf("unexpected REGRESSED section: %q", out)
	}
}

func TestWriteDiffDetail_RegressedItems(t *testing.T) {
	r := &report.Report{
		Findings: []report.Finding{
			{Fingerprint: "fp2", RuleID: "gosec", File: "cmd/main.go", Line: 7, Message: "G304: file inclusion"},
		},
		Diff: &report.DiffSummary{
			Regressed: []report.DiffItem{{Fingerprint: "fp2", RuleID: "gosec", File: "cmd/main.go"}},
		},
	}
	var buf bytes.Buffer
	writeDiffDetail(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "REGRESSED (1)") {
		t.Errorf("missing REGRESSED header: %q", out)
	}
	if !strings.Contains(out, "cmd/main.go:7") {
		t.Errorf("missing file:line: %q", out)
	}
	if strings.Contains(out, "NEW") {
		t.Errorf("unexpected NEW section: %q", out)
	}
}

func TestWriteDiffDetail_BothNewAndRegressed(t *testing.T) {
	r := &report.Report{
		Findings: []report.Finding{
			{Fingerprint: "fp1", RuleID: "errcheck", File: "a.go", Line: 1, Message: "msg1"},
			{Fingerprint: "fp2", RuleID: "gosec", File: "b.go", Line: 2, Message: "msg2"},
		},
		Diff: &report.DiffSummary{
			New:       []report.DiffItem{{Fingerprint: "fp1"}},
			Regressed: []report.DiffItem{{Fingerprint: "fp2"}},
		},
	}
	var buf bytes.Buffer
	writeDiffDetail(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "NEW (1)") {
		t.Errorf("missing NEW header: %q", out)
	}
	if !strings.Contains(out, "REGRESSED (1)") {
		t.Errorf("missing REGRESSED header: %q", out)
	}
	newIdx := strings.Index(out, "NEW")
	regIdx := strings.Index(out, "REGRESSED")
	if newIdx > regIdx {
		t.Errorf("expected NEW before REGRESSED in output")
	}
}

func TestWriteDiffDetail_FingerprintMiss_Fallback(t *testing.T) {
	// DiffItem fingerprint not present in Findings → fallback to item.File + item.RuleID only.
	r := &report.Report{
		Findings: []report.Finding{},
		Diff: &report.DiffSummary{
			New: []report.DiffItem{{Fingerprint: "missing-fp", RuleID: "unused", File: "pkg/x.go"}},
		},
	}
	var buf bytes.Buffer
	writeDiffDetail(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "pkg/x.go") {
		t.Errorf("missing fallback file: %q", out)
	}
	if !strings.Contains(out, "unused") {
		t.Errorf("missing fallback rule ID: %q", out)
	}
}

func TestWriteDiffDetail_LineZero_NoColon(t *testing.T) {
	// Line=0 means no line info — loc should be just the file path, no ":0" suffix.
	r := &report.Report{
		Findings: []report.Finding{
			{Fingerprint: "fp1", RuleID: "revive", File: "pkg/foo.go", Line: 0, Message: "missing comment"},
		},
		Diff: &report.DiffSummary{
			New: []report.DiffItem{{Fingerprint: "fp1"}},
		},
	}
	var buf bytes.Buffer
	writeDiffDetail(&buf, r)
	out := buf.String()
	if strings.Contains(out, "pkg/foo.go:0") {
		t.Errorf("line=0 should not produce ':0' suffix: %q", out)
	}
	if !strings.Contains(out, "pkg/foo.go") {
		t.Errorf("missing file in output: %q", out)
	}
}
