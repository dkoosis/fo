package wrapdiag

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/sarif"
)

// fo-gn0 regression: an oversize line between two valid diagnostics must
// not abort conversion.
func TestDiagConvert_OversizeLineDoesNotAbort(t *testing.T) {
	huge := strings.Repeat("Z", lineread.MaxLineLen+1024)
	input := "main.go:1:1: first\n" + huge + "\nmain.go:9:1: third\n"

	var buf bytes.Buffer
	if err := Convert(strings.NewReader(input), &buf, DiagOpts{Tool: "govet", Level: "error"}); err != nil {
		t.Fatalf("Convert err = %v", err)
	}
	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("invalid SARIF: %v", err)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Errorf("got %d results, want 2 (first + third)", len(doc.Runs[0].Results))
	}
}

// fo-2mb: oversize-line drops must surface as a stderr warning so a
// silently-truncated diagnostic doesn't yield a false-clean SARIF.
func TestDiagConvert_OversizeLineWarnsStderr(t *testing.T) {
	huge := strings.Repeat("Z", lineread.MaxLineLen+1024)
	input := huge + "\nmain.go:42:1: kept\n"

	var out, errBuf bytes.Buffer
	if err := Convert(strings.NewReader(input), &out, DiagOpts{
		Tool:   "govet",
		Level:  "error",
		Stderr: &errBuf,
	}); err != nil {
		t.Fatalf("Convert err = %v", err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("invalid SARIF: %v", err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Errorf("got %d results, want 1 (the kept line)", len(doc.Runs[0].Results))
	}

	got := errBuf.String()
	if !strings.Contains(got, "wrapdiag: dropped 1") {
		t.Errorf("stderr missing oversize warning: %q", got)
	}
}

// fo-2mb: when no lines are dropped, no stderr warning is emitted.
func TestDiagConvert_NoOversizeNoWarning(t *testing.T) {
	var out, errBuf bytes.Buffer
	if err := Convert(strings.NewReader("main.go:1:1: only\n"), &out, DiagOpts{
		Tool:   "govet",
		Level:  "error",
		Stderr: &errBuf,
	}); err != nil {
		t.Fatalf("Convert err = %v", err)
	}
	if errBuf.Len() != 0 {
		t.Errorf("unexpected stderr output: %q", errBuf.String())
	}
}
