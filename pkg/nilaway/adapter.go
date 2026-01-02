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

// AnalyzerResult represents the nilaway analyzer output within a package.
type AnalyzerResult struct {
	Nilaway []Finding `json:"nilaway"`
}

// Parse reads nilaway JSON output from a reader.
// nilaway -json outputs a nested structure:
//
//	{"package.path": {"nilaway": [{"posn":"...", "message":"..."}]}}
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	findings, err := parseNestedJSON(data)
	if err != nil {
		return nil, err
	}

	return &Result{Findings: findings}, nil
}

// parseNestedJSON parses the nested nilaway JSON format.
func parseNestedJSON(data []byte) ([]Finding, error) {
	var findings []Finding

	// Try nested format first: {"pkg": {"nilaway": [...]}}
	var nested map[string]AnalyzerResult
	if err := json.Unmarshal(data, &nested); err == nil {
		for _, ar := range nested {
			for _, f := range ar.Nilaway {
				if f.Posn != "" {
					f.File, f.Line, f.Column = parsePosition(f.Posn)
				}
				findings = append(findings, f)
			}
		}
		if len(findings) > 0 {
			return findings, nil
		}
	}

	// Fallback: try NDJSON format (one JSON object per line)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var f Finding
		if err := json.Unmarshal(line, &f); err != nil {
			continue
		}

		if f.Posn != "" {
			f.File, f.Line, f.Column = parsePosition(f.Posn)
		}
		findings = append(findings, f)
	}

	return findings, scanner.Err()
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
	return file, line, col
}

// ParseBytes parses nilaway JSON output from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	findings, err := parseNestedJSON(data)
	if err != nil {
		return nil, err
	}
	return &Result{Findings: findings}, nil
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
// nilaway -json outputs nested structure: {"pkg": {"nilaway": [...]}}
func IsNilawayOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return false
	}

	// Try nested format: {"pkg.path": {"nilaway": [{"posn":"...", "message":"..."}]}}
	var nested map[string]AnalyzerResult
	if err := json.Unmarshal(data, &nested); err == nil {
		for _, ar := range nested {
			if len(ar.Nilaway) > 0 {
				// Verify it has posn field (nilaway-specific)
				for _, f := range ar.Nilaway {
					if f.Posn != "" {
						return true
					}
				}
			}
		}
	}

	// Fallback: check for NDJSON format with posn/message fields
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(line, &obj); err != nil {
			continue
		}

		_, hasPosn := obj["posn"]
		_, hasMessage := obj["message"]
		if hasPosn && hasMessage {
			return true
		}
	}

	return false
}
