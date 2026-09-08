package sarif_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
)

// realFixture reads the largest captured golangci-lint fixture — real tool
// output, not a fabricated sample — for use as a base in the mutation
// tests below.
func realFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/golangci-lint-113-post-cleanup.sarif")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// TestReadBytes_TruncatedRealDocument_ErrorsNotFalseClean is the SARIF
// analog of the #222/#239 silent-drop class: cutting a real multi-result
// document mid-stream must produce a clean decode error, never a smaller
// document that looks like a legitimately clean/short run (fo-s38).
func TestReadBytes_TruncatedRealDocument_ErrorsNotFalseClean(t *testing.T) {
	data := realFixture(t)

	for _, frac := range []int{2, 3, 4, 8} {
		cut := len(data) / frac
		truncated := data[:cut]
		doc, err := sarif.ReadBytes(truncated)
		if err == nil {
			t.Fatalf("truncated to %d/%d (%d bytes): expected an error, got a doc with %d run(s) — silent drop",
				frac, frac, cut, len(doc.Runs))
		}
	}
}

// TestReadBytes_CRLFLineEndings_StillParses verifies that a real SARIF
// capture with CRLF line endings (a common artifact of tools run under a
// Windows CI runner) parses identically to the LF original.
func TestReadBytes_CRLFLineEndings_StillParses(t *testing.T) {
	data := realFixture(t)
	crlf := bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))

	want, err := sarif.ReadBytes(data)
	if err != nil {
		t.Fatalf("baseline ReadBytes: %v", err)
	}
	got, err := sarif.ReadBytes(crlf)
	if err != nil {
		t.Fatalf("CRLF ReadBytes: %v", err)
	}

	wantResults := 0
	for _, r := range want.Runs {
		wantResults += len(r.Results)
	}
	gotResults := 0
	for _, r := range got.Runs {
		gotResults += len(r.Results)
	}
	if gotResults != wantResults {
		t.Errorf("CRLF input: results = %d, want %d (same as LF original)", gotResults, wantResults)
	}
}

// TestReadBytes_OversizedNumericLiteral_ErrorsCleanly verifies the
// NaN/Inf-poisoning class: a JSON number so large it would overflow
// float64/int on unmarshal must be rejected by the decoder, never
// silently coerced into a poisoned ±Inf value that later fails to
// re-marshal as JSON.
func TestReadBytes_OversizedNumericLiteral_ErrorsCleanly(t *testing.T) {
	input := []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"t"}},"results":[` +
		`{"ruleId":"r","level":"error","message":{"text":"m"},"locations":[` +
		`{"physicalLocation":{"artifactLocation":{"uri":"f"},"region":{"startLine":1e400}}}]}]}]}`)

	_, err := sarif.ReadBytes(input)
	if err == nil {
		t.Fatal("expected an error for an out-of-range numeric literal, got nil")
	}
	// The depth-guard sentinel is unexported (pkg/sarif keeps it internal —
	// no caller outside the package branches on it); assert on the message
	// instead of the sentinel to confirm this isn't misclassified as a
	// depth-bomb rejection.
	if strings.Contains(err.Error(), "nesting too deep") {
		t.Fatalf("got a nesting-too-deep error, want a decode error for the oversized literal: %v", err)
	}
}
