package fo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

// TestResult represents a single test with its status.
type TestResult struct {
	Name   string
	Status string // "PASS", "FAIL", "SKIP"
}

// TestPackageResult represents the results for a single test package.
type TestPackageResult struct {
	Name        string
	Passed      int
	Failed      int
	Skipped     int
	Duration    time.Duration
	Coverage    float64
	FailedTests []string
	AllTests    []TestResult // All tests with their status (when ShowAllTests is enabled)
}

// TestTableConfig configures how test results are rendered.
type TestTableConfig struct {
	// SparkbarFilled is the character used for filled portions of the sparkbar
	SparkbarFilled string
	// SparkbarEmpty is the character used for empty portions of the sparkbar
	SparkbarEmpty string
	// SparkbarLength is the number of characters in the sparkbar
	SparkbarLength int
	// CoverageThresholds defines the coverage ranges for color coding
	CoverageThresholds []CoverageThreshold
	// ShowPercentage controls whether to show percentage after sparkbar
	ShowPercentage bool
	// UseTreeChars controls whether to use tree characters (├─, └─)
	UseTreeChars bool
	// NoTestIcon is the icon to use for packages with no tests
	NoTestIcon string
	// NoTestColor is the color key for packages with no tests
	NoTestColor string
	// ShowAllTests controls whether to show all tests (including passed) with their status
	ShowAllTests bool
	// SubtestConfig controls subtest hierarchy rendering
	SubtestConfig SubtestConfig
	// ProjectLayout configures how to extract group names from package paths
	ProjectLayout ProjectLayout
}

// SubtestConfig configures how subtests are rendered.
type SubtestConfig struct {
	GroupByParent    bool
	IndentSize       int
	ShowParentStatus bool
	HumanizeNames    bool
}

// ProjectLayout configures how to extract group names from package paths.
type ProjectLayout struct {
	TopDirs      []string
	ModulePrefix string
	GroupFunc    func(pkgPath string) string
}

// CoverageThreshold defines a coverage range and its associated color.
type CoverageThreshold struct {
	Min      float64
	Max      float64
	ColorKey string
}

// TestRenderer renders test results in a clean tree-style format.
type TestRenderer struct {
	console      *Console
	writer       io.Writer
	config       TestTableConfig
	width        int
	currentGroup string
	groupPkgs    []TestPackageResult // packages in current group
	allGroups    []groupData         // all groups for final render
}

type groupData struct {
	name     string
	packages []TestPackageResult
}

// NewTestRenderer creates a new test renderer.
func NewTestRenderer(console *Console, writer io.Writer) *TestRenderer {
	config := buildTestTableConfig(console)

	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}

	return &TestRenderer{
		console: console,
		writer:  writer,
		config:  config,
		width:   width,
	}
}

// buildTestTableConfig builds test table configuration from the console's design config.
func buildTestTableConfig(console *Console) TestTableConfig {
	cfg := console.designConf

	thresholds := []CoverageThreshold{
		{Min: cfg.Tests.CoverageGoodMin, Max: 100, ColorKey: "GreenFg"},
		{Min: cfg.Tests.CoverageWarningMin, Max: cfg.Tests.CoverageGoodMin - 0.1, ColorKey: "Warning"},
		{Min: 0, Max: cfg.Tests.CoverageWarningMin - 0.1, ColorKey: "Error"},
	}

	return TestTableConfig{
		SparkbarFilled:     cfg.Tests.SparkbarFilled,
		SparkbarEmpty:      cfg.Tests.SparkbarEmpty,
		SparkbarLength:     cfg.Tests.SparkbarLength,
		CoverageThresholds: thresholds,
		ShowPercentage:     cfg.Tests.ShowPercentage,
		UseTreeChars:       true,
		NoTestIcon:         cfg.Tests.NoTestIcon,
		NoTestColor:        cfg.Tests.NoTestColor,
		SubtestConfig: SubtestConfig{
			GroupByParent:    true,
			IndentSize:       4,
			ShowParentStatus: false,
			HumanizeNames:    true,
		},
		ProjectLayout: ProjectLayout{
			TopDirs:      []string{"internal", "cmd", "pkg", "examples"},
			ModulePrefix: detectModulePrefix(),
		},
	}
}

// GetConfig returns the current test table configuration.
func (r *TestRenderer) GetConfig() TestTableConfig {
	return r.config
}

// SetConfig updates the test table configuration.
func (r *TestRenderer) SetConfig(config TestTableConfig) {
	r.config = config
}

// RenderTableHeader is now a no-op - header rendered in RenderSummary.
func (r *TestRenderer) RenderTableHeader() {
	// No-op: we render everything at the end now
}

