package dashboard

import (
	"fmt"
	"strings"
)

// JTBDCoverageFormatter handles jtbd-coverage -format=dashboard output.
type JTBDCoverageFormatter struct{}

func (f *JTBDCoverageFormatter) Matches(command string) bool {
	return strings.Contains(command, "jtbd-coverage") && strings.Contains(command, "-format=dashboard")
}

// JTBDCoverageReport represents the jtbd-coverage dashboard JSON output.
type JTBDCoverageReport struct {
	Status          string             `json:"status"`
	TotalJTBDs      int                `json:"total_jtbds"`
	CoveredJTBDs    int                `json:"covered_jtbds"`
	TotalTestFiles  int                `json:"total_test_files"`
	MappedTestFiles int                `json:"mapped_test_files"`
	Coverage        []JTBDCoverageItem `json:"coverage"`
}

// JTBDCoverageItem represents coverage for a single JTBD.
type JTBDCoverageItem struct {
	JTBDID    string   `json:"jtbd_id"`
	Principle string   `json:"principle"`
	Statement string   `json:"statement"`
	TestFiles []string `json:"test_files"`
	Count     int      `json:"count"`
}

func (f *JTBDCoverageFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	var report JTBDCoverageReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	// Header
	b.WriteString(s.Header.Render("◉ JTBD Test Coverage"))
	b.WriteString("\n\n")

	// Summary metrics
	coveragePct := 0.0
	if report.TotalJTBDs > 0 {
		coveragePct = float64(report.CoveredJTBDs) / float64(report.TotalJTBDs) * 100
	}

	if report.Status == "pass" {
		b.WriteString(s.Success.Render(fmt.Sprintf("✓ %d/%d JTBDs covered (%.0f%%)",
			report.CoveredJTBDs, report.TotalJTBDs, coveragePct)))
	} else {
		b.WriteString(s.Warn.Render(fmt.Sprintf("△ %d/%d JTBDs covered (%.0f%%)",
			report.CoveredJTBDs, report.TotalJTBDs, coveragePct)))
	}
	b.WriteString("\n")
	b.WriteString(s.Muted.Render(fmt.Sprintf("  %d test files mapped", report.MappedTestFiles)))
	b.WriteString("\n\n")

	// Show coverage breakdown by JTBD (limit to 5)
	shown := 0
	for _, item := range report.Coverage {
		if shown >= 5 {
			break
		}

		icon := s.Success.Render("✓")
		if item.Count == 0 {
			icon = s.Error.Render("✗")
		}

		// Truncate statement if too long
		statement := item.Statement
		maxLen := width - 10
		if maxLen < 30 {
			maxLen = 30
		}
		if len(statement) > maxLen {
			statement = statement[:maxLen-3] + "..."
		}

		b.WriteString(fmt.Sprintf("  %s %s\n", icon, s.File.Render(item.JTBDID)))
		b.WriteString(fmt.Sprintf("    %s\n", s.Muted.Render(statement)))
		shown++
	}

	if len(report.Coverage) > 5 {
		b.WriteString(s.Muted.Render(fmt.Sprintf("\n  ... and %d more\n", len(report.Coverage)-5)))
	}

	return b.String()
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *JTBDCoverageFormatter) GetStatus(lines []string) IndicatorStatus {
	var report JTBDCoverageReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return IndicatorDefault
	}

	if report.Status != "pass" || report.CoveredJTBDs < report.TotalJTBDs {
		return IndicatorWarning
	}
	return IndicatorSuccess
}
