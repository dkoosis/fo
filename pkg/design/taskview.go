// Package design implements pattern-based CLI output visualization.
//
// This file provides the TaskView renderer - a clean separation between
// task data and its visual representation. This design enables:
// 1. Easy testing of rendering logic
// 2. Future migration to Bubble Tea (TaskView becomes a View() function)
// 3. Different renderers for different contexts (terminal, CI, JSON)
package design

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TaskData holds the data needed to render a task.
// This is separate from Task to enable clean rendering without coupling.
type TaskData struct {
	Label     string
	Status    string // "running", "success", "warning", "error"
	Duration  time.Duration
	Lines     []LineData
	ShowLines bool // Whether to show output lines
}

// LineData holds data for a single output line.
type LineData struct {
	Content string
	Type    string // "detail", "error", "warning", "info", "success"
	Indent  int
}

// TaskView renders task data using a theme.
type TaskView struct {
	theme    *Theme
	useBoxes bool
	width    int
}

// NewTaskView creates a task renderer with the given theme.
func NewTaskView(theme *Theme) *TaskView {
	return &TaskView{
		theme:    theme,
		useBoxes: true,
		width:    terminalWidth(),
	}
}

// UseBoxes enables or disables box rendering.
func (v *TaskView) UseBoxes(use bool) *TaskView {
	v.useBoxes = use
	return v
}

// Width sets the render width.
func (v *TaskView) Width(w int) *TaskView {
	v.width = w
	return v
}

// RenderStart renders the task start state (header + "Running...").
func (v *TaskView) RenderStart(data TaskData) string {
	if !v.useBoxes {
		return v.renderStartPlain(data)
	}

	// Build content for the start box
	runningLine := v.theme.Styles.TextNormal.Render(
		"  " + v.theme.Icons.Running + " Running...",
	)

	box := NewBox(v.theme).
		Width(v.width).
		Title(strings.ToUpper(data.Label)).
		AddLine("").
		AddLine(runningLine)

	return box.String()
}

// RenderComplete renders the task completion state.
func (v *TaskView) RenderComplete(data TaskData) string {
	if !v.useBoxes {
		return v.renderCompletePlain(data)
	}

	// Build status line
	statusLine := v.renderStatusLine(data)

	// Build the box with all content
	box := NewBox(v.theme).
		Width(v.width).
		Title(strings.ToUpper(data.Label)).
		AddLine("")

	// Add output lines if requested
	if data.ShowLines && len(data.Lines) > 0 {
		for _, line := range data.Lines {
			box.AddLine(v.renderLine(line))
		}
		box.AddLine("")
	}

	box.AddLine(statusLine)

	return box.String()
}

// RenderUpdate renders an in-progress update (for live mode).
// Returns just the content lines without the box frame.
func (v *TaskView) RenderUpdate(data TaskData) string {
	var lines []string

	for _, line := range data.Lines {
		lines = append(lines, v.renderLine(line))
	}

	return strings.Join(lines, "\n")
}

// renderStatusLine renders the status indicator line.
func (v *TaskView) renderStatusLine(data TaskData) string {
	var icon, text string
	var style lipgloss.Style

	switch data.Status {
	case "success":
		icon = v.theme.Icons.Success
		text = "Complete"
		style = v.theme.Styles.StatusSuccess
	case "warning":
		icon = v.theme.Icons.Warning
		text = "Completed with warnings"
		style = v.theme.Styles.StatusWarning
	case "error":
		icon = v.theme.Icons.Error
		text = "Failed"
		style = v.theme.Styles.StatusError
	default:
		icon = v.theme.Icons.Info
		text = "Done"
		style = v.theme.Styles.TextNormal
	}

	statusPart := style.Render("  " + icon + " " + text)

	// Add duration if present
	if data.Duration > 0 {
		durationStr := formatDurationCompact(data.Duration)
		durationPart := v.theme.Styles.TextMuted.Render(" (" + durationStr + ")")
		return statusPart + durationPart
	}

	return statusPart
}

// renderLine renders a single output line with appropriate styling.
func (v *TaskView) renderLine(line LineData) string {
	indent := strings.Repeat("  ", line.Indent+1) // +1 for base indent

	var style lipgloss.Style
	var prefix string

	switch line.Type {
	case "error":
		style = v.theme.Styles.StatusError
		prefix = v.theme.Icons.Error + " "
	case "warning":
		style = v.theme.Styles.StatusWarning
		prefix = v.theme.Icons.Warning + " "
	case "success":
		style = v.theme.Styles.StatusSuccess
		prefix = v.theme.Icons.Success + " "
	case "info":
		style = v.theme.Styles.TextNormal
		prefix = v.theme.Icons.Info + " "
	default:
		style = v.theme.Styles.TextMuted
		prefix = ""
	}

	return indent + style.Render(prefix+line.Content)
}

// renderStartPlain renders start state without boxes.
func (v *TaskView) renderStartPlain(data TaskData) string {
	return fmt.Sprintf("%s %s...", v.theme.Icons.Running, data.Label)
}

// renderCompletePlain renders completion state without boxes.
func (v *TaskView) renderCompletePlain(data TaskData) string {
	var lines []string

	// Output lines
	if data.ShowLines {
		for _, line := range data.Lines {
			lines = append(lines, v.renderLinePlain(line))
		}
	}

	// Status line
	var icon, text string
	switch data.Status {
	case "success":
		icon = v.theme.Icons.Success
		text = data.Label
	case "warning":
		icon = v.theme.Icons.Warning
		text = data.Label + " (warnings)"
	case "error":
		icon = v.theme.Icons.Error
		text = data.Label + " (failed)"
	default:
		icon = v.theme.Icons.Info
		text = data.Label
	}

	statusLine := icon + " " + text
	if data.Duration > 0 {
		statusLine += " (" + formatDurationCompact(data.Duration) + ")"
	}
	lines = append(lines, statusLine)

	return strings.Join(lines, "\n")
}

// renderLinePlain renders a line without styling.
func (v *TaskView) renderLinePlain(line LineData) string {
	indent := strings.Repeat("  ", line.Indent)
	return indent + line.Content
}

// formatDurationCompact formats a duration in a compact human-readable form.
func formatDurationCompact(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02ds", m, s)
}
