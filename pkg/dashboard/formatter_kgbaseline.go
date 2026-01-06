package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KGBaselineFormatter handles kg-baseline test suite JSON output.
type KGBaselineFormatter struct{}

func (f *KGBaselineFormatter) Matches(command string) bool {
	return strings.Contains(command, "kg-subsys.json") || strings.Contains(command, "kg-baseline.json")
}

// KGBaselineReport matches the JSON output from kg baseline tests.
type KGBaselineReport struct {
	TestSuite string             `json:"test_suite"`
	Version   string             `json:"version"`
	Timestamp string             `json:"timestamp"`
	Results   []KGBaselineResult `json:"results"`
	Summary   KGBaselineSummary  `json:"summary"`
}

// KGBaselineResult represents a single capability test result.
type KGBaselineResult struct {
	Capability string            `json:"capability"`
	Status     string            `json:"status"` // pass, fail, skip
	Metrics    KGBaselineMetrics `json:"metrics"`
	Timestamp  string            `json:"timestamp"`
	Error      string            `json:"error,omitempty"`
}

// KGBaselineMetrics contains performance metrics.
type KGBaselineMetrics struct {
	DurationMs  int64          `json:"duration_ms"`
	Throughput  float64        `json:"throughput,omitempty"`
	Accuracy    float64        `json:"accuracy,omitempty"`
	MemoryBytes int64          `json:"memory_bytes,omitempty"`
	Custom      map[string]any `json:"custom,omitempty"`
}

