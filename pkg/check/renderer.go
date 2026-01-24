package check

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// Renderer renders check reports using fo's design system.
type Renderer struct {
	config  RendererConfig
	foTheme *design.Config
}

// NewRenderer creates a renderer with the given configuration.
func NewRenderer(config RendererConfig, foTheme *design.Config) *Renderer {
	return &Renderer{
		config:  config,
		foTheme: foTheme,
	}
}

// Render renders a check report to a string.
func (r *Renderer) Render(report *Report) string {
	if report == nil {
		return ""
	}

	var sb strings.Builder

	// Render header with tool name and status
	r.renderHeader(&sb, report)

	// Render summary line
	if report.Summary != "" {
		r.renderSummary(&sb, report)
	}

	// Render metrics if enabled and present
	if r.config.ShowMetrics && len(report.Metrics) > 0 {
		r.renderMetrics(&sb, report)
	}

	// Render trend sparkline if enabled and present
	if r.config.ShowTrend && len(report.Trend) > 0 {
		r.renderTrend(&sb, report)
	}

	// Render items if enabled and present
	if r.config.ShowItems && len(report.Items) > 0 {
		r.renderItems(&sb, report)
	}

	return sb.String()
}

func (r *Renderer) renderHeader(sb *strings.Builder, report *Report) {
	// Get status icon and color
	icon := r.getStatusIcon(report.Status)
	colorKey := r.getStatusColorKey(report.Status)

	// Format: icon tool_name
	toolName := report.Tool
	if toolName == "" {
		toolName = "check"
	}

	line := design.RenderDirectMessage(r.foTheme, colorKey, icon, toolName, 0)
	sb.WriteString(line)
}

func (r *Renderer) renderSummary(sb *strings.Builder, report *Report) {
	// Summary uses the status color but slightly muted
	colorKey := r.getStatusColorKey(report.Status)
	line := design.RenderDirectMessage(r.foTheme, colorKey, "", report.Summary, 1)
	sb.WriteString(line)
}

func (r *Renderer) renderMetrics(sb *strings.Builder, report *Report) {
	for _, m := range report.Metrics {
		// Format: name: value unit (threshold: X)
		var metricLine strings.Builder
		metricLine.WriteString(fmt.Sprintf("  %s: %.0f", m.Name, m.Value))

		if m.Unit != "" {
			metricLine.WriteString(" ")
			metricLine.WriteString(m.Unit)
		}

		if m.Threshold > 0 {
			metricLine.WriteString(fmt.Sprintf(" (threshold: %.0f)", m.Threshold))
		}

		line := design.RenderDirectMessage(r.foTheme, "info", "", metricLine.String(), 0)
		sb.WriteString(line)
	}
}

func (r *Renderer) renderTrend(sb *strings.Builder, report *Report) {
	if len(report.Trend) == 0 {
		return
	}

	// Build sparkline from trend data
	sparkline := r.buildSparkline(report.Trend)

	// Format: Trend: sparkline [first -> last]
	first := report.Trend[0]
	last := report.Trend[len(report.Trend)-1]
	trendLine := fmt.Sprintf("  trend: %s [%d -> %d]", sparkline, first, last)

	// Color based on trend direction
	colorKey := "info"
	if last > first {
		colorKey = "warning" // Increasing (usually bad for lint counts)
	} else if last < first {
		colorKey = "success" // Decreasing (usually good)
	}

	line := design.RenderDirectMessage(r.foTheme, colorKey, "", trendLine, 0)
	sb.WriteString(line)
}

func (r *Renderer) buildSparkline(data []int) string {
	if len(data) == 0 {
		return ""
	}

	// Sparkline characters (8 levels)
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	// Find min/max for scaling
	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Handle flat line case
	if maxVal == minVal {
		return strings.Repeat(string(chars[3]), min(len(data), r.config.TrendWidth))
	}

	// Sample data if too long
	displayData := data
	if len(data) > r.config.TrendWidth && r.config.TrendWidth > 0 {
		displayData = r.sampleData(data, r.config.TrendWidth)
	}

	// Build sparkline
	var result strings.Builder
	valRange := float64(maxVal - minVal)
	for _, v := range displayData {
		// Scale to 0-7 range
		scaled := int(float64(v-minVal) / valRange * 7)
		if scaled > 7 {
			scaled = 7
		}
		if scaled < 0 {
			scaled = 0
		}
		result.WriteRune(chars[scaled])
	}

	return result.String()
}

func (r *Renderer) sampleData(data []int, targetLen int) []int {
	if len(data) <= targetLen {
		return data
	}

	result := make([]int, targetLen)
	step := float64(len(data)-1) / float64(targetLen-1)

	for i := 0; i < targetLen; i++ {
		idx := int(float64(i) * step)
		if idx >= len(data) {
			idx = len(data) - 1
		}
		result[i] = data[idx]
	}

	return result
}

func (r *Renderer) renderItems(sb *strings.Builder, report *Report) {
	limit := r.config.ItemLimit
	items := report.Items
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	for _, item := range items {
		icon := r.getSeverityIcon(item.Severity)
		colorKey := r.mapSeverity(item.Severity)

		// Format: icon label: value
		var itemLine strings.Builder
		itemLine.WriteString(item.Label)
		if item.Value != "" {
			itemLine.WriteString(": ")
			itemLine.WriteString(item.Value)
		}
		if item.Message != "" {
			itemLine.WriteString(" - ")
			itemLine.WriteString(item.Message)
		}

		line := design.RenderDirectMessage(r.foTheme, colorKey, icon, itemLine.String(), 1)
		sb.WriteString(line)
	}

	// Show truncation notice if items were limited
	if r.config.ItemLimit > 0 && len(report.Items) > r.config.ItemLimit {
		remaining := len(report.Items) - r.config.ItemLimit
		notice := fmt.Sprintf("  ... and %d more items", remaining)
		line := design.RenderDirectMessage(r.foTheme, "muted", "", notice, 0)
		sb.WriteString(line)
	}
}

func (r *Renderer) getStatusIcon(status string) string {
	switch status {
	case StatusPass:
		return r.foTheme.GetIcon("Success")
	case StatusWarn:
		return r.foTheme.GetIcon("Warning")
	case StatusFail:
		return r.foTheme.GetIcon("Error")
	default:
		return ""
	}
}

func (r *Renderer) getStatusColorKey(status string) string {
	switch status {
	case StatusPass:
		return "success"
	case StatusWarn:
		return "warning"
	case StatusFail:
		return "error"
	default:
		return "info"
	}
}

func (r *Renderer) getSeverityIcon(severity string) string {
	switch severity {
	case SeverityError:
		return r.foTheme.GetIcon("Error")
	case SeverityWarning:
		return r.foTheme.GetIcon("Warning")
	case SeverityInfo:
		return r.foTheme.GetIcon("Info")
	default:
		return ""
	}
}

func (r *Renderer) mapSeverity(severity string) string {
	if mapped, ok := r.config.SeverityMapping[severity]; ok {
		return mapped
	}
	return "info"
}

// RenderFile reads and renders a check report file.
func (r *Renderer) RenderFile(path string) (string, error) {
	report, err := ReadFile(path)
	if err != nil {
		return "", err
	}
	return r.Render(report), nil
}

// QuickRender renders a check report file with default settings.
func QuickRender(path string) (string, error) {
	report, err := ReadFile(path)
	if err != nil {
		return "", err
	}

	config := DefaultRendererConfig()
	theme := design.DefaultConfig()

	renderer := NewRenderer(config, theme)
	return renderer.Render(report), nil
}
