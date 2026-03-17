// Package detect sniffs stdin to determine the input format.
package detect

import (
	"bytes"
	"encoding/json"

	"github.com/dkoosis/fo/internal/report"
)

// Format represents a recognized input format.
type Format int

const (
	Unknown    Format = iota
	SARIF             // SARIF 2.1.0 JSON document
	GoTestJSON        // go test -json NDJSON stream
	Report            // Delimited multi-tool report
)

// Sniff examines the first bytes of input to determine format.
// Returns the detected format. Input must contain at least the first line.
func Sniff(data []byte) Format {
	data = bytes.TrimLeft(data, " \t\n\r")
	if len(data) == 0 {
		return Unknown
	}

	// Check for report delimiter before requiring '{' — reports start with '---'
	if firstLine := extractFirstLine(data); report.IsDelimiter(firstLine) {
		return Report
	}

	// Must start with '{' for SARIF or go test -json
	if data[0] != '{' {
		return Unknown
	}

	// Try SARIF detection first — look for "version" field with SARIF-like value
	// SARIF is a complete JSON document; go test -json is NDJSON (one object per line)
	if isSARIF(data) {
		return SARIF
	}

	if isGoTestJSON(data) {
		return GoTestJSON
	}

	return Unknown
}

func isSARIF(data []byte) bool {
	var probe struct {
		Version string             `json:"version"`
		Schema  string             `json:"$schema"`
		Runs    []json.RawMessage  `json:"runs"`
	}
	// Use Decoder instead of Unmarshal to tolerate trailing text
	// (golangci-lint v2 appends a text summary after the SARIF JSON).
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&probe); err != nil {
		return false
	}
	// SARIF version is "2.1.0" and has runs array
	return probe.Version != "" && probe.Runs != nil
}

func isGoTestJSON(data []byte) bool {
	var event struct {
		Action  string `json:"Action"`
		Package string `json:"Package"`
	}
	if err := json.Unmarshal(extractFirstLine(data), &event); err != nil {
		return false
	}

	switch event.Action {
	case "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip":
		return true
	default:
		return false
	}
}

func extractFirstLine(data []byte) []byte {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return data[:i]
		}
	}
	return data
}
