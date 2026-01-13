package dashboard

import (
	"fmt"
	"strings"
)

// HTMLHygieneFormatter handles html-export-hygiene -format=dashboard output.
type HTMLHygieneFormatter struct{}

func (f *HTMLHygieneFormatter) Matches(command string) bool {
	return strings.Contains(command, "html-export-hygiene") && strings.Contains(command, "-format=dashboard")
}

// HTMLHygieneReport matches the JSON output from html-export-hygiene -format=dashboard.
type HTMLHygieneReport struct {
	Status   string             `json:"status"`
	Summary  string             `json:"summary"`
	Stats    map[string]int     `json:"stats"`
	Issues   []HTMLHygieneIssue `json:"issues"`
	ExitCode int                `json:"exit_code"`
}

// HTMLHygieneIssue represents a single hygiene issue.
type HTMLHygieneIssue struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

func (f *HTMLHygieneFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	var report HTMLHygieneReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	// Header
	b.WriteString(s.Header.Render("◉ HTML Export Hygiene"))
	b.WriteString("\n\n")

	// Summary based on status
	if report.Status == "pass" && len(report.Issues) == 0 {
		b.WriteString(s.Success.Render("✓ No issues found"))
		b.WriteString("\n")
		return b.String()
	}

	// Count by severity
	errors, warnings, infos := 0, 0, 0
	for _, issue := range report.Issues {
		switch issue.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		case "info":
			infos++
		}
	}

	// Status line
	if errors > 0 {
		b.WriteString(s.Error.Render(fmt.Sprintf("✗ %d errors", errors)))
	} else {
		b.WriteString(s.Success.Render("  0 errors"))
	}
	b.WriteString("  ")
	if warnings > 0 {
		b.WriteString(s.Warn.Render(fmt.Sprintf("%d warnings", warnings)))
	} else {
		b.WriteString(s.Muted.Render("0 warnings"))
	}
	b.WriteString("  ")
	b.WriteString(s.Muted.Render(fmt.Sprintf("%d info", infos)))
	b.WriteString("\n\n")

	// Show errors first, then warnings (skip info for brevity)
	maxShow := 10
	shown := 0

	for _, issue := range report.Issues {
		if issue.Severity != "error" {
			continue
		}
		if shown >= maxShow {
			break
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", s.Error.Render("✗"), issue.Message))
		if issue.Path != "" {
			b.WriteString(fmt.Sprintf("    %s\n", s.Muted.Render(issue.Path)))
		}
		shown++
	}

	for _, issue := range report.Issues {
		if issue.Severity != "warning" {
			continue
		}
		if shown >= maxShow {
			break
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", s.Warn.Render("⚠"), issue.Message))
		shown++
	}

	remaining := len(report.Issues) - shown
	if remaining > 0 {
		b.WriteString(s.Muted.Render(fmt.Sprintf("\n  ... and %d more issues\n", remaining)))
	}

	return b.String()
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *HTMLHygieneFormatter) GetStatus(lines []string) IndicatorStatus {
	var report HTMLHygieneReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return IndicatorDefault
	}

	// Check status field from the report
	if report.Status == "fail" {
		return IndicatorError
	}

	// Also check for any error-severity issues
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			return IndicatorError
		}
	}

	// Check for warnings
	for _, issue := range report.Issues {
		if issue.Severity == "warning" {
			return IndicatorWarning
		}
	}

	return IndicatorSuccess
}
