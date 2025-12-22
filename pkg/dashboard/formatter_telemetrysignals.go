package dashboard

import (
	"encoding/json"
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
	Details any                      `json:"details"`
	Message string                   `json:"message"`
	Metrics TelemetrySignalsMetrics  `json:"metrics"`
}

// TelemetrySignalsMetrics contains the metrics data.
type TelemetrySignalsMetrics struct {
	Timestamp   string                  `json:"timestamp"`
	Period      string                  `json:"period"`
	TotalCalls  int                     `json:"total_calls"`
	Sessions    int                     `json:"sessions"`
	Adoption    TelemetryAdoption       `json:"adoption"`
	Entropy     []TelemetryEntropy      `json:"entropy"`
	SelfRepeat  TelemetrySelfRepeat     `json:"self_repeat"`
	ZeroResults TelemetryZeroResults    `json:"zero_results"`
}

// TelemetryAdoption contains adoption metrics.
type TelemetryAdoption struct {
	OrcaMCPPercent  float64 `json:"orca_mcp_percent"`
	BashPercent     float64 `json:"bash_percent"`
	GitOpsPercent   float64 `json:"git_ops_percent"`
	NavigationPercent float64 `json:"navigation_percent"`
	OrcaMCPCount    int     `json:"orca_mcp_count"`
	BashCount       int     `json:"bash_count"`
	NavigationCount int     `json:"navigation_count"`
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

	// Find the JSON object in the output (skip any build/download messages)
	fullOutput := strings.Join(lines, "\n")
	jsonStart := strings.Index(fullOutput, "{")
	if jsonStart == -1 {
		return (&PlainFormatter{}).Format(lines, width)
	}
	jsonOutput := fullOutput[jsonStart:]

	var report TelemetrySignalsReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Styles
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	// Header with summary message
	b.WriteString(headerStyle.Render("◉ Telemetry Signals"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s", report.Metrics.Period)))
	b.WriteString("\n\n")

	// Key metrics in a compact format
	m := report.Metrics

	// Adoption bar
	b.WriteString(labelStyle.Render("Adoption"))
	b.WriteString("  ")
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.0f%% Orca", m.Adoption.OrcaMCPPercent)))
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %.0f%% Bash  %.0f%% Git", m.Adoption.BashPercent, m.Adoption.GitOpsPercent)))
	b.WriteString("\n")

	// Mini bar for adoption
	barWidth := max(min(width-4, 40), 20)
	orcaWidth := int(m.Adoption.OrcaMCPPercent * float64(barWidth) / 100)
	bashWidth := int(m.Adoption.BashPercent * float64(barWidth) / 100)
	gitWidth := int(m.Adoption.GitOpsPercent * float64(barWidth) / 100)
	otherWidth := barWidth - orcaWidth - bashWidth - gitWidth
	if otherWidth < 0 {
		otherWidth = 0
	}

	b.WriteString("  ")
	orcaBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#04B575"))
	bashBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#0077B6"))
	gitBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#FFBD2E"))
	otherBarStyle := lipgloss.NewStyle().Background(lipgloss.Color("#444444"))
	b.WriteString(orcaBarStyle.Render(strings.Repeat(" ", orcaWidth)))
	b.WriteString(bashBarStyle.Render(strings.Repeat(" ", bashWidth)))
	b.WriteString(gitBarStyle.Render(strings.Repeat(" ", gitWidth)))
	b.WriteString(otherBarStyle.Render(strings.Repeat(" ", otherWidth)))
	b.WriteString("\n\n")

	// Entropy (average)
	if len(m.Entropy) > 0 {
		var totalEntropy float64
		for _, e := range m.Entropy {
			totalEntropy += e.Entropy
		}
		avgEntropy := totalEntropy / float64(len(m.Entropy))
		b.WriteString(labelStyle.Render("Entropy"))
		b.WriteString("    ")
		b.WriteString(valueStyle.Render(fmt.Sprintf("%.2f avg", avgEntropy)))
		b.WriteString(mutedStyle.Render("  (lower = more predictable)"))
		b.WriteString("\n")
	}

	// Refinement rate
	b.WriteString(labelStyle.Render("Refinement"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.0f%%", m.SelfRepeat.RefinementPct)))
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %d refinements, %d retries", m.SelfRepeat.TotalRefinements, m.SelfRepeat.TotalRetries)))
	b.WriteString("\n")

	// Zero results
	b.WriteString(labelStyle.Render("Zero Results"))
	b.WriteString(" ")
	b.WriteString(valueStyle.Render(fmt.Sprintf("%.0f%%", m.ZeroResults.ZeroResultPercent)))
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %d total", m.ZeroResults.ZeroResultCount)))
	b.WriteString("\n")

	// Session count
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("%d sessions, %d tool calls", m.Sessions, m.TotalCalls)))
	b.WriteString("\n")

	return b.String()
}
