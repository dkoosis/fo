package sarif

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Read parses SARIF from an io.Reader.
func Read(r io.Reader) (*Document, error) {
	dec := json.NewDecoder(r)
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode sarif: %w", err)
	}
	// Trailing data is tolerated: golangci-lint v2 appends a text summary
	// after the SARIF JSON document, and the decoder already consumed the
	// complete first JSON value successfully.
	if doc.Version == "" {
		return nil, fmt.Errorf("missing sarif version")
	}
	return &doc, nil
}

// ReadBytes parses SARIF from a byte slice.
func ReadBytes(data []byte) (*Document, error) {
	return Read(bytes.NewReader(data))
}
