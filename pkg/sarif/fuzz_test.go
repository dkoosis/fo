package sarif

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// FuzzReadBytes fuzzes the SARIF decoder against the two invariants that
// matter for fo's wire contract (fo-s38):
//
//   - no-silent-drop: ReadBytes either errors cleanly, or returns a
//     document that actually satisfies the version check — it never
//     hands back a doc that looks parsed but is hollow.
//   - valid-JSON-out: whatever fo goes on to emit for --format json is
//     always valid JSON, even when the input was adversarial.
//
// Seeded with real captured golangci-lint/go-arch-lint SARIF plus the
// specific wire-contract-drift shapes fo has regressed on before:
// truncated mid-document, trailing garbage, CRLF line endings, and a
// depth-bomb (confirming the #269 guard still holds, not re-adding it).
func FuzzReadBytes(f *testing.F) {
	seedReadBytesCorpus(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := ReadBytes(data)
		if err != nil {
			// A clean, explicit error is the correct outcome for
			// malformed/truncated/adversarial input.
			return
		}
		if doc == nil {
			t.Fatal("ReadBytes returned nil doc with nil error")
		}
		// no-silent-drop: Read already guards this internally, but the
		// fuzzer mutates raw bytes directly against ReadBytes, so assert
		// the invariant holds from the caller's side too.
		if doc.Version == "" {
			t.Fatal("ReadBytes returned a doc with empty Version and nil error — missing-version guard should have errored")
		}

		r := ToReport(doc)
		b, merr := json.Marshal(r)
		if merr != nil {
			t.Fatalf("ToReport produced a Report that fails to marshal to JSON: %v", merr)
		}
		if !json.Valid(b) {
			t.Fatal("marshaled report bytes are not valid JSON")
		}
	})
}

// seedReadBytesCorpus adds the minimal valid document, every real captured
// fixture, and hand-crafted mutations of the historical wire-contract-drift
// shapes (truncation, trailing garbage, CRLF, depth-bomb, oversized numeric
// literal) to f's corpus.
func seedReadBytesCorpus(f *testing.F) {
	f.Helper()
	f.Add([]byte(minimalSARIF))

	fixtures, err := os.ReadDir("testdata")
	if err != nil {
		f.Fatalf("read testdata dir: %v", err)
	}
	for _, entry := range fixtures {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sarif") {
			continue
		}
		data, err := os.ReadFile("testdata/" + entry.Name())
		if err != nil {
			f.Fatalf("read fixture %s: %v", entry.Name(), err)
		}
		f.Add(data)

		// Truncated mid-document: chop a real capture partway through —
		// the historical "false-clean" risk is a truncation that happens
		// to land on syntactically-complete-but-semantically-incomplete
		// JSON and gets silently accepted as a smaller, valid document.
		if len(data) > 40 {
			f.Add(data[:len(data)/2])
			f.Add(data[:len(data)/3])
		}

		// CRLF line endings.
		f.Add(bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n")))

		// Trailing garbage after the SARIF document (golangci-lint v2
		// itself appends a text summary — Read already tolerates this,
		// fuzz to keep it that way).
		f.Add(append(append([]byte(nil), data...), []byte("\ngarbage not json\x00\x01")...))
	}

	// Depth-bomb: confirm the #269 guard holds under fuzzing, not just the
	// fixed-depth unit test.
	depth := maxNestingDepth + 50
	f.Add([]byte(strings.Repeat("[", depth) + strings.Repeat("]", depth)))
	f.Add([]byte(strings.Repeat("{\"a\":", depth) + "1" + strings.Repeat("}", depth)))

	// Oversized numeric literal in an int-typed field (region line/col) —
	// the NaN/Inf-poisoning class. encoding/json rejects an out-of-range
	// number for an int field, but must do so cleanly, never by silently
	// coercing to a sentinel and continuing.
	f.Add([]byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"t"}},"results":[` +
		`{"ruleId":"r","level":"error","message":{"text":"m"},"locations":[` +
		`{"physicalLocation":{"artifactLocation":{"uri":"f"},"region":{"startLine":1e400}}}]}]}]}`))
}
