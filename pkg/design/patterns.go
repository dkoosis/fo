// Package design implements pattern-based CLI output visualization
package design

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DensityMode controls the space-efficiency of pattern rendering.
// Based on Tufte's data-ink ratio principle: maximize information per line.
type DensityMode string

const (
	// DensityDetailed shows one item per line with full context (current default).
	DensityDetailed DensityMode = "detailed"

	// DensityBalanced shows 2 columns where appropriate.
	DensityBalanced DensityMode = "balanced"

	// DensityCompact shows 3 columns with minimal spacing.
	DensityCompact DensityMode = "compact"
)

// PatternType represents the six standard semantic pattern types in the design system.
// These types map semantic meaning (what to show) to visual presentation (how to show it).
type PatternType string

const (
	// PatternTypeSparkline represents word-sized trend graphics using Unicode blocks.
	// Use for: test duration trends, coverage changes, build size progression, error count trends.
	PatternTypeSparkline PatternType = "sparkline"

	// PatternTypeLeaderboard represents ranked lists showing top/bottom N items by metric.
	// Use for: slowest tests, largest binaries, files with most warnings, packages with lowest coverage.
	PatternTypeLeaderboard PatternType = "leaderboard"

	// PatternTypeTestTable represents comprehensive test results with status and timing.
	// Use for: complete test suite results, package-level test summaries.
	PatternTypeTestTable PatternType = "test-table"

	// PatternTypeSummary represents high-level summaries with key metrics and counts.
	// Use for: at-a-glance understanding of overall results, rollup statistics.
	PatternTypeSummary PatternType = "summary"

	// PatternTypeComparison represents before/after comparisons of metrics.
	// Use for: showing changes over time, version comparisons, delta analysis.
	PatternTypeComparison PatternType = "comparison"

	// PatternTypeInventory represents lists of generated artifacts or files.
	// Use for: build outputs, generated files, deployment artifacts, file listings.
	PatternTypeInventory PatternType = "inventory"
)

// AllPatternTypes returns all six standard pattern types.
func AllPatternTypes() []PatternType {
	return []PatternType{
		PatternTypeSparkline,
		PatternTypeLeaderboard,
		PatternTypeTestTable,
		PatternTypeSummary,
		PatternTypeComparison,
		PatternTypeInventory,
		PatternTypeQualityReport,
	}
}

// IsValidPatternType checks if a string represents a valid pattern type.
func IsValidPatternType(s string) bool {
	for _, pt := range AllPatternTypes() {
		if string(pt) == s {
			return true
		}
	}
	return false
}

// Pattern is the interface that all output patterns implement.
// Patterns represent different ways of visualizing command output data.
//
// Contract:
//   - Patterns are semantic: they represent what to show, not how to show it
//   - Patterns are theme-independent: the same pattern can be rendered with different themes
//   - Patterns are composable: multiple patterns can be combined to create dashboards
//   - Patterns implement Render() which takes a Config (theme) and returns formatted output
type Pattern interface {
	// Render returns the formatted string representation of the pattern
	// using the provided theme configuration.
	// The Config parameter controls visual presentation (colors, icons, density, etc.)
	// while the pattern itself controls semantic content (data, structure, meaning).
	Render(cfg *Config) string

	// PatternType returns the standard type identifier for this pattern.
	// This enables type-based routing, validation, and theme selection.
	PatternType() PatternType
}

// Sparkline represents a word-sized graphic showing trends using Unicode blocks.
// Inspired by Tufte's sparklines - intense, simple, word-sized graphics.
//
// Use cases:
//   - Test duration trends over last N runs
//   - Coverage percentage changes
//   - Build size progression
//   - Error count trends
type Sparkline struct {
	Label  string    // Label for the sparkline (e.g., "Build time trend")
	Values []float64 // Data points to visualize
	Min    float64   // Optional: explicit minimum for scale (0 = auto-detect)
	Max    float64   // Optional: explicit maximum for scale (0 = auto-detect)
	Unit   string    // Optional: unit suffix (e.g., "ms", "%", "MB")
}

// PatternType implements the Pattern interface.
func (s *Sparkline) PatternType() PatternType {
	return PatternTypeSparkline
}

