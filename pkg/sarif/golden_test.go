package sarif_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
)

// TestGoldenFixtures_RoundTripSanity parses every captured real SARIF
// document under testdata/ — golangci-lint (with findings, and a clean
// zero-issue run) and go-arch-lint (converted through the archlint
// wrapper) — and asserts basic contract sanity: it parses without error,
// the resulting Report is structurally sane, and it always marshals to
// valid JSON — the wire contract fo promises downstream --format json
// consumers (fo-s38).
func TestGoldenFixtures_RoundTripSanity(t *testing.T) {
	fixtures, err := filepath.Glob("testdata/*.sarif")
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

			doc, err := sarif.ReadBytes(data)
			if err != nil {
				t.Fatalf("ReadBytes: %v", err)
			}
			if doc.Version == "" {
				t.Fatal("parsed doc has empty Version")
			}

			r := sarif.ToReportWithMeta(doc, data)
			if r.Tool == "" {
				t.Error("Report.Tool is empty — expected a driver name from every real capture")
			}
			if r.DataHash == "" {
				t.Error("Report.DataHash is empty — ToReportWithMeta should stamp it from rawInput")
			}
			for _, f := range r.Findings {
				if len(f.Fingerprint) != 64 {
					t.Errorf("finding %q: Fingerprint len = %d, want 64", f.RuleID, len(f.Fingerprint))
				}
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