// KGBaselineSummary contains overall test results.
type KGBaselineSummary struct {
	Total      int    `json:"total"`
	Passed     int    `json:"passed"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	DurationMs int64  `json:"duration_ms"`
	Grade      string `json:"grade"`
}

// kgResultWidths holds alignment information for result formatting.
type kgResultWidths struct {
	name       int
	duration   int
	throughput int
}

// computeResultWidths calculates column widths for alignment.
func computeResultWidths(results []KGBaselineResult) kgResultWidths {
	w := kgResultWidths{}
	for _, r := range results {
		capName := strings.ReplaceAll(r.Capability, "_", " ")
		if len(capName) > w.name {
			w.name = len(capName)
		}
		if r.Metrics.DurationMs > int64(w.duration) {
			w.duration = int(r.Metrics.DurationMs)
		}
		if r.Metrics.Throughput > float64(w.throughput) {
			w.throughput = int(r.Metrics.Throughput)
		}
	}
	w.duration = len(fmt.Sprintf("%d", w.duration))
	w.throughput = len(fmt.Sprintf("%d", w.throughput))
	return w
}

// formatResult formats a single result line.
func formatResult(r KGBaselineResult, w kgResultWidths, s *FormatterStyles) string {
	var b strings.Builder

	statusIcon, statusStyle := IconSuccess, s.Success
	switch r.Status {
	case "fail":
		statusIcon, statusStyle = IconError, s.Error
	case "skip":
		statusIcon, statusStyle = IconPending, s.File
	}

	capName := strings.ReplaceAll(r.Capability, "_", " ")
	paddedName := fmt.Sprintf("%-*s", w.name, capName)

	b.WriteString(fmt.Sprintf("  %s %s", statusStyle.Render(statusIcon), s.File.Render(paddedName)))

	if r.Status == "pass" {
		b.WriteString(fmt.Sprintf("  %s", s.File.Render(fmt.Sprintf("%*dms", w.duration, r.Metrics.DurationMs))))
		if r.Metrics.Throughput > 0 {
			b.WriteString(fmt.Sprintf("  %s", s.Success.Render(fmt.Sprintf("%*.0f ops/s", w.throughput, r.Metrics.Throughput))))
		} else {
			b.WriteString(strings.Repeat(" ", w.throughput+7))
		}
		if r.Metrics.Accuracy > 0 {
			b.WriteString(fmt.Sprintf("  %s", s.Success.Render(fmt.Sprintf("%.0f%% acc", r.Metrics.Accuracy*100))))
		}
	}

	if r.Status == "skip" && r.Error != "" {
		errMsg := r.Error
		if len(errMsg) > 60 {
			errMsg = errMsg[:57] + "..."
		}
		b.WriteString(fmt.Sprintf("\n    %s", s.File.Render(errMsg)))
	}

	b.WriteString("\n")
	return b.String()
}

// formatProgressBar renders the summary progress bar.
// Returns empty string if bar would be single-color (uninformative).
func formatProgressBar(summary KGBaselineSummary, barWidth int) string {
	if summary.Total == 0 {
		return ""
	}

	// Count non-zero categories - skip bar if only one (uninformative)
	categories := 0
	if summary.Passed > 0 {
		categories++
	}
	if summary.Skipped > 0 {
		categories++
	}
	if summary.Failed > 0 {
		categories++
	}
	if categories <= 1 {
		return "" // Single-color bar is meaningless
	}

	var b strings.Builder

	passWidth := (summary.Passed * barWidth) / summary.Total
	skipWidth := (summary.Skipped * barWidth) / summary.Total
	failWidth := (summary.Failed * barWidth) / summary.Total

	// Ensure at least 1 char if non-zero
	if passWidth == 0 && summary.Passed > 0 {
		passWidth = 1
	}
	if skipWidth == 0 && summary.Skipped > 0 {
		skipWidth = 1
	}
	if failWidth == 0 && summary.Failed > 0 {
		failWidth = 1
	}

	if passWidth > 0 {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#04B575")).Render(strings.Repeat(" ", passWidth)))
	}
	if skipWidth > 0 {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#6B7280")).Render(strings.Repeat(" ", skipWidth)))
	}
	if failWidth > 0 {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("#FF5F56")).Render(strings.Repeat(" ", failWidth)))
	}
	return b.String()
}

// gradeStyle returns the appropriate style for a grade.
func gradeStyle(grade string, s *FormatterStyles) lipgloss.Style {
	switch grade {
	case "C", "D":
		return s.Warn
	case "F":
		return s.Error
	default:
		return s.Success
	}
}

func (f *KGBaselineFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Try to parse as KG baseline report
	var report KGBaselineReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return (&PlainFormatter{}).Format(lines, width)
	}
	if report.TestSuite != "kg-baseline" && report.TestSuite != "kg-subsys" {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	// Header with grade
	b.WriteString(s.Header.Render("◉ KG Subsystem"))
	b.WriteString("  ")
	b.WriteString(gradeStyle(report.Summary.Grade, s).Render(fmt.Sprintf("Grade: %s", report.Summary.Grade)))
	b.WriteString("  ")
	b.WriteString(s.File.Render(fmt.Sprintf("(%d/%d passed, %d skipped)",
		report.Summary.Passed, report.Summary.Total, report.Summary.Skipped)))
	b.WriteString("\n\n")

	// Results
	widths := computeResultWidths(report.Results)
	for _, r := range report.Results {
		b.WriteString(formatResult(r, widths, s))
	}
	b.WriteString("\n")

	// Summary bar
	b.WriteString(s.Header.Render("Summary"))
	b.WriteString("\n  ")
	barWidth := max(min(width-6, 60), 20)
	bar := formatProgressBar(report.Summary, barWidth)
	if bar != "" {
		b.WriteString(bar)
		b.WriteString("\n  ")
	}
	b.WriteString(s.File.Render(fmt.Sprintf("Duration: %.1fs", float64(report.Summary.DurationMs)/1000.0)))
	b.WriteString("\n")

	return b.String()
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *KGBaselineFormatter) GetStatus(lines []string) IndicatorStatus {
	var report KGBaselineReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return IndicatorDefault
	}

	// Verify it's actually a KG baseline report
	if report.TestSuite != "kg-baseline" && report.TestSuite != "kg-subsys" {
		return IndicatorDefault
	}

	// Grade-based status
	switch report.Summary.Grade {
	case "F":
		return IndicatorError
	case "C", "D":
		return IndicatorWarning
	default:
		// A, B grades are success
		return IndicatorSuccess
	}
}
