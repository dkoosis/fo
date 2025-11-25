// Package design implements pattern-based CLI output visualization
package design

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PadRight pads a string to the specified visual width, using spaces.
// This correctly handles Unicode characters, emojis, and wide characters
// that occupy multiple terminal cells.
// Replaces fmt.Sprintf("%-*s", width, s) with correct rune width handling.
func PadRight(s string, width int) string {
	vw := lipgloss.Width(s)
	if vw >= width {
		return s
	}
	return s + strings.Repeat(" ", width-vw)
}

// PadLeft pads a string to the specified visual width, using spaces.
// This correctly handles Unicode characters, emojis, and wide characters
// that occupy multiple terminal cells.
// Replaces fmt.Sprintf("%*s", width, s) with correct rune width handling.
func PadLeft(s string, width int) string {
	vw := lipgloss.Width(s)
	if vw >= width {
		return s
	}
	return strings.Repeat(" ", width-vw) + s
}

// VisualWidth returns the display width of a string in terminal cells.
// This correctly handles Unicode characters, emojis, and wide characters
// that occupy multiple terminal cells.
// This is a wrapper around lipgloss.Width for consistency.
func VisualWidth(s string) int {
	return lipgloss.Width(s)
}

// truncateToWidth truncates a string to fit within the specified visual width,
// preserving visual width rather than byte length. This is useful for truncating
// strings that may contain wide characters or emojis.
func truncateToWidth(s string, maxWidth int) string {
	if VisualWidth(s) <= maxWidth {
		return s
	}
	// Truncate character by character until we fit
	runes := []rune(s)
	result := strings.Builder{}
	currentWidth := 0
	for _, r := range runes {
		runeStr := string(r)
		runeWidth := VisualWidth(runeStr)
		if currentWidth+runeWidth > maxWidth {
			break
		}
		result.WriteString(runeStr)
		currentWidth += runeWidth
	}
	return result.String()
}

