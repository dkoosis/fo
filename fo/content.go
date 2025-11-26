// fo/content.go - Content rendering primitives for structured section output.
//
// This file provides high-level content rendering functions that produce
// properly formatted, themed output inside section boxes. These primitives
// are the building blocks for creating consistent, beautiful mage output.
package fo

import (
	"fmt"
	"strings"

)

// MetricLine represents a key-value metric with optional trend indicator.
// Example: "Total files:     367 ↑".
type MetricLine struct {
	Label string // e.g., "Total files"
	Value string // e.g., "367"
	Trend string // e.g., "↑", "↓", "" (empty for no trend)
	Color string // Optional color for the value (e.g., "success", "warning", "error")
}

// BulletItem represents a bulleted list item.
type BulletItem struct {
	Text  string // Main text content
	Color string // Optional color for the text
}

// RankedItem represents an item in a ranked/numbered list.
type RankedItem struct {
	Rank  int    // 1-based rank
	Value string // The metric value (e.g., "2562")
	Label string // Description (e.g., "internal/server/handler.go")
	Color string // Optional color for the value
}

// SparklineData represents data for a sparkline chart.
type SparklineData struct {
	Label    string // Row label (e.g., "Week -1")
	Segments []SparklineSegment
}

// SparklineSegment represents one colored segment of a sparkline.
type SparklineSegment struct {
	Proportion float64 // 0.0 to 1.0
	Color      string  // Color name (e.g., "success", "warning", "error")
}

// PrintMetricLine renders a single metric line inside a section box.
// The label is left-aligned, value is right-aligned at a fixed column.
func (c *Console) PrintMetricLine(m MetricLine) {
	cfg := c.designConf
	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	// Format: "      Label:     Value Trend"
	indent := "      "
	labelWidth := 18 // Fixed width for label alignment

	var valueColor string
	if m.Color != "" {
		valueColor = cfg.GetColor(m.Color)
	}

	// Build trend indicator with color
	var trendStr string
	if m.Trend != "" {
		switch m.Trend {
		case "↑":
			trendStr = cfg.GetColor("error") + m.Trend + reset // Up is bad for LOC
		case "↓":
			trendStr = cfg.GetColor("success") + m.Trend + reset // Down is good
		default:
			trendStr = m.Trend
		}
	}

	// Format the line
	labelPadded := fmt.Sprintf("%-*s", labelWidth, m.Label+":")
	valueFormatted := m.Value
	if valueColor != "" {
		valueFormatted = valueColor + m.Value + reset
	}

	line := fmt.Sprintf("%s%s %s %s", indent, labelPadded, valueFormatted, trendStr)

	// Render with box border
	c.printBoxLine(box, line)
}

// PrintBulletHeader renders a bulleted section header inside a section box.
// Example: "   ◉ File Metrics (Non-Test Files)".
func (c *Console) PrintBulletHeader(text string) {
	cfg := c.designConf
	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	bulletColor := cfg.GetColor("process") // Use theme's process color
	bullet := cfg.Icons.Bullet
	if bullet == "" {
		bullet = "◉"
	}

	// Format: "   ◉ Header Text"
	line := fmt.Sprintf("   %s%s%s %s", bulletColor, bullet, reset, text)

	c.printBoxLine(box, line)
}

// PrintBulletItem renders a bulleted list item.
func (c *Console) PrintBulletItem(item BulletItem) {
	cfg := c.designConf
	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	bulletColor := cfg.GetColor("muted")
	bullet := "•"

	var textColor string
	if item.Color != "" {
		textColor = cfg.GetColor(item.Color)
	}

	text := item.Text
	if textColor != "" {
		text = textColor + item.Text + reset
	}

	line := fmt.Sprintf("      %s%s%s %s", bulletColor, bullet, reset, text)

	c.printBoxLine(box, line)
}

