package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GoArchLintFormatter handles go-arch-lint --json output.
type GoArchLintFormatter struct{}

func (f *GoArchLintFormatter) Matches(command string) bool {
	return strings.Contains(command, "go-arch-lint")
}

// archLintReport represents the go-arch-lint JSON output structure.
type archLintReport struct {
	Type    string `json:"Type"`
	Payload struct {
		ArchHasWarnings  bool `json:"ArchHasWarnings"`
		ArchWarningsDeps []struct {
			ComponentFrom string `json:"ComponentFrom"`
			ComponentTo   string `json:"ComponentTo"`
			FileRelPath   string `json:"FileRelativePath"`
		} `json:"ArchWarningsDeps"`
		ArchWarningsNotMatched []struct {
			FileRelPath string `json:"FileRelativePath"`
		} `json:"ArchWarningsNotMatched"`
		ArchWarningsDeepScan []struct {
			Gate        string `json:"Gate"`
			ComponentTo string `json:"ComponentTo"`
			FileRelPath string `json:"FileRelativePath"`
		} `json:"ArchWarningsDeepScan"`
		OmittedCount int `json:"OmittedCount"`
	} `json:"Payload"`
}

func (f *GoArchLintFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Parse JSON
	fullOutput := strings.Join(lines, "\n")
	var report archLintReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Check for warnings
	depCount := len(report.Payload.ArchWarningsDeps)
	unmatchedCount := len(report.Payload.ArchWarningsNotMatched)
	deepScanCount := len(report.Payload.ArchWarningsDeepScan)
	totalIssues := depCount + unmatchedCount + deepScanCount

	if !report.Payload.ArchHasWarnings || totalIssues == 0 {
		b.WriteString(successStyle.Render("✓ No architecture violations\n"))
		return b.String()
	}

	// Summary
	b.WriteString(warnStyle.Render(fmt.Sprintf("△ %d architecture issues", totalIssues)))
	b.WriteString("\n\n")

	// Dependency violations
	if depCount > 0 {
		b.WriteString(headerStyle.Render("◉ Dependency Violations"))
		b.WriteString(errorStyle.Render(fmt.Sprintf(" (%d)", depCount)))
		b.WriteString("\n")
		for i, dep := range report.Payload.ArchWarningsDeps {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", depCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				warnStyle.Render(dep.ComponentFrom),
				errorStyle.Render(dep.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", fileStyle.Render(shortPath(dep.FileRelPath))))
		}
		b.WriteString("\n")
	}

	// Unmatched files
	if unmatchedCount > 0 {
		b.WriteString(headerStyle.Render("◉ Unmatched Files"))
		b.WriteString(warnStyle.Render(fmt.Sprintf(" (%d)", unmatchedCount)))
		b.WriteString("\n")
		for i, um := range report.Payload.ArchWarningsNotMatched {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", unmatchedCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(um.FileRelPath)))
		}
		b.WriteString("\n")
	}

	// Deep scan violations
	if deepScanCount > 0 {
		b.WriteString(headerStyle.Render("◉ Deep Scan Violations"))
		b.WriteString(errorStyle.Render(fmt.Sprintf(" (%d)", deepScanCount)))
		b.WriteString("\n")
		for i, ds := range report.Payload.ArchWarningsDeepScan {
			if i >= 10 {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", deepScanCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				warnStyle.Render(ds.Gate),
				errorStyle.Render(ds.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", fileStyle.Render(shortPath(ds.FileRelPath))))
		}
	}

	return b.String()
}
