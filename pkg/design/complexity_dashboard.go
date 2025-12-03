// Package design implements pattern-based CLI output visualization
package design

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PatternTypeComplexityDashboard represents complexity metrics dashboards.
// Use for: codebase health metrics, file size trends, cyclomatic complexity tracking.
const PatternTypeComplexityDashboard PatternType = "complexity-dashboard"

// ComplexityDashboard represents a unified complexity metrics view with trend tracking.
// Combines file size metrics with cyclomatic complexity and surfaces actionable hotspots.
type ComplexityDashboard struct {
	Title          string              // Section title (default: "COMPLEXITY")
	Metrics        []ComplexityMetric  // Current metrics with historical comparison
	Hotspots       []ComplexityHotspot // Top files by combined size x complexity score
	TrendWindow    string              // Time window for comparison (e.g., "4w")
	ShowSparklines bool                // Whether to show inline sparklines for trends
}

// ComplexityMetric represents a single complexity metric with trend data.
type ComplexityMetric struct {
	Label       string    // Metric name (e.g., "Files >500 LOC")
	Current     float64   // Current value
	Previous    float64   // Previous value (from trend window ago)
	Trend       string    // "improving", "stable", "degrading"
	Unit        string    // Optional unit (e.g., "files", "avg")
	LowerBetter bool      // Whether lower values are better (true for most complexity metrics)
	History     []float64 // Optional: historical values for sparkline (8 values for 8 weeks)
}

// ComplexityHotspot represents a file that needs attention based on size and complexity.
type ComplexityHotspot struct {
	Path          string // File path
	LOC           int    // Lines of code
	MaxComplexity int    // Highest cyclomatic complexity in the file
	Score         int    // Combined score (LOC x MaxComplexity)
}

// PatternType returns the pattern type identifier.
func (c *ComplexityDashboard) PatternType() PatternType {
	return PatternTypeComplexityDashboard
}

// Render formats the complexity dashboard using the provided theme.
func (c *ComplexityDashboard) Render(cfg *Config) string {
	var sb strings.Builder

	// Get styles from config
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(cfg.Colors.Muted).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(cfg.Colors.Process)

	mutedStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Muted)

	successStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Success)

	warningStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Warning)

	errorStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Error)

	// Title
	title := c.Title
	if title == "" {
		title = "COMPLEXITY"
	}
	sb.WriteString(headerStyle.Render(title))
	sb.WriteString("\n")

	// Column headers if we have metrics
	if len(c.Metrics) > 0 {
		trendLabel := c.TrendWindow
		if trendLabel == "" {
			trendLabel = "4w"
		}

		// Calculate column widths
		maxLabelWidth := 0
		for _, m := range c.Metrics {
			if len(m.Label) > maxLabelWidth {
				maxLabelWidth = len(m.Label)
			}
		}
		if maxLabelWidth < 30 {
			maxLabelWidth = 30
		}

		// Header row
		headerLine := fmt.Sprintf("  %-*s %8s %8s    %s",
			maxLabelWidth, "", "now", trendLabel+" ago", "trend")
		sb.WriteString(mutedStyle.Render(headerLine))
		sb.WriteString("\n")

		// Render metrics
		for _, m := range c.Metrics {
			sb.WriteString(c.renderMetricLine(cfg, m, maxLabelWidth))
			sb.WriteString("\n")
		}
	}

	// Hotspots section
	if len(c.Hotspots) > 0 {
		sb.WriteString("\n")
		sb.WriteString(mutedStyle.Render("  Hotspots (size × complexity)"))
		sb.WriteString("\n")

		for _, h := range c.Hotspots {
			// Format: "  2562 LOC  cc:23  internal/mcp/server/tslsp_handler.go"
			line := fmt.Sprintf("    %4d LOC  ", h.LOC)

			// Color complexity badge based on value
			ccBadge := fmt.Sprintf("cc:%d", h.MaxComplexity)
			var ccStyled string
			switch {
			case h.MaxComplexity >= 20:
				ccStyled = errorStyle.Render(ccBadge)
			case h.MaxComplexity >= 15:
				ccStyled = warningStyle.Render(ccBadge)
			default:
				ccStyled = successStyle.Render(ccBadge)
			}

			sb.WriteString(mutedStyle.Render(line))
			sb.WriteString(ccStyled)
			sb.WriteString("  ")
			sb.WriteString(mutedStyle.Render(h.Path))
			sb.WriteString("\n")
		}
	}

	return boxStyle.Render(sb.String())
}

// renderMetricLine formats a single metric row with alignment and trend indicator.
func (c *ComplexityDashboard) renderMetricLine(cfg *Config, m ComplexityMetric, labelWidth int) string {
	var sb strings.Builder

	mutedStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Muted)

	successStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Success)

	warningStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Warning)

	errorStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Error)

	// Label
	sb.WriteString("  ")
	sb.WriteString(PadRight(m.Label, labelWidth))
	sb.WriteString(" ")

	// Current value
	currentStr := formatMetricValue(m.Current, m.Unit)
	sb.WriteString(PadLeft(currentStr, 8))
	sb.WriteString(" ")

	// Previous value (if available)
	if m.Previous != 0 || m.Trend != "" {
		prevStr := formatMetricValue(m.Previous, m.Unit)
		sb.WriteString(mutedStyle.Render(PadLeft(prevStr, 8)))
	} else {
		sb.WriteString(PadLeft("—", 8))
	}
	sb.WriteString("    ")

	// Trend indicator with color
	trend := m.Trend
	if trend == "" {
		trend = calculateTrend(m.Current, m.Previous, m.LowerBetter)
	}

	var trendIcon string
	var trendStyle lipgloss.Style
	switch trend {
	case "improving":
		trendIcon = "↓"
		if !m.LowerBetter {
			trendIcon = "↑"
		}
		trendStyle = successStyle
	case "degrading":
		trendIcon = "↑"
		if !m.LowerBetter {
			trendIcon = "↓"
		}
		trendStyle = errorStyle
	default: // "stable"
		trendIcon = "→"
		trendStyle = warningStyle
	}

	trendText := trendIcon + " " + trend
	sb.WriteString(trendStyle.Render(trendText))

	// Optional sparkline
	if c.ShowSparklines && len(m.History) > 0 {
		sb.WriteString("  ")
		sparkline := renderInlineSparkline(m.History)
		sb.WriteString(mutedStyle.Render(sparkline))
	}

	return sb.String()
}

// formatMetricValue formats a numeric value with optional unit.
func formatMetricValue(value float64, unit string) string {
	// Check if value is a whole number
	if value == float64(int(value)) {
		return fmt.Sprintf("%d", int(value))
	}
	return fmt.Sprintf("%.1f", value)
}

// calculateTrend determines the trend direction based on current vs previous values.
func calculateTrend(current, previous float64, lowerBetter bool) string {
	if previous == 0 {
		return "stable"
	}

	diff := current - previous
	threshold := previous * 0.05 // 5% threshold for "stable"

	if diff < -threshold {
		if lowerBetter {
			return "improving"
		}
		return "degrading"
	}
	if diff > threshold {
		if lowerBetter {
			return "degrading"
		}
		return "improving"
	}
	return "stable"
}

// renderInlineSparkline creates a compact sparkline from historical values.
func renderInlineSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}

	// Find min/max for scaling
	minVal, maxVal := values[0], values[0]
	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1
	}

	// Unicode block elements (8 levels)
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var sb strings.Builder
	for _, v := range values {
		normalized := (v - minVal) / valueRange
		idx := int(normalized * 7)
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		sb.WriteRune(blocks[idx])
	}

	return sb.String()
}
