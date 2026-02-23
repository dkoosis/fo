package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dkoosis/fo/pkg/pattern"
)

// Terminal renders patterns as styled terminal output via lipgloss.
type Terminal struct {
	theme Theme
	width int
}

// NewTerminal creates a terminal renderer with the given theme.
func NewTerminal(theme Theme, width int) *Terminal {
	if width <= 0 {
		width = 80
	}
	return &Terminal{theme: theme, width: width}
}

// Render formats all patterns for terminal display.
func (t *Terminal) Render(patterns []pattern.Pattern) string {
	var sections []string
	for _, p := range patterns {
		s := t.renderOne(p)
		if s != "" {
			sections = append(sections, s)
		}
	}
	return strings.Join(sections, "\n")
}

func (t *Terminal) renderOne(p pattern.Pattern) string {
	switch v := p.(type) {
	case *pattern.Summary:
		return t.renderSummary(v)
	case *pattern.Leaderboard:
		return t.renderLeaderboard(v)
	case *pattern.TestTable:
		return t.renderTestTable(v)
	case *pattern.Sparkline:
		return t.renderSparkline(v)
	case *pattern.Comparison:
		return t.renderComparison(v)
	default:
		return ""
	}
}

func (t *Terminal) renderSummary(s *pattern.Summary) string {
	var sb strings.Builder
	if s.Label != "" {
		sb.WriteString(t.theme.Bold.Render(s.Label))
		sb.WriteString("\n")
	}
	for _, m := range s.Metrics {
		sb.WriteString("  ")
		icon, style := t.iconStyle(m.Kind)
		sb.WriteString(style.Render(icon + " " + m.Label + ": " + m.Value))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t *Terminal) renderLeaderboard(l *pattern.Leaderboard) string {
	if len(l.Items) == 0 {
		return ""
	}
	var sb strings.Builder
	if l.Label != "" {
		header := l.Label
		if l.TotalCount > len(l.Items) {
			header += fmt.Sprintf(" (top %d of %d)", len(l.Items), l.TotalCount)
		}
		sb.WriteString(t.theme.Bold.Render(header))
		sb.WriteString("\n")
	}

	maxName, maxMetric := 0, 0
	for _, item := range l.Items {
		if len(item.Name) > maxName {
			maxName = len(item.Name)
		}
		if len(item.Metric) > maxMetric {
			maxMetric = len(item.Metric)
		}
	}
	if maxName > 50 {
		maxName = 50
	}

	for _, item := range l.Items {
		sb.WriteString("  ")
		if l.ShowRank {
			sb.WriteString(t.theme.Muted.Render(fmt.Sprintf("%2d. ", item.Rank)))
		}
		name := item.Name
		if len([]rune(name)) > maxName {
			name = string([]rune(name)[:maxName-3]) + "..."
		}
		sb.WriteString(t.theme.Primary.Render(padRight(name, maxName)))
		sb.WriteString("  ")
		sb.WriteString(t.theme.Warning.Render(padLeft(item.Metric, maxMetric)))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t *Terminal) renderTestTable(tt *pattern.TestTable) string {
	if len(tt.Results) == 0 {
		return ""
	}
	var sb strings.Builder
	if tt.Label != "" {
		sb.WriteString(t.theme.Bold.Render(tt.Label))
		sb.WriteString("\n")
	}

	maxName, maxDur := 0, 0
	for _, r := range tt.Results {
		if len(r.Name) > maxName {
			maxName = len(r.Name)
		}
		if len(r.Duration) > maxDur {
			maxDur = len(r.Duration)
		}
	}
	if maxName > 60 {
		maxName = 60
	}

	for _, r := range tt.Results {
		sb.WriteString("  ")
		icon, style := t.statusIconStyle(r.Status)
		sb.WriteString(style.Render(icon + " "))

		name := r.Name
		if len([]rune(name)) > maxName {
			name = string([]rune(name)[:maxName-3]) + "..."
		}
		sb.WriteString(padRight(name, maxName))

		if r.Count > 0 {
			sb.WriteString(t.theme.Muted.Render(fmt.Sprintf("  %d tests", r.Count)))
		}
		if r.Duration != "" {
			sb.WriteString("  ")
			sb.WriteString(t.theme.Muted.Render(padLeft(r.Duration, maxDur)))
		}

		if r.Details != "" {
			lines := strings.Split(r.Details, "\n")
			for _, line := range lines {
				sb.WriteString("\n    ")
				sb.WriteString(t.theme.Muted.Render(line))
			}
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t *Terminal) renderSparkline(s *pattern.Sparkline) string {
	if len(s.Values) == 0 {
		return ""
	}
	var sb strings.Builder
	if s.Label != "" {
		sb.WriteString(t.theme.Primary.Render(s.Label + ": "))
	}

	minVal, maxVal := s.Min, s.Max
	if minVal == 0 && maxVal == 0 {
		minVal, maxVal = s.Values[0], s.Values[0]
		for _, v := range s.Values {
			if v < minVal {
				minVal = v
			}
			if v > maxVal {
				maxVal = v
			}
		}
	}
	valueRange := maxVal - minVal
	if valueRange == 0 {
		valueRange = 1
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var spark strings.Builder
	for _, v := range s.Values {
		idx := int((v - minVal) / valueRange * 7)
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		spark.WriteRune(blocks[idx])
	}
	sb.WriteString(t.theme.Success.Render(spark.String()))

	latest := s.Values[len(s.Values)-1]
	sb.WriteString(t.theme.Muted.Render(fmt.Sprintf(" %.1f%s", latest, s.Unit)))
	sb.WriteString("\n")
	return sb.String()
}

func (t *Terminal) renderComparison(c *pattern.Comparison) string {
	if len(c.Changes) == 0 {
		return ""
	}
	var sb strings.Builder
	if c.Label != "" {
		sb.WriteString(t.theme.Bold.Render(c.Label))
		sb.WriteString("\n")
	}
	for _, item := range c.Changes {
		sb.WriteString("  ")
		sb.WriteString(item.Label + ": ")
		sb.WriteString(t.theme.Muted.Render(item.Before + " → " + item.After))
		sb.WriteString(" ")

		var arrow string
		var style lipgloss.Style
		switch {
		case item.Change > 0:
			arrow = "↑"
			style = t.theme.Warning
		case item.Change < 0:
			arrow = "↓"
			style = t.theme.Success
		default:
			arrow = "="
			style = t.theme.Muted
		}
		abs := item.Change
		if abs < 0 {
			abs = -abs
		}
		sb.WriteString(style.Render(fmt.Sprintf("%s %.1f%s", arrow, abs, item.Unit)))
		sb.WriteString("\n")
	}
	return sb.String()
}

func (t *Terminal) iconStyle(kind string) (string, lipgloss.Style) {
	switch kind {
	case "success":
		return t.theme.Icons.Pass, t.theme.Success
	case "error":
		return t.theme.Icons.Fail, t.theme.Error
	case "warning":
		return t.theme.Icons.Warn, t.theme.Warning
	default:
		return t.theme.Icons.Info, t.theme.Primary
	}
}

func (t *Terminal) statusIconStyle(status string) (string, lipgloss.Style) {
	switch status {
	case "pass":
		return t.theme.Icons.Pass, t.theme.Success
	case "fail":
		return t.theme.Icons.Fail, t.theme.Error
	case "skip":
		return t.theme.Icons.Warn, t.theme.Warning
	case "wip":
		return t.theme.Icons.WIP, t.theme.Muted
	default:
		return t.theme.Icons.Info, t.theme.Muted
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func padLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}
