// Package detect sniffs stdin to determine the input format.
package detect

import (
	"encoding/json"
)

// Format represents a recognized input format.
type Format int

const (
	Unknown    Format = iota
	SARIF             // SARIF 2.1.0 JSON document
	GoTestJSON        // go test -json NDJSON stream
)

// Sniff examines the first bytes of input to determine format.
// Returns the detected format. Input must contain at least the first line.
func Sniff(data []byte) Format {
	// Trim leading whitespace
	for len(data) > 0 && (data[0] == ' ' || data[0] == '\t' || data[0] == '\n' || data[0] == '\r') {
		data = data[1:]
	}
	if len(data) == 0 {
		return Unknown
	}

	// Must start with '{' for either format
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
		Version string `json:"version"`
		Schema  string `json:"$schema"`
		Runs    []json.RawMessage `json:"runs"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return false
	}
	// SARIF version is "2.1.0" and has runs array
	return probe.Version != "" && probe.Runs != nil
}

func isGoTestJSON(data []byte) bool {
	// Find first complete line
	end := 0
	for end < len(data) && data[end] != '\n' {
		end++
	}
	firstLine := data[:end]

	var event struct {
		Action  string `json:"Action"`
		Package string `json:"Package"`
	}
	if err := json.Unmarshal(firstLine, &event); err != nil {
		return false
	}

	validActions := map[string]bool{
		"start": true, "run": true, "pause": true, "cont": true,
		"pass": true, "bench": true, "fail": true, "output": true, "skip": true,
	}
	return validActions[event.Action]
}
