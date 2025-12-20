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
	&RaceFormatter{}, // Must be before GoTestFormatter (more specific match)
	&GoTestFormatter{},
	&FilesizeDashboardFormatter{}, // Must be before SARIF to match dashboard format
	&MCPErrorsFormatter{},         // mcp-errors -format=dashboard output
	&NugstatsFormatter{},          // nugstats -format=dashboard output
	&GolangciLintFormatter{},      // Per-linter sections for golangci-lint
	&GofmtFormatter{},             // gofmt -l output
	&GoVetFormatter{},             // go vet output
	&GoBuildFormatter{},           // go build output
	&GoArchLintFormatter{},        // go-arch-lint output
	&NilawayFormatter{},           // nilaway -json output
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

// Status constants for test/lint results.
const (
	statusRun     = "run"
	statusPass    = "pass"
	statusFail    = "fail"
	statusSkip    = "skip"
	statusError   = "error"
	statusWarning = "warning"
)

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
	return goTestStyles{
		pass:    lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true),
		fail:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true),
		skip:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")),
		run:     lipgloss.NewStyle().Foreground(lipgloss.Color("#48CAE4")),
		pkg:     lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true),
		test:    lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")),
		muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")),
		pending: lipgloss.NewStyle().Foreground(lipgloss.Color("#FFDD88")),
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
	case "output":
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
	case "output":
		recordCoverage(pr, event.Output)
	}
}

func recordCoverage(pr *pkgResult, output string) {
	if !strings.HasPrefix(output, "coverage:") {
		return
	}
	var cov float64
	_, _ = fmt.Sscanf(output, "coverage: %f%%", &cov)
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

func renderSubsystems(b *strings.Builder, subsystems []subsystemResult, width int, styles goTestStyles) []pkgFailure {
	maxNameLen := maxSubsystemNameLength(subsystems)
	var allFailures []pkgFailure
	for _, ss := range subsystems {
		totalPkgs := ss.passedCount + ss.failedCount
		nameField := fmt.Sprintf("%-*s", maxNameLen, ss.name)
		icon := subsystemIcon(totalPkgs, ss.failedCount, ss.pkgCount, styles)
		coverageStr := subsystemCoverage(totalPkgs, ss.avgCoverage, styles)

		b.WriteString(fmt.Sprintf("  %s %s%s\n", icon, nameField, coverageStr))
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
		b.WriteString(fmt.Sprintf(" %s %s\n", styles.fail.Render("✗"), styles.pkg.Render(shortPkg)))
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
		b.WriteString(fmt.Sprintf("   %s\n", styles.muted.Render("(build/import error)")))
		return
	}
	for _, testName := range failedTests {
		displayName := humanizeTestNameWithSubtest(testName)
		maxNameWidth := width - 3
		if len(displayName) > maxNameWidth && maxNameWidth > 20 {
			displayName = truncateAtWord(displayName, maxNameWidth-3) + "..."
		}
		b.WriteString(fmt.Sprintf("   %s\n", styles.test.Render(displayName)))
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

// ============================================================================
// Golangci-lint Formatter (per-linter sections with smart rendering)
// ============================================================================

// Formatting constants for golangci-lint output.
const (
	lintItemsPerSection  = 5  // max items shown per linter section
	lintComplexityWarn   = 30 // complexity threshold for red highlighting
	lintMsgTruncateLen   = 50 // max message length before truncation
	lintFileListMaxLen   = 40 // max length for file list in goconst
	lintFuncNameColWidth = 24 // column width for function names
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
			if iss.level == statusError {
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

// Gocyclo display limits.
const (
	gocycloMaxItems     = 15 // max items to show
	gocycloFileColWidth = 20 // column width for filenames
)

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

	// Calculate max filename width for alignment
	maxFileLen := 0
	for i, item := range items {
		if i >= gocycloMaxItems {
			break
		}
		if len(shortPath(item.file)) > maxFileLen {
			maxFileLen = len(shortPath(item.file))
		}
	}

	// Format: "54  formatter.go         (*GoTestFormatter).Format"
	for i, item := range items {
		if i >= gocycloMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(items)-gocycloMaxItems)))
			break
		}
		scoreStyle := warnStyle
		if item.complexity > lintComplexityWarn {
			scoreStyle = errorStyle
		}
		filename := shortPath(item.file)
		// Pad filename before styling to ensure alignment
		paddedFilename := fmt.Sprintf("%-*s", maxFileLen, filename)
		_, _ = fmt.Fprintf(b, "  %s  %s  %s\n",
			scoreStyle.Render(fmt.Sprintf("%2d", item.complexity)),
			fileStyle.Render(paddedFilename),
			mutedStyle.Render(item.funcName))
	}
}