// RenderGroupHeader starts collecting packages for a group.
func (r *TestRenderer) RenderGroupHeader(dirName string) {
	// Save previous group if exists
	if r.currentGroup != "" && len(r.groupPkgs) > 0 {
		r.allGroups = append(r.allGroups, groupData{
			name:     r.currentGroup,
			packages: r.groupPkgs,
		})
	}
	r.currentGroup = dirName
	r.groupPkgs = nil
}

// RenderPackageLine adds a package to the current group.
func (r *TestRenderer) RenderPackageLine(pkg TestPackageResult) {
	r.groupPkgs = append(r.groupPkgs, pkg)
}

// RenderGroupFooter finishes the current group.
func (r *TestRenderer) RenderGroupFooter() {
	if r.currentGroup != "" && len(r.groupPkgs) > 0 {
		r.allGroups = append(r.allGroups, groupData{
			name:     r.currentGroup,
			packages: r.groupPkgs,
		})
	}
	r.currentGroup = ""
	r.groupPkgs = nil
}

// RenderAll outputs the complete test summary.
// Call this after all groups have been added.
func (r *TestRenderer) RenderAll() {
	// Calculate totals
	var totalPassed, totalFailed, totalSkipped int
	var totalDuration time.Duration

	for _, g := range r.allGroups {
		for _, pkg := range g.packages {
			totalPassed += pkg.Passed
			totalFailed += pkg.Failed
			totalSkipped += pkg.Skipped
			totalDuration += pkg.Duration
		}
	}

	// Get colors
	successColor := r.console.GetSuccessColor()
	errorColor := r.console.GetErrorColor()
	mutedColor := r.console.GetMutedColor()
	reset := r.console.ResetColor()

	// Build all lines into a buffer
	var lines []string

	// Summary line
	if totalFailed > 0 {
		lines = append(lines, fmt.Sprintf("%sPassed: %d%s  |  %sFailed: %d%s  |  Duration: %s",
			successColor, totalPassed, reset,
			errorColor, totalFailed, reset,
			r.formatDuration(totalDuration)))
	} else {
		lines = append(lines, fmt.Sprintf("%sPassed: %d%s  |  Failed: 0  |  Duration: %s",
			successColor, totalPassed, reset,
			r.formatDuration(totalDuration)))
	}
	lines = append(lines, "")

	// Column header
	lines = append(lines, fmt.Sprintf("%sSTATUS   PACKAGE                    TESTS      TIME     COVERAGE%s",
		mutedColor, reset))
	lines = append(lines, fmt.Sprintf("%s────────────────────────────────────────────────────────────────────%s",
		mutedColor, reset))

	// Render each group
	for _, g := range r.allGroups {
		lines = append(lines, r.renderGroupLines(g)...)
	}

	// Footer line
	lines = append(lines, fmt.Sprintf("%s────────────────────────────────────────────────────────────────────%s",
		mutedColor, reset))

	// Final status
	if totalFailed > 0 {
		lines = append(lines, fmt.Sprintf("%s✗%s  TESTS FAILED", errorColor, reset))
	} else {
		lines = append(lines, fmt.Sprintf("%s✓%s  ALL TESTS PASSED", successColor, reset))
	}

	// Render as a complete box using the console's section system
	r.console.PrintSectionHeader("Tests")
	for _, line := range lines {
		r.console.PrintSectionLine(line)
	}
	r.console.PrintSectionFooter()
}

func (r *TestRenderer) renderGroupLines(g groupData) []string {
	mutedColor := r.console.GetMutedColor()
	reset := r.console.ResetColor()

	var lines []string

	// Check if this is a multi-package group (needs tree structure)
	if len(g.packages) > 1 || r.needsGroupHeader(g) {
		// Group header (directory name)
		lines = append(lines, fmt.Sprintf("         %s%s/%s", mutedColor, g.name, reset))

		// Render packages with tree chars
		for i, pkg := range g.packages {
			isLast := i == len(g.packages)-1
			lines = append(lines, r.renderPackageWithTreeLine(pkg, isLast))
			// Add failed tests if any
			lines = append(lines, r.renderFailedTestLines(pkg, "        ")...)
		}
		lines = append(lines, "")
	} else if len(g.packages) == 1 {
		// Single package - render flat
		lines = append(lines, r.renderPackageFlatLine(g.packages[0]))
		lines = append(lines, r.renderFailedTestLines(g.packages[0], "     ")...)
	}

	return lines
}

func (r *TestRenderer) needsGroupHeader(g groupData) bool {
	// Show group header for standard Go project directories
	switch g.name {
	case "internal", "pkg", "cmd", "examples":
		return true
	}
	return false
}

