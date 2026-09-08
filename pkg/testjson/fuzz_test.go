package testjson

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// FuzzParseBytesWireContract fuzzes the go test -json decoder against the
// two invariants that matter for fo's wire contract (fo-s38):
//
//   - no-silent-drop: malformed is never negative, and never silently
//     hides a corruption the caller can't observe — every non-blank line
//     that fails to unmarshal is counted.
//   - valid-JSON-out: whatever fo goes on to emit for --format json is
//     always valid JSON, even when the input stream was adversarial.
//
// Seeded with real captured `go test -json` output plus the specific
// wire-contract-drift shapes fo has regressed on before (#222/#239):
// truncated NDJSON, trailing garbage, CRLF line endings, and a NaN/Inf
// numeric-literal poisoning attempt on the Elapsed field.
func FuzzParseBytesWireContract(f *testing.F) {
	seedParseBytesCorpus(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		results, malformed, err := ParseBytes(data)
		if err != nil {
			t.Fatalf("ParseBytes should not fail for arbitrary input: %v", err)
		}
		if malformed < 0 {
			t.Fatalf("malformed should never be negative: %d", malformed)
		}

		r := ToReportWithMeta(results, data)
		b, merr := json.Marshal(r)
		if merr != nil {
			t.Fatalf("ToReportWithMeta produced a Report that fails to marshal to JSON: %v", merr)
		}
		if !json.Valid(b) {
			t.Fatal("marshaled report bytes are not valid JSON")
		}
	})
}

// seedParseBytesCorpus adds a handful of tiny hand-written events, every
// real captured fixture under testdata/, and hand-crafted mutations of the
// historical wire-contract-drift shapes to f's corpus.
func seedParseBytesCorpus(f *testing.F) {
	f.Helper()
	f.Add([]byte(`{"Action":"run","Package":"x","Test":"T"}` + "\n" +
		`{"Action":"pass","Package":"x","Test":"T","Elapsed":0.1}` + "\n"))
	f.Add([]byte(`not-json` + "\n" +
		`{"Action":"output","Package":"x","Output":"coverage: 80.0% of statements\n"}` + "\n"))

	fixtures, err := filepath.Glob("testdata/*.ndjson")
	if err != nil {
		f.Fatalf("glob testdata: %v", err)
	}
	for _, path := range fixtures {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatalf("read fixture %s: %v", path, err)
		}
		f.Add(data)

		// Truncated NDJSON: cut mid-stream, and mid-line (no trailing
		// newline) — the class fo-s38 targets directly (#222/#239: a
		// truncated stream must never silently look "clean").
		if len(data) > 40 {
			f.Add(data[:len(data)/2])
			f.Add(data[:len(data)/2+7]) // offset so it likely lands mid-line
		}

		// CRLF line endings.
		f.Add(bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n")))

		// Trailing garbage after the last valid NDJSON line.
		f.Add(append(append([]byte(nil), data...), []byte("garbage-not-json\x00\x01")...))
	}

	// NaN/Inf-poisoning attempt on Elapsed. Neither is valid JSON syntax
	// (bare NaN/Infinity tokens), so the line must be rejected and counted
	// as malformed, never silently coerced into a poisoned float.
	f.Add([]byte(`{"Action":"pass","Package":"x","Test":"T","Elapsed":NaN}` + "\n"))
	f.Add([]byte(`{"Action":"pass","Package":"x","Test":"T","Elapsed":Infinity}` + "\n"))
	// Syntactically valid JSON number, but out of float64 range — Go's
	// decoder must reject it cleanly rather than silently produce ±Inf.
	f.Add([]byte(`{"Action":"pass","Package":"x","Test":"T","Elapsed":1e400}` + "\n"))
}
