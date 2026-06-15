package sarif

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ErrMissingSARIFVersion is returned when a decoded document has no version field.
var ErrMissingSARIFVersion = errors.New("missing sarif version")

// ErrNestingTooDeep is returned when a SARIF document nests objects/arrays
// past maxNestingDepth. encoding/json's Decode is recursive and a
// pathological input (e.g. a 1 MiB run of "[[[[…") can overflow the stack
// before any useful work happens. The guard runs first via a token walk
// (json.Decoder.Token is iterative, so it measures depth without
// recursing) and aborts before Decode is reached.
var ErrNestingTooDeep = errors.New("sarif nesting too deep")

// maxNestingDepth bounds object/array nesting. Real SARIF is shallow — the
// deepest path (run → results → locations → … → region) is well under 20.
// 256 leaves generous headroom while still stopping a depth-bomb.
const maxNestingDepth = 256

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
		return nil, ErrMissingSARIFVersion
	}
	return &doc, nil
}

// ReadBytes parses SARIF from a byte slice. It runs a depth guard before
// decoding so a depth-bomb cannot overflow the stack in Decode.
func ReadBytes(data []byte) (*Document, error) {
	if err := checkDepth(data); err != nil {
		return nil, err
	}
	return Read(bytes.NewReader(data))
}

// checkDepth walks the JSON token stream and returns ErrNestingTooDeep if
// object/array nesting exceeds maxNestingDepth. Token() is iterative, so it
// is itself safe on pathologically deep input. It returns nil on any token
// error (EOF, malformed) — the real Decode then surfaces the actual parse
// failure with its own message. The walk stops once the root value closes,
// so trailing data (golangci-lint v2 appends a text summary) is ignored,
// matching Read's trailing-data tolerance.
func checkDepth(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	depth, entered := 0, false
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			entered = true
			depth++
			if depth > maxNestingDepth {
				return fmt.Errorf("%w: exceeds %d", ErrNestingTooDeep, maxNestingDepth)
			}
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if entered && depth == 0 {
			return nil
		}
	}
}
