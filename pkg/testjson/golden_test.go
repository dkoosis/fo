package testjson_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkoosis/fo/pkg/testjson"
)

// TestGoldenFixtures_RoundTripSanity parses every captured real `go test
// -json` output under testdata/ and asserts basic contract sanity: it
// parses without error, no lines are misclassified as malformed (these are
// pristine captures — fo-s38), the resulting Report is non-trivial, and the
// Report always marshals to valid JSON (the wire contract fo promises
// downstream --format json consumers).
func TestGoldenFixtures_RoundTripSanity(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.ndjson")
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no golden fixtures found under testdata/ — corpus is empty")
	}

	for _, path := range fixtures {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			results, malformed, err := testjson.ParseBytes(data)
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if malformed != 0 {
				t.Errorf("malformed = %d, want 0 — this is a real, pristine `go test -json` capture", malformed)
			}
			if len(results) == 0 {
				t.Fatal("expected at least one package result, got 0 — non-trivial Report requirement")
			}

			r := testjson.ToReportWithMeta(results, data)
			if len(r.Tests) == 0 {
				t.Error("Report.Tests is empty — expected a non-trivial Report from real captured output")
			}
			if r.DataHash == "" {
				t.Error("Report.DataHash is empty — ToReportWithMeta should stamp it from rawInput")
			}

			b, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("Report failed to marshal to JSON: %v", err)
			}
			if !json.Valid(b) {
				t.Fatal("marshaled Report bytes are not valid JSON")
			}
		})
	}
}
