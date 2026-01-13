package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TelemetrySignalsFormatter handles telemetry-signals -format=dashboard output.
type TelemetrySignalsFormatter struct{}

func (f *TelemetrySignalsFormatter) Matches(command string) bool {
	return strings.Contains(command, "telemetry-signals") && strings.Contains(command, "-format=dashboard")
}

// TelemetrySignalsReport matches the JSON output from telemetry-signals -format=dashboard.
type TelemetrySignalsReport struct {
	Details any                     `json:"details"`
	Message string                  `json:"message"`
	Metrics TelemetrySignalsMetrics `json:"metrics"`
}

// TelemetrySignalsMetrics contains the metrics data.
type TelemetrySignalsMetrics struct {
	Timestamp   string               `json:"timestamp"`
	Period      string               `json:"period"`
	TotalCalls  int                  `json:"total_calls"`
	Sessions    int                  `json:"sessions"`
	Adoption    TelemetryAdoption    `json:"adoption"`
	Entropy     []TelemetryEntropy   `json:"entropy"`
	SelfRepeat  TelemetrySelfRepeat  `json:"self_repeat"`
	ZeroResults TelemetryZeroResults `json:"zero_results"`
}

// TelemetryAdoption contains adoption metrics.
type TelemetryAdoption struct {
	OrcaMCPPercent    float64 `json:"orca_mcp_percent"`
	BashPercent       float64 `json:"bash_percent"`
	GitOpsPercent     float64 `json:"git_ops_percent"`
	NavigationPercent float64 `json:"navigation_percent"`
	OrcaMCPCount      int     `json:"orca_mcp_count"`
	BashCount         int     `json:"bash_count"`
	NavigationCount   int     `json:"navigation_count"`
}

// TelemetryEntropy contains entropy data for a tool.
type TelemetryEntropy struct {
	Tool        string  `json:"tool"`
	Entropy     float64 `json:"entropy"`
	Transitions int     `json:"transitions"`
}

// TelemetrySelfRepeat contains self-repeat metrics.
type TelemetrySelfRepeat struct {
	RefinementPct    float64 `json:"search_nugs_refinement_pct"`
	RetryPct         float64 `json:"search_nugs_retry_pct"`
	TotalRefinements int     `json:"total_refinements"`
	TotalRetries     int     `json:"total_retries"`
}

// TelemetryZeroResults contains zero-result metrics.
type TelemetryZeroResults struct {
	ZeroResultCount   int     `json:"zero_result_count"`
	ZeroResultPercent float64 `json:"zero_result_percent"`
	RetryAfterZero    int     `json:"retry_after_zero"`
	SaveAfterZero     int     `json:"save_after_zero"`
	ExploreAfterZero  int     `json:"explore_after_zero"`
}