// Goconst display limits.
const (
	goconstMaxItems      = 15 // max items to show
	goconstLiteralWidth  = 16 // column width for quoted literals
	goconstFileListWidth = 40 // max width for file list
)

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
		_, _ = fmt.Sscanf(iss.message, "string `"+literal+"` has %d occurrences", &count)

		if byLiteral[literal] == nil {
			byLiteral[literal] = &constItem{literal: literal, count: count}
		}
		// Dedupe files
		filename := shortPath(iss.file)
		found := false
		for _, f := range byLiteral[literal].files {
			if f == filename {
				found = true
				break
			}
		}
		if !found {
			byLiteral[literal].files = append(byLiteral[literal].files, filename)
		}
	}

	items := make([]*constItem, 0, len(byLiteral))
	for _, item := range byLiteral {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})

	// Calculate max literal width for alignment
	maxLiteralLen := 0
	for i, item := range items {
		if i >= goconstMaxItems {
			break
		}
		quoted := fmt.Sprintf("%q", item.literal)
		if len(quoted) > goconstLiteralWidth {
			quoted = quoted[:goconstLiteralWidth-3] + "...\""
		}
		if len(quoted) > maxLiteralLen {
			maxLiteralLen = len(quoted)
		}
	}

	// Format: " 9x "fail"          formatter.go, housekeeping.go"
	for i, item := range items {
		if i >= goconstMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(items)-goconstMaxItems)))
			break
		}
		// Quoted literal, truncate if needed
		quoted := fmt.Sprintf("%q", item.literal)
		if len(quoted) > goconstLiteralWidth {
			quoted = quoted[:goconstLiteralWidth-3] + "...\""
		}
		// Pad before styling
		paddedQuoted := fmt.Sprintf("%-*s", maxLiteralLen, quoted)

		files := strings.Join(item.files, ", ")
		if len(files) > goconstFileListWidth {
			files = files[:goconstFileListWidth-3] + "..."
		}

		_, _ = fmt.Fprintf(b, "  %s %s  %s\n",
			mutedStyle.Render(fmt.Sprintf("%2dx", item.count)),
			mutedStyle.Render(paddedQuoted),
			fileStyle.Render(files))
	}
}

// Default linter display limits.
const (
	defaultMaxItems  = 15 // max items to show
	defaultMsgMaxLen = 70 // max message length before truncation
)

// renderDefault renders issues as a two-line format: file:line then message.
func (f *GolangciLintFormatter) renderDefault(b *strings.Builder, issues []lintIssue, fileStyle, mutedStyle, errorStyle, warnStyle lipgloss.Style) {
	for i, iss := range issues {
		if i >= defaultMaxItems {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-defaultMaxItems)))
			break
		}
		icon := mutedStyle.Render("·")
		switch iss.level {
		case statusError:
			icon = errorStyle.Render("✗")
		case statusWarning:
			icon = warnStyle.Render("△")
		}
		msg := iss.message
		if len(msg) > defaultMsgMaxLen {
			msg = msg[:defaultMsgMaxLen-3] + "..."
		}
		// Line 1: icon + file:line
		_, _ = fmt.Fprintf(b, "  %s %s\n",
			icon,
			fileStyle.Render(fmt.Sprintf("%s:%d", shortPath(iss.file), iss.line)))
		// Line 2: indented message
		_, _ = fmt.Fprintf(b, "    %s\n", mutedStyle.Render(msg))
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
// Gofmt Formatter (handles gofmt -l output)
// ============================================================================

type GofmtFormatter struct{}

func (f *GofmtFormatter) Matches(command string) bool {
	return strings.Contains(command, "gofmt")
}

func (f *GofmtFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))

	// Filter to non-empty lines (actual files)
	var files []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && strings.HasSuffix(trimmed, ".go") {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		b.WriteString(successStyle.Render("✓ All files formatted correctly\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d files need formatting:", len(files))))
	b.WriteString("\n\n")

	for _, file := range files {
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(file)))
	}

	return b.String()
}

// ============================================================================
// Go Vet Formatter (handles go vet output)
// ============================================================================

