package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FilesizeDashboardFormatter handles filesize -format=dashboard output.
type FilesizeDashboardFormatter struct{}

func (f *FilesizeDashboardFormatter) Matches(command string) bool {
	return strings.Contains(command, "filesize") && strings.Contains(command, "-format=dashboard")
}

// FilesizeDashboard represents the dashboard JSON output from filesize.
type FilesizeDashboard struct {
	Timestamp string                   `json:"timestamp"`
	Metrics   FilesizeDashboardMetrics `json:"metrics"`
	Deltas    FilesizeDashboardDeltas  `json:"deltas"`
	TopFiles  []FilesizeDashboardFile  `json:"top_files"`
	History   []FilesizeHistoryEntry   `json:"history"`
}

// FilesizeDashboardMetrics holds the file size metrics.
type FilesizeDashboardMetrics struct {
	Total     int `json:"total"`
	Green     int `json:"green"`
	Yellow    int `json:"yellow"`
	Red       int `json:"red"`
	TestFiles int `json:"test_files"`
	MDFiles   int `json:"md_files"`
	OrphanMD  int `json:"orphan_md"`
}

// FilesizeDashboardDeltas contains deltas at different time intervals.
type FilesizeDashboardDeltas struct {
	Day   FilesizeMetricDeltas `json:"day"`
	Week  FilesizeMetricDeltas `json:"week"`
	Month FilesizeMetricDeltas `json:"month"`
}

// FilesizeMetricDeltas holds delta values for each metric.
type FilesizeMetricDeltas struct {
	Total     int `json:"total"`
	Green     int `json:"green"`
	Yellow    int `json:"yellow"`
	Red       int `json:"red"`
	TestFiles int `json:"test_files"`
	MDFiles   int `json:"md_files"`
	OrphanMD  int `json:"orphan_md"`
}

// FilesizeDashboardFile represents a single file in the top files list.
type FilesizeDashboardFile struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Tier  string `json:"tier"`
}

