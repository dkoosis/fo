package fo

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Default success icon fallback.
const defaultSuccessIcon = "✓"

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
	// Format: [[0, 39, "Error"], [40, 69, "Warning"], [70, 100, "Success"]]
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
	// GroupByParent groups subtests under their parent test
	GroupByParent bool
	// IndentSize is the number of spaces to indent nested subtests
	IndentSize int
	// ShowParentStatus shows parent test status (if false, parent is just a header)
	ShowParentStatus bool
	// HumanizeNames applies HumanizeTestName to subtest names
	HumanizeNames bool
}

// ProjectLayout configures how to extract group names from package paths for test grouping.
type ProjectLayout struct {
	// TopDirs are directories to recognize as top-level groups (e.g., ["internal", "cmd", "pkg", "examples"])
	TopDirs []string
	// ModulePrefix is the module prefix to strip (auto-detected from go.mod if empty)
	ModulePrefix string
	// GroupFunc is an optional custom grouping function that overrides default behavior
	// If provided, this function takes a package path and returns a group name
	GroupFunc func(pkgPath string) string
}

// CoverageThreshold defines a coverage range and its associated color.
type CoverageThreshold struct {
	Min      float64
	Max      float64
	ColorKey string
}

// TestRenderer renders test results using the console's theme.
type TestRenderer struct {
	console    *Console
	writer     io.Writer
	config     TestTableConfig
	inGroupBox bool // Track if we're inside a group box
	boxWidth   int  // Width of the box for consistent borders
}

// getPaleGrayColor returns a very pale gray ANSI color code.
func (r *TestRenderer) getPaleGrayColor() string {
	// Very pale gray: ANSI 256-color code 252
	return "\033[38;5;252m"
}