type GoVetFormatter struct{}

func (f *GoVetFormatter) Matches(command string) bool {
	return strings.Contains(command, "go vet")
}

func (f *GoVetFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Filter to non-empty lines
	var issues []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			issues = append(issues, trimmed)
		}
	}

	if len(issues) == 0 {
		b.WriteString(successStyle.Render("✓ No issues found\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d issues:", len(issues))))
	b.WriteString("\n\n")

	for i, issue := range issues {
		if i >= 15 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-15)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(issue)))
	}

	return b.String()
}

// ============================================================================
// Go Build Formatter (handles go build output)
// ============================================================================

type GoBuildFormatter struct{}

func (f *GoBuildFormatter) Matches(command string) bool {
	return strings.Contains(command, "go build")
}

func (f *GoBuildFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Filter to non-empty lines (errors)
	var errors []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			errors = append(errors, trimmed)
		}
	}

	if len(errors) == 0 {
		b.WriteString(successStyle.Render("✓ Build successful\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render("✗ Build failed:"))
	b.WriteString("\n\n")

	for i, err := range errors {
		if i >= 20 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(errors)-20)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(err)))
	}

	return b.String()
}

// ============================================================================
// Go Arch Lint Formatter (handles go-arch-lint --json output)
// ============================================================================

type GoArchLintFormatter struct{}

func (f *GoArchLintFormatter) Matches(command string) bool {
	return strings.Contains(command, "go-arch-lint")
}

// archLintReport represents the go-arch-lint JSON output structure.
type archLintReport struct {
	Type    string `json:"Type"`
	Payload struct {
		ArchHasWarnings  bool `json:"ArchHasWarnings"`
		ArchWarningsDeps []struct {
			ComponentFrom string `json:"ComponentFrom"`
			ComponentTo   string `json:"ComponentTo"`
			FileRelPath   string `json:"FileRelativePath"`
		} `json:"ArchWarningsDeps"`
		ArchWarningsNotMatched []struct {
			FileRelPath string `json:"FileRelativePath"`
		} `json:"ArchWarningsNotMatched"`
		ArchWarningsDeepScan []struct {
			Gate        string `json:"Gate"`
			ComponentTo string `json:"ComponentTo"`
			FileRelPath string `json:"FileRelativePath"`
		} `json:"ArchWarningsDeepScan"`
		OmittedCount int `json:"OmittedCount"`
	} `json:"Payload"`
}

