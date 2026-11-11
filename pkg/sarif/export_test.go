package sarif

import (
	"fmt"
	"io"
)

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

// Document returns the constructed SARIF document without validation.
// Use WriteTo for production output — it validates driver name and levels.
// This method is the "I know what I'm doing" escape hatch for tests and inspection.
func (b *Builder) Document() *Document {
	return b.doc
}

// Read parses SARIF from an io.Reader. It fully buffers r and routes
// through the same depth guard ReadBytes runs (fo-n25.7) — without this,
// any caller of the exported Reader-based API would inherit the
// depth-bomb exposure ReadBytes was hardened against for #269.
func Read(r io.Reader) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read sarif: %w", err)
	}
	return ReadBytes(data)
}
