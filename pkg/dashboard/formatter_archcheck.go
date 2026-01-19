package dashboard

import (
	"fmt"
	"strings"
)

// ArchCheckFormatter handles arch-check -format=dashboard output.
type ArchCheckFormatter struct{}

func (f *ArchCheckFormatter) Matches(command string) bool {
	return strings.Contains(command, "arch-check") && strings.Contains(command, "-format=dashboard")
}

// ArchCheckReport represents the arch-check dashboard JSON output.
type ArchCheckReport struct {
	Status          string `json:"status"`
	GoArchLintOK    bool   `json:"go_arch_lint_ok"`
	DataOwnershipOK bool   `json:"data_ownership_ok"`
}

func (f *ArchCheckFormatter) Format(lines []string, _ int) string {
	var b strings.Builder

	var report ArchCheckReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return (&PlainFormatter{}).Format(lines, 0)
	}

	s := Styles()

	// Header
	b.WriteString(s.Header.Render("◉ Architecture Check"))
	b.WriteString("\n\n")

	// Overall status
	if report.Status == "pass" {
		b.WriteString(s.Success.Render("✓ All checks passed"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(s.Error.Render("✗ Architecture violations found"))
		b.WriteString("\n\n")
	}

	// Individual checks
	goArchIcon := s.Success.Render("✓")
	if !report.GoArchLintOK {
		goArchIcon = s.Error.Render("✗")
	}
	b.WriteString(fmt.Sprintf("  %s go-arch-lint boundaries\n", goArchIcon))

	dataOwnerIcon := s.Success.Render("✓")
	if !report.DataOwnershipOK {
		dataOwnerIcon = s.Error.Render("✗")
	}
	b.WriteString(fmt.Sprintf("  %s data ownership rules\n", dataOwnerIcon))

	return b.String()
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *ArchCheckFormatter) GetStatus(lines []string) IndicatorStatus {
	var report ArchCheckReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return IndicatorDefault
	}

	if report.Status != "pass" || !report.GoArchLintOK || !report.DataOwnershipOK {
		return IndicatorError
	}
	return IndicatorSuccess
}
