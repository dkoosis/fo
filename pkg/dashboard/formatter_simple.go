package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GofmtFormatter handles gofmt -l output.
type GofmtFormatter struct{}

func (f *GofmtFormatter) Matches(command string) bool {
	return strings.Contains(command, "gofmt")
}

func (f *GofmtFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))

	// Filter to non-empty lines (actual files)
	var files []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && strings.HasSuffix(trimmed, ".go") {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		b.WriteString(successStyle.Render("✓ All files formatted correctly\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d files need formatting:", len(files))))
	b.WriteString("\n\n")

	for _, file := range files {
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(file)))
	}

	return b.String()
}

// GoVetFormatter handles go vet output.
type GoVetFormatter struct{}

func (f *GoVetFormatter) Matches(command string) bool {
	return strings.Contains(command, "go vet")
}

func (f *GoVetFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Filter to non-empty lines
	var issues []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			issues = append(issues, trimmed)
		}
	}

	if len(issues) == 0 {
		b.WriteString(successStyle.Render("✓ No issues found\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render(fmt.Sprintf("✗ %d issues:", len(issues))))
	b.WriteString("\n\n")

	for i, issue := range issues {
		if i >= 15 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-15)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(issue)))
	}

	return b.String()
}

// GoBuildFormatter handles go build output.
type GoBuildFormatter struct{}

func (f *GoBuildFormatter) Matches(command string) bool {
	return strings.Contains(command, "go build")
}

func (f *GoBuildFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	// Styles
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Filter to actual errors (exclude go toolchain info messages)
	var errors []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Skip info messages from go toolchain
		if strings.HasPrefix(trimmed, "go: downloading") ||
			strings.HasPrefix(trimmed, "go: extracting") ||
			strings.HasPrefix(trimmed, "go: finding") ||
			strings.HasPrefix(trimmed, "go: upgraded") ||
			strings.HasPrefix(trimmed, "go: added") {
			continue
		}
		errors = append(errors, trimmed)
	}

	if len(errors) == 0 {
		b.WriteString(successStyle.Render("✓ Build successful\n"))
		return b.String()
	}

	b.WriteString(errorStyle.Render("✗ Build failed:"))
	b.WriteString("\n\n")

	for i, err := range errors {
		if i >= 20 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ... and %d more\n", len(errors)-20)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", fileStyle.Render(err)))
	}

	return b.String()
}

// PlainFormatter is the fallback formatter.
type PlainFormatter struct{}

func (f *PlainFormatter) Matches(_ string) bool {
	return true // always matches as fallback
}

func (f *PlainFormatter) Format(lines []string, _ int) string {
	// Apply basic styling - highlight errors and warnings
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E"))

	var result []string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "panic") {
			result = append(result, errorStyle.Render(line))
		} else if strings.Contains(lower, "warning") || strings.Contains(lower, "warn") {
			result = append(result, warnStyle.Render(line))
		} else {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}