func (f *TelemetrySignalsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	var report TelemetrySignalsReport
	if !decodeJSONLinesWithPrefix(lines, &report) {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	m := report.Metrics

	// Header
	b.WriteString(s.Header.Render("◉ Telemetry Signals"))
	b.WriteString("  ")
	b.WriteString(s.Success.Render(fmt.Sprintf("%d calls", m.TotalCalls)))
	b.WriteString(s.Muted.Render(fmt.Sprintf("  %d sessions", m.Sessions)))
	b.WriteString("\n\n")

	// Tool Adoption section
	b.WriteString(s.Header.Render("Tool Adoption"))
	b.WriteString("\n")
	adoptionData := []struct {
		label string
		count int
		pct   float64
		note  string
	}{
		{"Orca MCP", m.Adoption.OrcaMCPCount, m.Adoption.OrcaMCPPercent, ""},
		{"Bash", m.Adoption.BashCount, m.Adoption.BashPercent, ""},
		{"Navigation", m.Adoption.NavigationCount, m.Adoption.NavigationPercent, "cd overhead"},
	}
	for _, a := range adoptionData {
		paddedLabel := fmt.Sprintf("%-12s", a.label)
		paddedCount := fmt.Sprintf("%5d", a.count)
		paddedPct := fmt.Sprintf("%3.0f%%", a.pct)
		note := ""
		if a.note != "" {
			note = s.Muted.Render(fmt.Sprintf("  [%s]", a.note))
		}
		b.WriteString(fmt.Sprintf("  %s  %s  %s%s\n",
			s.File.Render(paddedLabel),
			s.Success.Render(paddedCount),
			s.Success.Render(paddedPct),
			note))
	}

	// Adoption bar
	barWidth := max(min(width-4, 40), 20)
	orcaWidth := int(m.Adoption.OrcaMCPPercent * float64(barWidth) / 100)
	bashWidth := int(m.Adoption.BashPercent * float64(barWidth) / 100)
	gitWidth := int(m.Adoption.GitOpsPercent * float64(barWidth) / 100)
	otherWidth := barWidth - orcaWidth - bashWidth - gitWidth
	if otherWidth < 0 {
		otherWidth = 0
	}
	b.WriteString("\n  ")
	orcaBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575"))
	bashBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#0077B6"))
	gitBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#FFBD2E"))
	otherBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#444444"))
	b.WriteString(orcaBarStyle.Render(strings.Repeat(" ", orcaWidth)))
	b.WriteString(bashBarStyle.Render(strings.Repeat(" ", bashWidth)))
	b.WriteString(gitBarStyle.Render(strings.Repeat(" ", gitWidth)))
	b.WriteString(otherBarStyle.Render(strings.Repeat(" ", otherWidth)))
	b.WriteString("\n\n")

	// Entropy section
	if len(m.Entropy) > 0 {
		b.WriteString(s.Header.Render("Entropy"))
		b.WriteString(s.Muted.Render("  (lower = less confusion)"))
		b.WriteString("\n")
		for _, e := range m.Entropy {
			paddedTool := fmt.Sprintf("%-18s", e.Tool)
			bar := strings.Repeat("█", int(e.Entropy*3))
			b.WriteString(fmt.Sprintf("  %s  %s %s\n",
				s.File.Render(paddedTool),
				s.Success.Render(fmt.Sprintf("%4.2f", e.Entropy)),
				s.Muted.Render(bar)))
		}
		b.WriteString("\n")
	}

	// Self-Repeat section
	b.WriteString(s.Header.Render("Self-Repeat"))
	b.WriteString(s.Muted.Render("  (search_nugs)"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
		s.File.Render(fmt.Sprintf("%-12s", "Refinement")),
		s.Success.Render(fmt.Sprintf("%5d", m.SelfRepeat.TotalRefinements)),
		s.Success.Render(fmt.Sprintf("%3.0f%%", m.SelfRepeat.RefinementPct))))
	b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
		s.File.Render(fmt.Sprintf("%-12s", "Retry")),
		s.Success.Render(fmt.Sprintf("%5d", m.SelfRepeat.TotalRetries)),
		s.Success.Render(fmt.Sprintf("%3.0f%%", m.SelfRepeat.RetryPct)),
		s.Muted.Render("[frustrated]")))
	b.WriteString("\n")

	// Zero Results section
	b.WriteString(s.Header.Render("Zero Results"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s\n",
		s.File.Render(fmt.Sprintf("%-12s", "Zero results")),
		s.Success.Render(fmt.Sprintf("%5d", m.ZeroResults.ZeroResultCount)),
		s.Success.Render(fmt.Sprintf("%3.0f%%", m.ZeroResults.ZeroResultPercent))))
	b.WriteString(fmt.Sprintf("  %s retry=%d  save=%d  explore=%d\n",
		s.Muted.Render("After zero:"),
		m.ZeroResults.RetryAfterZero,
		m.ZeroResults.SaveAfterZero,
		m.ZeroResults.ExploreAfterZero))

	return b.String()
}
