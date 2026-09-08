package testjson

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// realFixture reads a real captured `go test -json` stream — not a
// fabricated sample — for use as a base in the mutation tests below.
func realFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/go-test-testjson-pkg.ndjson")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// TestParseBytes_CRLFLineEndings_StillParses verifies that a real capture
// with CRLF line endings (a common artifact of `go test -json` piped
// through a Windows CI runner) parses identically to the LF original —
// lineread strips a trailing \r along with \n.
func TestParseBytes_CRLFLineEndings_StillParses(t *testing.T) {
	data := realFixture(t)
	crlf := bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))

	wantResults, wantMalformed, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("baseline ParseBytes: %v", err)
	}
	gotResults, gotMalformed, err := ParseBytes(crlf)
	if err != nil {
		t.Fatalf("CRLF ParseBytes: %v", err)
	}
	if gotMalformed != wantMalformed {
		t.Errorf("CRLF malformed = %d, want %d (same as LF original)", gotMalformed, wantMalformed)
	}
	if len(gotResults) != len(wantResults) {
		t.Errorf("CRLF packages = %d, want %d", len(gotResults), len(wantResults))
	}
}

// TestParseBytes_TruncatedMidLine_CountsAsMalformed is the direct
// regression test for the #222/#239 silent-drop class: cutting a real
// stream mid-line (no trailing newline) must count the dangling partial
// line as malformed, never silently discard it with zero signal (fo-s38).
func TestParseBytes_TruncatedMidLine_CountsAsMalformed(t *testing.T) {
	data := realFixture(t)

	// Find a cut point that lands strictly inside a line (not on a
	// newline boundary) so the final "line" delivered to the parser is
	// genuinely a partial, invalid JSON fragment.
	nl := bytes.IndexByte(data, '\n')
	if nl < 0 || len(data) < nl+20 {
		t.Fatal("fixture too small/shaped to find a good cut point")
	}
	cut := nl + 10 // partway into the second line
	truncated := data[:cut]

	_, malformed, err := ParseBytes(truncated)
	if err != nil {
		t.Fatalf("ParseBytes on truncated input: %v", err)
	}
	if malformed == 0 {
		t.Fatal("truncated mid-line input reported 0 malformed lines — the dangling partial line was silently dropped")
	}
}

// TestParseBytes_TrailingGarbageAfterValidStream_CountsAsMalformed
// verifies that garbage appended after a complete, valid stream is
// counted, and does not corrupt or hide the already-parsed valid results.
func TestParseBytes_TrailingGarbageAfterValidStream_CountsAsMalformed(t *testing.T) {
	data := realFixture(t)

	baseResults, baseMalformed, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("baseline ParseBytes: %v", err)
	}

	withGarbage := append(append([]byte(nil), data...), []byte("garbage-not-json\n")...)
	results, malformed, err := ParseBytes(withGarbage)
	if err != nil {
		t.Fatalf("ParseBytes with trailing garbage: %v", err)
	}
	if malformed != baseMalformed+1 {
		t.Errorf("malformed = %d, want %d (baseline + 1 garbage line)", malformed, baseMalformed+1)
	}
	if len(results) != len(baseResults) {
		t.Errorf("packages = %d, want %d — trailing garbage should not perturb already-parsed results", len(results), len(baseResults))
	}
}

// TestParseBytes_NaNInfElapsed_RejectedNotPoisoned verifies the
// NaN/Inf-poisoning class on the Elapsed field: neither a bare NaN/Infinity
// token (invalid JSON syntax) nor an out-of-range numeric literal (valid
// JSON syntax, but overflows float64) may silently produce a poisoned
// Duration that later fails to marshal as JSON. Both must be rejected and
// counted as malformed instead.
func TestParseBytes_NaNInfElapsed_RejectedNotPoisoned(t *testing.T) {
	cases := []string{
		`{"Action":"pass","Package":"x","Elapsed":NaN}` + "\n",
		`{"Action":"pass","Package":"x","Elapsed":Infinity}` + "\n",
		`{"Action":"pass","Package":"x","Elapsed":1e400}` + "\n",
	}
	for _, tc := range cases {
		results, malformed, err := ParseBytes([]byte(tc))
		if err != nil {
			t.Fatalf("%q: ParseBytes error: %v", tc, err)
		}
		if malformed != 1 {
			t.Errorf("%q: malformed = %d, want 1 (line should be rejected, not silently accepted)", tc, malformed)
		}

		rep := ToReportWithMeta(results, []byte(tc))
		b, merr := json.Marshal(rep)
		if merr != nil {
			t.Fatalf("%q: Report failed to marshal: %v", tc, merr)
		}
		if !json.Valid(b) {
			t.Fatalf("%q: marshaled Report is not valid JSON", tc)
		}
	}
}

// TestParseBytes_MixedValidAndNaNElapsed verifies that a NaN/Inf-poisoning
// attempt mixed alongside otherwise-valid events doesn't take down the
// whole stream: the valid events still parse, and the poisoned line is
// counted as malformed rather than silently merged in or aborting the
// parse.
func TestParseBytes_MixedValidAndNaNElapsed(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Package":"x","Test":"TestA"}`,
		`{"Action":"pass","Package":"x","Test":"TestA","Elapsed":1e400}`, // poisoned — rejected
		`{"Action":"run","Package":"x","Test":"TestB"}`,
		`{"Action":"pass","Package":"x","Test":"TestB","Elapsed":0.01}`, // valid — must still land
		`{"Action":"pass","Package":"x","Elapsed":0.01}`,
	}, "\n") + "\n"

	results, malformed, err := ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if malformed != 1 {
		t.Fatalf("malformed = %d, want 1", malformed)
	}
	if len(results) != 1 {
		t.Fatalf("packages = %d, want 1", len(results))
	}
	if results[0].Passed != 1 {
		t.Fatalf("passed = %d, want 1 (TestB should still land despite TestA's poisoned line)", results[0].Passed)
	}
}