// Render creates the sparkline visualization using Unicode block elements.
// Uses ▁▂▃▄▅▆▇█ for value representation.
func (s *Sparkline) Render(cfg *Config) string {
	if len(s.Values) == 0 {
		return ""
	}

	var sb strings.Builder

	// Determine scale
	minVal, maxVal := s.Min, s.Max
	if minVal == 0 && maxVal == 0 {
		// Auto-detect range
		minVal, maxVal = s.Values[0], s.Values[0]
		for _, v := range s.Values {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}

	// Handle edge case where all values are the same
	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1 // Prevent division by zero
	}

	// Unicode block elements for sparkline (8 levels)
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Get styles (Phase 2: using lipgloss.Style instead of manual concatenation)
	labelStyle := cfg.GetStyle("Process")
	if cfg.GetColor("Process") == "" && !cfg.IsMonochrome {
		labelStyle = cfg.GetStyle("Detail")
	}
	sparklineStyle := cfg.GetStyle("Success")
	if cfg.GetColor("Success") == "" && !cfg.IsMonochrome {
		sparklineStyle = cfg.GetStyle("Process")
	}
	unitStyle := cfg.GetStyle("Muted")

	// Build output using lipgloss styles
	if s.Label != "" {
		labelText := s.Label + ": "
		sb.WriteString(labelStyle.Render(labelText))
	}

	// Render sparkline
	var sparklineBuilder strings.Builder
	for _, value := range s.Values {
		// Normalize value to 0-1 range
		normalized := (value - minVal) / valueRange
		// Map to block index (0-7)
		blockIndex := int(normalized * 7)
		if blockIndex < 0 {
			blockIndex = 0
		}
		if blockIndex > 7 {
			blockIndex = 7
		}
		sparklineBuilder.WriteRune(blocks[blockIndex])
	}
	sparklineStr := sparklineBuilder.String()
	if !cfg.IsMonochrome {
		sb.WriteString(sparklineStyle.Render(sparklineStr))
	} else {
		sb.WriteString(sparklineStr)
	}

	// Add latest value with unit
	if len(s.Values) > 0 {
		latest := s.Values[len(s.Values)-1]
		unitText := fmt.Sprintf(" %.1f%s", latest, s.Unit)
		if !cfg.IsMonochrome {
			sb.WriteString(unitStyle.Render(unitText))
		} else {
			sb.WriteString(unitText)
		}
	}

	return sb.String()
}

// Leaderboard represents a ranked list of items sorted by a specific metric.
// Shows only the top/bottom N items to highlight optimization targets or achievements.
//
// Use cases:
//   - Slowest N tests (optimization targets)
//   - Largest N binaries (size analysis)
//   - Files with most linting warnings (quality hotspots)
//   - Packages with lowest coverage (test gap identification)
//   - Top contributors by commits
type Leaderboard struct {
	Label      string            // Title for the leaderboard (e.g., "Slowest Tests")
	MetricName string            // Name of the metric being ranked (e.g., "Duration", "Size", "Warnings")
	Items      []LeaderboardItem // Ranked items
	Direction  string            // "highest" or "lowest" - what's being shown
	TotalCount int               // Total items before filtering to top N
	ShowRank   bool              // Whether to show rank numbers
}

// LeaderboardItem represents a single entry in a leaderboard.
type LeaderboardItem struct {
	Name    string  // Item name (e.g., test name, file name, package name)
	Metric  string  // Formatted metric value (e.g., "2.3s", "45MB", "12 warnings")
	Value   float64 // Numeric value for ranking/comparison
	Rank    int     // Position in ranking (1-based)
	Context string  // Additional context or details
}

// PatternType implements the Pattern interface.
func (l *Leaderboard) PatternType() PatternType {
	return PatternTypeLeaderboard
}

