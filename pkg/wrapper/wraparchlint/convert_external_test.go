package wraparchlint_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
)

// Black-box coverage for the package-level Convert. White-box cases
// (clean, single violation, FixCommand, invalid/empty input, full import
// path) live in archlint_test.go; this file adds: oversized input bound,
// writer failure, and the one-result-per-warning invariant.

func TestConvert_OversizedInput_ReturnsBoundError(t *testing.T) {
	t.Parallel()

	// Stream N+1 bytes lazily so we don't allocate 256 MiB in test memory.
	r := io.LimitReader(zeroReader{}, int64(boundread.DefaultMax)+1)

	err := wraparchlint.Convert(r, io.Discard)
	if !errors.Is(err, boundread.ErrInputTooLarge) {
		t.Fatalf("err = %v, want errors.Is(_, ErrInputTooLarge)", err)
	}
}

func TestConvert_WriterFailure_ReturnsWriterError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("write failed")
	const clean = `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[]}}`

	err := wraparchlint.Convert(strings.NewReader(clean), failingWriter{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want errors.Is(_, %v)", err, wantErr)
	}
}

func TestConvert_MultipleViolations_OneResultPerWarning(t *testing.T) {
	t.Parallel()

	const input = `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[` +
		`{"ComponentName":"a","FileRelativePath":"a.go","ResolvedImportName":"x"},` +
		`{"ComponentName":"b","FileRelativePath":"b.go","ResolvedImportName":"y"},` +
		`{"ComponentName":"c","FileRelativePath":"c.go","ResolvedImportName":"z"}` +
		`]}}`

	var buf bytes.Buffer
	if err := wraparchlint.Convert(strings.NewReader(input), &buf); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output not valid SARIF JSON: %v", err)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("Runs = %d, want 1", len(doc.Runs))
	}
	if got, want := len(doc.Runs[0].Results), 3; got != want {
		t.Fatalf("Results = %d, want %d", got, want)
	}
	wantURIs := []string{"a.go", "b.go", "c.go"}
	for i, r := range doc.Runs[0].Results {
		if got := r.Locations[0].PhysicalLocation.ArtifactLocation.URI; got != wantURIs[i] {
			t.Errorf("Results[%d].URI = %q, want %q", i, got, wantURIs[i])
		}
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '0'
	}
	return len(p), nil
}

type failingWriter struct{ err error }

func (f failingWriter) Write(_ []byte) (int, error) { return 0, f.err }
