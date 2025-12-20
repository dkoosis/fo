package dashboard

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// JscpdFormatter handles jscpd --reporters console output.
type JscpdFormatter struct{}

func (f *JscpdFormatter) Matches(command string) bool {
	return strings.Contains(command, "jscpd")
}

// ansiRegex matches ANSI escape codes for stripping.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func (f *JscpdFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Count clones and parse summary
	cloneCount := 0
	var clones []string // file pairs with duplication

	for _, line := range lines {
		clean := ansiRegex.ReplaceAllString(line, "")
		clean = strings.TrimSpace(clean)

		if strings.HasPrefix(clean, "Clone found") {
			cloneCount++
		} else if strings.HasPrefix(clean, "- ") && strings.Contains(clean, ".go") {
			// Extract file path from clone detail
			if len(clones) < 10 { // limit to first 10
				clones = append(clones, extractFilePath(clean))
			}
		} else if strings.HasPrefix(clean, "Found ") && strings.Contains(clean, "clones") {
			// Parse "Found X clones." line
			if n := parseCloneCount(clean); n > 0 {
				cloneCount = n
			}
		}
	}

	// Header
	b.WriteString(headerStyle.Render("◉ Code Duplication"))
	b.WriteString("\n\n")

	if cloneCount == 0 {
		b.WriteString(successStyle.Render("✓ No duplicates found"))
		b.WriteString("\n")
		return b.String()
	}

	// Summary
	b.WriteString(warnStyle.Render(fmt.Sprintf("⚠ %d clones found", cloneCount)))
	b.WriteString("\n\n")

	// Show unique files with clones
	shown := make(map[string]bool)
	for _, file := range clones {
		if file != "" && !shown[file] {
			b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(file)))
			shown[file] = true
		}
	}

	if cloneCount > len(shown) {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("\n  ... and %d more\n", cloneCount-len(shown))))
	}

	return b.String()
}

func extractFilePath(line string) string {
	// Line format: "- internal/foo/bar.go [1:2 - 3:4] (X lines, Y tokens)"
	line = strings.TrimPrefix(line, "- ")
	if idx := strings.Index(line, " ["); idx > 0 {
		return line[:idx]
	}
	return ""
}

func parseCloneCount(line string) int {
	// Line format: "Found 29 clones."
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		if n, err := strconv.Atoi(parts[1]); err == nil {
			return n
		}
	}
	return 0
}
