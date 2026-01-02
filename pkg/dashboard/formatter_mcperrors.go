package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// MCPErrorsFormatter handles mcp-logscan (formerly mcp-errors) -format=dashboard output.
type MCPErrorsFormatter struct{}

func (f *MCPErrorsFormatter) Matches(command string) bool {
	return (strings.Contains(command, "mcp-errors") || strings.Contains(command, "mcp-logscan")) &&
		strings.Contains(command, "-format=dashboard")
}

// MCPErrorsReport matches the JSON output from mcp-logscan -format=dashboard.
type MCPErrorsReport struct {
	Timestamp  string           `json:"timestamp"`
	LogFiles   []string         `json:"log_files"`
	ErrorCount int              `json:"error_count"`
	WarnCount  int              `json:"warn_count"`
	Errors     []MCPErrorDetail `json:"errors"`
}

// MCPErrorDetail represents a single error entry.
type MCPErrorDetail struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// maxDisplayErrors is the maximum number of errors to show in the detail panel.
const maxDisplayErrors = 10

// maxErrorAgeDays filters out errors older than this many days.
const maxErrorAgeDays = 5

func (f *MCPErrorsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Find the JSON object in the output (skip any build/download messages)
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return (&PlainFormatter{}).Format(lines, width)
	}
	jsonOutput := fullOutput[jsonStart:]

	var report MCPErrorsReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#666666"))

	// Header with summary
	b.WriteString(s.Header.Render("◉ MCP Server Logs"))
	b.WriteString("\n\n")

	if report.ErrorCount == 0 && report.WarnCount == 0 {
		b.WriteString(s.Success.Render("✓ No errors or warnings"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary counts
	if report.ErrorCount > 0 {
		b.WriteString(s.Error.Render(fmt.Sprintf("  %d errors", report.ErrorCount)))
	} else {
		b.WriteString(s.Success.Render("  0 errors"))
	}
	b.WriteString("  ")
	if report.WarnCount > 0 {
		b.WriteString(s.Warn.Render(fmt.Sprintf("%d warnings", report.WarnCount)))
	} else {
		b.WriteString(s.Muted.Render("0 warnings"))
	}
	b.WriteString("\n\n")

	// Filter and show recent errors (up to maxDisplayErrors, within maxErrorAgeDays)
	if len(report.Errors) > 0 {
		b.WriteString(s.Header.Render("Recent Errors"))
		b.WriteString("\n")
		shown := 0
		skipped := 0
		cutoff := time.Now().AddDate(0, 0, -maxErrorAgeDays)

		// Iterate backwards for reverse chronological order (newest first)
		for i := len(report.Errors) - 1; i >= 0 && shown < maxDisplayErrors; i-- {
			e := report.Errors[i]

			// Filter by age
			if errorTime, err := time.Parse(time.RFC3339, e.Time); err == nil {
				if errorTime.Before(cutoff) {
					skipped++
					continue
				}
			}

			// Icon instead of [ERROR]/[WARN]
			icon := "✗"
			levelStyle := s.Error
			if e.Level == "WARN" || e.Level == "WARNING" {
				icon = "⚠"
				levelStyle = s.Warn
			}

			// Format timestamp
			timestamp := ""
			if e.Time != "" {
				if t, err := time.Parse(time.RFC3339, e.Time); err == nil {
					timestamp = t.Format("Jan 2 15:04")
				}
			}

			msg := stripMCPLogPrefix(e.Message)
			// Reserve space for icon (2) + timestamp (12) + padding (4)
			maxMsgWidth := width - 18
			if maxMsgWidth > 3 && len(msg) > maxMsgWidth {
				msg = msg[:maxMsgWidth-3] + "..."
			}

			b.WriteString(fmt.Sprintf("  %s %s  %s\n",
				levelStyle.Render(icon),
				timeStyle.Render(timestamp),
				msg))

			if e.Detail != "" {
				detail := e.Detail
				if len(detail) > width-12 && width > 15 {
					detail = detail[:width-15] + "..."
				}
				b.WriteString(fmt.Sprintf("       %s\n", detailStyle.Render(detail)))
			}
			shown++
		}
		remaining := len(report.Errors) - shown - skipped
		if remaining > 0 {
			b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", remaining)))
		}
	}

	return b.String()
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *MCPErrorsFormatter) GetStatus(lines []string) IndicatorStatus {
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return IndicatorDefault
	}
	jsonOutput := fullOutput[jsonStart:]

	var report MCPErrorsReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		return IndicatorDefault
	}

	// MCP log errors are informational (lock conflicts, connection issues)
	// not build-breaking - show as warning to surface but not fail QA
	if report.ErrorCount > 0 || report.WarnCount > 0 {
		return IndicatorWarning
	}
	return IndicatorSuccess
}

// stripMCPLogPrefix removes the verbose MCP log prefix that appears in stderr.
// Example: "   [ERROR] (https://modelcontextprotocol.io/docs/tools/debugging) actual message"
func stripMCPLogPrefix(msg string) string {
	// Remove leading whitespace and [LEVEL] tag since we render our own
	msg = strings.TrimLeft(msg, " \t")
	for _, level := range []string{"[ERROR]", "[WARN]", "[WARNING]", "[INFO]"} {
		if after, found := strings.CutPrefix(msg, level); found {
			msg = strings.TrimLeft(after, " \t")
			break
		}
	}
	// Remove the documentation URL prefix
	const docURL = "(https://modelcontextprotocol.io/docs/tools/debugging)"
	if after, found := strings.CutPrefix(msg, docURL); found {
		msg = strings.TrimLeft(after, " \t")
	}
	return msg
}
