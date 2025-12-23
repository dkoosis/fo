package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
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

	s := Styles()

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
		b.WriteString(s.Success.Render("✓ No architecture violations\n"))
		return b.String()
	}

	// Summary
	b.WriteString(s.Warn.Render(fmt.Sprintf("△ %d architecture issues", totalIssues)))
	b.WriteString("\n\n")

	// Dependency violations
	if depCount > 0 {
		b.WriteString(s.Header.Render("◉ Dependency Violations"))
		b.WriteString(s.Error.Render(fmt.Sprintf(" (%d)", depCount)))
		b.WriteString("\n")
		for i, dep := range report.Payload.ArchWarningsDeps {
			if i >= 10 {
				b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", depCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				s.Warn.Render(dep.ComponentFrom),
				s.Error.Render(dep.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", s.File.Render(shortPath(dep.FileRelPath))))
		}
		b.WriteString("\n")
	}

	// Unmatched files
	if unmatchedCount > 0 {
		b.WriteString(s.Header.Render("◉ Unmatched Files"))
		b.WriteString(s.Warn.Render(fmt.Sprintf(" (%d)", unmatchedCount)))
		b.WriteString("\n")
		for i, um := range report.Payload.ArchWarningsNotMatched {
			if i >= 10 {
				b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", unmatchedCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s\n", s.File.Render(um.FileRelPath)))
		}
		b.WriteString("\n")
	}

	// Deep scan violations
	if deepScanCount > 0 {
		b.WriteString(s.Header.Render("◉ Deep Scan Violations"))
		b.WriteString(s.Error.Render(fmt.Sprintf(" (%d)", deepScanCount)))
		b.WriteString("\n")
		for i, ds := range report.Payload.ArchWarningsDeepScan {
			if i >= 10 {
				b.WriteString(s.Muted.Render(fmt.Sprintf("  ... and %d more\n", deepScanCount-10)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s → %s\n",
				s.Warn.Render(ds.Gate),
				s.Error.Render(ds.ComponentTo)))
			b.WriteString(fmt.Sprintf("    %s\n", s.File.Render(shortPath(ds.FileRelPath))))
		}
	}

	return b.String()
}