// FilesizeHistoryEntry represents a historical data point.
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
	var dashboard FilesizeDashboard
	if !decodeJSONLines(lines, &dashboard) {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Validate we got actual data
	if dashboard.Metrics.Total == 0 && len(dashboard.TopFiles) == 0 {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	m := dashboard.Metrics

	// ── Top 5 Largest Files ──────────────────────────────────────────────
	b.WriteString(s.Header.Render("◉ Largest Source Files"))
	b.WriteString("\n\n")

	for i, file := range dashboard.TopFiles {
		if i >= 5 {
			break
		}
		var tierStyle lipgloss.Style
		switch file.Tier {
		case "red":
			tierStyle = s.Error
		case "yellow":
			tierStyle = s.Warn
		default:
			tierStyle = s.Success
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			tierStyle.Render(fmt.Sprintf("%4d", file.Lines)),
			s.File.Render(file.Path)))
	}
	b.WriteString("\n")

	// ── File Size Distribution ───────────────────────────────────────────
	b.WriteString(s.Header.Render("◉ Size Distribution"))
	b.WriteString("\n")

	// Calculate max delta width for alignment (needed for header alignment)
	maxDelta := 0
	for _, delta := range []int{
		abs(dashboard.Deltas.Day.Red), abs(dashboard.Deltas.Week.Red), abs(dashboard.Deltas.Month.Red),
		abs(dashboard.Deltas.Day.Yellow), abs(dashboard.Deltas.Week.Yellow), abs(dashboard.Deltas.Month.Yellow),
		abs(dashboard.Deltas.Day.Green), abs(dashboard.Deltas.Week.Green), abs(dashboard.Deltas.Month.Green),
		abs(dashboard.Deltas.Day.TestFiles), abs(dashboard.Deltas.Week.TestFiles), abs(dashboard.Deltas.Month.TestFiles),
		abs(dashboard.Deltas.Day.MDFiles), abs(dashboard.Deltas.Week.MDFiles), abs(dashboard.Deltas.Month.MDFiles),
		abs(dashboard.Deltas.Day.OrphanMD), abs(dashboard.Deltas.Week.OrphanMD), abs(dashboard.Deltas.Month.OrphanMD),
	} {
		if delta > maxDelta {
			maxDelta = delta
		}
	}
	deltaWidth := max(len(fmt.Sprintf("%d", maxDelta)), 1)
	// Delta columns are arrow (1) + number (deltaWidth) = deltaWidth+1 chars
	deltaColWidth := deltaWidth + 1

	// Header row with delta time periods
	// Label is 14 chars + colon = 15, count is 4 chars
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		strings.Repeat(" ", 15), // label space (14 + colon)
		strings.Repeat(" ", 4),  // count space
		headerStyle.Render(fmt.Sprintf("%*s", deltaColWidth, "1d")),
		headerStyle.Render(fmt.Sprintf("%*s", deltaColWidth, "1w")),
		headerStyle.Render(fmt.Sprintf("%*s", deltaColWidth, "1mo"))))
	b.WriteString("\n")

	deltaUpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))   // red - up is bad
	deltaDownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")) // green - down is good

	// Red (>1000 LOC) - up is bad
	redStyle := s.Success
	if m.Red > 0 {
		redStyle = s.Error
	}
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", ">1000 LOC")),
		redStyle.Render(fmt.Sprintf("%4d", m.Red)),
		renderDelta(dashboard.Deltas.Day.Red, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Week.Red, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Month.Red, deltaWidth, deltaUpStyle, deltaDownStyle, true)))

	// Yellow (500-999 LOC) - up is bad
	yellowStyle := s.Success
	if m.Yellow > 0 {
		yellowStyle = s.Warn
	}
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "500-999 LOC")),
		yellowStyle.Render(fmt.Sprintf("%4d", m.Yellow)),
		renderDelta(dashboard.Deltas.Day.Yellow, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Week.Yellow, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Month.Yellow, deltaWidth, deltaUpStyle, deltaDownStyle, true)))

	// Green (<500 LOC) - up is good
	deltaUpGood := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))  // green - up is good
	deltaDownBad := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")) // red - down is bad
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "<500 LOC")),
		s.Success.Render(fmt.Sprintf("%4d", m.Green)),
		renderDelta(dashboard.Deltas.Day.Green, deltaWidth, deltaUpGood, deltaDownBad, false),
		renderDelta(dashboard.Deltas.Week.Green, deltaWidth, deltaUpGood, deltaDownBad, false),
		renderDelta(dashboard.Deltas.Month.Green, deltaWidth, deltaUpGood, deltaDownBad, false)))

	b.WriteString("\n")

	// Neutral style for test/MD files (gray arrows)
	deltaNeutral := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Test files (neutral)
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Test files")),
		s.File.Render(fmt.Sprintf("%4d", m.TestFiles)),
		renderDelta(dashboard.Deltas.Day.TestFiles, deltaWidth, deltaNeutral, deltaNeutral, true),
		renderDelta(dashboard.Deltas.Week.TestFiles, deltaWidth, deltaNeutral, deltaNeutral, true),
		renderDelta(dashboard.Deltas.Month.TestFiles, deltaWidth, deltaNeutral, deltaNeutral, true)))

	// MD files (neutral)
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Markdown files")),
		s.File.Render(fmt.Sprintf("%4d", m.MDFiles)),
		renderDelta(dashboard.Deltas.Day.MDFiles, deltaWidth, deltaNeutral, deltaNeutral, true),
		renderDelta(dashboard.Deltas.Week.MDFiles, deltaWidth, deltaNeutral, deltaNeutral, true),
		renderDelta(dashboard.Deltas.Month.MDFiles, deltaWidth, deltaNeutral, deltaNeutral, true)))

	// Orphan MD (any > 0 is wrong) - up is bad
	orphanStyle := s.Success
	if m.OrphanMD > 0 {
		orphanStyle = s.Error
	}
	b.WriteString(fmt.Sprintf("  %s %s  %s  %s  %s\n",
		labelStyle.Render(fmt.Sprintf("%14s:", "Orphan docs")),
		orphanStyle.Render(fmt.Sprintf("%4d", m.OrphanMD)),
		renderDelta(dashboard.Deltas.Day.OrphanMD, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Week.OrphanMD, deltaWidth, deltaUpStyle, deltaDownStyle, true),
		renderDelta(dashboard.Deltas.Month.OrphanMD, deltaWidth, deltaUpStyle, deltaDownStyle, true)))

	// ── Weekly Trend (if history available) ──────────────────────────────
	if len(dashboard.History) > 1 {
		b.WriteString("\n")
		b.WriteString(s.Header.Render("◉ 4-Week Trend"))
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
					s.Muted.Render(h.Week),
					s.Muted.Render(strings.Repeat("·", barWidth))))
				continue
			}

			greenChars := (h.Green * barWidth) / total
			yellowChars := (h.Yellow * barWidth) / total
			redChars := barWidth - greenChars - yellowChars

			bar := s.Success.Render(strings.Repeat("█", greenChars)) +
				s.Warn.Render(strings.Repeat("█", yellowChars)) +
				s.Error.Render(strings.Repeat("█", redChars))

			paddedWeek := fmt.Sprintf("%-10s", h.Week)
			b.WriteString(fmt.Sprintf("  %s %s\n", s.Muted.Render(paddedWeek), bar))
		}
	}

	return b.String()
}

// renderDelta formats a delta value with arrow and right-aligned number.
// Follows the nugstats pattern: "↑  5" or "↓ 12" with fixed width.
func renderDelta(delta, width int, upStyle, downStyle lipgloss.Style, upIsBad bool) string {
	if delta == 0 {
		return strings.Repeat(" ", width+1) // arrow (1) + number (width)
	}

	// Swap styles if up is good
	if !upIsBad {
		upStyle, downStyle = downStyle, upStyle
	}

	if delta > 0 {
		return upStyle.Render(fmt.Sprintf("↑%*d", width, delta))
	}
	return downStyle.Render(fmt.Sprintf("↓%*d", width, -delta))
}

// abs returns the absolute value of an integer.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
