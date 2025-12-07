// Package design implements pattern-based CLI output visualization
package design

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PatternTypeHousekeeping represents project hygiene check dashboards.
// Use for: documentation sprawl warnings, technical debt markers, lint-adjacent concerns.
const PatternTypeHousekeeping PatternType = "housekeeping"

// Housekeeping represents a collection of project hygiene checks.
// These are "clean up your room" items - not blocking, but worth tracking.
type Housekeeping struct {
	Title  string              // Section title (default: "HOUSEKEEPING")
	Checks []HousekeepingCheck // Individual hygiene checks
}

// HousekeepingCheck represents a single hygiene check result.
type HousekeepingCheck struct {
	Name      string   // Check name (e.g., "Markdown files", "TODO comments")
	Status    string   // "pass", "warn", "fail"
	Current   int      // Current value
	Threshold int      // Threshold value (for comparison)
	Details   string   // Additional details (e.g., "7 older than 90 days")
	Items     []string // Optional: specific files or issues needing attention
}

// PatternType returns the pattern type identifier.
func (h *Housekeeping) PatternType() PatternType {
	return PatternTypeHousekeeping
}

// PassCount returns the number of passing checks.
func (h *Housekeeping) PassCount() int {
	count := 0
	for _, c := range h.Checks {
		if c.Status == "pass" {
			count++
		}
	}
	return count
}

// WarnCount returns the number of warning checks.
func (h *Housekeeping) WarnCount() int {
	count := 0
	for _, c := range h.Checks {
		if c.Status == "warn" {
			count++
		}
	}
	return count
}

// FailCount returns the number of failing checks.
func (h *Housekeeping) FailCount() int {
	count := 0
	for _, c := range h.Checks {
		if c.Status == "fail" {
			count++
		}
	}
	return count
}

// Render formats the housekeeping section using the provided theme.
func (h *Housekeeping) Render(cfg *Config) string {
	if len(h.Checks) == 0 {
		return ""
	}

	// Get styles from config
	boxStyle := lipgloss.NewStyle().
		BorderStyle(BorderFromConfig(cfg)).
		BorderForeground(cfg.Colors.Muted).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(cfg.Colors.Process)

	successStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Success)

	warningStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Warning)

	errorStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Error)

	mutedStyle := lipgloss.NewStyle().
		Foreground(cfg.Colors.Muted)

	title := h.Title
	if title == "" {
		title = "HOUSEKEEPING"
	}

	passCount := h.PassCount()
	warnCount := h.WarnCount()
	failCount := h.FailCount()
	totalChecks := len(h.Checks)

	// Collapsed view when all checks pass
	if warnCount == 0 && failCount == 0 {
		summary := fmt.Sprintf("%s  %d/%d %s", title, passCount, totalChecks, cfg.Icons.Success)
		return boxStyle.Render(headerStyle.Render(summary))
	}

	// Expanded view
	var sb strings.Builder

	// Header with summary
	sb.WriteString(headerStyle.Render(title))
	sb.WriteString("\n")

	// Calculate column widths
	maxNameWidth := 0
	for _, c := range h.Checks {
		if len(c.Name) > maxNameWidth {
			maxNameWidth = len(c.Name)
		}
	}
	if maxNameWidth < 20 {
		maxNameWidth = 20
	}
	if maxNameWidth > 30 {
		maxNameWidth = 30
	}

	// Render checks
	for _, c := range h.Checks {
		var icon string
		var style lipgloss.Style

		switch c.Status {
		case "pass":
			icon = cfg.Icons.Success
			style = successStyle
		case "warn":
			icon = cfg.Icons.Warning
			style = warningStyle
		case "fail":
			icon = cfg.Icons.Error
			style = errorStyle
		default:
			icon = cfg.Icons.Info
			style = mutedStyle
		}

		// Build the line
		sb.WriteString("  ")
		sb.WriteString(style.Render(icon))
		sb.WriteString("  ")

		// Name (padded)
		sb.WriteString(PadRight(c.Name, maxNameWidth))
		sb.WriteString("  ")

		// Value display
		if c.Status == "pass" {
			// Just show as passing
			sb.WriteString(successStyle.Render("OK"))
		} else if c.Threshold > 0 {
			// Show current / threshold format
			valueStr := fmt.Sprintf("%d / %d limit", c.Current, c.Threshold)
			sb.WriteString(style.Render(valueStr))
		} else if c.Current > 0 {
			// Show count only
			valueStr := fmt.Sprintf("%d", c.Current)
			sb.WriteString(style.Render(valueStr))
		}

		// Additional details
		if c.Details != "" {
			sb.WriteString("  ")
			sb.WriteString(mutedStyle.Render("(" + c.Details + ")"))
		}

		sb.WriteString("\n")

		// Show specific items if present and check failed/warned
		if c.Status != "pass" && len(c.Items) > 0 {
			// Show up to 3 items
			maxItems := 3
			for i, item := range c.Items {
				if i >= maxItems {
					remaining := len(c.Items) - maxItems
					sb.WriteString(fmt.Sprintf("       %s\n", mutedStyle.Render(fmt.Sprintf("... and %d more", remaining))))
					break
				}
				sb.WriteString(fmt.Sprintf("       %s\n", mutedStyle.Render(item)))
			}
		}
	}

	return boxStyle.Render(sb.String())
}

// HousekeepingCheckType represents standard check types for consistency.
type HousekeepingCheckType string

const (
	// CheckMarkdownCount tracks documentation sprawl.
	CheckMarkdownCount HousekeepingCheckType = "markdown_count"
	// CheckTodoComments tracks technical debt markers.
	CheckTodoComments HousekeepingCheckType = "todo_comments"
	// CheckOrphanTests finds test files without corresponding source.
	CheckOrphanTests HousekeepingCheckType = "orphan_tests"
	// CheckPackageDocs verifies package documentation exists.
	CheckPackageDocs HousekeepingCheckType = "package_docs"
	// CheckDeadCode identifies unused exports.
	CheckDeadCode HousekeepingCheckType = "dead_code"
	// CheckDeprecatedDeps finds deprecated dependencies.
	CheckDeprecatedDeps HousekeepingCheckType = "deprecated_deps"
	// CheckLicenseHeaders verifies required license headers.
	CheckLicenseHeaders HousekeepingCheckType = "license_headers"
	// CheckGeneratedFreshness checks if generated files are stale.
	CheckGeneratedFreshness HousekeepingCheckType = "generated_freshness"
)

// NewHousekeepingCheck creates a check with sensible defaults.
func NewHousekeepingCheck(name string, current, threshold int) HousekeepingCheck {
	check := HousekeepingCheck{
		Name:      name,
		Current:   current,
		Threshold: threshold,
	}

	// Determine status based on threshold
	if threshold > 0 {
		if current <= threshold {
			check.Status = "pass"
		} else {
			ratio := float64(current) / float64(threshold)
			if ratio > 1.5 {
				check.Status = "fail"
			} else {
				check.Status = "warn"
			}
		}
	} else {
		// For checks where 0 is the goal (orphan tests, dead code, etc.)
		if current == 0 {
			check.Status = "pass"
		} else if current <= 3 {
			check.Status = "warn"
		} else {
			check.Status = "fail"
		}
	}

	return check
}
