package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KGBaselineFormatter handles kg-baseline test suite JSON output.
type KGBaselineFormatter struct{}

func (f *KGBaselineFormatter) Matches(command string) bool {
	return strings.Contains(command, "kg-baseline.json")
}

// KGBaselineReport matches the JSON output from kg baseline tests.
type KGBaselineReport struct {
	TestSuite string                `json:"test_suite"`
	Version   string                `json:"version"`
	Timestamp string                `json:"timestamp"`
	Results   []KGBaselineResult    `json:"results"`
	Summary   KGBaselineSummary     `json:"summary"`
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
	DurationMs  int64              `json:"duration_ms"`
	Throughput  float64            `json:"throughput,omitempty"`
	Accuracy    float64            `json:"accuracy,omitempty"`
	MemoryBytes int64              `json:"memory_bytes,omitempty"`
	Custom      map[string]any     `json:"custom,omitempty"`
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

func (f *KGBaselineFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Find JSON in output
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return (&PlainFormatter{}).Format(lines, width)
	}
	jsonOutput := fullOutput[jsonStart:]

	// Try to parse as KG baseline report
	var report KGBaselineReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Verify it's actually a KG baseline report
	if report.TestSuite != "kg-baseline" {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	// Header with grade
	b.WriteString(s.Header.Render("◉ KG Baseline Suite"))
	b.WriteString("  ")
	gradeStyle := s.Success
	if report.Summary.Grade == "C" || report.Summary.Grade == "D" {
		gradeStyle = s.Warn
	} else if report.Summary.Grade == "F" {
		gradeStyle = s.Error
	}
	b.WriteString(gradeStyle.Render(fmt.Sprintf("Grade: %s", report.Summary.Grade)))
	b.WriteString("  ")
	b.WriteString(s.File.Render(fmt.Sprintf("(%d/%d passed, %d skipped)",
		report.Summary.Passed, report.Summary.Total, report.Summary.Skipped)))
	b.WriteString("\n\n")

	// Group results by category
	categories := map[string][]KGBaselineResult{
		"Storage":       {},
		"Search":        {},
		"Semantic":      {},
		"Import/Export": {},
		"Validation":    {},
		"Migrations":    {},
	}

	for _, r := range report.Results {
		cap := r.Capability
		if strings.HasPrefix(cap, "storage_crud") {
			categories["Storage"] = append(categories["Storage"], r)
		} else if strings.HasPrefix(cap, "search_bm25") {
			categories["Search"] = append(categories["Search"], r)
		} else if strings.HasPrefix(cap, "semantic_") || strings.HasPrefix(cap, "hybrid_") {
			categories["Semantic"] = append(categories["Semantic"], r)
		} else if strings.HasPrefix(cap, "import_export") {
			categories["Import/Export"] = append(categories["Import/Export"], r)
		} else if strings.Contains(cap, "validation") {
			categories["Validation"] = append(categories["Validation"], r)
		} else if strings.Contains(cap, "migration") {
			categories["Migrations"] = append(categories["Migrations"], r)
		}
	}

	// Display categories in order
	catOrder := []string{"Storage", "Search", "Semantic", "Import/Export", "Validation", "Migrations"}
	for _, cat := range catOrder {
		results := categories[cat]
		if len(results) == 0 {
			continue
		}

		b.WriteString(s.Header.Render(cat))
		b.WriteString("\n")

		for _, r := range results {
			// Status icon
			statusIcon := "✓"
			statusStyle := s.Success
			if r.Status == "fail" {
				statusIcon = "✗"
				statusStyle = s.Error
			} else if r.Status == "skip" {
				statusIcon = "○"
				statusStyle = s.File
			}

			// Capability name (clean up underscores)
			capName := strings.ReplaceAll(r.Capability, "_", " ")
			capName = strings.TrimPrefix(capName, "storage crud ")
			capName = strings.TrimPrefix(capName, "search bm25 ")
			capName = strings.TrimPrefix(capName, "import export ")
			capName = strings.TrimPrefix(capName, "semantic ")

			b.WriteString(fmt.Sprintf("  %s %s",
				statusStyle.Render(statusIcon),
				s.File.Render(capName)))

			// Metrics
			if r.Status == "pass" && r.Metrics.DurationMs > 0 {
				b.WriteString(fmt.Sprintf("  %s",
					s.File.Render(fmt.Sprintf("(%dms", r.Metrics.DurationMs))))

				if r.Metrics.Throughput > 0 {
					b.WriteString(fmt.Sprintf(", %s",
						s.Success.Render(fmt.Sprintf("%.0f ops/s", r.Metrics.Throughput))))
				}
				if r.Metrics.Accuracy > 0 {
					b.WriteString(fmt.Sprintf(", %s",
						s.Success.Render(fmt.Sprintf("%.0f%% acc", r.Metrics.Accuracy*100))))
				}
				b.WriteString(s.File.Render(")"))
			}

			// Error message for failures/skips
			if r.Status == "skip" && r.Error != "" {
				errMsg := r.Error
				if len(errMsg) > 60 {
					errMsg = errMsg[:57] + "..."
				}
				b.WriteString(fmt.Sprintf("\n    %s", s.File.Render(errMsg)))
			}

			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Summary bar
	b.WriteString(s.Header.Render("Summary"))
	b.WriteString("\n  ")
	barWidth := max(min(width-6, 60), 20)

	// Progress bar: green for pass, gray for skip, red for fail
	if report.Summary.Total > 0 {
		passWidth := (report.Summary.Passed * barWidth) / report.Summary.Total
		skipWidth := (report.Summary.Skipped * barWidth) / report.Summary.Total
		failWidth := (report.Summary.Failed * barWidth) / report.Summary.Total

		// Ensure at least 1 char if non-zero
		if passWidth == 0 && report.Summary.Passed > 0 {
			passWidth = 1
		}
		if skipWidth == 0 && report.Summary.Skipped > 0 {
			skipWidth = 1
		}
		if failWidth == 0 && report.Summary.Failed > 0 {
			failWidth = 1
		}

		// Render bars
		if passWidth > 0 {
			passStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575"))
			b.WriteString(passStyle.Render(strings.Repeat(" ", passWidth)))
		}
		if skipWidth > 0 {
			skipStyle := lipgloss.NewStyle().Background(lipgloss.Color("#6B7280"))
			b.WriteString(skipStyle.Render(strings.Repeat(" ", skipWidth)))
		}
		if failWidth > 0 {
			failStyle := lipgloss.NewStyle().Background(lipgloss.Color("#FF5F56"))
			b.WriteString(failStyle.Render(strings.Repeat(" ", failWidth)))
		}
	}

	b.WriteString("\n  ")
	b.WriteString(s.File.Render(fmt.Sprintf("Duration: %.1fs", float64(report.Summary.DurationMs)/1000.0)))
	b.WriteString("\n")

	return b.String()
}
