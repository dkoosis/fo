package check

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// SchemaID is the identifier for lintkit-check format.
const SchemaID = "lintkit-check"

// ReadFile parses a lintkit-check file from disk.
func ReadFile(path string) (*Report, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open check file: %w", err)
	}
	defer f.Close()

	return Read(f)
}

// Read parses a lintkit-check report from an io.Reader.
func Read(r io.Reader) (*Report, error) {
	var report Report
	if err := json.NewDecoder(r).Decode(&report); err != nil {
		return nil, fmt.Errorf("decode check report: %w", err)
	}
	return validateReport(&report)
}

// ReadBytes parses a lintkit-check report from a byte slice.
func ReadBytes(data []byte) (*Report, error) {
	return Read(bytes.NewReader(data))
}

func validateReport(report *Report) (*Report, error) {
	if report.Schema != SchemaID {
		return nil, fmt.Errorf("invalid schema: expected %q, got %q", SchemaID, report.Schema)
	}
	return report, nil
}

// IsCheck checks if data looks like a lintkit-check document.
// Returns true if the $schema field equals "lintkit-check".
func IsCheck(data []byte) bool {
	// Trim whitespace and ensure we have JSON
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return false
	}

	// Find the end of the JSON object by counting braces
	// (same approach as SARIF detection for tools that append text)
	depth := 0
	jsonEnd := -1
	for i, b := range data {
		if b == '{' {
			depth++
		} else if b == '}' {
			depth--
			if depth == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if jsonEnd <= 0 {
		return false
	}

	// Parse just the JSON portion to check the schema field
	var probe struct {
		Schema string `json:"$schema"`
	}
	if err := json.Unmarshal(data[:jsonEnd], &probe); err != nil {
		return false
	}
	return probe.Schema == SchemaID
}

// ExtractCheck extracts the lintkit-check JSON from data that may have trailing text.
func ExtractCheck(data []byte) []byte {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	// Find the end of the JSON object
	depth := 0
	for i, b := range data {
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[:i+1]
			}
		}
	}
	return nil
}

// Stats aggregates statistics from a check report.
type Stats struct {
	TotalItems   int
	ErrorCount   int
	WarningCount int
	InfoCount    int
	BySeverity   map[string]int
}

// ComputeStats calculates aggregate statistics from a check report.
func ComputeStats(report *Report) Stats {
	stats := Stats{
		BySeverity: make(map[string]int),
	}

	for _, item := range report.Items {
		stats.TotalItems++
		stats.BySeverity[item.Severity]++

		switch item.Severity {
		case SeverityError:
			stats.ErrorCount++
		case SeverityWarning:
			stats.WarningCount++
		case SeverityInfo:
			stats.InfoCount++
		}
	}

	return stats
}
