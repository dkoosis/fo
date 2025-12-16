// Package nilaway provides parsing and rendering for nilaway JSON output.
// nilaway is Uber's static analyzer for detecting nil pointer dereferences.
package nilaway

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Finding represents a single nilaway finding.
type Finding struct {
	// Position in format "file.go:line:col"
	Posn string `json:"posn"`
	// Description of the nil pointer risk
	Message string `json:"message"`
	// Explanation of why this might be nil
	Reason string `json:"reason"`

	// Parsed fields (not from JSON)
	File   string `json:"-"`
	Line   int    `json:"-"`
	Column int    `json:"-"`
}

// Result represents parsed nilaway output.
type Result struct {
	Findings []Finding
}

// Adapter parses and renders nilaway JSON output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Parse reads nilaway JSON output from a reader.
// Each line is a separate JSON object.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	var findings []Finding
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var f Finding
		if err := json.Unmarshal(line, &f); err != nil {
			// Skip non-JSON lines (might be progress output)
			continue
		}

		// Parse position field
		if f.Posn != "" {
			f.File, f.Line, f.Column = parsePosition(f.Posn)
		}

		findings = append(findings, f)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Result{Findings: findings}, nil
}

// parsePosition extracts file, line, column from "file.go:line:col" format.
func parsePosition(posn string) (file string, line, col int) {
	parts := strings.Split(posn, ":")
	if len(parts) >= 1 {
		file = parts[0]
	}
	if len(parts) >= 2 {
		line, _ = strconv.Atoi(parts[1])
	}
	if len(parts) >= 3 {
		col, _ = strconv.Atoi(parts[2])
	}
	return
}

// ParseBytes parses nilaway JSON output from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	return a.Parse(bytes.NewReader(data))
}

// ParseString parses nilaway JSON output from a string.
func (a *Adapter) ParseString(s string) (*Result, error) {
	return a.Parse(strings.NewReader(s))
}

// Render renders a nilaway result to a string.
func (a *Adapter) Render(result *Result) string {
	patterns := MapToPatterns(result)

	var sb strings.Builder
	for i, p := range patterns {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(p.Render(a.theme))
	}

	return sb.String()
}

// RenderReader reads and renders nilaway JSON output from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// MapToPatterns converts nilaway result to fo patterns.
func MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Summary pattern
	patterns = append(patterns, mapToSummary(r))

	// Findings table if there are any
	if len(r.Findings) > 0 {
		patterns = append(patterns, mapToFindingsTable(r))
	}

	return patterns
}

func mapToSummary(r *Result) *design.Summary {
	if len(r.Findings) == 0 {
		return &design.Summary{
			Label: "Nil Safety",
			Metrics: []design.SummaryItem{
				{Label: "Status", Value: "OK", Type: "success"},
				{Label: "Findings", Value: "None", Type: "info"},
			},
		}
	}

	// Count unique files
	files := make(map[string]bool)
	for _, f := range r.Findings {
		if f.File != "" {
			files[f.File] = true
		}
	}

	return &design.Summary{
		Label: "Nil Safety",
		Metrics: []design.SummaryItem{
			{Label: "Status", Value: "WARN", Type: "warning"},
			{Label: "Findings", Value: fmt.Sprintf("%d", len(r.Findings)), Type: "warning"},
			{Label: "Files", Value: fmt.Sprintf("%d", len(files)), Type: "info"},
		},
	}
}

func mapToFindingsTable(r *Result) *design.TestTable {
	// Group findings by file for better organization
	byFile := make(map[string][]Finding)
	for _, f := range r.Findings {
		file := f.File
		if file == "" {
			file = "unknown"
		}
		byFile[file] = append(byFile[file], f)
	}

	// Sort files for consistent output
	var files []string
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	items := make([]design.TestTableItem, 0, len(r.Findings))
	multipleFiles := len(files) > 1

	for _, file := range files {
		findings := byFile[file]
		for _, f := range findings {
			// Build location string
			var name string
			if multipleFiles {
				name = fmt.Sprintf("%s:%d", filepath.Base(file), f.Line)
			} else {
				name = fmt.Sprintf(":%d", f.Line)
			}

			// Build details - message with optional reason
			details := f.Message
			if f.Reason != "" && f.Reason != f.Message {
				details = fmt.Sprintf("%s (%s)", f.Message, f.Reason)
			}

			items = append(items, design.TestTableItem{
				Name:    name,
				Status:  "skip", // warning indicator
				Details: details,
			})
		}
	}

	density := design.DensityBalanced
	if len(items) > 10 {
		density = design.DensityCompact
	}

	return &design.TestTable{
		Label:   "Nil Pointer Risks",
		Results: items,
		Density: density,
	}
}

// QuickRender renders nilaway JSON output with default theme.
func QuickRender(output string) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(output)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsNilawayOutput detects if the data looks like nilaway JSON output.
// nilaway outputs JSON objects with posn, message, and reason fields.
func IsNilawayOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	if len(lines) == 0 {
		return false
	}

	// Check if at least one line looks like nilaway JSON
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Must start with { to be JSON
		if line[0] != '{' {
			continue
		}

		// Check for nilaway-specific fields
		var obj map[string]interface{}
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}

		// nilaway output has posn and message fields
		_, hasPosn := obj["posn"]
		_, hasMessage := obj["message"]
		if hasPosn && hasMessage {
			return true
		}
	}

	return false
}
