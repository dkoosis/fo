// Package design implements pattern-based CLI output visualization.
//
// This file provides lipgloss-idiomatic box rendering.
// Instead of rendering borders piecemeal (top, content lines, bottom),
// we build complete content and apply box styling in one pass.
package design

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Box renders content inside a styled box using lipgloss.
// This is the idiomatic way to use lipgloss - build content, then style.
type Box struct {
	theme    *Theme
	width    int
	title    string
	content  []string
	footer   string
	disabled bool // For monochrome/no-box mode
}

// NewBox creates a new box renderer with the given theme.
func NewBox(theme *Theme) *Box {
	width := terminalWidth()
	return &Box{
		theme:   theme,
		width:   width,
		content: make([]string, 0),
	}
}

// Title sets the box title (rendered in the header area).
func (b *Box) Title(title string) *Box {
	b.title = title
	return b
}

// AddLine adds a content line to the box.
func (b *Box) AddLine(line string) *Box {
	b.content = append(b.content, line)
	return b
}

// AddLines adds multiple content lines.
func (b *Box) AddLines(lines ...string) *Box {
	b.content = append(b.content, lines...)
	return b
}

// Footer sets a footer line (rendered after content, before bottom border).
func (b *Box) Footer(footer string) *Box {
	b.footer = footer
	return b
}

// Disable turns off box rendering (content only, no borders).
func (b *Box) Disable() *Box {
	b.disabled = true
	return b
}

// Width sets explicit width (default is terminal width).
func (b *Box) Width(w int) *Box {
	b.width = w
	return b
}

// String renders the box to a string.
// This is where lipgloss does the work - we build content, apply style once.
func (b *Box) String() string {
	if b.disabled || b.theme == nil {
		// No box mode - just return content
		return b.renderPlain()
	}

	// Build styled content parts
	parts := b.buildContentParts(true)

	// Join all content
	innerContent := strings.Join(parts, "\n")

	// Apply box style - lipgloss handles borders, padding, width
	// Width calculation: subtract 2 for borders + 2 for horizontal padding (Padding(0,1) = 1 each side)
	boxStyle := b.theme.Styles.Box.Width(b.width - 4)

	return boxStyle.Render(innerContent)
}

// renderPlain renders content without box styling.
func (b *Box) renderPlain() string {
	parts := b.buildContentParts(false)
	return strings.Join(parts, "\n")
}

// buildContentParts assembles the content parts (title, content, footer).
// When styled=true, applies theme styles to title and footer.
func (b *Box) buildContentParts(styled bool) []string {
	var parts []string

	// Title line (if set)
	if b.title != "" {
		if styled && b.theme != nil {
			parts = append(parts, b.theme.Styles.Header.Render(b.title))
		} else {
			parts = append(parts, b.title)
		}
	}

	// Content lines
	parts = append(parts, b.content...)

	// Footer (if set)
	if b.footer != "" {
		parts = append(parts, "") // Empty line before footer
		if styled && b.theme != nil {
			parts = append(parts, b.theme.Styles.TextMuted.Render(b.footer))
		} else {
			parts = append(parts, b.footer)
		}
	}

	return parts
}

// terminalWidth returns the current terminal width, defaulting to 80.
func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// RenderBox is a convenience function for simple box rendering.
// Usage: RenderBox(theme, "Title", "Line 1", "Line 2")
func RenderBox(theme *Theme, title string, lines ...string) string {
	return NewBox(theme).Title(title).AddLines(lines...).String()
}

// RenderInlineStatus renders a status line (icon + message + optional duration).
// This is commonly used for task completion messages.
func RenderInlineStatus(theme *Theme, status, message, duration string) string {
	var icon string
	var style lipgloss.Style

	switch status {
	case StatusSuccess:
		icon = theme.Icons.Success
		style = theme.Styles.StatusSuccess
	case StatusWarning:
		icon = theme.Icons.Warning
		style = theme.Styles.StatusWarning
	case StatusError:
		icon = theme.Icons.Error
		style = theme.Styles.StatusError
	default:
		icon = theme.Icons.Info
		style = theme.Styles.TextNormal
	}

	statusPart := style.Render(icon + " " + message)

	if duration != "" {
		durationPart := theme.Styles.TextMuted.Render(" (" + duration + ")")
		return lipgloss.JoinHorizontal(lipgloss.Top, statusPart, durationPart)
	}

	return statusPart
}
