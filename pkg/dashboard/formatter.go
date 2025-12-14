package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

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

// Debug flag - set to true to see package parsing details
var DebugTestFormatter = false

func (f *GoTestFormatter) Format(lines []string, width int) string {
	var b strings.Builder
	var debugLines []string

	// Styles - use theme colors if available
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	skipStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E"))
	runStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#48CAE4"))
	pkgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	testStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	pendingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD88")) // Pale yellow for empty subsystems

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
				// Check for coverage info - line must START with "coverage:"
				// (the "ok" summary line also contains "coverage:" but we can't parse it)
				if strings.HasPrefix(event.Output, "coverage:") {
					var cov float64
					fmt.Sscanf(event.Output, "coverage: %f%%", &cov)
					if cov > 0 {
						pr.coverage = cov
					}
				}
			}
		}
	}

	// Debug: show parsed packages and coverage
	if DebugTestFormatter {
		debugLines = append(debugLines, fmt.Sprintf("DEBUG: Parsed %d packages from %d lines", len(packages), len(lines)))
		for _, pkg := range pkgOrder {
			pr := packages[pkg]
			debugLines = append(debugLines, fmt.Sprintf("  %s: status=%s cov=%.1f%% tests=%d", pkg, pr.status, pr.coverage, len(pr.tests)))
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

	// Calculate subsystem stats
	subsystems := calculateSubsystemStats(packages)

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

	// Calculate max name width for alignment
	maxNameLen := 0
	for _, ss := range subsystems {
		if len(ss.name) > maxNameLen {
			maxNameLen = len(ss.name)
		}
	}

	// Subsystem-centric view with status and coverage
	var allFailures []pkgFailure
	for _, ss := range subsystems {
		totalPkgs := ss.passedCount + ss.failedCount
		nameField := fmt.Sprintf("%-*s", maxNameLen, ss.name)

		// Determine status icon
		var icon string
		if totalPkgs == 0 && ss.pkgCount == 0 {
			icon = pendingStyle.Render("○")
		} else if ss.failedCount > 0 {
			icon = failStyle.Render("✗")
		} else {
			icon = passStyle.Render("✓")
		}

		// Coverage bar (show for any subsystem with packages)
		coverageStr := ""
		if totalPkgs > 0 {
			coverageStr = "  " + renderCoverageBar(ss.avgCoverage, mutedStyle, passStyle)
		}

		b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, nameField, coverageStr))

		// Collect failures for display in lower section
		allFailures = append(allFailures, ss.failures...)
	}

	// Show all failures in lower section
	if len(allFailures) > 0 {
		b.WriteString("\n")
		for _, failure := range allFailures {
			// Shorten package name
			shortPkg := failure.pkg
			if pkgParts := strings.Split(failure.pkg, "/"); len(pkgParts) > 2 {
				shortPkg = ".../" + strings.Join(pkgParts[len(pkgParts)-2:], "/")
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", failStyle.Render("✗"), pkgStyle.Render(shortPkg)))

			// Show failed test names (no icon, minimal indent for more space)
			for _, testName := range failure.failedTests {
				displayName := humanizeTestNameWithSubtest(testName)
				maxNameWidth := width - 4 // minimal indent
				if len(displayName) > maxNameWidth && maxNameWidth > 20 {
					displayName = truncateAtWord(displayName, maxNameWidth-3) + "..."
				}
				b.WriteString(fmt.Sprintf("    %s\n", testStyle.Render(displayName)))
			}
		}
	}

	// Append debug output at the end
	if DebugTestFormatter && len(debugLines) > 0 {
		b.WriteString("\n\n")
		for _, line := range debugLines {
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

// truncateAtWord truncates a string at word boundaries (spaces) to fit within maxLen.
func truncateAtWord(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find last space before maxLen
	lastSpace := -1
	for i := maxLen; i >= maxLen/2; i-- {
		if s[i] == ' ' {
			lastSpace = i
			break
		}
	}
	if lastSpace > 0 {
		return s[:lastSpace]
	}
	// No good break point, hard truncate
	return s[:maxLen]
}

// splitCamelCase inserts spaces between camelCase words.
// e.g., "ReaderLoop" -> "Reader Loop", "HTTPServer" -> "HTTP Server"
func splitCamelCase(s string) string {
	if len(s) <= 1 {
		return s
	}
	var result strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			// Insert space before uppercase if previous was lowercase
			// or if this starts a new word after an acronym (e.g., "HTTPServer" -> "HTTP Server")
			if unicode.IsUpper(r) && unicode.IsLower(prev) {
				result.WriteRune(' ')
			} else if i+1 < len(runes) && unicode.IsUpper(r) && unicode.IsUpper(prev) && unicode.IsLower(runes[i+1]) {
				// Handle "HTTPServer" -> "HTTP Server" (space before 'S')
				result.WriteRune(' ')
			}
		}
		result.WriteRune(r)
	}
	return result.String()
}

// humanizeTestNameWithSubtest converts test names including subtests to human-friendly format.
// e.g., "TestClient_ReaderLoop_RoutesMessages/response_message" -> "Client Reader Loop Routes Messages / response message"
func humanizeTestNameWithSubtest(name string) string {
	// Handle subtest names (split on /)
	if idx := strings.Index(name, "/"); idx != -1 {
		mainTest := humanizeTestName(name[:idx])
		subtest := splitCamelCase(strings.ReplaceAll(name[idx+1:], "_", " "))
		return mainTest + " / " + subtest
	}
	return humanizeTestName(name)
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

	// Split camelCase: "ReaderLoop" -> "Reader Loop"
	name = splitCamelCase(name)

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

// wrapTestName wraps long test names at natural break points
func wrapTestName(name string, maxWidth int) string {
	if len(name) <= maxWidth {
		return name
	}

	// Find best break point (underscore or slash) before maxWidth
	breakPoint := -1
	for i := maxWidth - 1; i > maxWidth/2; i-- {
		if name[i] == '_' || name[i] == '/' {
			breakPoint = i
			break
		}
	}

	if breakPoint == -1 {
		// No good break point, hard wrap
		return name[:maxWidth-3] + "..."
	}

	// Wrap with continuation on next line
	first := name[:breakPoint+1]
	rest := name[breakPoint+1:]
	if len(rest) > maxWidth-4 {
		rest = rest[:maxWidth-7] + "..."
	}
	return first + "\n      " + rest
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

// renderCoverageBar creates a sparkbar for coverage percentage with threshold colors
func renderCoverageBar(coverage float64, mutedStyle, _ lipgloss.Style) string {
	barLen := 10
	filled := int(coverage / 10)
	if filled > barLen {
		filled = barLen
	}

	bar := strings.Repeat("▮", filled) + strings.Repeat("▯", barLen-filled)
	pct := fmt.Sprintf("%.0f%%", coverage)

	// Pale threshold colors
	paleGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#7CB97C"))  // Good: >= 70%
	paleYellow := lipgloss.NewStyle().Foreground(lipgloss.Color("#C9B458")) // OK: >= 40%
	paleRed := lipgloss.NewStyle().Foreground(lipgloss.Color("#C97C7C"))    // Poor: < 40%

	var barStyle lipgloss.Style
	if coverage >= 70 {
		barStyle = paleGreen
	} else if coverage >= 40 {
		barStyle = paleYellow
	} else {
		barStyle = paleRed
	}

	return barStyle.Render(bar) + " " + mutedStyle.Render(pct)
}

// subsystemResult holds aggregated stats for an architectural subsystem
type subsystemResult struct {
	name        string
	avgCoverage float64
	pkgCount    int
	passedCount int
	failedCount int
	failures    []pkgFailure // failed packages with test details
}

// pkgFailure tracks a failed package and its failed tests
type pkgFailure struct {
	pkg         string
	failedTests []string // test names that failed
}

// getSubsystems returns the configured subsystems from theme, or defaults.
func getSubsystems() []SubsystemConfig {
	if activeTheme != nil && len(activeTheme.Subsystems) > 0 {
		return activeTheme.Subsystems
	}
	return DefaultSubsystems()
}

// getSubsystemForPackage returns the subsystem name for a package path
func getSubsystemForPackage(pkg string) string {
	subsystems := getSubsystems()
	for _, ss := range subsystems {
		for _, pattern := range ss.Patterns {
			if strings.Contains(pkg, pattern) {
				return ss.Name
			}
		}
	}
	// Default to last subsystem (typically "util") or "other"
	if len(subsystems) > 0 {
		return subsystems[len(subsystems)-1].Name
	}
	return "other"
}

// calculateSubsystemStats aggregates package stats by architectural subsystem
func calculateSubsystemStats(packages map[string]*pkgResult) []subsystemResult {
	subsystems := getSubsystems()

	// Initialize subsystem accumulators
	type accumulator struct {
		totalCoverage float64
		coverageCount int
		passedCount   int
		failedCount   int
		failures      []pkgFailure
	}
	accum := make(map[string]*accumulator)
	for _, ss := range subsystems {
		accum[ss.Name] = &accumulator{}
	}

	// Categorize each package
	for pkg, pr := range packages {
		subsystem := getSubsystemForPackage(pkg)
		a := accum[subsystem]

		// Track coverage
		if pr.coverage > 0 {
			a.totalCoverage += pr.coverage
			a.coverageCount++
		}

		// Infer package status from tests if not explicitly set
		status := pr.status
		if status == "run" && len(pr.tests) > 0 {
			// Check if any tests failed
			hasFailed := false
			allDone := true
			for _, tr := range pr.tests {
				if tr.status == "fail" {
					hasFailed = true
				}
				if tr.status == "run" {
					allDone = false
				}
			}
			if allDone {
				if hasFailed {
					status = "fail"
				} else {
					status = "pass"
				}
			}
		}

		// Track pass/fail
		if status == "pass" {
			a.passedCount++
		} else if status == "fail" {
			a.failedCount++
			// Collect failed test names
			var failedTests []string
			for _, testName := range pr.testOrder {
				if tr, ok := pr.tests[testName]; ok && tr.status == "fail" {
					failedTests = append(failedTests, testName)
				}
			}
			a.failures = append(a.failures, pkgFailure{
				pkg:         pkg,
				failedTests: failedTests,
			})
		}
	}

	// Build results in defined order
	var results []subsystemResult
	for _, ss := range subsystems {
		a := accum[ss.Name]
		avg := 0.0
		if a.coverageCount > 0 {
			avg = a.totalCoverage / float64(a.coverageCount)
		}
		results = append(results, subsystemResult{
			name:        ss.Name,
			avgCoverage: avg,
			pkgCount:    a.coverageCount,
			passedCount: a.passedCount,
			failedCount: a.failedCount,
			failures:    a.failures,
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
