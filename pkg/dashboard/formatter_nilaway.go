package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// NilawayFormatter handles nilaway -json output.
type NilawayFormatter struct{}

func (f *NilawayFormatter) Matches(command string) bool {
	return strings.Contains(command, "nilaway")
}

// nilawayFinding represents a single nilaway finding from JSON.
type nilawayFinding struct {
	Posn    string `json:"posn"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

// nilawayAnalyzerResult represents the nilaway analyzer output within a package.
type nilawayAnalyzerResult struct {
	Nilaway []nilawayFinding `json:"nilaway"`
}

func (f *NilawayFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	s := Styles()
	// reasonStyle is a lighter gray for secondary info
	reasonStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	// Combine lines and parse JSON
	combined := strings.Join(lines, "\n")
	if strings.TrimSpace(combined) == "" {
		b.WriteString(s.Success.Render("✓ No nil pointer issues found\n"))
		return b.String()
	}

	// Try parsing as nested format: {"pkg": {"nilaway": [...]}}
	var nested map[string]nilawayAnalyzerResult
	var allFindings []nilawayFinding

	if err := json.Unmarshal([]byte(combined), &nested); err == nil {
		for _, ar := range nested {
			allFindings = append(allFindings, ar.Nilaway...)
		}
	}

	if len(allFindings) == 0 {
		// Check if output looks like an error or empty result
		if strings.Contains(combined, "error") || strings.Contains(combined, "Error") {
			b.WriteString(s.Error.Render("✗ nilaway encountered errors:\n\n"))
			b.WriteString(s.File.Render(combined))
			return b.String()
		}
		b.WriteString(s.Success.Render("✓ No nil pointer issues found\n"))
		return b.String()
	}

	b.WriteString(s.Error.Render(fmt.Sprintf("✗ %d potential nil pointer issues:", len(allFindings))))
	b.WriteString("\n\n")

	// Group by file for better display
	byFile := make(map[string][]nilawayFinding)
	fileOrder := []string{}
	for _, finding := range allFindings {
		// Extract file from posn (format: "file.go:line:col")
		file := finding.Posn
		if idx := strings.Index(finding.Posn, ":"); idx > 0 {
			file = finding.Posn[:idx]
		}
		if _, exists := byFile[file]; !exists {
			fileOrder = append(fileOrder, file)
		}
		byFile[file] = append(byFile[file], finding)
	}

	displayed := 0
	maxDisplay := 15

	for _, file := range fileOrder {
		if displayed >= maxDisplay {
			remaining := len(allFindings) - displayed
			b.WriteString(s.Muted.Render(fmt.Sprintf("\n  ... and %d more issues\n", remaining)))
			break
		}

		findings := byFile[file]
		b.WriteString(s.Header.Render(file))
		b.WriteString("\n")

		for _, finding := range findings {
			if displayed >= maxDisplay {
				break
			}

			// Extract line:col from posn
			loc := ""
			if parts := strings.SplitN(finding.Posn, ":", 3); len(parts) >= 2 {
				loc = parts[1]
				if len(parts) == 3 {
					loc = parts[1] + ":" + parts[2]
				}
			}

			b.WriteString(fmt.Sprintf("  %s %s\n", s.Muted.Render(loc+":"), s.File.Render(finding.Message)))
			if finding.Reason != "" {
				b.WriteString(fmt.Sprintf("      %s\n", reasonStyle.Render(finding.Reason)))
			}
			displayed++
		}
		b.WriteString("\n")
	}

	return b.String()
}
