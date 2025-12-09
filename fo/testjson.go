// Package fo provides go test -json parsing and rendering.
package fo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// TestEvent represents a single event from go test -json output.
type TestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Elapsed float64   `json:"Elapsed"`
	Output  string    `json:"Output"`
}

// TestPackageAggregator aggregates test events by package.
type TestPackageAggregator struct {
	packages map[string]*packageState
	order    []string // preserve order
}

type packageState struct {
	name        string
	passed      int
	failed      int
	skipped     int
	duration    time.Duration
	coverage    float64
	failedTests []string
	allTests    []TestResult
}

// NewTestPackageAggregator creates a new aggregator.
func NewTestPackageAggregator() *TestPackageAggregator {
	return &TestPackageAggregator{
		packages: make(map[string]*packageState),
	}
}

// ParseTestJSON parses go test -json output and returns aggregated results.
func ParseTestJSON(data []byte) ([]TestPackageResult, error) {
	agg := NewTestPackageAggregator()

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event TestEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue // Skip malformed lines
		}

		agg.processEvent(event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning test output: %w", err)
	}

	return agg.Results(), nil
}

func (a *TestPackageAggregator) processEvent(event TestEvent) {
	pkg := a.getOrCreatePackage(event.Package)

	switch event.Action {
	case "pass":
		if event.Test != "" {
			pkg.passed++
			pkg.allTests = append(pkg.allTests, TestResult{
				Name:   event.Test,
				Status: "PASS",
			})
		} else {
			// Package completed
			pkg.duration = time.Duration(event.Elapsed * float64(time.Second))
		}

	case "fail":
		if event.Test != "" {
			pkg.failed++
			pkg.failedTests = append(pkg.failedTests, event.Test)
			pkg.allTests = append(pkg.allTests, TestResult{
				Name:   event.Test,
				Status: "FAIL",
			})
		} else {
			// Package completed
			pkg.duration = time.Duration(event.Elapsed * float64(time.Second))
		}

	case "skip":
		if event.Test != "" {
			pkg.skipped++
			pkg.allTests = append(pkg.allTests, TestResult{
				Name:   event.Test,
				Status: "SKIP",
			})
		}

	case "output":
		// Parse coverage from output
		if strings.Contains(event.Output, "coverage:") && strings.Contains(event.Output, "% of statements") {
			var cov float64
			fmt.Sscanf(event.Output, "coverage: %f%% of statements", &cov)
			if cov > 0 {
				pkg.coverage = cov
			}
		}
	}
}

func (a *TestPackageAggregator) getOrCreatePackage(name string) *packageState {
	if pkg, ok := a.packages[name]; ok {
		return pkg
	}
	pkg := &packageState{name: name}
	a.packages[name] = pkg
	a.order = append(a.order, name)
	return pkg
}

// Results returns the aggregated test results.
func (a *TestPackageAggregator) Results() []TestPackageResult {
	var results []TestPackageResult
	for _, name := range a.order {
		pkg := a.packages[name]
		// Skip packages with no tests
		if pkg.passed == 0 && pkg.failed == 0 && pkg.skipped == 0 {
			continue
		}
		results = append(results, TestPackageResult{
			Name:        pkg.name,
			Passed:      pkg.passed,
			Failed:      pkg.failed,
			Skipped:     pkg.skipped,
			Duration:    pkg.duration,
			Coverage:    pkg.coverage,
			FailedTests: pkg.failedTests,
			AllTests:    pkg.allTests,
		})
	}
	return results
}

// IsGoTestJSON checks if data looks like go test -json output.
// It checks for NDJSON with TestEvent-like structure.
func IsGoTestJSON(data []byte) bool {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return false
	}

	// Must start with '{'
	if data[0] != '{' {
		return false
	}

	// Find first complete line
	idx := bytes.IndexByte(data, '\n')
	if idx == -1 {
		idx = len(data)
	}

	firstLine := data[:idx]

	// Try to parse as TestEvent
	var event TestEvent
	if err := json.Unmarshal(firstLine, &event); err != nil {
		return false
	}

	// Check for go test -json specific fields
	// Action is required, and must be one of the known actions
	validActions := map[string]bool{
		"start": true, "run": true, "pause": true, "cont": true,
		"pass": true, "bench": true, "fail": true, "output": true, "skip": true,
	}

	return validActions[event.Action]
}

// RenderTestResults renders test results to a writer using the TestRenderer.
func RenderTestResults(w io.Writer, results []TestPackageResult, console *Console) {
	if len(results) == 0 {
		return
	}

	renderer := NewTestRenderer(console, w)

	// Group packages by top-level directory
	groups := make(map[string][]TestPackageResult)
	var groupOrder []string

	for _, pkg := range results {
		dir := getTopLevelDir(pkg.Name)
		if _, exists := groups[dir]; !exists {
			groupOrder = append(groupOrder, dir)
		}
		groups[dir] = append(groups[dir], pkg)
	}

	// Sort groups
	sort.Strings(groupOrder)

	// Sort packages within groups
	for _, pkgs := range groups {
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].Name < pkgs[j].Name
		})
	}

	// Render each group
	for _, dir := range groupOrder {
		pkgs := groups[dir]
		renderer.RenderGroupHeader(dir)
		for _, pkg := range pkgs {
			renderer.RenderPackageLine(TestPackageResult{
				Name:        getPackageBaseName(pkg.Name),
				Passed:      pkg.Passed,
				Failed:      pkg.Failed,
				Skipped:     pkg.Skipped,
				Duration:    pkg.Duration,
				Coverage:    pkg.Coverage,
				FailedTests: pkg.FailedTests,
				AllTests:    pkg.AllTests,
			})
		}
		renderer.RenderGroupFooter()
	}

	renderer.RenderAll()
}

func getTopLevelDir(name string) string {
	if strings.Contains(name, "/internal/") {
		return "internal"
	}
	if strings.Contains(name, "/cmd/") {
		return "cmd"
	}
	if strings.Contains(name, "/examples/") {
		return "examples"
	}
	if strings.Contains(name, "/pkg/") {
		return "pkg"
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func getPackageBaseName(name string) string {
	if idx := strings.Index(name, "/internal/"); idx != -1 {
		return name[idx+10:]
	}
	if idx := strings.Index(name, "/cmd/"); idx != -1 {
		return name[idx+5:]
	}
	if idx := strings.Index(name, "/examples/"); idx != -1 {
		return name[idx+10:]
	}
	if idx := strings.Index(name, "/pkg/"); idx != -1 {
		return name[idx+5:]
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}
