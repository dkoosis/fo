package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// OutputFormatter formats task output for display in the detail panel.
type OutputFormatter interface {
	// Format takes raw output lines and returns formatted content.
	Format(lines []string, width int) string
	// Matches returns true if this formatter should handle the given command.
	Matches(command string) bool
}

// FormatterRegistry holds registered formatters.
var formatters = []OutputFormatter{
	&GoTestFormatter{},
	&GolangciLintFormatter{},
	&PlainFormatter{}, // fallback, always last
}

// FormatOutput selects the appropriate formatter and formats the output.
func FormatOutput(command string, lines []string, width int) string {
	for _, f := range formatters {
		if f.Matches(command) {
			return f.Format(lines, width)
		}
	}
	return strings.Join(lines, "\n")
}

// ============================================================================
// Go Test Formatter (handles go test -json output)
// ============================================================================

type GoTestFormatter struct{}

func (f *GoTestFormatter) Matches(command string) bool {
	return strings.Contains(command, "go test") && strings.Contains(command, "-json")
}

// GoTestEvent represents a single event from go test -json output.
type GoTestEvent struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"`
	Package string  `json:"Package"`
	Test    string  `json:"Test"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
}

// testResult tracks individual test results
type testResult struct {
	name    string
	status  string // pass, fail, skip, run
	elapsed float64
	output  []string
}

// pkgResult tracks package-level results
type pkgResult struct {
	status   string // pass, fail, run
	elapsed  float64
	coverage float64
	tests    map[string]*testResult
	testOrder []string
}

func (f *GoTestFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles - use theme colors if available
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E"))
	runStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#48CAE4"))
	pkgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	testStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Get spinner frame for running indicator
	spinnerFrame := "⟳"
	if activeTheme != nil && len(activeTheme.SpinnerFrames) > 0 {
		idx := int(time.Now().UnixMilli()/int64(activeTheme.SpinnerInterval)) % len(activeTheme.SpinnerFrames)
		spinnerFrame = activeTheme.SpinnerFrames[idx]
	}

	// Parse events
	packages := make(map[string]*pkgResult)
	pkgOrder := []string{}

	for _, line := range lines {
		if line == "" {
			continue
		}
		var event GoTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		pkg := event.Package
		if pkg == "" {
			continue
		}

		if _, ok := packages[pkg]; !ok {
			packages[pkg] = &pkgResult{
				status: "run",
				tests:  make(map[string]*testResult),
			}
			pkgOrder = append(pkgOrder, pkg)
		}
		pr := packages[pkg]

		// Handle test-level events
		if event.Test != "" {
			if _, ok := pr.tests[event.Test]; !ok {
				pr.tests[event.Test] = &testResult{name: event.Test, status: "run"}
				pr.testOrder = append(pr.testOrder, event.Test)
			}
			tr := pr.tests[event.Test]

			switch event.Action {
			case "run":
				tr.status = "run"
			case "pass":
				tr.status = "pass"
				tr.elapsed = event.Elapsed
			case "fail":
				tr.status = "fail"
				tr.elapsed = event.Elapsed
			case "skip":
				tr.status = "skip"
			case "output":
				// Capture test output (for failures)
				out := strings.TrimRight(event.Output, "\n")
				if out != "" && !strings.HasPrefix(out, "=== ") && !strings.HasPrefix(out, "--- ") {
					tr.output = append(tr.output, out)
				}
			}
		} else {
			// Package-level events
			switch event.Action {
			case "pass":
				pr.status = "pass"
				pr.elapsed = event.Elapsed
			case "fail":
				pr.status = "fail"
				pr.elapsed = event.Elapsed
			case "output":
				// Check for coverage info
				if strings.Contains(event.Output, "coverage:") {
					var cov float64
					fmt.Sscanf(event.Output, "coverage: %f%%", &cov)
					pr.coverage = cov
				}
			}
		}
	}

	// Count totals
	totalPassed, totalFailed, totalSkipped, totalRunning := 0, 0, 0, 0
	for _, pkg := range pkgOrder {
		pr := packages[pkg]
		for _, tr := range pr.tests {
			switch tr.status {
			case "pass":
				totalPassed++
			case "fail":
				totalFailed++
			case "skip":
				totalSkipped++
			case "run":
				totalRunning++
			}
		}
	}

	// Calculate subsystem coverage
	subsystems := calculateSubsystemCoverage(packages)

	// Header summary - unified format with colored counts
	totalTests := totalPassed + totalFailed + totalSkipped + totalRunning

	// Status indicator and total
	if totalRunning > 0 {
		b.WriteString(runStyle.Render(fmt.Sprintf("%s Running %d tests", spinnerFrame, totalTests)))
	} else if totalFailed > 0 {
		b.WriteString(failStyle.Render(fmt.Sprintf("✗ %d tests", totalTests)))
	} else {
		b.WriteString(passStyle.Render(fmt.Sprintf("✓ %d tests", totalTests)))
	}

	// Breakdown with colored counts
	b.WriteString(mutedStyle.Render(":  "))

	parts := []string{}
	if totalPassed > 0 {
		parts = append(parts, passStyle.Render(fmt.Sprintf("%d passed", totalPassed)))
	}
	if totalSkipped > 0 {
		parts = append(parts, skipStyle.Render(fmt.Sprintf("%d skipped", totalSkipped)))
	}
	if totalFailed > 0 {
		parts = append(parts, failStyle.Render(fmt.Sprintf("%d failed", totalFailed)))
	}
	if totalRunning > 0 {
		parts = append(parts, runStyle.Render(fmt.Sprintf("%d running", totalRunning)))
	}

	b.WriteString(strings.Join(parts, mutedStyle.Render(" | ")))
	b.WriteString("\n\n")

	// Subsystem coverage summary
	if len(subsystems) > 0 {
		b.WriteString(pkgStyle.Render("Coverage by Subsystem") + "\n")
		for _, ss := range subsystems {
			if ss.pkgCount > 0 {
				bar := renderCoverageBar(ss.avgCoverage, mutedStyle, passStyle)
				b.WriteString(fmt.Sprintf("  %-8s %s\n", ss.name, bar))
			}
		}
		b.WriteString("\n")
	}

	// Package details - show failed packages first
	failedPkgs := []string{}
	passedPkgs := []string{}
	for _, pkg := range pkgOrder {
		if packages[pkg].status == "fail" {
			failedPkgs = append(failedPkgs, pkg)
		} else {
			passedPkgs = append(passedPkgs, pkg)
		}
	}
	sortedPkgs := append(failedPkgs, passedPkgs...)

	for _, pkg := range sortedPkgs {
		pr := packages[pkg]

		// Package icon
		var icon string
		switch pr.status {
		case "pass":
			icon = passStyle.Render("✓")
		case "fail":
			icon = failStyle.Render("✗")
		default:
			icon = runStyle.Render(spinnerFrame)
		}

		// Shorten package name
		shortPkg := pkg
		if parts := strings.Split(pkg, "/"); len(parts) > 2 {
			shortPkg = ".../" + strings.Join(parts[len(parts)-2:], "/")
		}

		// Coverage sparkbar
		coverageStr := ""
		if pr.coverage > 0 {
			coverageStr = " " + renderCoverageBar(pr.coverage, mutedStyle, passStyle)
		}

		// Duration
		elapsed := ""
		if pr.elapsed > 0 {
			elapsed = mutedStyle.Render(fmt.Sprintf(" %.2fs", pr.elapsed))
		}

		b.WriteString(fmt.Sprintf("%s %s%s%s\n", icon, pkgStyle.Render(shortPkg), coverageStr, elapsed))

		// Show failed tests with output
		if pr.status == "fail" {
			for _, testName := range pr.testOrder {
				tr := pr.tests[testName]
				if tr.status != "fail" {
					continue
				}

				// Humanized test name
				friendlyName := humanizeTestName(tr.name)
				b.WriteString(fmt.Sprintf("  %s %s\n", failStyle.Render("✗"), testStyle.Render(friendlyName)))

				// Filter and show only useful output
				var useful []string
				for _, out := range tr.output {
					if isUsefulOutput(out) {
						useful = append(useful, out)
					}
				}
				// Limit to last 5 useful lines
				if len(useful) > 5 {
					useful = useful[len(useful)-5:]
				}
				for _, out := range useful {
					// Indent and truncate long lines
					if len(out) > width-8 {
						out = out[:width-11] + "..."
					}
					b.WriteString(outputStyle.Render("    │ " + out) + "\n")
				}
			}
		}
	}

	return b.String()
}

// humanizeTestName converts TestFoo_Bar_Baz to "Foo Bar Baz" while preserving acronyms
func humanizeTestName(name string) string {
	// Remove "Test" prefix
	name = strings.TrimPrefix(name, "Test")

	// Known acronyms to preserve
	acronyms := map[string]string{
		"sql": "SQL", "Sql": "SQL",
		"utf8": "UTF8", "Utf8": "UTF8",
		"utf16": "UTF16", "Utf16": "UTF16",
		"json": "JSON", "Json": "JSON",
		"xml": "XML", "Xml": "XML",
		"html": "HTML", "Html": "HTML",
		"http": "HTTP", "Http": "HTTP",
		"https": "HTTPS", "Https": "HTTPS",
		"url": "URL", "Url": "URL",
		"uri": "URI", "Uri": "URI",
		"api": "API", "Api": "API",
		"id": "ID", "Id": "ID",
		"db": "DB", "Db": "DB",
		"io": "IO", "Io": "IO",
		"mcp": "MCP", "Mcp": "MCP",
		"rpc": "RPC", "Rpc": "RPC",
		"grpc": "gRPC", "Grpc": "gRPC",
		"tcp": "TCP", "Tcp": "TCP",
		"udp": "UDP", "Udp": "UDP",
		"tls": "TLS", "Tls": "TLS",
		"ssl": "SSL", "Ssl": "SSL",
		"cpu": "CPU", "Cpu": "CPU",
		"gpu": "GPU", "Gpu": "GPU",
		"ram": "RAM", "Ram": "RAM",
		"os": "OS", "Os": "OS",
		"ui": "UI", "Ui": "UI",
		"ux": "UX", "Ux": "UX",
		"ci": "CI", "Ci": "CI",
		"cd": "CD", "Cd": "CD",
		"ok": "OK", "Ok": "OK",
		"kg": "KG", "Kg": "KG",
		"lsp": "LSP", "Lsp": "LSP",
	}

	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")

	// Split into words and process
	words := strings.Fields(name)
	for i, word := range words {
		lower := strings.ToLower(word)
		if acronym, ok := acronyms[lower]; ok {
			words[i] = acronym
		} else if acronym, ok := acronyms[word]; ok {
			words[i] = acronym
		}
	}

	result := strings.Join(words, " ")
	if result == "" {
		return name
	}
	return result
}

// isUsefulOutput filters out noisy log lines
func isUsefulOutput(line string) bool {
	// Skip timestamp-heavy log lines
	if strings.Contains(line, "time=") && strings.Contains(line, "level=") {
		return false
	}
	// Skip empty or whitespace-only
	if strings.TrimSpace(line) == "" {
		return false
	}
	// Keep error messages, assertions, panics
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") ||
		strings.Contains(lower, "fail") ||
		strings.Contains(lower, "panic") ||
		strings.Contains(lower, "expected") ||
		strings.Contains(lower, "got") ||
		strings.Contains(lower, "assert") ||
		strings.Contains(line, "!=") ||
		strings.Contains(line, "want") {
		return true
	}
	// Skip most other lines
	return false
}

// renderCoverageBar creates a sparkbar for coverage percentage
func renderCoverageBar(coverage float64, mutedStyle, goodStyle lipgloss.Style) string {
	barLen := 10
	filled := int(coverage / 10)
	if filled > barLen {
		filled = barLen
	}

	bar := strings.Repeat("▮", filled) + strings.Repeat("▯", barLen-filled)
	pct := fmt.Sprintf("%.0f%%", coverage)

	if coverage >= 70 {
		return goodStyle.Render(bar) + " " + mutedStyle.Render(pct)
	}
	return mutedStyle.Render(bar + " " + pct)
}

// subsystemResult holds aggregated coverage for an architectural subsystem
type subsystemResult struct {
	name        string
	avgCoverage float64
	pkgCount    int
}

// architecturalSubsystems defines path patterns for each subsystem
// Based on .go-arch-lint.yml component definitions
var architecturalSubsystems = []struct {
	name     string
	patterns []string
}{
	{"core", []string{"/internal/core/", "/core/"}},
	{"kg", []string{"/internal/kg/", "/kg/"}},
	{"domain", []string{"/internal/domain/", "/internal/tools/", "/domain/", "/tools/"}},
	{"adapter", []string{"/internal/mcp/", "/mcp/"}},
	{"worker", []string{"/internal/proc/", "/proc/"}},
	{"kits", []string{"/internal/codekit/", "/internal/testkit/", "/codekit/", "/testkit/"}},
	{"util", []string{"/internal/util/", "/util/", "/pkg/"}},
}

// calculateSubsystemCoverage aggregates package coverage by architectural subsystem
func calculateSubsystemCoverage(packages map[string]*pkgResult) []subsystemResult {
	// Initialize subsystem accumulators
	type accumulator struct {
		totalCoverage float64
		count         int
	}
	accum := make(map[string]*accumulator)
	for _, ss := range architecturalSubsystems {
		accum[ss.name] = &accumulator{}
	}

	// Categorize each package
	for pkg, pr := range packages {
		if pr.coverage <= 0 {
			continue // Skip packages without coverage data
		}

		subsystem := ""
		for _, ss := range architecturalSubsystems {
			for _, pattern := range ss.patterns {
				if strings.Contains(pkg, pattern) {
					subsystem = ss.name
					break
				}
			}
			if subsystem != "" {
				break
			}
		}

		if subsystem == "" {
			subsystem = "util" // Default unmapped packages to util
		}

		if a, ok := accum[subsystem]; ok {
			a.totalCoverage += pr.coverage
			a.count++
		}
	}

	// Build results in defined order
	var results []subsystemResult
	for _, ss := range architecturalSubsystems {
		a := accum[ss.name]
		avg := 0.0
		if a.count > 0 {
			avg = a.totalCoverage / float64(a.count)
		}
		results = append(results, subsystemResult{
			name:        ss.name,
			avgCoverage: avg,
			pkgCount:    a.count,
		})
	}

	return results
}

// ============================================================================
// Golangci-lint Formatter (handles SARIF output)
// ============================================================================

type GolangciLintFormatter struct{}

func (f *GolangciLintFormatter) Matches(command string) bool {
	return strings.Contains(command, "golangci-lint")
}

type SARIFReport struct {
	Runs []struct {
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine   int `json:"startLine"`
						StartColumn int `json:"startColumn"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

