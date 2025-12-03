// Package design implements pattern-based CLI output visualization
package design

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PatternTypeQualityReport represents quality assessment reports.
// Use for: MCP Interviewer output, code quality dashboards, audit results.
const PatternTypeQualityReport PatternType = "quality-report"

// QualityReport represents a quality assessment with category scores and issues.
// Designed for MCP Interviewer tool scorecards but generalizable to other quality metrics.
type QualityReport struct {
	ServerName    string                // Server or project name being assessed
	ServerVersion string                // Version of the server/project
	Protocol      string                // Protocol version (for MCP)
	ToolCount     int                   // Number of tools assessed
	ResourceCount int                   // Number of resources
	Categories    []QualityCategory     // Score categories (Name, Description, Schema, etc.)
	Issues        []QualityIssue        // Grouped failures that need attention
	Constraints   []ConstraintViolation // Constraint violations (limits exceeded)
}

// QualityCategory represents a scoring category with pass/total counts.
type QualityCategory struct {
	Name   string // Category name (e.g., "Name", "Description")
	Passed int    // Number of passed checks
	Total  int    // Total number of checks
}

// QualityIssue represents a grouped failure type.
type QualityIssue struct {
	Category  string   // Issue type (e.g., "description.examples")
	ToolCount int      // Number of tools affected
	ToolNames []string // Names of affected tools
}

// ConstraintViolation represents a violated constraint.
type ConstraintViolation struct {
	Code     string // Short code (e.g., "OTC")
	Message  string // Human-readable message
	Severity string // "error" or "warning"
	Details  int    // Actual value
	Limit    int    // Limit that was exceeded
}

// PatternType returns the pattern type identifier.
func (r *QualityReport) PatternType() PatternType {
	return PatternTypeQualityReport
}

// Render formats the quality report using the provided theme.
func (r *QualityReport) Render(cfg *Config) string {
	var sb strings.Builder

	// Get styles from config
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(cfg.Colors.Muted).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(cfg.Colors.Process)

	successStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Success)

	warningStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Warning)

	mutedStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Muted)

	errorStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Error)

	// Build header
	title := fmt.Sprintf("MCP QUALITY REPORT: %s", r.ServerName)
	if r.ServerVersion != "" {
		title += " v" + r.ServerVersion
	}
	sb.WriteString(headerStyle.Render(title))
	sb.WriteString("\n")

	// Server info line
	info := fmt.Sprintf("Tools: %d    Resources: %d", r.ToolCount, r.ResourceCount)
	if r.Protocol != "" {
		info += fmt.Sprintf("    Protocol: %s", r.Protocol)
	}
	sb.WriteString(mutedStyle.Render(info))
	sb.WriteString("\n\n")

	// Quality scores with progress bars
	sb.WriteString(headerStyle.Render("QUALITY SCORES"))
	sb.WriteString("\n")

	for _, cat := range r.Categories {
		if cat.Total == 0 {
			continue
		}

		pct := float64(cat.Passed) / float64(cat.Total) * 100
		barWidth := 20
		filled := int(float64(barWidth) * float64(cat.Passed) / float64(cat.Total))

		// Build progress bar
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		// Color based on percentage
		var barStyled string
		if pct >= 90 {
			barStyled = successStyle.Render(bar)
		} else if pct >= 70 {
			barStyled = warningStyle.Render(bar)
		} else {
			barStyled = errorStyle.Render(bar)
		}

		line := fmt.Sprintf("  %-18s %s  %2d/%-2d  %3.0f%%",
			cat.Name, barStyled, cat.Passed, cat.Total, pct)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	// Issues section
	if len(r.Issues) > 0 {
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render("NEEDS ATTENTION"))
		sb.WriteString("\n")

		for _, issue := range r.Issues {
			icon := cfg.Icons.Warning
			label := strings.Replace(issue.Category, ".", " → ", 1)
			line := fmt.Sprintf("  %s %-30s (%d tools)", icon, label, issue.ToolCount)
			sb.WriteString(warningStyle.Render(line))
			sb.WriteString("\n")
		}
	}

	// Constraints section
	if len(r.Constraints) > 0 {
		sb.WriteString("\n")
		sb.WriteString(headerStyle.Render("CONSTRAINTS"))
		sb.WriteString("\n")

		for _, c := range r.Constraints {
			icon := cfg.Icons.Warning
			if c.Severity == "error" {
				icon = cfg.Icons.Error
			}
			line := fmt.Sprintf("  %s %s: %d exceeds %d limit",
				icon, c.Code, c.Details, c.Limit)
			sb.WriteString(warningStyle.Render(line))
			sb.WriteString("\n")
		}
	}

	// Wrap in box
	return boxStyle.Render(sb.String())
}
