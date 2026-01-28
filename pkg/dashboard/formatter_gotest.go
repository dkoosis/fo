package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// GoTestFormatter handles go test -json output.
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
	status    string // pass, fail, run
	elapsed   float64
	coverage  float64
	tests     map[string]*testResult
	testOrder []string
}

// Debug flag - set to true to see package parsing details
var DebugTestFormatter = false

func (f *GoTestFormatter) Format(lines []string, width int) string {
	styles := newGoTestStyles()
	spinnerFrame := spinnerFrameForTheme()

	packages, pkgOrder := parseGoTestEvents(lines)
	debugLines := goTestDebugLines(packages, pkgOrder, len(lines))

	totals := countTestTotals(packages, pkgOrder)
	subsystems := calculateSubsystemStats(packages)

	var b strings.Builder
	renderTestHeader(&b, totals, spinnerFrame, styles)
	allFailures := renderSubsystems(&b, subsystems, width, styles)
	renderFailures(&b, allFailures, width, styles)
	appendDebugLines(&b, debugLines)

	return b.String()
}

type goTestStyles struct {
	pass    lipgloss.Style
	fail    lipgloss.Style
	skip    lipgloss.Style
	run     lipgloss.Style
	pkg     lipgloss.Style
	test    lipgloss.Style
	muted   lipgloss.Style
	pending lipgloss.Style
}

func newGoTestStyles() goTestStyles {
	s := Styles()
	return goTestStyles{
		pass:    s.Success,
		fail:    s.Error,
		skip:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")), // non-bold warn
		run:     lipgloss.NewStyle().Foreground(lipgloss.Color("#48CAE4")), // test-specific cyan
		pkg:     s.Header,
		test:    s.File,
		muted:   s.Muted,
		pending: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD88")), // test-specific pending
	}
}

func spinnerFrameForTheme() string {
	if activeTheme != nil && len(activeTheme.SpinnerFrames) > 0 {
		idx := int(time.Now().UnixMilli()/int64(activeTheme.SpinnerInterval)) % len(activeTheme.SpinnerFrames)
		return activeTheme.SpinnerFrames[idx]
	}
	return "⟳"
}

func parseGoTestEvents(lines []string) (map[string]*pkgResult, []string) {
	packages := make(map[string]*pkgResult)
	pkgOrder := []string{}

	for _, line := range lines {
		if line == "" {
			continue
		}
		event, ok := parseGoTestEvent(line)
		if !ok || event.Package == "" {
			continue
		}

		pr, updatedOrder := ensurePackage(packages, event.Package, pkgOrder)
		pkgOrder = updatedOrder

		if event.Test != "" {
			handleTestEvent(pr, event)
			continue
		}

		handlePackageEvent(pr, event)
	}

	return packages, pkgOrder
}

func parseGoTestEvent(line string) (GoTestEvent, bool) {
	var event GoTestEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return GoTestEvent{}, false
	}
	return event, true
}

func ensurePackage(packages map[string]*pkgResult, pkg string, pkgOrder []string) (*pkgResult, []string) {
	if pr, ok := packages[pkg]; ok {
		return pr, pkgOrder
	}
	pr := &pkgResult{
		status: statusRun,
		tests:  make(map[string]*testResult),
	}
	packages[pkg] = pr
	return pr, append(pkgOrder, pkg)
}

func handleTestEvent(pr *pkgResult, event GoTestEvent) {
	tr := ensureTest(pr, event.Test)

	switch event.Action {
	case statusRun:
		tr.status = statusRun
	case statusPass:
		tr.status = statusPass
		tr.elapsed = event.Elapsed
	case statusFail:
		tr.status = statusFail
		tr.elapsed = event.Elapsed
	case statusSkip:
		tr.status = statusSkip
	case actionOutput:
		captureTestOutput(tr, event.Output)
	}
}

func ensureTest(pr *pkgResult, testName string) *testResult {
	if tr, ok := pr.tests[testName]; ok {
		return tr
	}
	tr := &testResult{name: testName, status: statusRun}
	pr.tests[testName] = tr
	pr.testOrder = append(pr.testOrder, testName)
	return tr
}

func captureTestOutput(tr *testResult, output string) {
	out := strings.TrimRight(output, "\n")
	if out == "" || strings.HasPrefix(out, "=== ") || strings.HasPrefix(out, "--- ") {
		return
	}
	tr.output = append(tr.output, out)
}

func handlePackageEvent(pr *pkgResult, event GoTestEvent) {
	switch event.Action {
	case statusPass:
		pr.status = statusPass
		pr.elapsed = event.Elapsed
	case statusFail:
		pr.status = statusFail
		pr.elapsed = event.Elapsed
	case actionOutput:
		recordCoverage(pr, event.Output)
	}
}

func recordCoverage(pr *pkgResult, output string) {
	// Coverage lines may have leading tabs (e.g., from packages with no tests)
	// Look for "coverage:" anywhere in the output
	idx := strings.Index(output, "coverage:")
	if idx == -1 {
		return
	}
	var cov float64
	_, _ = fmt.Sscanf(output[idx:], "coverage: %f%%", &cov)
	if cov > 0 {
		pr.coverage = cov
	}
}