func (f *GolangciLintFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6"))
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)

	// Try to parse SARIF
	fullOutput := strings.Join(lines, "\n")
	var report SARIFReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		// Not SARIF, fall back to plain
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Count issues
	errors, warnings := 0, 0
	type issue struct {
		file    string
		line    int
		rule    string
		message string
		level   string
	}
	var issues []issue

	for _, run := range report.Runs {
		for _, result := range run.Results {
			if result.Level == "error" {
				errors++
			} else {
				warnings++
			}

			file := ""
			line := 0
			if len(result.Locations) > 0 {
				loc := result.Locations[0].PhysicalLocation
				file = loc.ArtifactLocation.URI
				line = loc.Region.StartLine
			}
			issues = append(issues, issue{
				file:    file,
				line:    line,
				rule:    result.RuleID,
				message: result.Message.Text,
				level:   result.Level,
			})
		}
	}

	// Summary
	if errors == 0 && warnings == 0 {
		b.WriteString(successStyle.Render("✓ No issues found\n"))
		return b.String()
	}

	if errors > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d errors", errors)))
	}
	if warnings > 0 {
		if errors > 0 {
			b.WriteString(", ")
		}
		b.WriteString(warnStyle.Render(fmt.Sprintf("△ %d warnings", warnings)))
	}
	b.WriteString("\n\n")

	// Issues (limit to first 20)
	shown := 0
	for _, iss := range issues {
		if shown >= 20 {
			b.WriteString(ruleStyle.Render(fmt.Sprintf("\n... and %d more issues", len(issues)-20)))
			break
		}

		icon := warnStyle.Render("△")
		if iss.level == "error" {
			icon = errorStyle.Render("✗")
		}

		location := fileStyle.Render(fmt.Sprintf("%s:%d", iss.file, iss.line))
		rule := ruleStyle.Render(fmt.Sprintf("[%s]", iss.rule))
		b.WriteString(fmt.Sprintf("%s %s %s\n", icon, location, rule))
		b.WriteString(fmt.Sprintf("   %s\n", iss.message))
		shown++
	}

	return b.String()
}

// ============================================================================
// Plain Formatter (fallback)
// ============================================================================

type PlainFormatter struct{}

func (f *PlainFormatter) Matches(command string) bool {
	return true // always matches as fallback
}

func (f *PlainFormatter) Format(lines []string, width int) string {
	// Apply basic styling - highlight errors and warnings
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E"))

	var result []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic") {
			result = append(result, errorStyle.Render(line))
		} else if strings.Contains(lower, "warning") || strings.Contains(lower, "warn") {
			result = append(result, warnStyle.Render(line))
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
