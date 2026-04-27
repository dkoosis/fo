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
