package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// MCPErrorsFormatter handles mcp-errors -format=dashboard output.
type MCPErrorsFormatter struct{}

func (f *MCPErrorsFormatter) Matches(command string) bool {
	return strings.Contains(command, "mcp-errors") && strings.Contains(command, "-format=dashboard")
}

// MCPErrorsReport matches the JSON output from mcp-errors -format=dashboard.
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

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Header with summary
	b.WriteString(headerStyle.Render("◉ MCP Server Logs"))
	b.WriteString("\n\n")

	if report.ErrorCount == 0 && report.WarnCount == 0 {
		b.WriteString(successStyle.Render("✓ No errors or warnings"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary counts
	if report.ErrorCount > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  %d errors", report.ErrorCount)))
	} else {
		b.WriteString(successStyle.Render("  0 errors"))
	}
	b.WriteString("  ")
	if report.WarnCount > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf("%d warnings", report.WarnCount)))
	} else {
		b.WriteString(mutedStyle.Render("0 warnings"))
	}
	b.WriteString("\n\n")

	// Show recent errors (up to 5)
	if len(report.Errors) > 0 {
		b.WriteString(headerStyle.Render("Recent Errors"))
		b.WriteString("\n")
		shown := 0
		for i := len(report.Errors) - 1; i >= 0 && shown < 5; i-- {
			e := report.Errors[i]
			levelStyle := errorStyle
			if e.Level == "WARN" {
				levelStyle = warnStyle
			}
			msg := e.Message
			if len(msg) > width-15 && width > 18 {
				msg = msg[:width-18] + "..."
			}
			b.WriteString(fmt.Sprintf("  %s %s\n",
				levelStyle.Render("["+e.Level+"]"),
				msg))
			if e.Detail != "" {
				detail := e.Detail
				if len(detail) > width-12 && width > 15 {
					detail = detail[:width-15] + "..."
				}
				b.WriteString(fmt.Sprintf("         %s\n", detailStyle.Render(detail)))
			}
			shown++
		}
		if len(report.Errors) > 5 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(report.Errors)-5)))
		}
	}

	return b.String()
}
