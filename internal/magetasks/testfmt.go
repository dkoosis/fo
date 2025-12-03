package magetasks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dkoosis/fo/fo"
	"golang.org/x/term"
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

// PackageResult holds aggregated test results for a package.
type PackageResult struct {
	Name        string
	Passed      int
	Failed      int
	Skipped     int
	Duration    time.Duration
	Coverage    float64
	FailedTests []string
	AllTests    []fo.TestResult
	CurrentTest string
	StartTime   time.Time
}

// TestFormatter processes and renders test output.
type TestFormatter struct {
	packages   map[string]*PackageResult
	completed  []*PackageResult
	writer     *os.File
	isTerminal bool
	console    *fo.Console
	renderer   *fo.TestRenderer
}

// NewTestFormatter creates a test formatter.
func NewTestFormatter(w *os.File, isTerminal bool) *TestFormatter {
	projectCfg := fo.LoadProjectConfig()
	console := fo.NewConsole(fo.ConsoleConfig{ThemeName: projectCfg.Theme})
	renderer := fo.NewTestRenderer(console, w)
	return &TestFormatter{
		packages:   make(map[string]*PackageResult),
		writer:     w,
		isTerminal: isTerminal,
		console:    console,
		renderer:   renderer,
	}
}

// RunTests executes go test with JSON output and formats results.
func (f *TestFormatter) RunTests(args []string) error {
	// Build command with -json flag
	cmdArgs := []string{"test", "-json"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tests: %w", err)
	}

	// Process JSON events
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var event TestEvent
		if err = json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue // Skip malformed lines
		}
		f.processEvent(event)
	}

	// Wait for command to finish
	err = cmd.Wait()

	// Render final summary
	f.renderFinalSummary()

	return err
}

func (f *TestFormatter) processEvent(event TestEvent) {
	pkg := f.getOrCreatePackage(event.Package)

	switch event.Action {
	case "run":
		if event.Test != "" {
			pkg.CurrentTest = event.Test
		} else {
			pkg.StartTime = event.Time
		}

	case "pass":
		if event.Test != "" {
			pkg.Passed++
			pkg.CurrentTest = event.Test
			pkg.AllTests = append(pkg.AllTests, fo.TestResult{
				Name:   event.Test,
				Status: "PASS",
			})
		} else {
			// Package completed
			pkg.Duration = time.Duration(event.Elapsed * float64(time.Second))
			f.renderPackageResult(pkg)
		}

	case "fail":
		if event.Test != "" {
			pkg.Failed++
			pkg.FailedTests = append(pkg.FailedTests, event.Test)
			pkg.CurrentTest = event.Test
			pkg.AllTests = append(pkg.AllTests, fo.TestResult{
				Name:   event.Test,
				Status: "FAIL",
			})
		} else {
			// Package completed
			pkg.Duration = time.Duration(event.Elapsed * float64(time.Second))
			f.renderPackageResult(pkg)
		}

	case "skip":
		if event.Test != "" {
			pkg.Skipped++
			pkg.CurrentTest = event.Test
			pkg.AllTests = append(pkg.AllTests, fo.TestResult{
				Name:   event.Test,
				Status: "SKIP",
			})
		}

	case "output":
		// Parse coverage from output
		if strings.Contains(event.Output, "coverage:") && strings.Contains(event.Output, "% of statements") {
			f.parseCoverage(pkg, event.Output)
		}
	}
}

func (f *TestFormatter) getOrCreatePackage(name string) *PackageResult {
	if pkg, ok := f.packages[name]; ok {
		return pkg
	}
	pkg := &PackageResult{
		Name:      name,
		StartTime: time.Now(),
	}
	f.packages[name] = pkg
	return pkg
}

func (f *TestFormatter) parseCoverage(pkg *PackageResult, output string) {
	// Parse "coverage: 45.2% of statements"
	var cov float64
	_, _ = fmt.Sscanf(output, "coverage: %f%% of statements", &cov)
	if cov > 0 {
		pkg.Coverage = cov
	}
}

func (f *TestFormatter) renderPackageResult(pkg *PackageResult) {
	// Buffer the result for final grouped rendering
	f.completed = append(f.completed, pkg)
}

func (f *TestFormatter) renderFinalSummary() {
	// Group packages by top-level directory
	groups := make(map[string][]*PackageResult)
	var groupOrder []string

	for _, pkg := range f.completed {
		dir := f.getTopLevelDir(pkg.Name)
		if _, exists := groups[dir]; !exists {
			groupOrder = append(groupOrder, dir)
		}
		groups[dir] = append(groups[dir], pkg)
	}

	// Sort group order for consistent output
	for i := 0; i < len(groupOrder)-1; i++ {
		for j := i + 1; j < len(groupOrder); j++ {
			if groupOrder[i] > groupOrder[j] {
				groupOrder[i], groupOrder[j] = groupOrder[j], groupOrder[i]
			}
		}
	}

	// Sort packages within each group
	for _, pkgs := range groups {
		for i := 0; i < len(pkgs)-1; i++ {
			for j := i + 1; j < len(pkgs); j++ {
				if pkgs[i].Name > pkgs[j].Name {
					pkgs[i], pkgs[j] = pkgs[j], pkgs[i]
				}
			}
		}
	}

	// Collect all groups into renderer
	for _, dir := range groupOrder {
		pkgs := groups[dir]

		// Start group
		f.renderer.RenderGroupHeader(dir)

		// Add packages in this group
		for _, pkg := range pkgs {
			testPkg := fo.TestPackageResult{
				Name:        f.getPackageBaseName(pkg.Name),
				Passed:      pkg.Passed,
				Failed:      pkg.Failed,
				Skipped:     pkg.Skipped,
				Duration:    pkg.Duration,
				Coverage:    pkg.Coverage,
				FailedTests: pkg.FailedTests,
				AllTests:    pkg.AllTests,
			}
			f.renderer.RenderPackageLine(testPkg)
		}

		// End group
		f.renderer.RenderGroupFooter()
	}

	// Render everything at once
	f.renderer.RenderAll()
}

func (f *TestFormatter) getTopLevelDir(name string) string {
	// Extract top-level directory from package path
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
	// Fallback to last part
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

func (f *TestFormatter) getPackageBaseName(name string) string {
	// Get package name relative to its top-level directory
	if idx := strings.Index(name, "/internal/"); idx != -1 {
		return name[idx+10:] // after "/internal/"
	}
	if idx := strings.Index(name, "/cmd/"); idx != -1 {
		return name[idx+5:] // after "/cmd/"
	}
	if idx := strings.Index(name, "/examples/"); idx != -1 {
		return name[idx+10:] // after "/examples/"
	}
	if idx := strings.Index(name, "/pkg/"); idx != -1 {
		return name[idx+5:] // after "/pkg/"
	}
	// Fallback to last part
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

// RunFormattedTests runs tests with the custom formatter.
func RunFormattedTests(args []string) error {
	// Determine if terminal
	isTerminal := term.IsTerminal(int(os.Stdout.Fd()))

	formatter := NewTestFormatter(os.Stdout, isTerminal)
	return formatter.RunTests(args)
}
