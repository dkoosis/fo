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
	s := Styles()

	// Filter to non-empty lines (actual files)
	var files []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && strings.HasSuffix(trimmed, ".go") {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		b.WriteString(s.Success.Render("✓ All files formatted correctly\n"))
		return b.String()
	}

	b.WriteString(s.Error.Render(fmt.Sprintf("✗ %d files need formatting:", len(files))))
	b.WriteString("\n\n")

	for _, file := range files {
		b.WriteString(fmt.Sprintf("  %s\n", s.File.Render(file)))
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
	s := Styles()

	// Filter to non-empty lines
	var issues []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			issues = append(issues, trimmed)
		}
	}

	if len(issues) == 0 {
		b.WriteString(s.Success.Render("✓ No issues found\n"))
		return b.String()
	}

	b.WriteString(s.Error.Render(fmt.Sprintf("✗ %d issues:", len(issues))))
	b.WriteString("\n\n")

	for i, issue := range issues {
		if i >= 15 {
			b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", len(issues)-15)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", s.File.Render(issue)))
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
	s := Styles()

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
		b.WriteString(s.Success.Render("✓ Build successful\n"))
		return b.String()
	}

	b.WriteString(s.Error.Render("✗ Build failed:"))
	b.WriteString("\n\n")

	for i, err := range errors {
		if i >= 20 {
			b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", len(errors)-20)))
			break
		}
		b.WriteString(fmt.Sprintf("  %s\n", s.File.Render(err)))
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

		// Skip structured log lines (slog format) - level=ERROR is just a log level, not an error
		if isStructuredLogLine(line) {
			result = append(result, line)
			continue
		}

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

// isStructuredLogLine detects slog-style structured log output.
// These lines contain level=ERROR/WARN as metadata, not actual errors.
func isStructuredLogLine(line string) bool {
	// slog text format: time=... level=...
	return strings.Contains(line, "time=") && strings.Contains(line, "level=")
}