func goTestDebugLines(packages map[string]*pkgResult, pkgOrder []string, lineCount int) []string {
	if !DebugTestFormatter {
		return nil
	}
	debugLines := []string{fmt.Sprintf("DEBUG: Parsed %d packages from %d lines", len(packages), lineCount)}
	for _, pkg := range pkgOrder {
		pr := packages[pkg]
		debugLines = append(debugLines, fmt.Sprintf("  %s: status=%s cov=%.1f%% tests=%d", pkg, pr.status, pr.coverage, len(pr.tests)))
	}
	return debugLines
}

type testTotals struct {
	passed  int
	failed  int
	skipped int
	running int
}

func countTestTotals(packages map[string]*pkgResult, pkgOrder []string) testTotals {
	var totals testTotals
	for _, pkg := range pkgOrder {
		pr := packages[pkg]
		for _, tr := range pr.tests {
			switch tr.status {
			case statusPass:
				totals.passed++
			case statusFail:
				totals.failed++
			case statusSkip:
				totals.skipped++
			case statusRun:
				totals.running++
			}
		}
	}
	return totals
}

func renderTestHeader(b *strings.Builder, totals testTotals, spinnerFrame string, styles goTestStyles) {
	totalTests := totals.passed + totals.failed + totals.skipped + totals.running
	switch {
	case totals.running > 0:
		b.WriteString(styles.run.Render(fmt.Sprintf("%s Running %d tests", spinnerFrame, totalTests)))
	case totals.failed > 0:
		b.WriteString(styles.fail.Render(fmt.Sprintf("✗ %d tests", totalTests)))
	default:
		b.WriteString(styles.pass.Render(fmt.Sprintf("✓ %d tests", totalTests)))
	}

	b.WriteString(styles.muted.Render(":  "))
	b.WriteString(strings.Join(headerParts(totals, styles), styles.muted.Render(" | ")))
	b.WriteString("\n\n")
}

func headerParts(totals testTotals, styles goTestStyles) []string {
	parts := []string{}
	if totals.passed > 0 {
		parts = append(parts, styles.pass.Render(fmt.Sprintf("%d passed", totals.passed)))
	}
	if totals.skipped > 0 {
		parts = append(parts, styles.skip.Render(fmt.Sprintf("%d skipped", totals.skipped)))
	}
	if totals.failed > 0 {
		parts = append(parts, styles.fail.Render(fmt.Sprintf("%d failed", totals.failed)))
	}
	if totals.running > 0 {
		parts = append(parts, styles.run.Render(fmt.Sprintf("%d running", totals.running)))
	}
	return parts
}

func renderSubsystems(b *strings.Builder, subsystems []subsystemResult, _ int, styles goTestStyles) []pkgFailure {
	maxNameLen := maxSubsystemNameLength(subsystems)
	var allFailures []pkgFailure
	for _, ss := range subsystems {
		totalPkgs := ss.passedCount + ss.failedCount
		nameField := fmt.Sprintf("%-*s", maxNameLen, ss.name)
		icon := subsystemIcon(totalPkgs, ss.failedCount, ss.pkgCount, styles)
		coverageStr := subsystemCoverage(totalPkgs, ss.avgCoverage, styles)

		_, _ = fmt.Fprintf(b, "  %s %s%s\n", icon, nameField, coverageStr)
		allFailures = append(allFailures, ss.failures...)
	}
	return allFailures
}

func maxSubsystemNameLength(subsystems []subsystemResult) int {
	maxNameLen := 0
	for _, ss := range subsystems {
		if len(ss.name) > maxNameLen {
			maxNameLen = len(ss.name)
		}
	}
	return maxNameLen
}

func subsystemIcon(totalPkgs, failedCount, pkgCount int, styles goTestStyles) string {
	switch {
	case totalPkgs == 0 && pkgCount == 0:
		return styles.pending.Render("○")
	case failedCount > 0:
		return styles.fail.Render("✗")
	default:
		return styles.pass.Render("✓")
	}
}

func subsystemCoverage(totalPkgs int, avgCoverage float64, styles goTestStyles) string {
	if totalPkgs == 0 {
		return ""
	}
	return "  " + renderCoverageBar(avgCoverage, styles.muted, styles.pass)
}

func renderFailures(b *strings.Builder, failures []pkgFailure, width int, styles goTestStyles) {
	if len(failures) == 0 {
		return
	}
	b.WriteString("\n")
	for _, failure := range failures {
		shortPkg := shortenPackageName(failure.pkg)
		_, _ = fmt.Fprintf(b, " %s %s\n", styles.fail.Render("✗"), styles.pkg.Render(shortPkg))
		renderFailedTests(b, failure.failedTests, width, styles)
	}
}

func shortenPackageName(pkg string) string {
	if pkgParts := strings.Split(pkg, "/"); len(pkgParts) > 2 {
		return ".../" + strings.Join(pkgParts[len(pkgParts)-2:], "/")
	}
	return pkg
}

