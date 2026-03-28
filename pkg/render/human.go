package render

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dkoosis/fo/pkg/pattern"
)

// Human renders patterns as styled terminal output via lipgloss.
type Human struct {
	theme Theme
}

// NewHuman creates a human renderer with the given theme.
func NewHuman(theme Theme) *Human {
	return &Human{theme: theme}
}

// Render formats all patterns for human display.
func (t *Human) Render(patterns []pattern.Pattern) string {
	var sections []string
	for _, p := range patterns {
		s := t.renderOne(p)
		if s != "" {
			sections = append(sections, s)
		}
	}
	return strings.Join(sections, "\n")
}

func (t *Human) renderOne(p pattern.Pattern) string {
	switch v := p.(type) {
	case *pattern.Summary:
		return t.renderSummary(v)
	case *pattern.Leaderboard:
		return t.renderLeaderboard(v)
	case *pattern.TestTable:
		return t.renderTestTable(v)
	case *pattern.Error:
		return t.renderError(v)
	default:
		return ""
	}
}

func (t *Human) renderSummary(s *pattern.Summary) string {
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

func (t *Human) renderLeaderboard(l *pattern.Leaderboard) string {
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

func (t *Human) renderTestTable(tt *pattern.TestTable) string {
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

func (t *Human) iconStyle(kind pattern.ItemKind) (string, lipgloss.Style) {
	switch kind {
	case pattern.KindSuccess:
		return t.theme.Icons.Pass, t.theme.Success
	case pattern.KindError:
		return t.theme.Icons.Fail, t.theme.Error
	case pattern.KindWarning:
		return t.theme.Icons.Warn, t.theme.Warning
	default:
		return t.theme.Icons.Info, t.theme.Primary
	}
}

func (t *Human) statusIconStyle(status pattern.Status) (string, lipgloss.Style) {
	switch status {
	case pattern.StatusPass:
		return t.theme.Icons.Pass, t.theme.Success
	case pattern.StatusFail:
		return t.theme.Icons.Fail, t.theme.Error
	case pattern.StatusSkip:
		return t.theme.Icons.Warn, t.theme.Warning
	default:
		return t.theme.Icons.Info, t.theme.Muted
	}
}

func (t *Human) renderError(e *pattern.Error) string {
	return t.theme.Error.Render(fmt.Sprintf("  %s %s: %s", t.theme.Icons.Fail, e.Source, e.Message)) + "\n"
}

func padRight(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

func padLeft(s string, width int) string {
	n := len([]rune(s))
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}
