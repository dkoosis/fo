package dashboard

import (
	"encoding/json"
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

	for i, file := range dashboard.TopFiles {
		if i >= 5 {
			break
		}
		var tierStyle lipgloss.Style
		switch file.Tier {
		case "red":
			tierStyle = errorStyle
		case "yellow":
			tierStyle = warnStyle
		default:
			tierStyle = successStyle
		}
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			tierStyle.Render(fmt.Sprintf("%4d", file.Lines)),
			fileStyle.Render(file.Path)))
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

			paddedWeek := fmt.Sprintf("%-10s", h.Week)
			b.WriteString(fmt.Sprintf("  %s %s\n", mutedStyle.Render(paddedWeek), bar))
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
