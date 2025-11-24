// Package adapter provides stream adapters for parsing structured command output
// into rich visualization patterns.
package adapter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// StreamAdapter detects and parses structured output formats into patterns.
// This is used for tools that emit structured output (JSON, etc.) that can be
// parsed into rich visualization patterns like TestTable, Inventory, etc.
type StreamAdapter interface {
	// Detect returns true if the adapter can parse the given output format.
	// firstLines contains the first few lines of output for format detection.
	Detect(firstLines []string) bool

	// Parse reads the output stream and converts it to a Pattern.
	// Returns nil if the output doesn't match the expected format.
	Parse(output io.Reader) (design.Pattern, error)

	// Name returns the adapter name for logging/debugging.
	Name() string
}

// Registry holds all registered stream adapters.
type Registry struct {
	adapters []StreamAdapter
}

// NewRegistry creates a new adapter registry with default adapters.
func NewRegistry() *Registry {
	r := &Registry{}
	// Register built-in adapters
	r.Register(&GoTestJSONAdapter{})
	return r
}

// Register adds a stream adapter to the registry.
func (r *Registry) Register(adapter StreamAdapter) {
	r.adapters = append(r.adapters, adapter)
}

// Detect finds an adapter that can handle the given output.
// Returns nil if no adapter matches.
func (r *Registry) Detect(firstLines []string) StreamAdapter {
	for _, adapter := range r.adapters {
		if adapter.Detect(firstLines) {
			return adapter
		}
	}
	return nil
}

// GoTestJSONAdapter parses Go test JSON output into TestTable and Summary patterns.
//
// Go test JSON format:
//
//	{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}
//	{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Test":"TestFoo","Elapsed":0.1}
type GoTestJSONAdapter struct{}

// Name returns the adapter name.
func (a *GoTestJSONAdapter) Name() string {
	return "go-test-json"
}

// Detect checks if the output is Go test JSON format.
func (a *GoTestJSONAdapter) Detect(firstLines []string) bool {
	if len(firstLines) == 0 {
		return false
	}

	// Check if first non-empty line looks like Go test JSON
	for _, line := range firstLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Go test JSON has "Action" and either "Package" or "Test" fields
		if strings.Contains(line, `"Action"`) &&
			(strings.Contains(line, `"Package"`) || strings.Contains(line, `"Test"`)) {
			return true
		}
		// If we see a non-JSON line early, it's probably not pure JSON output
		if !strings.HasPrefix(line, "{") {
			return false
		}
	}
	return false
}

// Parse converts Go test JSON output into a TestTable pattern.
func (a *GoTestJSONAdapter) Parse(output io.Reader) (design.Pattern, error) {
	scanner := bufio.NewScanner(output)

	// Track package results
	type packageResult struct {
		name     string
		status   string
		duration float64
		tests    int
		failed   int
		skipped  int
	}
	packages := make(map[string]*packageResult)
	packageOrder := []string{} // Preserve order

	// JSON event structure for Go test output
	type testEvent struct {
		Package string  `json:"Package"`
		Test    string  `json:"Test"`
		Action  string  `json:"Action"`
		Elapsed float64 `json:"Elapsed"`
		Output  string  `json:"Output"`
	}

	for scanner.Scan() {
		line := scanner.Text()
		var event testEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // Skip non-JSON lines
		}

		// Track package if we haven't seen it
		if event.Package != "" {
			if _, exists := packages[event.Package]; !exists {
				packages[event.Package] = &packageResult{
					name:   event.Package,
					status: "pass",
				}
				packageOrder = append(packageOrder, event.Package)
			}
		}

		pkg := packages[event.Package]
		if pkg == nil {
			continue
		}

		switch event.Action {
		case "run":
			if event.Test != "" {
				pkg.tests++
			}
		case "pass":
			if event.Test == "" {
				// Package-level pass
				pkg.duration = event.Elapsed
			}
		case "fail":
			if event.Test == "" {
				// Package-level fail
				pkg.status = "fail"
				pkg.duration = event.Elapsed
			} else {
				// Test-level fail
				pkg.failed++
				pkg.status = "fail"
			}
		case "skip":
			if event.Test != "" {
				pkg.skipped++
				if pkg.status == "pass" {
					pkg.status = "skip"
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Convert to TestTable
	results := make([]design.TestTableItem, 0, len(packageOrder))

	for _, pkgName := range packageOrder {
		pkg := packages[pkgName]

		// Format duration
		duration := formatDuration(pkg.duration)

		// Create table item
		item := design.TestTableItem{
			Name:     pkg.name,
			Status:   pkg.status,
			Duration: duration,
			Count:    pkg.tests,
		}

		// Add details for failed packages
		if pkg.failed > 0 {
			item.Details = fmt.Sprintf("%d test(s) failed", pkg.failed)
		} else if pkg.skipped > 0 && pkg.status == "skip" {
			item.Details = fmt.Sprintf("%d test(s) skipped", pkg.skipped)
		}

		results = append(results, item)
	}

	return &design.TestTable{
		Label:   "Go Test Results",
		Results: results,
	}, nil
}

// formatDuration converts seconds to a human-readable duration string.
func formatDuration(seconds float64) string {
	if seconds < 0.001 {
		return "0.0s"
	}
	if seconds < 1 {
		return fmt.Sprintf("%.0fms", seconds*1000)
	}
	return fmt.Sprintf("%.1fs", seconds)
}