// stripANSI removes ANSI escape sequences from a string to calculate visual width.
func stripANSI(s string) string {
	// Remove ANSI escape sequences (ESC [ ... m)
	var result strings.Builder
	inEscape := false
	for i := range len(s) {
		switch {
		case s[i] == '\033':
			inEscape = true
		case inEscape && s[i] == 'm':
			inEscape = false
		case !inEscape:
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// NewTestRenderer creates a new test renderer with the given console and writer.
func NewTestRenderer(console *Console, writer io.Writer) *TestRenderer {
	// Build configuration from the console's design config
	config := buildTestTableConfig(console)
	return &TestRenderer{
		console: console,
		writer:  writer,
		config:  config,
	}
}

// buildTestTableConfig builds test table configuration from the console's design config.
func buildTestTableConfig(console *Console) TestTableConfig {
	cfg := console.designConf

	// Build coverage thresholds from theme configuration
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
		UseTreeChars:       false,
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

// RenderTableHeader renders the table header with column labels in a complete box.
func (r *TestRenderer) RenderTableHeader() {
	paleGray := r.getPaleGrayColor()
	reset := r.console.ResetColor()
	cfg := r.console.designConf

	// Box width: accommodate the header line content
	// "  PASS  PATH                               TESTS   TIME    COVERAGE" = 67 chars
	// Add 2 for left/right borders = 69 total
	r.boxWidth = 67

	fmt.Fprintf(r.writer, "\n")

	// Top border: ╭───────────────────────────────╮
	topCorner := cfg.Border.TopCornerChar
	headerChar := cfg.Border.HeaderChar
	closingCorner := "╮"
	if topCorner == "╔" {
		closingCorner = "╗"
	}
	fmt.Fprintf(r.writer, "%s%s%s%s%s\n", paleGray, topCorner, strings.Repeat(headerChar, r.boxWidth), closingCorner, reset)

	// Header line with borders: │  PASS  PATH... │
	fmt.Fprintf(r.writer, "%s%s%s  PASS  PATH                               TESTS   TIME    COVERAGE %s%s%s\n",
		paleGray, cfg.Border.VerticalChar, reset, paleGray, cfg.Border.VerticalChar, reset)

	// Separator line: ├───────────────────────────────┤
	separatorChar := "├"
	separatorEnd := "┤"
	if cfg.Border.TopCornerChar == "╔" {
		separatorChar = "╠"
		separatorEnd = "╣"
	}
	fmt.Fprintf(r.writer, "%s%s%s%s%s\n", paleGray, separatorChar, strings.Repeat(headerChar, r.boxWidth), separatorEnd, reset)
}

// RenderGroupHeader renders the directory group header and starts a box.
func (r *TestRenderer) RenderGroupHeader(dirName string) {
	paleGray := r.getPaleGrayColor()
	blueColor := r.console.GetBlueFgColor()
	reset := r.console.ResetColor()
	cfg := r.console.designConf

	// Start a new box for the group
	r.inGroupBox = true

	// Group header line with borders: │  ⊙ dirname/ │
	fmt.Fprintf(r.writer, "%s%s%s  %s%s%s %s/ %s%s%s\n",
		paleGray, cfg.Border.VerticalChar, reset,
		blueColor, "⊙", reset, dirName,
		paleGray, cfg.Border.VerticalChar, reset)
}

// RenderPackageLine renders a single package test result line.
func (r *TestRenderer) RenderPackageLine(pkg TestPackageResult) {
	total := pkg.Passed + pkg.Failed + pkg.Skipped

	// Determine status icon and color using semantic methods
	var statusIcon string
	var statusColor string
	switch {
	case pkg.Failed > 0:
		statusIcon = r.console.FormatStatusIcon("FAIL")
		// Extract color for manual formatting (since we need to style package name separately)
		statusColor = r.console.GetErrorColor()
	case total == 0:
		// Use configured icon and color for packages with no tests
		statusIcon = r.config.NoTestIcon
		statusColor = r.console.GetColor(r.config.NoTestColor)
	default:
		statusIcon = r.console.FormatStatusIcon("PASS")
		// Extract color for manual formatting
		statusColor = r.console.GetSuccessColor()
	}
	reset := r.console.ResetColor()

	// Build result string (X/Y format) - right-aligned in 8 chars
	result := fmt.Sprintf("%8s", fmt.Sprintf("%d/%d", pkg.Passed, total))

	// Coverage sparkbar
	sparkbar := r.renderCoverageSparkbar(pkg.Coverage)

	// Duration (right-aligned, 8 chars)
	duration := r.formatAlignedDuration(pkg.Duration)

	// Print summary line with box borders if inside a group box
	paleGray := r.getPaleGrayColor()
	cfg := r.console.designConf
	if r.inGroupBox {
		// Format the line content
		lineContent := fmt.Sprintf("    %s%s%s   %-30s %8s %8s   %s",
			statusColor, statusIcon, reset, pkg.Name, result, duration, sparkbar)

		// Calculate actual visual width (without ANSI codes)
		visualWidth := len(stripANSI(lineContent))
		padding := r.boxWidth - visualWidth
		if padding < 0 {
			padding = 0
		}

		fmt.Fprintf(r.writer, "%s%s%s%s%s%s%s%s\n",
			paleGray, cfg.Border.VerticalChar, reset,
			lineContent,
			strings.Repeat(" ", padding),
			paleGray, cfg.Border.VerticalChar, reset)
	} else {
		fmt.Fprintf(r.writer, "    %s%s%s   %-30s %8s %8s   %s\n",
			statusColor, statusIcon, reset, pkg.Name, result, duration, sparkbar)
	}

	// Print tests based on configuration
	if r.config.ShowAllTests && len(pkg.AllTests) > 0 {
		if r.config.SubtestConfig.GroupByParent {
			r.renderTestsWithHierarchy(pkg.AllTests)
		} else {
			// Show all tests with their status (flat list)
			for _, test := range pkg.AllTests {
				r.renderTestLine(test, test.Status)
			}
		}
	} else if len(pkg.FailedTests) > 0 {
		// Show only failed tests (default behavior)
		if r.config.SubtestConfig.GroupByParent {
			// Convert failed test names to TestResult format for hierarchy rendering
			failedTestResults := make([]TestResult, len(pkg.FailedTests))
			for i, name := range pkg.FailedTests {
				failedTestResults[i] = TestResult{Name: name, Status: "FAIL"}
			}
			r.renderTestsWithHierarchy(failedTestResults)
		} else {
			// Flat list of failed tests
			for _, testName := range pkg.FailedTests {
				r.renderTestLine(TestResult{Name: testName, Status: "FAIL"}, "FAIL")
			}
		}
	}
}

// renderTestLine renders a single test line with appropriate styling.
func (r *TestRenderer) renderTestLine(test TestResult, status string) {
	statusUpper := strings.ToUpper(status)
	styledTestName := r.console.FormatTestName(test.Name, statusUpper)
	statusText := r.console.FormatStatusText(fmt.Sprintf("(%s)", statusUpper), statusUpper)

	if r.inGroupBox {
		paleGray := r.getPaleGrayColor()
		cfg := r.console.designConf
		reset := r.console.ResetColor()
		lineContent := fmt.Sprintf("        %s %s", styledTestName, statusText)
		visualWidth := len(stripANSI(lineContent))
		padding := r.boxWidth - visualWidth
		if padding < 0 {
			padding = 0
		}
		fmt.Fprintf(r.writer, "%s%s%s%s%s%s%s%s\n",
			paleGray, cfg.Border.VerticalChar, reset,
			lineContent,
			strings.Repeat(" ", padding),
			paleGray, cfg.Border.VerticalChar, reset)
	} else {
		fmt.Fprintf(r.writer, "        %s %s\n", styledTestName, statusText)
	}
}

// renderTestsWithHierarchy renders tests grouped by parent with proper indentation.
func (r *TestRenderer) renderTestsWithHierarchy(tests []TestResult) {
	// Group tests by parent prefix
	parentMap := make(map[string][]TestResult) // parent -> children
	standaloneTests := []TestResult{}
	parentTests := make(map[string]bool) // track which tests are parents

	for _, test := range tests {
		lastSlash := strings.LastIndex(test.Name, "/")
		if lastSlash == -1 {
			// Standalone test (no subtest)
			standaloneTests = append(standaloneTests, test)
		} else {
			// Subtest - extract parent
			parent := test.Name[:lastSlash]
			parentMap[parent] = append(parentMap[parent], test)
			parentTests[parent] = true
		}
	}

	// Render standalone tests first
	for _, test := range standaloneTests {
		// Skip if this test is a parent (has subtests)
		if !parentTests[test.Name] {
			r.renderTestLine(test, test.Status)
		}
	}

	// Render parent tests with their subtests
	for parent, children := range parentMap {
		// Find parent test status (if it exists in the test list)
		parentStatus := "PASS" // default
		for _, test := range tests {
			if test.Name == parent {
				parentStatus = test.Status
				break
			}
		}

		// Render parent as header (or with status if configured)
		parentName := parent
		if r.config.SubtestConfig.HumanizeNames {
			parentName = HumanizeTestName(parent)
		}

		if r.config.SubtestConfig.ShowParentStatus {
			// Render parent with status
			r.renderTestLine(TestResult{Name: parent, Status: parentStatus}, parentStatus)
		} else {
			// Render parent as plain header
			if r.inGroupBox {
				paleGray := r.getPaleGrayColor()
				cfg := r.console.designConf
				reset := r.console.ResetColor()
				lineContent := "        " + parentName
				visualWidth := len(stripANSI(lineContent))
				padding := r.boxWidth - visualWidth
				if padding < 0 {
					padding = 0
				}
				fmt.Fprintf(r.writer, "%s%s%s%s%s%s%s%s\n",
					paleGray, cfg.Border.VerticalChar, reset,
					lineContent,
					strings.Repeat(" ", padding),
					paleGray, cfg.Border.VerticalChar, reset)
			} else {
				fmt.Fprintf(r.writer, "        %s\n", parentName)
			}
		}

		// Render children with indentation
		for _, child := range children {
			childName := child.Name
			if r.config.SubtestConfig.HumanizeNames {
				// Extract just the subtest name (after last '/')
				lastSlash := strings.LastIndex(child.Name, "/")
				subtestName := child.Name[lastSlash+1:]
				childName = HumanizeTestName(subtestName)
			}

			// Create a modified test result with just the subtest name for display
			displayTest := TestResult{
				Name:   childName,
				Status: child.Status,
			}

			// Render with additional indentation
			originalIndent := "        "
			extraIndent := strings.Repeat(" ", r.config.SubtestConfig.IndentSize)
			statusUpper := strings.ToUpper(child.Status)
			styledTestName := r.console.FormatTestName(displayTest.Name, statusUpper)
			statusText := r.console.FormatStatusText(fmt.Sprintf("(%s)", statusUpper), statusUpper)

			if r.inGroupBox {
				paleGray := r.getPaleGrayColor()
				cfg := r.console.designConf
				reset := r.console.ResetColor()
				lineContent := originalIndent + extraIndent + styledTestName + " " + statusText
				visualWidth := len(stripANSI(lineContent))
				padding := r.boxWidth - visualWidth
				if padding < 0 {
					padding = 0
				}
				fmt.Fprintf(r.writer, "%s%s%s%s%s%s%s%s\n",
					paleGray, cfg.Border.VerticalChar, reset,
					lineContent,
					strings.Repeat(" ", padding),
					paleGray, cfg.Border.VerticalChar, reset)
			} else {
				fmt.Fprintf(r.writer, "%s%s%s %s\n", originalIndent, extraIndent, styledTestName, statusText)
			}
		}
	}
}

// RenderGroupFooter renders the bottom border of the group box.
func (r *TestRenderer) RenderGroupFooter() {
	if r.inGroupBox {
		paleGray := r.getPaleGrayColor()
		reset := r.console.ResetColor()
		cfg := r.console.designConf

		// Bottom border: ╰───────────────────────────────╯
		bottomCorner := cfg.Border.BottomCornerChar
		headerChar := cfg.Border.HeaderChar
		bottomClosingCorner := "╯"
		if bottomCorner == "╚" {
			bottomClosingCorner = "╝"
		}
		fmt.Fprintf(r.writer, "%s%s%s%s%s\n", paleGray, bottomCorner, strings.Repeat(headerChar, r.boxWidth), bottomClosingCorner, reset)
		r.inGroupBox = false
	}
	fmt.Fprintf(r.writer, "\n")
}

// renderCoverageSparkbar renders a coverage sparkbar with theme colors.
func (r *TestRenderer) renderCoverageSparkbar(coverage float64) string {
	filled := int(coverage * float64(r.config.SparkbarLength) / 100)
	if filled > r.config.SparkbarLength {
		filled = r.config.SparkbarLength
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

	// Muted color for empty portion
	emptyColor := r.console.GetMutedColor()
	reset := r.console.ResetColor()

	// Build sparkbar
	if coverage == 0 {
		// No coverage: show empty bar
		return fmt.Sprintf("%s%s%s", emptyColor, strings.Repeat(r.config.SparkbarEmpty, r.config.SparkbarLength), reset)
	}

	filledPart := strings.Repeat(r.config.SparkbarFilled, filled)
	emptyPart := strings.Repeat(r.config.SparkbarEmpty, r.config.SparkbarLength-filled)

	if r.config.ShowPercentage {
		return fmt.Sprintf("%s%s%s%s%s%s   %.0f%%", filledColor, filledPart, reset, emptyColor, emptyPart, reset, coverage)
	}
	return fmt.Sprintf("%s%s%s%s%s%s", filledColor, filledPart, reset, emptyColor, emptyPart, reset)
}

// formatAlignedDuration formats a duration for display with consistent width.
func (r *TestRenderer) formatAlignedDuration(d time.Duration) string {
	// Right-align duration values (8 chars total)
	if d < time.Second {
		// Format: "     6ms" (6 chars for number + "ms" = 8 chars total)
		return fmt.Sprintf("%6dms", d.Milliseconds())
	}
	// Show one decimal for seconds (e.g., "   16.5s")
	if d < time.Minute {
		// Format: "   16.5s" (7 chars including decimal + "s" = 8 chars total)
		return fmt.Sprintf("%7.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	remainingSeconds := int(d.Seconds()) % 60
	if remainingSeconds == 0 {
		return fmt.Sprintf("%7dm", minutes)
	}
	// For m:ss format, calculate total width dynamically
	return fmt.Sprintf("%dm%ds", minutes, remainingSeconds)
}

// detectModulePrefix attempts to detect the module prefix from go.mod.
// Returns empty string if detection fails.
func detectModulePrefix() string {
	// Look for go.mod in current directory and parent directories
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			// Read go.mod to extract module name
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
			// Reached root
			break
		}
		dir = parent
	}

	return ""
}

// GetGroupName extracts a group name from a package path using the ProjectLayout configuration.
// This allows consumers to group test packages by directory structure.
func (r *TestRenderer) GetGroupName(pkgPath string) string {
	layout := r.config.ProjectLayout

	// Use custom grouping function if provided
	if layout.GroupFunc != nil {
		return layout.GroupFunc(pkgPath)
	}

	// Strip module prefix if configured
	path := pkgPath
	if layout.ModulePrefix != "" {
		prefix := layout.ModulePrefix + "/"
		path = strings.TrimPrefix(path, prefix)
	}

	// Check for top-level directories
	for _, topDir := range layout.TopDirs {
		pattern := "/" + topDir + "/"
		if strings.Contains(path, pattern) {
			return topDir
		}
		// Also check if path starts with topDir
		if strings.HasPrefix(path, topDir+"/") {
			return topDir
		}
	}

	// If no top-level directory matches, return the first path component
	parts := strings.Split(path, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}

	// Fallback: return empty string (no grouping)
	return ""
}

// HumanizeTestName converts Go test names to human-friendly format.
// Test<Component>_<Behavior>_When_<Condition> -> "Component: Behavior - When Condition".
func HumanizeTestName(testName string) string {
	// Remove "Test" prefix
	name := strings.TrimPrefix(testName, "Test")

	// Split on underscores
	parts := strings.Split(name, "_")

	// Handle different patterns
	if len(parts) == 1 {
		// Single part: TestSomething -> "Something"
		return humanizePart(parts[0])
	}

	// Find "When" part if it exists
	whenIndex := -1
	for i, part := range parts {
		if strings.EqualFold(part, "When") {
			whenIndex = i
			break
		}
	}

	if whenIndex > 0 {
		// Component_Behavior_When_Condition format
		component := joinHumanized(parts[:whenIndex])
		condition := joinHumanized(parts[whenIndex+1:])

		if whenIndex == 1 {
			// Simple: Component_When_Condition -> "Component - When Condition"
			return fmt.Sprintf("%s - When %s", component, condition)
		}
		// Standard: Component_Behavior_When_Condition -> "Component: Behavior - When Condition"
		behavior := joinHumanized(parts[1:whenIndex])
		return fmt.Sprintf("%s: %s - When %s", humanizePart(parts[0]), behavior, condition)
	}

	// No "When" part - just Component_Behavior format
	if len(parts) == 2 {
		return fmt.Sprintf("%s - %s", humanizePart(parts[0]), humanizePart(parts[1]))
	}

	// Multiple parts without "When" - join with " - "
	humanized := make([]string, len(parts))
	for i, part := range parts {
		humanized[i] = humanizePart(part)
	}
	return strings.Join(humanized, " - ")
}

// joinHumanized joins multiple parts into a humanized string.
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

// humanizePart converts a CamelCase part into space-separated words with proper capitalization.
func humanizePart(part string) string {
	if part == "" {
		return ""
	}

	// Common acronyms that should stay uppercase
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

	// Fix "SQ Lite" -> "SQLite" (case-insensitive)
	part = regexp.MustCompile(`(?i)\bSQ\s+Lite\b`).ReplaceAllString(part, "SQLite")

	var result strings.Builder
	var currentWord strings.Builder
	runes := []rune(part)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check for UTF[0-9]+ pattern
		if i+3 < len(runes) && strings.ToUpper(string(runes[i:i+3])) == "UTF" {
			// Check if followed by digits
			digitStart := i + 3
			digitEnd := digitStart
			for digitEnd < len(runes) && runes[digitEnd] >= '0' && runes[digitEnd] <= '9' {
				digitEnd++
			}
			if digitEnd > digitStart {
				// Found UTF[0-9]+ pattern
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
				// Add UTF token
				utfToken := "UTF" + string(runes[digitStart:digitEnd])
				if result.Len() > 0 {
					result.WriteRune(' ')
				}
				result.WriteString(utfToken)
				result.WriteRune(' ')
				// Skip past the UTF token
				i = digitEnd - 1
				continue
			}
		}

		// Detect word boundaries
		startNewWord := false
		if i > 0 {
			prev := runes[i-1]
			// New word if: uppercase after lowercase (e.g., "endTo")
			if r >= 'A' && r <= 'Z' && prev >= 'a' && prev <= 'z' {
				startNewWord = true
			}
			// New word if: lowercase after uppercase and next is lowercase (e.g., "HTTPClient" -> "HTTP" + "Client")
			if i+1 < len(runes) {
				next := runes[i+1]
				if r >= 'A' && r <= 'Z' && prev >= 'A' && prev <= 'Z' && next >= 'a' && next <= 'z' {
					startNewWord = true
				}
			}
		}

		if startNewWord && currentWord.Len() > 0 {
			// Finish current word
			word := currentWord.String()
			if result.Len() > 0 {
				result.WriteRune(' ')
			}
			// Check if it's an acronym
			if acronyms[strings.ToUpper(word)] {
				result.WriteString(strings.ToUpper(word))
			} else {
				result.WriteString(word)
			}
			currentWord.Reset()
		}

		currentWord.WriteRune(r)
	}

	// Add the last word
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