func (r *TestRenderer) renderPackageFlatLine(pkg TestPackageResult) string {
	statusIcon, statusColor := r.getStatusIconAndColor(pkg)
	reset := r.console.ResetColor()

	total := pkg.Passed + pkg.Failed + pkg.Skipped
	testCount := fmt.Sprintf("%d/%d", pkg.Passed, total)
	duration := r.formatDurationShort(pkg.Duration)
	sparkbar := r.renderSparkbar(pkg.Coverage)
	coverage := fmt.Sprintf("%3.0f%%", pkg.Coverage)

	return fmt.Sprintf("%s%s%s  %-24s %8s %8s     %s  %s",
		statusColor, statusIcon, reset,
		pkg.Name,
		testCount,
		duration,
		sparkbar,
		coverage)
}

func (r *TestRenderer) renderPackageWithTreeLine(pkg TestPackageResult, isLast bool) string {
	statusIcon, statusColor := r.getStatusIconAndColor(pkg)
	reset := r.console.ResetColor()
	mutedColor := r.console.GetMutedColor()

	treeChar := "├─"
	if isLast {
		treeChar = "└─"
	}

	total := pkg.Passed + pkg.Failed + pkg.Skipped
	testCount := fmt.Sprintf("%d/%d", pkg.Passed, total)
	duration := r.formatDurationShort(pkg.Duration)
	sparkbar := r.renderSparkbar(pkg.Coverage)
	coverage := fmt.Sprintf("%3.0f%%", pkg.Coverage)

	return fmt.Sprintf("%s%s%s  %s%s%s %-20s %8s %8s     %s  %s",
		statusColor, statusIcon, reset,
		mutedColor, treeChar, reset,
		pkg.Name,
		testCount,
		duration,
		sparkbar,
		coverage)
}

func (r *TestRenderer) renderFailedTestLines(pkg TestPackageResult, indent string) []string {
	if len(pkg.FailedTests) == 0 {
		return nil
	}

	errorColor := r.console.GetErrorColor()
	reset := r.console.ResetColor()

	var lines []string
	for _, testName := range pkg.FailedTests {
		displayName := testName
		if r.config.SubtestConfig.HumanizeNames {
			displayName = HumanizeTestName(testName)
		}
		lines = append(lines, fmt.Sprintf("%s     %s✗ %s%s",
			indent, errorColor, displayName, reset))
	}
	return lines
}

func (r *TestRenderer) getStatusIconAndColor(pkg TestPackageResult) (string, string) {
	total := pkg.Passed + pkg.Failed + pkg.Skipped

	switch {
	case pkg.Failed > 0:
		return "✗", r.console.GetErrorColor()
	case total == 0:
		return r.config.NoTestIcon, r.console.GetColor(r.config.NoTestColor)
	default:
		return "✓", r.console.GetSuccessColor()
	}
}

func (r *TestRenderer) renderSparkbar(coverage float64) string {
	length := r.config.SparkbarLength
	filled := int(coverage * float64(length) / 100)
	if filled > length {
		filled = length
	}

	// Determine color based on coverage thresholds
	var filledColor string
	for _, threshold := range r.config.CoverageThresholds {
		if coverage >= threshold.Min && coverage <= threshold.Max {
			filledColor = r.console.GetColor(threshold.ColorKey)
			break
		}
	}
	if filledColor == "" {
		filledColor = r.console.GetMutedColor()
	}

	emptyColor := r.console.GetMutedColor()
	reset := r.console.ResetColor()

	if coverage == 0 {
		return fmt.Sprintf("%s%s%s",
			emptyColor,
			strings.Repeat(r.config.SparkbarEmpty, length),
			reset)
	}

	return fmt.Sprintf("%s%s%s%s%s%s",
		filledColor,
		strings.Repeat(r.config.SparkbarFilled, filled),
		reset,
		emptyColor,
		strings.Repeat(r.config.SparkbarEmpty, length-filled),
		reset)
}

func (r *TestRenderer) formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

func (r *TestRenderer) formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// GetGroupName extracts a group name from a package path.
func (r *TestRenderer) GetGroupName(pkgPath string) string {
	layout := r.config.ProjectLayout

	if layout.GroupFunc != nil {
		return layout.GroupFunc(pkgPath)
	}

	path := pkgPath
	if layout.ModulePrefix != "" {
		prefix := layout.ModulePrefix + "/"
		path = strings.TrimPrefix(path, prefix)
	}

	for _, topDir := range layout.TopDirs {
		pattern := "/" + topDir + "/"
		if strings.Contains(path, pattern) {
			return topDir
		}
		if strings.HasPrefix(path, topDir+"/") {
			return topDir
		}
	}

	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	return ""
}

