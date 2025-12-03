// Package adapter provides stream adapters for parsing structured command output
// into rich visualization patterns.
package adapter

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// HousekeepingAdapter parses housekeeping check JSON into Housekeeping patterns.
//
// Expected JSON format:
//
//	{
//	  "checks": [
//	    {
//	      "name": "markdown_count",
//	      "status": "warn",
//	      "current": 62,
//	      "threshold": 50,
//	      "details": "",
//	      "items": []
//	    },
//	    {
//	      "name": "todo_comments",
//	      "status": "warn",
//	      "current": 23,
//	      "threshold": 0,
//	      "details": "7 older than 90 days",
//	      "items": ["pkg/server/handler.go:42", "pkg/client/api.go:156"]
//	    }
//	  ]
//	}
type HousekeepingAdapter struct{}

// Name returns the adapter name.
func (a *HousekeepingAdapter) Name() string {
	return "housekeeping"
}

// Detect checks if the output is housekeeping JSON format.
func (a *HousekeepingAdapter) Detect(firstLines []string) bool {
	if len(firstLines) == 0 {
		return false
	}

	combined := strings.Join(firstLines, " ")

	// Housekeeping JSON has characteristic fields
	hasChecks := strings.Contains(combined, `"checks"`)
	hasStatus := strings.Contains(combined, `"status"`)
	hasThreshold := strings.Contains(combined, `"threshold"`) ||
		strings.Contains(combined, `"current"`)

	// Also detect by check names
	hasCheckNames := strings.Contains(combined, `"markdown_count"`) ||
		strings.Contains(combined, `"todo_comments"`) ||
		strings.Contains(combined, `"orphan_tests"`) ||
		strings.Contains(combined, `"package_docs"`) ||
		strings.Contains(combined, `"dead_code"`)

	return hasChecks && (hasStatus || hasThreshold || hasCheckNames)
}

// housekeepingJSON represents the expected JSON structure.
type housekeepingJSON struct {
	Title  string `json:"title"`
	Checks []struct {
		Name      string   `json:"name"`
		Status    string   `json:"status"`
		Current   int      `json:"current"`
		Threshold int      `json:"threshold"`
		Details   string   `json:"details"`
		Items     []string `json:"items"`
	} `json:"checks"`
}

// Parse converts housekeeping JSON into a Housekeeping pattern.
func (a *HousekeepingAdapter) Parse(output io.Reader) (design.Pattern, error) {
	// Read all content
	var buf strings.Builder
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		buf.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var data housekeepingJSON
	if err := json.Unmarshal([]byte(buf.String()), &data); err != nil {
		return nil, err
	}

	// Build the pattern
	housekeeping := &design.Housekeeping{
		Title: data.Title,
	}

	if housekeeping.Title == "" {
		housekeeping.Title = "HOUSEKEEPING"
	}

	// Convert checks
	for _, c := range data.Checks {
		check := design.HousekeepingCheck{
			Name:      formatCheckName(c.Name),
			Status:    c.Status,
			Current:   c.Current,
			Threshold: c.Threshold,
			Details:   c.Details,
			Items:     c.Items,
		}

		// Validate status
		if check.Status == "" {
			check.Status = "pass"
		}

		housekeeping.Checks = append(housekeeping.Checks, check)
	}

	return housekeeping, nil
}

// formatCheckName converts snake_case check names to human-readable format.
func formatCheckName(name string) string {
	// Map of known check names to friendly labels
	nameMap := map[string]string{
		"markdown_count":      "Markdown files",
		"todo_comments":       "TODO comments",
		"orphan_tests":        "Orphan test files",
		"package_docs":        "Package documentation",
		"dead_code":           "Dead code",
		"deprecated_deps":     "Deprecated dependencies",
		"license_headers":     "License headers",
		"generated_freshness": "Generated file freshness",
	}

	if friendly, ok := nameMap[name]; ok {
		return friendly
	}

	// Default: convert snake_case to Title Case
	parts := strings.Split(name, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}
