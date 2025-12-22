package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NugstatsFormatter handles nugstats -format=dashboard output.
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

// NugstatsKindCount represents a count by kind.
type NugstatsKindCount struct {
	Kind     string `json:"kind"`
	Count    int    `json:"count"`
	ThisWeek int    `json:"this_week"`
	Delta    int    `json:"delta"`
}

// NugstatsWeekly represents weekly data.
type NugstatsWeekly struct {
	Week  string `json:"week"`
	Added int    `json:"added"`
}

func (f *NugstatsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Find the JSON object in the output (skip any build/download messages)
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return (&PlainFormatter{}).Format(lines, width)
	}
	jsonOutput := fullOutput[jsonStart:]

	var report NugstatsReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
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

	// Find max width for kind names, counts, and deltas for alignment
	maxKindLen := 0
	maxCount := 0
	maxDelta := 0
	for _, k := range report.ByKind {
		if len(k.Kind) > maxKindLen {
			maxKindLen = len(k.Kind)
		}
		if k.Count > maxCount {
			maxCount = k.Count
		}
		absDelta := k.Delta
		if absDelta < 0 {
			absDelta = -absDelta
		}
		if absDelta > maxDelta {
			maxDelta = absDelta
		}
	}
	countWidth := max(len(fmt.Sprintf("%d", maxCount)), 3)
	deltaWidth := max(len(fmt.Sprintf("%d", maxDelta)), 1)

	for _, k := range report.ByKind {
		// Render delta with fixed-width: arrow aligned, number right-aligned
		delta := strings.Repeat(" ", deltaWidth+2) // space for "↑" + space + number
		if k.Delta > 0 {
			delta = deltaUpStyle.Render(fmt.Sprintf("↑%*d", deltaWidth, k.Delta))
		} else if k.Delta < 0 {
			delta = deltaDownStyle.Render(fmt.Sprintf("↓%*d", deltaWidth, -k.Delta))
		}
		// Right-align count column, then delta column
		b.WriteString(fmt.Sprintf("  %-*s  %s  %s\n",
			maxKindLen,
			kindStyle.Render(k.Kind),
			countStyle.Render(fmt.Sprintf("%*d", countWidth, k.Count)),
			delta))
	}

	// Horizontal stacked bar proportional to kind counts
	if len(report.ByKind) > 0 && report.Total > 0 {
		b.WriteString("\n  ")
		barWidth := max(min(width-6, 60), 20) // clamp to [20, 60]

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
