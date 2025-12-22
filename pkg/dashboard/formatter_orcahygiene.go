package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// OrcaHygieneFormatter handles orca-hygiene -format=dashboard output.
type OrcaHygieneFormatter struct{}

func (f *OrcaHygieneFormatter) Matches(command string) bool {
	return strings.Contains(command, "orca-hygiene") && strings.Contains(command, "-format=dashboard")
}

// OrcaHygieneReport matches the JSON output from orca-hygiene -format=dashboard.
type OrcaHygieneReport struct {
	Status   string              `json:"status"`
	Summary  string              `json:"summary"`
	Issues   []OrcaHygieneIssue  `json:"issues"`
	ExitCode int                 `json:"exit_code"`
}

// OrcaHygieneIssue represents a single hygiene issue.
type OrcaHygieneIssue struct {
	Severity string `json:"severity"`
	Category string `json:"category"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

func (f *OrcaHygieneFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Find the JSON object in the output (skip any build/download messages)
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return (&PlainFormatter{}).Format(lines, width)
	}
	jsonOutput := fullOutput[jsonStart:]

	var report OrcaHygieneReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Styles
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Header
	b.WriteString(headerStyle.Render("◉ Orca Hygiene"))
	b.WriteString("\n\n")

	if len(report.Issues) == 0 {
		b.WriteString(successStyle.Render("✓ All checks passed"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary
	b.WriteString(mutedStyle.Render(report.Summary))
	b.WriteString("\n\n")

	// Group issues by severity
	var errors, warnings []OrcaHygieneIssue
	for _, issue := range report.Issues {
		if issue.Severity == "error" {
			errors = append(errors, issue)
		} else {
			warnings = append(warnings, issue)
		}
	}

	// Show errors first
	if len(errors) > 0 {
		b.WriteString(errorStyle.Render("Errors"))
		b.WriteString("\n")
		for _, issue := range errors {
			renderHygieneIssue(&b, issue, pathStyle, mutedStyle, width)
		}
		b.WriteString("\n")
	}

	// Then warnings
	if len(warnings) > 0 {
		b.WriteString(warnStyle.Render("Warnings"))
		b.WriteString("\n")
		for _, issue := range warnings {
			renderHygieneIssue(&b, issue, pathStyle, mutedStyle, width)
		}
	}

	return b.String()
}

func renderHygieneIssue(b *strings.Builder, issue OrcaHygieneIssue, pathStyle, mutedStyle lipgloss.Style, width int) {
	// Show category and message
	msg := issue.Message
	if len(msg) > width-10 && width > 13 {
		msg = msg[:width-13] + "..."
	}
	fmt.Fprintf(b, "  [%s] %s\n", issue.Category, msg)

	// Show path if present
	if issue.Path != "" {
		path := issue.Path
		if len(path) > width-6 && width > 9 {
			path = "..." + path[len(path)-(width-9):]
		}
		fmt.Fprintf(b, "    %s\n", pathStyle.Render(path))
	}

	// Show fix suggestion if present
	if issue.Fix != "" {
		fix := issue.Fix
		if len(fix) > width-8 && width > 11 {
			fix = fix[:width-11] + "..."
		}
		fmt.Fprintf(b, "    %s\n", mutedStyle.Render("→ "+fix))
	}
}
