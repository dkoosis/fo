package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
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
	&FilesizeDashboardFormatter{}, // Must be before SARIF to match dashboard format
	&GolangciLintFormatter{},      // Per-linter sections for golangci-lint
	&SARIFFormatter{},
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
			if len(failure.failedTests) == 0 {
				b.WriteString(fmt.Sprintf("    %s\n", mutedStyle.Render("(build/import error)")))
			} else {
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

	filledBar := strings.Repeat("▮", filled)
	emptyBar := strings.Repeat("▯", barLen-filled)
	pct := fmt.Sprintf("%.0f%%", coverage)

	// Pale threshold colors for filled portion
	paleGreen := lipgloss.NewStyle().Foreground(lipgloss.Color("#7CB97C"))  // Good: >= 70%
	paleYellow := lipgloss.NewStyle().Foreground(lipgloss.Color("#C9B458")) // OK: >= 40%
	paleRed := lipgloss.NewStyle().Foreground(lipgloss.Color("#C97C7C"))    // Poor: < 40%
	paleGray := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))   // Empty boxes

	var barStyle lipgloss.Style
	if coverage >= 70 {
		barStyle = paleGreen
	} else if coverage >= 40 {
		barStyle = paleYellow
	} else {
		barStyle = paleRed
	}

	return barStyle.Render(filledBar) + paleGray.Render(emptyBar) + " " + mutedStyle.Render(pct)
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
// Golangci-lint Formatter (per-linter sections with smart rendering)
// ============================================================================

// Formatting constants for golangci-lint output.
const (
	lintItemsPerSection   = 5  // max items shown per linter section
	lintComplexityWarn    = 30 // complexity threshold for red highlighting
	lintMsgTruncateLen    = 50 // max message length before truncation
	lintFileListMaxLen    = 40 // max length for file list in goconst
	lintFuncNameColWidth  = 24 // column width for function names
)

type GolangciLintFormatter struct{}

func (f *GolangciLintFormatter) Matches(command string) bool {
	return strings.Contains(command, "golangci-lint")
}

// lintIssue represents a single issue from golangci-lint SARIF output.
type lintIssue struct {
	linter  string
	file    string
	line    int
	message string
	level   string
}

