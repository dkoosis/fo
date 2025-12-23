package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SARIFFormatter handles SARIF output from any static analyzer.
type SARIFFormatter struct{}

func (f *SARIFFormatter) Matches(command string) bool {
	// Match any command that produces SARIF output.
	// Note: golangci-lint has its own dedicated formatter.
	// Add new SARIF-producing tools here as needed.
	return strings.Contains(command, "filesize")
}

// SARIFReport represents the SARIF report structure.
type SARIFReport struct {
	Runs []struct {
		Results []struct {
			RuleID  string `json:"ruleId"`
			Level   string `json:"level"`
			Message struct {
				Text string `json:"text"`
			} `json:"message"`
			Locations []struct {
				PhysicalLocation struct {
					ArtifactLocation struct {
						URI string `json:"uri"`
					} `json:"artifactLocation"`
					Region struct {
						StartLine   int `json:"startLine"`
						StartColumn int `json:"startColumn"`
					} `json:"region"`
				} `json:"physicalLocation"`
			} `json:"locations"`
		} `json:"results"`
	} `json:"runs"`
}

func (f *SARIFFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	s := Styles()
	// Non-bold variants for detail
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6"))
	ruleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Try to parse SARIF
	fullOutput := strings.Join(lines, "\n")
	var report SARIFReport
	if err := json.Unmarshal([]byte(fullOutput), &report); err != nil {
		// Not SARIF, fall back to plain
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Count issues
	errors, warnings := 0, 0
	type issue struct {
		file    string
		line    int
		rule    string
		message string
		level   string
	}
	var issues []issue

	for _, run := range report.Runs {
		for _, result := range run.Results {
			if result.Level == statusError {
				errors++
			} else {
				warnings++
			}

			file := ""
			line := 0
			if len(result.Locations) > 0 {
				loc := result.Locations[0].PhysicalLocation
				file = loc.ArtifactLocation.URI
				line = loc.Region.StartLine
			}
			issues = append(issues, issue{
				file:    file,
				line:    line,
				rule:    result.RuleID,
				message: result.Message.Text,
				level:   result.Level,
			})
		}
	}

	// Summary
	if errors == 0 && warnings == 0 {
		b.WriteString(s.Success.Render("✓ No issues found\n"))
		return b.String()
	}

	if errors > 0 {
		b.WriteString(s.Error.Render(fmt.Sprintf("✗ %d errors", errors)))
	}
	if warnings > 0 {
		if errors > 0 {
			b.WriteString(", ")
		}
		b.WriteString(s.Warn.Render(fmt.Sprintf("△ %d warnings", warnings)))
	}
	b.WriteString("\n\n")

	// Issues (limit to first 20)
	shown := 0
	for _, iss := range issues {
		if shown >= 20 {
			b.WriteString(ruleStyle.Render(fmt.Sprintf("\n... and %d more issues", len(issues)-20)))
			break
		}

		icon := s.Warn.Render("△")
		if iss.level == statusError {
			icon = s.Error.Render("✗")
		}

		location := fileStyle.Render(fmt.Sprintf("%s:%d", iss.file, iss.line))
		rule := ruleStyle.Render(fmt.Sprintf("[%s]", iss.rule))
		b.WriteString(fmt.Sprintf("%s %s %s\n", icon, location, rule))
		b.WriteString(fmt.Sprintf("   %s\n", iss.message))
		shown++
	}

	return b.String()
}