func renderFailedTests(b *strings.Builder, failedTests []string, width int, styles goTestStyles) {
	if len(failedTests) == 0 {
		_, _ = fmt.Fprintf(b, "   %s\n", styles.muted.Render("(build/import error)"))
		return
	}
	for _, testName := range failedTests {
		displayName := humanizeTestNameWithSubtest(testName)
		maxNameWidth := width - 3
		if len(displayName) > maxNameWidth && maxNameWidth > 20 {
			displayName = truncateAtWord(displayName, maxNameWidth-3) + "..."
		}
		_, _ = fmt.Fprintf(b, "   %s\n", styles.test.Render(displayName))
	}
}

func appendDebugLines(b *strings.Builder, debugLines []string) {
	if len(debugLines) == 0 {
		return
	}
	b.WriteString("\n\n")
	for _, line := range debugLines {
		b.WriteString(line + "\n")
	}
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

	accum := initializeSubsystemAccumulators(subsystems)

	for pkg, pr := range packages {
		subsystem := getSubsystemForPackage(pkg)
		a := ensureSubsystemAccumulator(accum, subsystem)
		updateSubsystemCoverage(a, pr)
		status := inferPackageStatus(pr)
		recordSubsystemStatus(a, status, pkg, pr)
	}

	return buildSubsystemResults(subsystems, accum)
}

type subsystemAccumulator struct {
	totalCoverage float64
	coverageCount int
	passedCount   int
	failedCount   int
	failures      []pkgFailure
}

func initializeSubsystemAccumulators(subsystems []SubsystemConfig) map[string]*subsystemAccumulator {
	accum := make(map[string]*subsystemAccumulator)
	for _, ss := range subsystems {
		accum[ss.Name] = &subsystemAccumulator{}
	}
	return accum
}

func ensureSubsystemAccumulator(accum map[string]*subsystemAccumulator, subsystem string) *subsystemAccumulator {
	if _, ok := accum[subsystem]; !ok {
		accum[subsystem] = &subsystemAccumulator{}
	}
	return accum[subsystem]
}

func updateSubsystemCoverage(accum *subsystemAccumulator, pr *pkgResult) {
	if pr.coverage <= 0 {
		return
	}
	accum.totalCoverage += pr.coverage
	accum.coverageCount++
}

func inferPackageStatus(pr *pkgResult) string {
	status := pr.status
	if status != statusRun || len(pr.tests) == 0 {
		return status
	}

	hasFailed, allDone := packageTestStatus(pr.tests)
	if !allDone {
		return status
	}
	if hasFailed {
		return statusFail
	}
	return statusPass
}

func packageTestStatus(tests map[string]*testResult) (hasFailed bool, allDone bool) {
	allDone = true
	for _, tr := range tests {
		if tr.status == statusFail {
			hasFailed = true
		}
		if tr.status == statusRun {
			allDone = false
		}
	}
	return hasFailed, allDone
}

func recordSubsystemStatus(accum *subsystemAccumulator, status, pkg string, pr *pkgResult) {
	switch status {
	case statusPass:
		accum.passedCount++
	case statusFail:
		accum.failedCount++
		accum.failures = append(accum.failures, pkgFailure{
			pkg:         pkg,
			failedTests: collectFailedTests(pr),
		})
	}
}

func collectFailedTests(pr *pkgResult) []string {
	var failedTests []string
	for _, testName := range pr.testOrder {
		if tr, ok := pr.tests[testName]; ok && tr.status == statusFail {
			failedTests = append(failedTests, testName)
		}
	}
	return failedTests
}

func buildSubsystemResults(subsystems []SubsystemConfig, accum map[string]*subsystemAccumulator) []subsystemResult {
	results := make([]subsystemResult, 0, len(subsystems))
	for _, ss := range subsystems {
		a := accum[ss.Name]
		results = append(results, subsystemResult{
			name:        ss.Name,
			avgCoverage: averageCoverage(a),
			pkgCount:    a.coverageCount,
			passedCount: a.passedCount,
			failedCount: a.failedCount,
			failures:    a.failures,
		})
	}
	return results
}

func averageCoverage(accum *subsystemAccumulator) float64 {
	if accum.coverageCount == 0 {
		return 0
	}
	return accum.totalCoverage / float64(accum.coverageCount)
}

// GetStatus implements StatusIndicator for content-aware menu icons.
// This ensures go test results are evaluated based on actual test outcomes,
// not just exit codes (which can be confused by output parsing issues).
func (f *GoTestFormatter) GetStatus(lines []string) IndicatorStatus {
	packages, pkgOrder := parseGoTestEvents(lines)
	totals := countTestTotals(packages, pkgOrder)

	if totals.failed > 0 {
		return IndicatorError
	}
	// If tests are still running, don't override the exit code
	if totals.running > 0 {
		return IndicatorDefault
	}
	// All tests passed or skipped
	if totals.passed > 0 || totals.skipped > 0 {
		return IndicatorSuccess
	}
	// No tests found - defer to exit code
	return IndicatorDefault
}