// Render creates the leaderboard visualization.
func (l *Leaderboard) Render(cfg *Config) string {
	if len(l.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Styles (Phase 2: using lipgloss.Style)
	headerStyle := cfg.GetStyleWithBold("Process")
	rankStyle := cfg.GetStyle("Muted")
	nameStyle := cfg.GetStyle("Detail")
	metricStyle := cfg.GetStyle("Success")
	contextStyle := cfg.GetStyle("Muted")

	// Header
	if l.Label != "" {
		headerText := l.Label
		if l.TotalCount > len(l.Items) {
			headerText += fmt.Sprintf(" (top %d of %d)", len(l.Items), l.TotalCount)
		}
		sb.WriteString(headerStyle.Render(headerText))
		sb.WriteString("\n")
	}

	// Calculate column widths for alignment
	maxRankWidth := len(strconv.Itoa(len(l.Items)))
	maxNameWidth := 0
	maxMetricWidth := 0
	for _, item := range l.Items {
		nameWidth := VisualWidth(item.Name)
		if nameWidth > maxNameWidth {
			maxNameWidth = nameWidth
		}
		metricWidth := VisualWidth(item.Metric)
		if metricWidth > maxMetricWidth {
			maxMetricWidth = metricWidth
		}
	}

	// Cap name width to prevent overly long lines
	const maxAllowedNameWidth = 50
	if maxNameWidth > maxAllowedNameWidth {
		maxNameWidth = maxAllowedNameWidth
	}

	// Render items
	for _, item := range l.Items {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Rank (if enabled)
		if l.ShowRank {
			rankText := fmt.Sprintf("%*d. ", maxRankWidth, item.Rank)
			sb.WriteString(rankStyle.Render(rankText))
		}

		// Name (truncated if needed)
		displayName := item.Name
		if VisualWidth(displayName) > maxNameWidth {
			// Truncate preserving visual width
			truncated := truncateToWidth(displayName, maxNameWidth-3)
			displayName = truncated + "..."
		}
		sb.WriteString(nameStyle.Render(PadRight(displayName, maxNameWidth)))

		// Metric (right-aligned)
		sb.WriteString("  ")
		sb.WriteString(metricStyle.Render(PadLeft(item.Metric, maxMetricWidth)))

		// Context (if provided)
		if item.Context != "" {
			sb.WriteString("  ")
			sb.WriteString(contextStyle.Render(item.Context))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// TestTable represents a table of test results showing packages/tests with status and timing.
// Provides a comprehensive view of all test outcomes.
type TestTable struct {
	Label   string          // Title for the test table
	Results []TestTableItem // Test results to display
	Density DensityMode     // Rendering density (detailed, balanced, compact)
}

// TestTableItem represents a single test result entry.
type TestTableItem struct {
	Name     string // Test or package name
	Status   string // "pass", "fail", "skip"
	Duration string // Formatted duration
	Count    int    // Number of tests (for package-level results)
	Details  string // Additional details or error message
}

// PatternType implements the Pattern interface.
func (t *TestTable) PatternType() PatternType {
	return PatternTypeTestTable
}

// Render creates the test table visualization.
// Supports different density modes for space-efficient rendering.
func (t *TestTable) Render(cfg *Config) string {
	if len(t.Results) == 0 {
		return ""
	}

	// Check for compact rendering based on density setting or config
	density := t.Density
	if density == "" {
		// Fall back to config density
		switch cfg.Style.Density {
		case "compact":
			density = DensityCompact
		case "balanced":
			density = DensityBalanced
		default:
			density = DensityDetailed
		}
	}

	// Use compact rendering for compact/balanced modes
	switch density {
	case DensityCompact:
		return t.renderCompact(cfg, 3) // 3 columns
	case DensityBalanced:
		return t.renderCompact(cfg, 2) // 2 columns
	case DensityDetailed:
		// Fall through to detailed rendering below
	}

	// Default detailed rendering
	var sb strings.Builder

	// Styles (Phase 2: using lipgloss.Style)
	headerStyle := cfg.GetStyleWithBold("Process")
	passStyle := cfg.GetStyle("Success")
	failStyle := cfg.GetStyle("Error")
	skipStyle := cfg.GetStyle("Warning")
	durationStyle := cfg.GetStyle("Muted")
	detailStyle := cfg.GetStyle("Muted")

	// Header
	if t.Label != "" {
		sb.WriteString(headerStyle.Render(t.Label))
		sb.WriteString("\n")
	}

	// Calculate column widths
	maxNameWidth := 0
	maxDurationWidth := 0
	for _, result := range t.Results {
		nameWidth := VisualWidth(result.Name)
		if nameWidth > maxNameWidth {
			maxNameWidth = nameWidth
		}
		durationWidth := VisualWidth(result.Duration)
		if durationWidth > maxDurationWidth {
			maxDurationWidth = durationWidth
		}
	}

	const maxAllowedNameWidth = 60
	if maxNameWidth > maxAllowedNameWidth {
		maxNameWidth = maxAllowedNameWidth
	}

	// Render results
	for _, result := range t.Results {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Status icon
		var statusIcon string
		var statusStyle lipgloss.Style
		switch result.Status {
		case "pass":
			statusIcon = cfg.GetIcon("Success")
			statusStyle = passStyle
		case "fail":
			statusIcon = cfg.GetIcon("Error")
			statusStyle = failStyle
		case "skip":
			statusIcon = cfg.GetIcon("Warning")
			statusStyle = skipStyle
		default:
			statusIcon = cfg.GetIcon("Info")
			statusStyle = lipgloss.NewStyle()
		}

		iconText := statusIcon + " "
		sb.WriteString(statusStyle.Render(iconText))

		// Name
		displayName := result.Name
		if VisualWidth(displayName) > maxNameWidth {
			// Truncate preserving visual width
			truncated := truncateToWidth(displayName, maxNameWidth-3)
			displayName = truncated + "..."
		}
		sb.WriteString(PadRight(displayName, maxNameWidth))

		// Count (if applicable)
		if result.Count > 0 {
			sb.WriteString(fmt.Sprintf("  %d tests", result.Count))
		}

		// Duration
		sb.WriteString("  ")
		sb.WriteString(durationStyle.Render(PadLeft(result.Duration, maxDurationWidth)))

		// Details (on next line if present)
		if result.Details != "" {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(cfg.GetIndentation(1))
			sb.WriteString(detailStyle.Render(result.Details))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// renderCompact creates a multi-column compact test table view.
// Maximizes data density by showing multiple results per line.
func (t *TestTable) renderCompact(cfg *Config, columns int) string {
	var sb strings.Builder

	// Styles (Phase 2: using lipgloss.Style)
	headerStyle := cfg.GetStyleWithBold("Process")
	passStyle := cfg.GetStyle("Success")
	failStyle := cfg.GetStyle("Error")
	skipStyle := cfg.GetStyle("Warning")
	durationStyle := cfg.GetStyle("Muted")

	// Header
	if t.Label != "" {
		sb.WriteString(headerStyle.Render(t.Label))
		sb.WriteString("\n")
	}

	// Calculate column width (assume ~80 char terminal, divide by columns)
	termWidth := 80
	colWidth := (termWidth / columns) - 2 // -2 for spacing

	// Render results in columns
	for i := 0; i < len(t.Results); i += columns {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Render up to 'columns' items on this line
		for col := 0; col < columns && i+col < len(t.Results); col++ {
			if col > 0 {
				sb.WriteString("  ") // Column separator
			}

			result := t.Results[i+col]

			// Status icon
			var statusIcon string
			var statusStyle lipgloss.Style
			switch result.Status {
			case "pass":
				statusIcon = cfg.GetIcon("Success")
				statusStyle = passStyle
			case "fail":
				statusIcon = cfg.GetIcon("Error")
				statusStyle = failStyle
			case "skip":
				statusIcon = cfg.GetIcon("Warning")
				statusStyle = skipStyle
			default:
				statusIcon = cfg.GetIcon("Info")
				statusStyle = lipgloss.NewStyle()
			}

			iconText := statusIcon + " "
			sb.WriteString(statusStyle.Render(iconText))

			// Name (truncated to fit column)
			maxNameLen := colWidth - 10 // Reserve space for duration
			displayName := result.Name
			if VisualWidth(displayName) > maxNameLen {
				// Truncate preserving visual width
				truncated := truncateToWidth(displayName, maxNameLen-3)
				displayName = truncated + "..."
			}
			sb.WriteString(PadRight(displayName, maxNameLen))

			// Duration (compact format)
			sb.WriteString(" ")
			sb.WriteString(durationStyle.Render(result.Duration))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// Summary represents a high-level summary with key metrics and counts.
// Provides at-a-glance understanding of overall results.
type Summary struct {
	Label   string        // Title for the summary
	Metrics []SummaryItem // Metrics to display
}

// SummaryItem represents a single summary metric.
type SummaryItem struct {
	Label string // Metric label (e.g., "Total Tests", "Passed", "Failed")
	Value string // Formatted value (e.g., "142", "98.5%")
	Type  string // "success", "error", "warning", "info" - affects coloring
}

// PatternType implements the Pattern interface.
func (s *Summary) PatternType() PatternType {
	return PatternTypeSummary
}

// Render creates the summary visualization.
func (s *Summary) Render(cfg *Config) string {
	if len(s.Metrics) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	headerStyle := cfg.GetStyleWithBold("Process")
	if s.Label != "" {
		sb.WriteString(headerStyle.Render(s.Label))
		sb.WriteString("\n")
	}

	// Render metrics
	for _, metric := range s.Metrics {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Icon and style based on type
		var icon string
		var valueStyle lipgloss.Style
		switch metric.Type {
		case "success":
			icon = cfg.GetIcon("Success")
			valueStyle = cfg.GetStyle("Success")
		case "error":
			icon = cfg.GetIcon("Error")
			valueStyle = cfg.GetStyle("Error")
		case "warning":
			icon = cfg.GetIcon("Warning")
			valueStyle = cfg.GetStyle("Warning")
		default:
			icon = cfg.GetIcon("Info")
			valueStyle = cfg.GetStyle("Process")
		}

		labelText := icon + " " + metric.Label + ": "
		sb.WriteString(labelText)
		sb.WriteString(valueStyle.Render(metric.Value))
		sb.WriteString("\n")
	}

	return sb.String()
}

// Comparison represents a before/after comparison of metrics.
// Useful for showing changes over time or between versions.
type Comparison struct {
	Label   string           // Title for the comparison
	Changes []ComparisonItem // Items being compared
}

// ComparisonItem represents a single metric comparison.
type ComparisonItem struct {
	Label  string  // Metric label
	Before string  // Before value (formatted)
	After  string  // After value (formatted)
	Change float64 // Numeric change (positive or negative)
	Unit   string  // Unit for the change (e.g., "%", "MB", "ms")
}

// PatternType implements the Pattern interface.
func (c *Comparison) PatternType() PatternType {
	return PatternTypeComparison
}

// Render creates the comparison visualization.
func (c *Comparison) Render(cfg *Config) string {
	if len(c.Changes) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	headerStyle := cfg.GetStyleWithBold("Process")
	if c.Label != "" {
		sb.WriteString(headerStyle.Render(c.Label))
		sb.WriteString("\n")
	}

	// Render comparisons
	for _, item := range c.Changes {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Label
		sb.WriteString(item.Label)
		sb.WriteString(": ")

		// Before → After
		mutedStyle := cfg.GetStyle("Muted")
		beforeAfterText := item.Before + " → " + item.After
		sb.WriteString(mutedStyle.Render(beforeAfterText))

		// Change indicator
		sb.WriteString(" ")
		var changeIcon string
		var changeStyle lipgloss.Style
		switch {
		case item.Change > 0:
			changeIcon = "↑"
			changeStyle = cfg.GetStyle("Warning") // Increase might be bad (e.g., build time)
		case item.Change < 0:
			changeIcon = "↓"
			changeStyle = cfg.GetStyle("Success") // Decrease might be good (e.g., build time)
		default:
			changeIcon = "="
			changeStyle = cfg.GetStyle("Process")
		}

		changeText := changeIcon + fmt.Sprintf(" %.1f%s", math.Abs(item.Change), item.Unit)
		sb.WriteString(changeStyle.Render(changeText))

		sb.WriteString("\n")
	}

	return sb.String()
}

// Inventory represents a list of generated artifacts or files.
// Useful for showing build outputs, generated files, or deployment artifacts.
type Inventory struct {
	Label string          // Title for the inventory
	Items []InventoryItem // Items in the inventory
}

// InventoryItem represents a single file or artifact.
type InventoryItem struct {
	Name string // File or artifact name
	Size string // Formatted size (e.g., "2.3MB", "450KB")
	Path string // Optional: full path or additional context
}

// PatternType implements the Pattern interface.
func (i *Inventory) PatternType() PatternType {
	return PatternTypeInventory
}

// Render creates the inventory visualization.
func (i *Inventory) Render(cfg *Config) string {
	if len(i.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	headerStyle := cfg.GetStyleWithBold("Process")
	if i.Label != "" {
		sb.WriteString(headerStyle.Render(i.Label))
		sb.WriteString("\n")
	}

	// Calculate column width
	maxNameWidth := 0
	for _, item := range i.Items {
		nameWidth := VisualWidth(item.Name)
		if nameWidth > maxNameWidth {
			maxNameWidth = nameWidth
		}
	}

	const maxAllowedNameWidth = 40
	if maxNameWidth > maxAllowedNameWidth {
		maxNameWidth = maxAllowedNameWidth
	}

	// Render items
	for _, item := range i.Items {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Icon
		icon := cfg.GetIcon("Info")
		sb.WriteString(icon)
		sb.WriteString(" ")

		// Name
		displayName := item.Name
		if VisualWidth(displayName) > maxNameWidth {
			// Truncate preserving visual width
			truncated := truncateToWidth(displayName, maxNameWidth-3)
			displayName = truncated + "..."
		}
		nameStyle := cfg.GetStyle("Detail")
		sb.WriteString(nameStyle.Render(PadRight(displayName, maxNameWidth)))

		// Size
		if item.Size != "" {
			sb.WriteString("  ")
			sizeStyle := cfg.GetStyle("Muted")
			sizeText := "[" + item.Size + "]"
			sb.WriteString(sizeStyle.Render(sizeText))
		}

		// Path (if provided)
		if item.Path != "" {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(cfg.GetIndentation(1))
			pathStyle := cfg.GetStyle("Muted")
			sb.WriteString(pathStyle.Render(item.Path))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