// detectModulePrefix attempts to detect the module prefix from go.mod.
func detectModulePrefix() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			content, err := os.ReadFile(modPath)
			if err != nil {
				return ""
			}

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module"))
					return moduleName
				}
			}
			return ""
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// HumanizeTestName converts Go test names to human-friendly format.
func HumanizeTestName(testName string) string {
	name := strings.TrimPrefix(testName, "Test")
	parts := strings.Split(name, "_")

	if len(parts) == 1 {
		return humanizePart(parts[0])
	}

	whenIndex := -1
	for i, part := range parts {
		if strings.EqualFold(part, "When") {
			whenIndex = i
			break
		}
	}

	if whenIndex > 0 {
		component := joinHumanized(parts[:whenIndex])
		condition := joinHumanized(parts[whenIndex+1:])

		if whenIndex == 1 {
			return fmt.Sprintf("%s - When %s", component, condition)
		}
		behavior := joinHumanized(parts[1:whenIndex])
		return fmt.Sprintf("%s: %s - When %s", humanizePart(parts[0]), behavior, condition)
	}

	if len(parts) == 2 {
		return fmt.Sprintf("%s - %s", humanizePart(parts[0]), humanizePart(parts[1]))
	}

	humanized := make([]string, len(parts))
	for i, part := range parts {
		humanized[i] = humanizePart(part)
	}
	return strings.Join(humanized, " - ")
}

func joinHumanized(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return humanizePart(parts[0])
	}
	humanized := make([]string, len(parts))
	for i, part := range parts {
		humanized[i] = humanizePart(part)
	}
	return strings.Join(humanized, " ")
}

func humanizePart(part string) string {
	if part == "" {
		return ""
	}

	acronyms := map[string]bool{
		"HTTP": true, "HTTPS": true, "API": true, "MCP": true, "SQL": true,
		"DB": true, "JSON": true, "XML": true, "HTML": true, "CSS": true,
		"JS": true, "TS": true, "ID": true, "URL": true, "URI": true,
		"UUID": true, "LSP": true, "RPC": true, "CLI": true, "TLS": true,
		"SSL": true, "SSH": true, "FTP": true, "TCP": true, "UDP": true,
		"IP": true, "DNS": true, "AWS": true, "KG": true, "OK": true,
		"CSV": true, "PDF": true, "PNG": true, "JPG": true, "GIF": true,
		"UI": true, "UX": true, "EOF": true, "OS": true, "CPU": true,
		"RAM": true, "IO": true, "UTF": true, "ASCII": true, "SQLite": true,
	}

	part = regexp.MustCompile(`(?i)\bSQ\s+Lite\b`).ReplaceAllString(part, "SQLite")

	var result strings.Builder
	var currentWord strings.Builder
	runes := []rune(part)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if i+3 < len(runes) && strings.ToUpper(string(runes[i:i+3])) == "UTF" {
			digitStart := i + 3
			digitEnd := digitStart
			for digitEnd < len(runes) && runes[digitEnd] >= '0' && runes[digitEnd] <= '9' {
				digitEnd++
			}
			if digitEnd > digitStart {
				if currentWord.Len() > 0 {
					word := currentWord.String()
					if result.Len() > 0 {
						result.WriteRune(' ')
					}
					if acronyms[strings.ToUpper(word)] {
						result.WriteString(strings.ToUpper(word))
					} else {
						result.WriteString(word)
					}
					currentWord.Reset()
				}
				utfToken := "UTF" + string(runes[digitStart:digitEnd])
				if result.Len() > 0 {
					result.WriteRune(' ')
				}
				result.WriteString(utfToken)
				result.WriteRune(' ')
				i = digitEnd - 1
				continue
			}
		}

		startNewWord := false
		if i > 0 {
			prev := runes[i-1]
			if r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z' {
				startNewWord = true
			}
			if i+1 < len(runes) {
				next := runes[i+1]
				if r >= 'A' && r <= 'Z' && prev >= 'A' && prev <= 'Z' && next >= 'a' && next <= 'z' {
					startNewWord = true
				}
			}
		}

		if startNewWord && currentWord.Len() > 0 {
			word := currentWord.String()
			if result.Len() > 0 {
				result.WriteRune(' ')
			}
			if acronyms[strings.ToUpper(word)] {
				result.WriteString(strings.ToUpper(word))
			} else {
				result.WriteString(word)
			}
			currentWord.Reset()
		}

		currentWord.WriteRune(r)
	}

	if currentWord.Len() > 0 {
		word := currentWord.String()
		if result.Len() > 0 {
			result.WriteRune(' ')
		}
		if acronyms[strings.ToUpper(word)] {
			result.WriteString(strings.ToUpper(word))
		} else {
			result.WriteString(word)
		}
	}

	return strings.TrimSpace(result.String())
}