// PrintRankedList renders a numbered list of ranked items.
// Example:
//
//  1. 2562  internal/server/handler.go
//  2. 1841  internal/client/client.go
func (c *Console) PrintRankedList(title string, items []RankedItem) {
	// Print header
	c.PrintBulletHeader(title)

	cfg := c.designConf
	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	for _, item := range items {
		// Format: "      1. 2562  path/to/file.go"
		line := fmt.Sprintf("      %d. %s  %s", item.Rank, item.Value, item.Label)

		if item.Color != "" {
			valueColor := cfg.GetColor(item.Color)
			line = fmt.Sprintf("      %d. %s%s%s  %s", item.Rank, valueColor, item.Value, reset, item.Label)
		}

		c.printBoxLine(box, line)
	}
}

// PrintSparkline renders a sparkline distribution chart.
// Each row shows a label and a colored bar representing proportions.
func (c *Console) PrintSparkline(title string, data []SparklineData, width int) {
	if width <= 0 {
		width = 40
	}

	// Print header
	c.PrintBulletHeader(title)

	cfg := c.designConf
	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	filledChar := "■"
	emptyChar := "□"

	for _, row := range data {
		var barBuilder strings.Builder

		// Handle empty/zero data
		hasData := false
		for _, seg := range row.Segments {
			if seg.Proportion > 0 {
				hasData = true
				break
			}
		}

		if !hasData {
			// Show dim empty bar
			barBuilder.WriteString(cfg.GetColor("muted"))
			barBuilder.WriteString(strings.Repeat(emptyChar, width))
			barBuilder.WriteString(reset)
		} else {
			// Build colored segments
			usedChars := 0
			for i, seg := range row.Segments {
				chars := int(seg.Proportion * float64(width))
				// Last segment gets remaining chars to ensure exact width
				if i == len(row.Segments)-1 {
					chars = width - usedChars
				}
				if chars > 0 {
					color := cfg.GetColor(seg.Color)
					barBuilder.WriteString(color)
					barBuilder.WriteString(strings.Repeat(filledChar, chars))
					barBuilder.WriteString(reset)
					usedChars += chars
				}
			}
		}

		// Format: "      Week -0:   ■■■■■■■■■■■■"
		labelPadded := fmt.Sprintf("%-10s", row.Label)
		line := fmt.Sprintf("      %s %s", labelPadded, barBuilder.String())

		c.printBoxLine(box, line)
	}
}

// PrintBlankLine prints an empty line inside the section box.
func (c *Console) PrintBlankLine() {
	box := c.calculateBoxLayout()
	c.printBoxLine(box, "")
}

// PrintText renders plain text inside the section box with proper indentation.
func (c *Console) PrintText(text string) {
	box := c.calculateBoxLayout()
	line := "      " + text
	c.printBoxLine(box, line)
}

// printBoxLine renders a single line with box borders.
// Content is expected to already include left padding/indentation (typically "      ").
// This uses the unified renderBoxLine function for consistent border rendering.
func (c *Console) printBoxLine(box *BoxLayout, content string) {
	// Use unified rendering function (lipgloss handles width calculations)
	c.renderBoxLine(box, content)
}

// FormatTrend returns a trend indicator string based on current vs average.
// Returns "↑" (bad), "↓" (good), or "" (neutral) with appropriate color.
func FormatTrend(current int, average float64, upIsBad bool) string {
	if average == 0 {
		return ""
	}

	diff := float64(current) - average
	percentChange := (diff / average) * 100

	if percentChange > 5 {
		if upIsBad {
			return "↑" // Will be colored red by PrintMetricLine
		}
		return "↑"
	} else if percentChange < -5 {
		if upIsBad {
			return "↓" // Will be colored green by PrintMetricLine
		}
		return "↓"
	}
	return ""
}

// FormatDuration formats a duration in a human-readable way for display.
func FormatDuration(d float64) string {
	if d < 1 {
		return fmt.Sprintf("%.0fms", d*1000)
	}
	return fmt.Sprintf("%.1fs", d)
}

// FormatCount formats a count with optional units.
func FormatCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