func (f *GolangciLintFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)

	// Parse SARIF - extract JSON from mixed output (stdout SARIF + stderr text)
	var report SARIFReport
	var parsed bool
	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, `"runs"`) {
			if err := json.Unmarshal([]byte(trimmed), &report); err == nil {
				parsed = true
				break
			}
		}
	}
	if !parsed {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Group issues by linter
	byLinter := make(map[string][]lintIssue)
	totalIssues := 0
	for _, run := range report.Runs {
		for _, result := range run.Results {
			filePath := ""
			lineNum := 0
			if len(result.Locations) > 0 {
				loc := result.Locations[0].PhysicalLocation
				filePath = loc.ArtifactLocation.URI
				lineNum = loc.Region.StartLine
			}
			issue := lintIssue{
				linter:  result.RuleID,
				file:    filePath,
				line:    lineNum,
				message: result.Message.Text,
				level:   result.Level,
			}
			byLinter[result.RuleID] = append(byLinter[result.RuleID], issue)
			totalIssues++
		}
	}

	// No issues
	if len(byLinter) == 0 {
		b.WriteString(successStyle.Render("✓ No issues found\n"))
		return b.String()
	}

	// Sort linters by issue count (descending)
	type linterGroup struct {
		name   string
		issues []lintIssue
	}
	groups := make([]linterGroup, 0, len(byLinter))
	for name, issues := range byLinter {
		groups = append(groups, linterGroup{name, issues})
	}
	sort.Slice(groups, func(i, j int) bool {
		return len(groups[i].issues) > len(groups[j].issues)
	})

	// Render each linter section
	for _, g := range groups {
		countStyle := warnStyle
		for _, iss := range g.issues {
			if iss.level == "error" {
				countStyle = errorStyle
				break
			}
		}

		b.WriteString(headerStyle.Render(fmt.Sprintf("◉ %s", g.name)))
		b.WriteString(countStyle.Render(fmt.Sprintf(" (%d)", len(g.issues))))
		b.WriteString("\n")

		// Dispatch to per-linter renderer
		switch g.name {
		case "gocyclo":
			f.renderGocyclo(&b, g.issues, fileStyle, errorStyle, warnStyle, mutedStyle)
		case "goconst":
			f.renderGoconst(&b, g.issues, fileStyle, mutedStyle)
		default:
			f.renderDefault(&b, g.issues, fileStyle, mutedStyle, errorStyle, warnStyle)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderGocyclo renders complexity issues as a ranked list.
func (f *GolangciLintFormatter) renderGocyclo(b *strings.Builder, issues []lintIssue, fileStyle, errorStyle, warnStyle, mutedStyle lipgloss.Style) {
	type complexityItem struct {
		funcName   string
		complexity int
		file       string
	}
	items := make([]complexityItem, 0, len(issues))

	for _, iss := range issues {
		var complexity int
		var funcName string
		// Parse: "cyclomatic complexity 25 of func `run` is high (> 20)"
		if _, err := fmt.Sscanf(iss.message, "cyclomatic complexity %d of func `", &complexity); err == nil {
			if start := strings.Index(iss.message, "`"); start >= 0 {
				if end := strings.Index(iss.message[start+1:], "`"); end >= 0 {
					funcName = iss.message[start+1 : start+1+end]
				}
			}
		}
		if funcName == "" {
			funcName = "?"
		}
		items = append(items, complexityItem{funcName, complexity, iss.file})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].complexity > items[j].complexity
	})

	for _, item := range items {
		scoreStyle := warnStyle
		if item.complexity > lintComplexityWarn {
			scoreStyle = errorStyle
		}
		b.WriteString(fmt.Sprintf("  %-*s %s  %s\n",
			lintFuncNameColWidth,
			item.funcName,
			scoreStyle.Render(fmt.Sprintf("%2d", item.complexity)),
			fileStyle.Render(shortPath(item.file))))
	}
}

// renderGoconst renders magic string issues grouped by literal.
func (f *GolangciLintFormatter) renderGoconst(b *strings.Builder, issues []lintIssue, fileStyle, mutedStyle lipgloss.Style) {
	type constItem struct {
		literal string
		count   int
		files   []string
	}
	byLiteral := make(map[string]*constItem)

	for _, iss := range issues {
		start := strings.Index(iss.message, "`")
		if start < 0 {
			continue
		}
		end := strings.Index(iss.message[start+1:], "`")
		if end < 0 {
			continue
		}
		literal := iss.message[start+1 : start+1+end]

		var count int
		fmt.Sscanf(iss.message, "string `"+literal+"` has %d occurrences", &count)

		if byLiteral[literal] == nil {
			byLiteral[literal] = &constItem{literal: literal, count: count}
		}
		byLiteral[literal].files = append(byLiteral[literal].files, shortPath(iss.file))
	}

	items := make([]*constItem, 0, len(byLiteral))
	for _, item := range byLiteral {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	for i, item := range items {
		if i >= lintItemsPerSection {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(items)-lintItemsPerSection)))
			break
		}
		files := strings.Join(item.files, ", ")
		if len(files) > lintFileListMaxLen {
			files = files[:lintFileListMaxLen-3] + "..."
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			mutedStyle.Render(fmt.Sprintf("%q", item.literal)),
			mutedStyle.Render(fmt.Sprintf("%d×", item.count)),
			fileStyle.Render(files)))
	}
}

// renderDefault renders issues as a simple list.
func (f *GolangciLintFormatter) renderDefault(b *strings.Builder, issues []lintIssue, fileStyle, mutedStyle, errorStyle, warnStyle lipgloss.Style) {
	for i, iss := range issues {
		if i >= lintItemsPerSection {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-lintItemsPerSection)))
			break
		}
		icon := mutedStyle.Render("·")
		switch iss.level {
		case "error":
			icon = errorStyle.Render("✗")
		case "warning":
			icon = warnStyle.Render("△")
		}
		msg := iss.message
		if len(msg) > lintMsgTruncateLen {
			msg = msg[:lintMsgTruncateLen-3] + "..."
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n",
			icon,
			fileStyle.Render(fmt.Sprintf("%s:%d", shortPath(iss.file), iss.line)),
			mutedStyle.Render(msg)))
	}
}