func (f *GoArchLintFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Parse JSON
	fullOutput := strings.Join(lines, "\n")
	var report archLintReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Check for warnings
	depCount := len(report.Payload.ArchWarningsDeps)
	unmatchedCount := len(report.Payload.ArchWarningsNotMatched)
	deepScanCount := len(report.Payload.ArchWarningsDeepScan)
	totalIssues := depCount + unmatchedCount + deepScanCount

	if !report.Payload.ArchHasWarnings || totalIssues == 0 {
		b.WriteString(successStyle.Render("✓ No architecture violations\n"))
		return b.String()
	}

	// Summary
	b.WriteString(warnStyle.Render(fmt.Sprintf("△ %d architecture issues", totalIssues)))
	b.WriteString("\n\n")

	// Dependency violations
	if depCount > 0 {
		b.WriteString(headerStyle.Render("◉ Dependency Violations"))
		b.WriteString(errorStyle.Render(fmt.Sprintf(" (%d)", depCount)))
		b.WriteString("\n")
		for i, dep := range report.Payload.ArchWarningsDeps {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", depCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				warnStyle.Render(dep.ComponentFrom),
				errorStyle.Render(dep.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", fileStyle.Render(shortPath(dep.FileRelPath))))
		}
		b.WriteString("\n")
	}

	// Unmatched files
	if unmatchedCount > 0 {
		b.WriteString(headerStyle.Render("◉ Unmatched Files"))
		b.WriteString(warnStyle.Render(fmt.Sprintf(" (%d)", unmatchedCount)))
		b.WriteString("\n")
		for i, um := range report.Payload.ArchWarningsNotMatched {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", unmatchedCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(um.FileRelPath)))
		}
		b.WriteString("\n")
	}

	// Deep scan violations
	if deepScanCount > 0 {
		b.WriteString(headerStyle.Render("◉ Deep Scan Violations"))
		b.WriteString(errorStyle.Render(fmt.Sprintf(" (%d)", deepScanCount)))
		b.WriteString("\n")
		for i, ds := range report.Payload.ArchWarningsDeepScan {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", deepScanCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				warnStyle.Render(ds.Gate),
				errorStyle.Render(ds.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", fileStyle.Render(shortPath(ds.FileRelPath))))
		}
	}

	return b.String()
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
			if result.Level == statusError {
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
		if iss.level == statusError {
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
	Timestamp string                   `json:"timestamp"`
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
// Nilaway Formatter (handles nilaway -json output)
// ============================================================================

type NilawayFormatter struct{}

func (f *NilawayFormatter) Matches(command string) bool {
	return strings.Contains(command, "nilaway")
}

// nilawayFinding represents a single nilaway finding from JSON.
type nilawayFinding struct {
	Posn    string `json:"posn"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

// nilawayAnalyzerResult represents the nilaway analyzer output within a package.
type nilawayAnalyzerResult struct {
	Nilaway []nilawayFinding `json:"nilaway"`
}

func (f *NilawayFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	messageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	reasonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Combine lines and parse JSON
	combined := strings.Join(lines, "\n")
	if strings.TrimSpace(combined) == "" {
		b.WriteString(successStyle.Render("✓ No nil pointer issues found\n"))
		return b.String()
	}

	// Try parsing as nested format: {"pkg": {"nilaway": [...]}}
	var nested map[string]nilawayAnalyzerResult
	var allFindings []nilawayFinding

	if err := json.Unmarshal([]byte(combined), &nested); err == nil {
		for _, ar := range nested {
			allFindings = append(allFindings, ar.Nilaway...)
		}
	}

	if len(allFindings) == 0 {
		// Check if output looks like an error or empty result
		if strings.Contains(combined, "error") || strings.Contains(combined, "Error") {
			b.WriteString(errorStyle.Render("✗ nilaway encountered errors:\n\n"))
			b.WriteString(messageStyle.Render(combined))
			return b.String()
		}
		b.WriteString(successStyle.Render("✓ No nil pointer issues found\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d potential nil pointer issues:", len(allFindings))))
	b.WriteString("\n\n")

	// Group by file for better display
	byFile := make(map[string][]nilawayFinding)
	fileOrder := []string{}
	for _, finding := range allFindings {
		// Extract file from posn (format: "file.go:line:col")
		file := finding.Posn
		if idx := strings.Index(finding.Posn, ":"); idx > 0 {
			file = finding.Posn[:idx]
		}
		if _, exists := byFile[file]; !exists {
			fileOrder = append(fileOrder, file)
		}
		byFile[file] = append(byFile[file], finding)
	}

	displayed := 0
	maxDisplay := 15

	for _, file := range fileOrder {
		if displayed >= maxDisplay {
			remaining := len(allFindings) - displayed
			b.WriteString(mutedStyle.Render(fmt.Sprintf("\n  ... and %d more issues\n", remaining)))
			break
		}

		findings := byFile[file]
		b.WriteString(fileStyle.Render(file))
		b.WriteString("\n")

		for _, finding := range findings {
			if displayed >= maxDisplay {
				break
			}

			// Extract line:col from posn
			loc := ""
			if parts := strings.SplitN(finding.Posn, ":", 3); len(parts) >= 2 {
				loc = parts[1]
				if len(parts) == 3 {
					loc = parts[1] + ":" + parts[2]
				}
			}

			b.WriteString(fmt.Sprintf("  %s %s\n", mutedStyle.Render(loc+":"), messageStyle.Render(finding.Message)))
			if finding.Reason != "" {
				b.WriteString(fmt.Sprintf("      %s\n", reasonStyle.Render(finding.Reason)))
			}
			displayed++
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ============================================================================
// Race Detector Formatter (handles go test -race output)
// ============================================================================

type RaceFormatter struct{}

func (f *RaceFormatter) Matches(command string) bool {
	return strings.Contains(command, "go test") && strings.Contains(command, "-race")
}

func (f *RaceFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6"))
	funcStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Parse JSON events and extract race warnings
	var races []string
	var testsPassed, testsFailed, testsSkipped int
	var raceDetected bool

	for _, line := range lines {
		if line == "" {
			continue
		}

		var event GoTestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not JSON, check for race warning
			if strings.Contains(line, "WARNING: DATA RACE") {
				raceDetected = true
			}
			continue
		}

		// Check for race warnings in output
		if strings.Contains(event.Output, "WARNING: DATA RACE") {
			raceDetected = true
		}

		// Collect race-related output
		if event.Action == "output" && event.Test == "" {
			output := strings.TrimSpace(event.Output)
			if strings.Contains(output, "DATA RACE") ||
				strings.Contains(output, "Read at") ||
				strings.Contains(output, "Write at") ||
				strings.Contains(output, "Previous write") ||
				strings.Contains(output, "Previous read") ||
				strings.Contains(output, "Goroutine") ||
				(strings.HasPrefix(output, "  ") && strings.Contains(output, ".go:")) {
				races = append(races, output)
			}
		}

		// Track test results
		if event.Action == "pass" && event.Test != "" {
			testsPassed++
		} else if event.Action == "fail" && event.Test != "" {
			testsFailed++
		} else if event.Action == "skip" && event.Test != "" {
			testsSkipped++
		}
	}

	// Build summary
	if raceDetected {
		b.WriteString(errorStyle.Render("✗ DATA RACE DETECTED"))
		b.WriteString("\n\n")

		// Show race details
		for i, line := range races {
			if i >= 30 { // Limit output
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more lines\n", len(races)-30)))
				break
			}

			// Style different parts of race output
			if strings.Contains(line, "DATA RACE") {
				b.WriteString(warningStyle.Render(line))
			} else if strings.Contains(line, ".go:") {
				b.WriteString(fileStyle.Render(line))
			} else if strings.Contains(line, "Read at") || strings.Contains(line, "Write at") ||
				strings.Contains(line, "Previous") {
				b.WriteString(funcStyle.Render(line))
			} else {
				b.WriteString(mutedStyle.Render(line))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString(successStyle.Render("✓ No data races detected"))
		b.WriteString("\n")
	}

	// Show test summary
	if testsPassed+testsFailed+testsSkipped > 0 {
		b.WriteString("\n")
		total := testsPassed + testsFailed + testsSkipped
		if testsFailed > 0 {
			b.WriteString(errorStyle.Render(fmt.Sprintf("Tests: %d passed, %d failed", testsPassed, testsFailed)))
		} else {
			b.WriteString(successStyle.Render(fmt.Sprintf("Tests: %d passed", testsPassed)))
		}
		if testsSkipped > 0 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf(", %d skipped", testsSkipped)))
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf(" (total: %d)", total)))
		b.WriteString("\n")
	}

	return b.String()
}

// ============================================================================
// MCP Errors Formatter (mcp-errors -format=dashboard output)
// ============================================================================

type MCPErrorsFormatter struct{}

func (f *MCPErrorsFormatter) Matches(command string) bool {
	return strings.Contains(command, "mcp-errors") && strings.Contains(command, "-format=dashboard")
}

// MCPErrorsReport matches the JSON output from mcp-errors -format=dashboard.
type MCPErrorsReport struct {
	Timestamp  string           `json:"timestamp"`
	LogFiles   []string         `json:"log_files"`
	ErrorCount int              `json:"error_count"`
	WarnCount  int              `json:"warn_count"`
	Errors     []MCPErrorDetail `json:"errors"`
}

type MCPErrorDetail struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

func (f *MCPErrorsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	fullOutput := strings.Join(lines, "\n")
	var report MCPErrorsReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Header with summary
	b.WriteString(headerStyle.Render("◉ MCP Server Logs"))
	b.WriteString("\n\n")

	if report.ErrorCount == 0 && report.WarnCount == 0 {
		b.WriteString(successStyle.Render("✓ No errors or warnings"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary counts
	if report.ErrorCount > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %d errors", report.ErrorCount)))
	} else {
		b.WriteString(successStyle.Render("  0 errors"))
	}
	b.WriteString("  ")
	if report.WarnCount > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf("%d warnings", report.WarnCount)))
	} else {
		b.WriteString(mutedStyle.Render("0 warnings"))
	}
	b.WriteString("\n\n")

	// Show recent errors (up to 5)
	if len(report.Errors) > 0 {
		b.WriteString(headerStyle.Render("Recent Errors"))
		b.WriteString("\n")
		shown := 0
		for i := len(report.Errors) - 1; i >= 0 && shown < 5; i-- {
			e := report.Errors[i]
			levelStyle := errorStyle
			if e.Level == "WARN" {
				levelStyle = warnStyle
			}
			msg := e.Message
			if len(msg) > width-15 && width > 18 {
				msg = msg[:width-18] + "..."
			}
			b.WriteString(fmt.Sprintf("  %s %s\n",
				levelStyle.Render("["+e.Level+"]"),
				msg))
			if e.Detail != "" {
				detail := e.Detail
				if len(detail) > width-12 && width > 15 {
					detail = detail[:width-15] + "..."
				}
				b.WriteString(fmt.Sprintf("         %s\n", detailStyle.Render(detail)))
			}
			shown++
		}
		if len(report.Errors) > 5 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(report.Errors)-5)))
		}
	}

	return b.String()
}

// ============================================================================
// Nugstats Formatter (nugstats -format=dashboard output)
// ============================================================================

type NugstatsFormatter struct{}

func (f *NugstatsFormatter) Matches(command string) bool {
	return strings.Contains(command, "nugstats") && strings.Contains(command, "-format=dashboard")
}

// NugstatsReport matches the JSON output from nugstats -format=dashboard.
type NugstatsReport struct {
	Timestamp string              `json:"timestamp"`
	Total     int                 `json:"total"`
	ByKind    []NugstatsKindCount `json:"by_kind"`
	Weekly    []NugstatsWeekly    `json:"weekly"`
}

type NugstatsKindCount struct {
	Kind     string `json:"kind"`
	Count    int    `json:"count"`
	ThisWeek int    `json:"this_week"`
	Delta    int    `json:"delta"`
}

type NugstatsWeekly struct {
	Week  string `json:"week"`
	Added int    `json:"added"`
}

func (f *NugstatsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	fullOutput := strings.Join(lines, "\n")
	var report NugstatsReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Styles
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	kindStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	deltaUpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	deltaDownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))

	// Header
	b.WriteString(headerStyle.Render("◉ Knowledge Graph"))
	b.WriteString("  ")
	b.WriteString(countStyle.Render(fmt.Sprintf("%d nuggets", report.Total)))
	b.WriteString("\n\n")

	// By kind - show all, with right-aligned columns
	b.WriteString(headerStyle.Render("By Kind"))
	b.WriteString("\n")

	// Find max width for kind names and counts for alignment
	maxKindLen := 0
	maxCount := 0
	for _, k := range report.ByKind {
		if len(k.Kind) > maxKindLen {
			maxKindLen = len(k.Kind)
		}
		if k.Count > maxCount {
			maxCount = k.Count
		}
	}
	countWidth := len(fmt.Sprintf("%d", maxCount))
	if countWidth < 3 {
		countWidth = 3
	}

	for _, k := range report.ByKind {
		delta := ""
		if k.Delta > 0 {
			delta = deltaUpStyle.Render(fmt.Sprintf(" ↑%d", k.Delta))
		} else if k.Delta < 0 {
			delta = deltaDownStyle.Render(fmt.Sprintf(" ↓%d", -k.Delta))
		}
		// Right-align count column
		b.WriteString(fmt.Sprintf("  %-*s  %s%s\n",
			maxKindLen,
			kindStyle.Render(k.Kind),
			countStyle.Render(fmt.Sprintf("%*d", countWidth, k.Count)),
			delta))
	}

	// Horizontal stacked bar proportional to kind counts
	if len(report.ByKind) > 0 && report.Total > 0 {
		b.WriteString("\n  ")
		barWidth := width - 6 // account for padding
		if barWidth < 20 {
			barWidth = 20
		}
		if barWidth > 60 {
			barWidth = 60
		}

		// Color palette for kinds (cycle through)
		colors := []lipgloss.Color{
			"#04B575", "#0077B6", "#FFBD2E", "#FF5F56",
			"#9B59B6", "#3498DB", "#E67E22", "#1ABC9C",
		}

		// Build proportional segments
		for i, k := range report.ByKind {
			segmentWidth := (k.Count * barWidth) / report.Total
			if segmentWidth == 0 && k.Count > 0 {
				segmentWidth = 1
			}
			color := colors[i%len(colors)]
			style := lipgloss.NewStyle().Background(color)
			b.WriteString(style.Render(strings.Repeat(" ", segmentWidth)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ============================================================================
// Plain Formatter (fallback)
// ============================================================================

type PlainFormatter struct{}

func (f *PlainFormatter) Matches(_ string) bool {
	return true // always matches as fallback
}

func (f *PlainFormatter) Format(lines []string, _ int) string {
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
