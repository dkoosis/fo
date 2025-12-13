// Package gofmt provides parsing and rendering for gofmt -l output.
package gofmt

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Result represents parsed gofmt -l output.
type Result struct {
	// Files lists files that need formatting.
	Files []string
}

// Adapter parses and renders gofmt -l output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Parse reads gofmt -l output from a reader.
// Each line is a file path that needs formatting.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	var files []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Result{Files: files}, nil
}

// ParseString parses gofmt -l output from a string.
func (a *Adapter) ParseString(s string) (*Result, error) {
	return a.Parse(strings.NewReader(s))
}

// Render renders a gofmt result to a string.
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

// RenderReader reads and renders gofmt -l output from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// MapToPatterns converts gofmt result to fo patterns.
func MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Summary pattern
	patterns = append(patterns, mapToSummary(r))

	// File list if there are any
	if len(r.Files) > 0 {
		patterns = append(patterns, mapToFileList(r))
	}

	return patterns
}

func mapToSummary(r *Result) *design.Summary {
	if len(r.Files) == 0 {
		return &design.Summary{
			Label: "Format Check",
			Metrics: []design.SummaryItem{
				{Label: "Status", Value: "OK", Type: "success"},
				{Label: "Files", Value: "All formatted", Type: "info"},
			},
		}
	}

	return &design.Summary{
		Label: "Format Check",
		Metrics: []design.SummaryItem{
			{Label: "Status", Value: "FAIL", Type: "error"},
			{Label: "Needs Formatting", Value: fmt.Sprintf("%d", len(r.Files)), Type: "warning"},
		},
	}
}

func mapToFileList(r *Result) *design.TestTable {
	items := make([]design.TestTableItem, 0, len(r.Files))

	// Group by directory for better organization
	byDir := make(map[string][]string)
	for _, f := range r.Files {
		dir := filepath.Dir(f)
		byDir[dir] = append(byDir[dir], filepath.Base(f))
	}

	for _, f := range r.Files {
		dir := filepath.Dir(f)
		base := filepath.Base(f)

		// Show directory context if multiple dirs
		name := base
		if len(byDir) > 1 {
			name = filepath.Join(filepath.Base(dir), base)
		}

		items = append(items, design.TestTableItem{
			Name:    name,
			Status:  "skip", // warning icon
			Details: "needs gofmt",
		})
	}

	density := design.DensityBalanced
	if len(items) > 10 {
		density = design.DensityCompact
	}

	return &design.TestTable{
		Label:   "Files Needing Format",
		Results: items,
		Density: density,
	}
}

// QuickRender renders gofmt -l output with default theme.
func QuickRender(output string) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(output)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsGofmtOutput detects if the data looks like gofmt -l output.
// gofmt -l outputs one file path per line, all ending in .go
func IsGofmtOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return false
	}

	// Check if all non-empty lines look like .go file paths
	goFiles := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasSuffix(line, ".go") {
			return false
		}
		goFiles++
	}

	return goFiles > 0
}