// shortPath extracts just the filename from a path.
func shortPath(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// ============================================================================
// SARIF Formatter (handles SARIF output from any static analyzer)
// ============================================================================

type SARIFFormatter struct{}

func (f *SARIFFormatter) Matches(command string) bool {
	// Match any command that produces SARIF output.
	// Note: golangci-lint has its own dedicated formatter.
	// Add new SARIF-producing tools here as needed.
	return strings.Contains(command, "filesize")
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

func (f *SARIFFormatter) Format(lines []string, width int) string {
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
// Filesize Dashboard Formatter (handles filesize -format=dashboard output)
// ============================================================================

type FilesizeDashboardFormatter struct{}

func (f *FilesizeDashboardFormatter) Matches(command string) bool {
	return strings.Contains(command, "filesize") && strings.Contains(command, "-format=dashboard")
}

// FilesizeDashboard represents the dashboard JSON output from filesize.
type FilesizeDashboard struct {
	Timestamp string                 `json:"timestamp"`
	Metrics   FilesizeDashboardMetrics `json:"metrics"`
	TopFiles  []FilesizeDashboardFile  `json:"top_files"`
	History   []FilesizeHistoryEntry   `json:"history"`
}

type FilesizeDashboardMetrics struct {
	Total     int `json:"total"`
	Green     int `json:"green"`
	Yellow    int `json:"yellow"`
	Red       int `json:"red"`
	TestFiles int `json:"test_files"`
	MDFiles   int `json:"md_files"`
	OrphanMD  int `json:"orphan_md"`
}

type FilesizeDashboardFile struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Tier  string `json:"tier"`
}

type FilesizeHistoryEntry struct {
	Week      string `json:"week"`
	Total     int    `json:"total"`
	Green     int    `json:"green"`
	Yellow    int    `json:"yellow"`
	Red       int    `json:"red"`
	TestFiles int    `json:"test_files"`
	MDFiles   int    `json:"md_files"`
	OrphanMD  int    `json:"orphan_md"`
}

func (f *FilesizeDashboardFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Parse dashboard JSON
	fullOutput := strings.Join(lines, "\n")
	var dashboard FilesizeDashboard
	if err := json.Unmarshal([]byte(fullOutput), &dashboard); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Validate we got actual data
	if dashboard.Metrics.Total == 0 && len(dashboard.TopFiles) == 0 {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	m := dashboard.Metrics

	// ── Top 5 Largest Files ──────────────────────────────────────────────
	b.WriteString(headerStyle.Render("◉ Largest Source Files"))
	b.WriteString("\n\n")

	for i, f := range dashboard.TopFiles {
		if i >= 5 {
			break
		}
		var tierStyle lipgloss.Style
		switch f.Tier {
		case "red":
			tierStyle = errorStyle
		case "yellow":
			tierStyle = warnStyle
		default:
			tierStyle = successStyle
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			tierStyle.Render(fmt.Sprintf("%4d", f.Lines)),
			fileStyle.Render(f.Path)))
	}
	b.WriteString("\n")

	// ── File Size Distribution ───────────────────────────────────────────
	b.WriteString(headerStyle.Render("◉ Size Distribution"))
	b.WriteString("\n\n")

	// Get previous values for trends (Week -1 if available)
	var prevRed, prevYellow, prevGreen int
	if len(dashboard.History) > 0 {
		prevRed = dashboard.History[0].Red
		prevYellow = dashboard.History[0].Yellow
		prevGreen = dashboard.History[0].Green
	}

	// Red (>1000 LOC)
	redArrow := trendArrow(m.Red, prevRed, true) // up is bad
	redStyle := successStyle
	if m.Red > 0 {
		redStyle = errorStyle
	}
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", ">1000 LOC")),
		redStyle.Render(fmt.Sprintf("%4d", m.Red)),
		redArrow))

	// Yellow (500-999 LOC)
	yellowArrow := trendArrow(m.Yellow, prevYellow, true) // up is bad
	yellowStyle := successStyle
	if m.Yellow > 0 {
		yellowStyle = warnStyle
	}
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "500-999 LOC")),
		yellowStyle.Render(fmt.Sprintf("%4d", m.Yellow)),
		yellowArrow))

	// Green (<500 LOC)
	greenArrow := trendArrow(m.Green, prevGreen, false) // up is good
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "<500 LOC")),
		successStyle.Render(fmt.Sprintf("%4d", m.Green)),
		greenArrow))

	b.WriteString("\n")

	// Get previous values for additional metrics
	var prevTest, prevMD, prevOrphan int
	if len(dashboard.History) > 0 {
		prevTest = dashboard.History[0].TestFiles
		prevMD = dashboard.History[0].MDFiles
		prevOrphan = dashboard.History[0].OrphanMD
	}

	// Test files (neutral - more is generally good)
	testArrow := trendArrowNeutral(m.TestFiles, prevTest)
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Test files")),
		fileStyle.Render(fmt.Sprintf("%4d", m.TestFiles)),
		testArrow))

	// MD files (neutral)
	mdArrow := trendArrowNeutral(m.MDFiles, prevMD)
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Markdown files")),
		fileStyle.Render(fmt.Sprintf("%4d", m.MDFiles)),
		mdArrow))

	// Orphan MD (any > 0 is wrong)
	orphanArrow := trendArrow(m.OrphanMD, prevOrphan, true) // up is bad
	orphanStyle := successStyle
	if m.OrphanMD > 0 {
		orphanStyle = errorStyle
	}
	b.WriteString(fmt.Sprintf("  %s %s %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Orphan docs")),
		orphanStyle.Render(fmt.Sprintf("%4d", m.OrphanMD)),
		orphanArrow))

	// ── Weekly Trend (if history available) ──────────────────────────────
	if len(dashboard.History) > 1 {
		b.WriteString("\n")
		b.WriteString(headerStyle.Render("◉ 4-Week Trend"))
		b.WriteString("\n\n")

		// Show last 4 weeks as mini sparkbars
		weeksToShow := 4
		if len(dashboard.History) < weeksToShow {
			weeksToShow = len(dashboard.History)
		}

		barWidth := 20
		for i := 0; i < weeksToShow; i++ {
			h := dashboard.History[i]
			total := h.Green + h.Yellow + h.Red
			if total == 0 {
				b.WriteString(fmt.Sprintf("  %-10s %s\n",
					mutedStyle.Render(h.Week),
					mutedStyle.Render(strings.Repeat("·", barWidth))))
				continue
			}

			greenChars := (h.Green * barWidth) / total
			yellowChars := (h.Yellow * barWidth) / total
			redChars := barWidth - greenChars - yellowChars

			bar := successStyle.Render(strings.Repeat("█", greenChars)) +
				warnStyle.Render(strings.Repeat("█", yellowChars)) +
				errorStyle.Render(strings.Repeat("█", redChars))

			b.WriteString(fmt.Sprintf("  %-10s %s\n", mutedStyle.Render(h.Week), bar))
		}
	}

	return b.String()
}

// trendArrow returns a colored arrow based on direction.
// upIsBad=true means increasing values are bad (red arrow up, green arrow down).
func trendArrow(current, previous int, upIsBad bool) string {
	if previous == 0 {
		return ""
	}
	diff := current - previous
	if diff == 0 {
		return ""
	}

	upStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))   // red
	downStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")) // green
	if !upIsBad {
		upStyle, downStyle = downStyle, upStyle // swap colors
	}

	if diff > 0 {
		return upStyle.Render("↑")
	}
	return downStyle.Render("↓")
}

// trendArrowNeutral returns a muted arrow (no good/bad coloring).
func trendArrowNeutral(current, previous int) string {
	if previous == 0 {
		return ""
	}
	diff := current - previous
	if diff == 0 {
		return ""
	}
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	if diff > 0 {
		return muted.Render("↑")
	}
	return muted.Render("↓")
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
