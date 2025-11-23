// Package design implements pattern-based CLI output visualization
package design

import (
	"fmt"
	"math"
	"strings"
)

// DensityMode controls the space-efficiency of pattern rendering.
// Based on Tufte's data-ink ratio principle: maximize information per line.
type DensityMode string

const (
	// DensityDetailed shows one item per line with full context (current default)
	DensityDetailed DensityMode = "detailed"

	// DensityBalanced shows 2 columns where appropriate
	DensityBalanced DensityMode = "balanced"

	// DensityCompact shows 3 columns with minimal spacing
	DensityCompact DensityMode = "compact"
)

// Pattern is the interface that all output patterns implement.
// Patterns represent different ways of visualizing command output data.
type Pattern interface {
	// Render returns the formatted string representation of the pattern
	Render(cfg *Config) string
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

// Render creates the sparkline visualization using Unicode block elements.
// Uses ▁▂▃▄▅▆▇█ for value representation.
func (s *Sparkline) Render(cfg *Config) string {
	if len(s.Values) == 0 {
		return ""
	}

	var sb strings.Builder

	// Determine scale
	min, max := s.Min, s.Max
	if min == 0 && max == 0 {
		// Auto-detect range
		min, max = s.Values[0], s.Values[0]
		for _, v := range s.Values {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
	}

	// Handle edge case where all values are the same
	valueRange := max - min
	if valueRange == 0 {
		valueRange = 1 // Prevent division by zero
	}

	// Unicode block elements for sparkline (8 levels)
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Get colors
	labelColor := cfg.GetColor("Process")
	if labelColor == "" && !cfg.IsMonochrome {
		labelColor = cfg.GetColor("Detail")
	}
	sparklineColor := cfg.GetColor("Success")
	if sparklineColor == "" && !cfg.IsMonochrome {
		sparklineColor = cfg.GetColor("Process")
	}
	unitColor := cfg.GetColor("Muted")

	// Build output
	if s.Label != "" {
		if !cfg.IsMonochrome {
			sb.WriteString(labelColor)
		}
		sb.WriteString(s.Label)
		sb.WriteString(": ")
		if !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}
	}

	// Render sparkline
	if !cfg.IsMonochrome {
		sb.WriteString(sparklineColor)
	}
	for _, value := range s.Values {
		// Normalize value to 0-1 range
		normalized := (value - min) / valueRange
		// Map to block index (0-7)
		blockIndex := int(normalized * 7)
		if blockIndex < 0 {
			blockIndex = 0
		}
		if blockIndex > 7 {
			blockIndex = 7
		}
		sb.WriteRune(blocks[blockIndex])
	}
	if !cfg.IsMonochrome {
		sb.WriteString(cfg.ResetColor())
	}

	// Add latest value with unit
	if len(s.Values) > 0 {
		latest := s.Values[len(s.Values)-1]
		sb.WriteString(" ")
		if !cfg.IsMonochrome {
			sb.WriteString(unitColor)
		}
		sb.WriteString(fmt.Sprintf("%.1f%s", latest, s.Unit))
		if !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
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

// Render creates the leaderboard visualization.
func (l *Leaderboard) Render(cfg *Config) string {
	if len(l.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Colors
	headerColor := cfg.GetColor("Process")
	rankColor := cfg.GetColor("Muted")
	nameColor := cfg.GetColor("Detail")
	metricColor := cfg.GetColor("Success")
	contextColor := cfg.GetColor("Muted")

	if cfg.IsMonochrome {
		headerColor = ""
		rankColor = ""
		nameColor = ""
		metricColor = ""
		contextColor = ""
	}

	// Header
	if l.Label != "" {
		if headerColor != "" {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(l.Label)
		if headerColor != "" {
			sb.WriteString(cfg.ResetColor())
		}
		if l.TotalCount > len(l.Items) {
			if rankColor != "" {
				sb.WriteString(" ")
				sb.WriteString(rankColor)
			}
			sb.WriteString(fmt.Sprintf(" (top %d of %d)", len(l.Items), l.TotalCount))
			if rankColor != "" {
				sb.WriteString(cfg.ResetColor())
			}
		}
		sb.WriteString("\n")
	}

	// Calculate column widths for alignment
	maxRankWidth := len(fmt.Sprintf("%d", len(l.Items)))
	maxNameWidth := 0
	maxMetricWidth := 0
	for _, item := range l.Items {
		nameWidth := len(item.Name)
		if nameWidth > maxNameWidth {
			maxNameWidth = nameWidth
		}
		metricWidth := len(item.Metric)
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
			if rankColor != "" {
				sb.WriteString(rankColor)
			}
			sb.WriteString(fmt.Sprintf("%*d. ", maxRankWidth, item.Rank))
			if rankColor != "" {
				sb.WriteString(cfg.ResetColor())
			}
		}

		// Name (truncated if needed)
		displayName := item.Name
		if len(displayName) > maxNameWidth {
			displayName = displayName[:maxNameWidth-3] + "..."
		}
		if nameColor != "" {
			sb.WriteString(nameColor)
		}
		sb.WriteString(fmt.Sprintf("%-*s", maxNameWidth, displayName))
		if nameColor != "" {
			sb.WriteString(cfg.ResetColor())
		}

		// Metric (right-aligned)
		sb.WriteString("  ")
		if metricColor != "" {
			sb.WriteString(metricColor)
		}
		sb.WriteString(fmt.Sprintf("%*s", maxMetricWidth, item.Metric))
		if metricColor != "" {
			sb.WriteString(cfg.ResetColor())
		}

		// Context (if provided)
		if item.Context != "" {
			sb.WriteString("  ")
			if contextColor != "" {
				sb.WriteString(contextColor)
			}
			sb.WriteString(item.Context)
			if contextColor != "" {
				sb.WriteString(cfg.ResetColor())
			}
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
	Name     string  // Test or package name
	Status   string  // "pass", "fail", "skip"
	Duration string  // Formatted duration
	Count    int     // Number of tests (for package-level results)
	Details  string  // Additional details or error message
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
	if density == DensityCompact {
		return t.renderCompact(cfg, 3) // 3 columns
	} else if density == DensityBalanced {
		return t.renderCompact(cfg, 2) // 2 columns
	}

	// Default detailed rendering
	var sb strings.Builder

	// Colors
	headerColor := cfg.GetColor("Process")
	passColor := cfg.GetColor("Success")
	failColor := cfg.GetColor("Error")
	skipColor := cfg.GetColor("Warning")
	durationColor := cfg.GetColor("Muted")

	if cfg.IsMonochrome {
		headerColor = ""
		passColor = ""
		failColor = ""
		skipColor = ""
		durationColor = ""
	}

	// Header
	if t.Label != "" {
		if headerColor != "" {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(t.Label)
		if headerColor != "" {
			sb.WriteString(cfg.ResetColor())
		}
		sb.WriteString("\n")
	}

	// Calculate column widths
	maxNameWidth := 0
	maxDurationWidth := 0
	for _, result := range t.Results {
		if len(result.Name) > maxNameWidth {
			maxNameWidth = len(result.Name)
		}
		if len(result.Duration) > maxDurationWidth {
			maxDurationWidth = len(result.Duration)
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
		var statusColor string
		switch result.Status {
		case "pass":
			statusIcon = cfg.GetIcon("Success")
			statusColor = passColor
		case "fail":
			statusIcon = cfg.GetIcon("Error")
			statusColor = failColor
		case "skip":
			statusIcon = cfg.GetIcon("Warning")
			statusColor = skipColor
		default:
			statusIcon = cfg.GetIcon("Info")
			statusColor = ""
		}

		if statusColor != "" {
			sb.WriteString(statusColor)
		}
		sb.WriteString(statusIcon)
		sb.WriteString(" ")
		if statusColor != "" {
			sb.WriteString(cfg.ResetColor())
		}

		// Name
		displayName := result.Name
		if len(displayName) > maxNameWidth {
			displayName = displayName[:maxNameWidth-3] + "..."
		}
		sb.WriteString(fmt.Sprintf("%-*s", maxNameWidth, displayName))

		// Count (if applicable)
		if result.Count > 0 {
			sb.WriteString(fmt.Sprintf("  %d tests", result.Count))
		}

		// Duration
		sb.WriteString("  ")
		if durationColor != "" {
			sb.WriteString(durationColor)
		}
		sb.WriteString(fmt.Sprintf("%*s", maxDurationWidth, result.Duration))
		if durationColor != "" {
			sb.WriteString(cfg.ResetColor())
		}

		// Details (on next line if present)
		if result.Details != "" {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(cfg.GetIndentation(1))
			detailColor := cfg.GetColor("Muted")
			if detailColor != "" {
				sb.WriteString(detailColor)
			}
			sb.WriteString(result.Details)
			if detailColor != "" {
				sb.WriteString(cfg.ResetColor())
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// renderCompact creates a multi-column compact test table view.
// Maximizes data density by showing multiple results per line.
func (t *TestTable) renderCompact(cfg *Config, columns int) string {
	var sb strings.Builder

	// Colors
	headerColor := cfg.GetColor("Process")
	passColor := cfg.GetColor("Success")
	failColor := cfg.GetColor("Error")
	skipColor := cfg.GetColor("Warning")
	durationColor := cfg.GetColor("Muted")

	if cfg.IsMonochrome {
		headerColor = ""
		passColor = ""
		failColor = ""
		skipColor = ""
		durationColor = ""
	}

	// Header
	if t.Label != "" {
		if headerColor != "" {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(t.Label)
		if headerColor != "" {
			sb.WriteString(cfg.ResetColor())
		}
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
			var statusColor string
			switch result.Status {
			case "pass":
				statusIcon = cfg.GetIcon("Success")
				statusColor = passColor
			case "fail":
				statusIcon = cfg.GetIcon("Error")
				statusColor = failColor
			case "skip":
				statusIcon = cfg.GetIcon("Warning")
				statusColor = skipColor
			default:
				statusIcon = cfg.GetIcon("Info")
				statusColor = ""
			}

			if statusColor != "" {
				sb.WriteString(statusColor)
			}
			sb.WriteString(statusIcon)
			sb.WriteString(" ")
			if statusColor != "" {
				sb.WriteString(cfg.ResetColor())
			}

			// Name (truncated to fit column)
			maxNameLen := colWidth - 10 // Reserve space for duration
			displayName := result.Name
			if len(displayName) > maxNameLen {
				displayName = displayName[:maxNameLen-3] + "..."
			}
			sb.WriteString(fmt.Sprintf("%-*s", maxNameLen, displayName))

			// Duration (compact format)
			sb.WriteString(" ")
			if durationColor != "" {
				sb.WriteString(durationColor)
			}
			sb.WriteString(result.Duration)
			if durationColor != "" {
				sb.WriteString(cfg.ResetColor())
			}
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

// Render creates the summary visualization.
func (s *Summary) Render(cfg *Config) string {
	if len(s.Metrics) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	if s.Label != "" {
		headerColor := cfg.GetColor("Process")
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(s.Label)
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}
		sb.WriteString("\n")
	}

	// Render metrics
	for _, metric := range s.Metrics {
		indent := cfg.GetIndentation(1)
		sb.WriteString(indent)

		// Icon based on type
		var icon string
		var valueColor string
		switch metric.Type {
		case "success":
			icon = cfg.GetIcon("Success")
			valueColor = cfg.GetColor("Success")
		case "error":
			icon = cfg.GetIcon("Error")
			valueColor = cfg.GetColor("Error")
		case "warning":
			icon = cfg.GetIcon("Warning")
			valueColor = cfg.GetColor("Warning")
		default:
			icon = cfg.GetIcon("Info")
			valueColor = cfg.GetColor("Process")
		}

		if cfg.IsMonochrome {
			valueColor = ""
		}

		sb.WriteString(icon)
		sb.WriteString(" ")
		sb.WriteString(metric.Label)
		sb.WriteString(": ")
		if valueColor != "" {
			sb.WriteString(valueColor)
		}
		sb.WriteString(metric.Value)
		if valueColor != "" {
			sb.WriteString(cfg.ResetColor())
		}
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

// Render creates the comparison visualization.
func (c *Comparison) Render(cfg *Config) string {
	if len(c.Changes) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	if c.Label != "" {
		headerColor := cfg.GetColor("Process")
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(c.Label)
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}
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
		mutedColor := cfg.GetColor("Muted")
		if mutedColor != "" && !cfg.IsMonochrome {
			sb.WriteString(mutedColor)
		}
		sb.WriteString(item.Before)
		sb.WriteString(" → ")
		sb.WriteString(item.After)
		if mutedColor != "" && !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}

		// Change indicator
		sb.WriteString(" ")
		var changeIcon string
		var changeColor string
		if item.Change > 0 {
			changeIcon = "↑"
			changeColor = cfg.GetColor("Warning") // Increase might be bad (e.g., build time)
		} else if item.Change < 0 {
			changeIcon = "↓"
			changeColor = cfg.GetColor("Success") // Decrease might be good (e.g., build time)
		} else {
			changeIcon = "="
			changeColor = cfg.GetColor("Process")
		}

		if cfg.IsMonochrome {
			changeColor = ""
		}

		if changeColor != "" {
			sb.WriteString(changeColor)
		}
		sb.WriteString(changeIcon)
		sb.WriteString(fmt.Sprintf(" %.1f%s", math.Abs(item.Change), item.Unit))
		if changeColor != "" {
			sb.WriteString(cfg.ResetColor())
		}

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

// Render creates the inventory visualization.
func (i *Inventory) Render(cfg *Config) string {
	if len(i.Items) == 0 {
		return ""
	}

	var sb strings.Builder

	// Header
	if i.Label != "" {
		headerColor := cfg.GetColor("Process")
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(headerColor)
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(i.Label)
		if headerColor != "" && !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}
		sb.WriteString("\n")
	}

	// Calculate column width
	maxNameWidth := 0
	for _, item := range i.Items {
		if len(item.Name) > maxNameWidth {
			maxNameWidth = len(item.Name)
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
		if len(displayName) > maxNameWidth {
			displayName = displayName[:maxNameWidth-3] + "..."
		}
		nameColor := cfg.GetColor("Detail")
		if nameColor != "" && !cfg.IsMonochrome {
			sb.WriteString(nameColor)
		}
		sb.WriteString(fmt.Sprintf("%-*s", maxNameWidth, displayName))
		if nameColor != "" && !cfg.IsMonochrome {
			sb.WriteString(cfg.ResetColor())
		}

		// Size
		if item.Size != "" {
			sb.WriteString("  ")
			sizeColor := cfg.GetColor("Muted")
			if sizeColor != "" && !cfg.IsMonochrome {
				sb.WriteString(sizeColor)
			}
			sb.WriteString("[")
			sb.WriteString(item.Size)
			sb.WriteString("]")
			if sizeColor != "" && !cfg.IsMonochrome {
				sb.WriteString(cfg.ResetColor())
			}
		}

		// Path (if provided)
		if item.Path != "" {
			sb.WriteString("\n")
			sb.WriteString(indent)
			sb.WriteString(cfg.GetIndentation(1))
			pathColor := cfg.GetColor("Muted")
			if pathColor != "" && !cfg.IsMonochrome {
				sb.WriteString(pathColor)
			}
			sb.WriteString(item.Path)
			if pathColor != "" && !cfg.IsMonochrome {
				sb.WriteString(cfg.ResetColor())
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
