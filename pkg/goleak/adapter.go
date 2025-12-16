// Package goleak provides parsing and rendering for goleak goroutine leak detector output.
// goleak is Uber's tool for detecting goroutine leaks in tests.
package goleak

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Goroutine represents a leaked goroutine.
type Goroutine struct {
	ID          int
	State       string
	TopFunction string
	CreatedBy   string
	CreatedAt   string // file:line
	Stack       []string
}

// Result represents parsed goleak output.
type Result struct {
	Goroutines []Goroutine
}

// Adapter parses and renders goleak output.
type Adapter struct {
	theme *design.Config
}

// NewAdapter creates a new adapter with the given theme.
func NewAdapter(theme *design.Config) *Adapter {
	return &Adapter{theme: theme}
}

// Patterns for parsing goroutine info
var (
	// Matches "goroutine 42 [chan receive]:" or similar
	goroutineHeaderRe = regexp.MustCompile(`goroutine\s+(\d+)\s+\[([^\]]+)\]:?`)
	// Matches "created by package.Function"
	createdByRe = regexp.MustCompile(`created by\s+(.+)`)
	// Matches file:line in stack traces like "    /path/to/file.go:123 +0x1a4"
	fileLineRe = regexp.MustCompile(`^\s+(\S+\.go:\d+)`)
	// Matches function calls like "github.com/foo/bar.(*Client).readLoop(...)"
	funcCallRe = regexp.MustCompile(`^(\S+)\(`)
)

// Parse reads goleak output from a reader.
func (a *Adapter) Parse(r io.Reader) (*Result, error) {
	var goroutines []Goroutine
	scanner := bufio.NewScanner(r)

	var current *Goroutine
	var inGoroutine bool
	var seenCreatedBy bool

	for scanner.Scan() {
		line := scanner.Text()

		// Check for goroutine header
		if matches := goroutineHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous goroutine if any
			if current != nil {
				goroutines = append(goroutines, *current)
			}

			id, _ := strconv.Atoi(matches[1])
			current = &Goroutine{
				ID:    id,
				State: matches[2],
			}
			inGoroutine = true
			seenCreatedBy = false
			continue
		}

		if !inGoroutine || current == nil {
			continue
		}

		// Check for "created by" line
		if matches := createdByRe.FindStringSubmatch(line); matches != nil {
			current.CreatedBy = matches[1]
			seenCreatedBy = true
			continue
		}

		// If we just saw "created by", the next line with file:line is the creation point
		if seenCreatedBy {
			if matches := fileLineRe.FindStringSubmatch(line); matches != nil {
				current.CreatedAt = matches[1]
			}
			seenCreatedBy = false
			continue
		}

		// Extract top function from first function call after header
		if current.TopFunction == "" {
			trimmed := strings.TrimSpace(line)
			if matches := funcCallRe.FindStringSubmatch(trimmed); matches != nil {
				current.TopFunction = matches[1]
			}
		}

		// Add to stack trace
		if strings.TrimSpace(line) != "" {
			current.Stack = append(current.Stack, line)
		}
	}

	// Don't forget the last goroutine
	if current != nil {
		goroutines = append(goroutines, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Result{Goroutines: goroutines}, nil
}

// ParseBytes parses goleak output from bytes.
func (a *Adapter) ParseBytes(data []byte) (*Result, error) {
	return a.Parse(bytes.NewReader(data))
}

// ParseString parses goleak output from a string.
func (a *Adapter) ParseString(s string) (*Result, error) {
	return a.Parse(strings.NewReader(s))
}

// Render renders a goleak result to a string.
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

// RenderReader reads and renders goleak output from a reader.
func (a *Adapter) RenderReader(r io.Reader) (string, error) {
	result, err := a.Parse(r)
	if err != nil {
		return "", err
	}
	return a.Render(result), nil
}

// MapToPatterns converts goleak result to fo patterns.
func MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Summary pattern
	patterns = append(patterns, mapToSummary(r))

	// Goroutine table if there are any
	if len(r.Goroutines) > 0 {
		patterns = append(patterns, mapToGoroutineTable(r))
	}

	return patterns
}

func mapToSummary(r *Result) *design.Summary {
	if len(r.Goroutines) == 0 {
		return &design.Summary{
			Label: "Goroutine Leaks",
			Metrics: []design.SummaryItem{
				{Label: "Status", Value: "OK", Type: "success"},
				{Label: "Leaks", Value: "None", Type: "info"},
			},
		}
	}

	// Group by state
	states := make(map[string]int)
	for _, g := range r.Goroutines {
		states[g.State]++
	}

	return &design.Summary{
		Label: "Goroutine Leaks",
		Metrics: []design.SummaryItem{
			{Label: "Status", Value: "FAIL", Type: "error"},
			{Label: "Leaked", Value: fmt.Sprintf("%d", len(r.Goroutines)), Type: "error"},
			{Label: "States", Value: fmt.Sprintf("%d", len(states)), Type: "info"},
		},
	}
}

func mapToGoroutineTable(r *Result) *design.TestTable {
	items := make([]design.TestTableItem, 0, len(r.Goroutines))

	for _, g := range r.Goroutines {
		// Build name from goroutine ID and state
		name := fmt.Sprintf("goroutine %d", g.ID)

		// Build details from top function and creation point
		var details string
		if g.TopFunction != "" {
			// Shorten function name for display
			fn := g.TopFunction
			if idx := strings.LastIndex(fn, "/"); idx != -1 {
				fn = fn[idx+1:]
			}
			details = fn
		}
		if g.CreatedBy != "" {
			createdBy := g.CreatedBy
			if idx := strings.LastIndex(createdBy, "/"); idx != -1 {
				createdBy = createdBy[idx+1:]
			}
			if details != "" {
				details += " <- " + createdBy
			} else {
				details = "created by " + createdBy
			}
		}
		if g.State != "" && details != "" {
			details = "[" + g.State + "] " + details
		}

		items = append(items, design.TestTableItem{
			Name:    name,
			Status:  "fail",
			Details: details,
		})
	}

	density := design.DensityBalanced
	if len(items) > 10 {
		density = design.DensityCompact
	}

	return &design.TestTable{
		Label:   "Leaked Goroutines",
		Results: items,
		Density: density,
	}
}

// QuickRender renders goleak output with default theme.
func QuickRender(output string) (string, error) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(output)
	if err != nil {
		return "", err
	}
	return adapter.Render(result), nil
}

// IsGoleakOutput detects if the data looks like goleak output.
// goleak outputs contain "found unexpected goroutines" or goroutine stack traces.
func IsGoleakOutput(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	s := string(data)

	// Primary indicator: goleak's specific error message
	if strings.Contains(s, "found unexpected goroutines") {
		return true
	}

	// Secondary: look for goroutine leak patterns
	// Must have goroutine headers and typically "created by" lines
	hasGoroutine := goroutineHeaderRe.MatchString(s)
	hasCreatedBy := strings.Contains(s, "created by ")

	return hasGoroutine && hasCreatedBy
}
