package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GovulncheckFormatter handles govulncheck output.
type GovulncheckFormatter struct{}

func (f *GovulncheckFormatter) Matches(command string) bool {
	return strings.Contains(command, "govulncheck")
}

// GovulncheckVuln represents a vulnerability finding.
type GovulncheckVuln struct {
	OSV struct {
		ID       string `json:"id"`
		Summary  string `json:"summary"`
		Severity []struct {
			Type  string `json:"type"`
			Score string `json:"score"`
		} `json:"severity"`
		Aliases []string `json:"aliases"`
	} `json:"osv"`
	Modules []struct {
		Path         string `json:"path"`
		FoundVersion string `json:"found_version"`
		FixedVersion string `json:"fixed_version"`
	} `json:"modules"`
}

func (f *GovulncheckFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	fullOutput := strings.Join(lines, "\n")

	s := Styles()
	// Non-bold variants for secondary info
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E"))
	idStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6"))

	// Header
	b.WriteString(s.Header.Render("◉ Vulnerability Check"))
	b.WriteString("\n\n")

	// Check for simple "no vulnerabilities" message (text mode)
	if strings.Contains(fullOutput, "No vulnerabilities found") {
		b.WriteString(s.Success.Render("✓ No vulnerabilities found"))
		b.WriteString("\n")
		return b.String()
	}

	// Parse NDJSON format - each line is a separate JSON object
	var vulns []GovulncheckVuln
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		// Try to parse as vulnerability
		var entry struct {
			Finding *GovulncheckVuln `json:"finding"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err == nil && entry.Finding != nil {
			vulns = append(vulns, *entry.Finding)
		}
	}

	if len(vulns) == 0 {
		b.WriteString(s.Success.Render("✓ No vulnerabilities found"))
		b.WriteString("\n")
		return b.String()
	}

	// Show vulnerability count
	b.WriteString(s.Error.Render(fmt.Sprintf("%d vulnerabilities found", len(vulns))))
	b.WriteString("\n\n")

	// Show each vulnerability (up to 5)
	shown := 0
	for _, v := range vulns {
		if shown >= 5 {
			break
		}

		// ID and summary
		id := v.OSV.ID
		if len(v.OSV.Aliases) > 0 {
			// Prefer CVE alias if available
			for _, alias := range v.OSV.Aliases {
				if strings.HasPrefix(alias, "CVE-") {
					id = alias
					break
				}
			}
		}
		b.WriteString(idStyle.Render(id))
		b.WriteString("\n")

		summary := v.OSV.Summary
		if len(summary) > width-4 && width > 7 {
			summary = summary[:width-7] + "..."
		}
		b.WriteString(fmt.Sprintf("  %s\n", summary))

		// Affected modules
		for _, mod := range v.Modules {
			fix := ""
			if mod.FixedVersion != "" {
				fix = warnStyle.Render(fmt.Sprintf(" → fix: %s", mod.FixedVersion))
			}
			b.WriteString(fmt.Sprintf("  %s%s\n", s.Muted.Render(mod.Path+"@"+mod.FoundVersion), fix))
		}
		b.WriteString("\n")
		shown++
	}

	if len(vulns) > 5 {
		b.WriteString(s.Muted.Render(fmt.Sprintf("... and %d more\n", len(vulns)-5)))
	}

	return b.String()
}
