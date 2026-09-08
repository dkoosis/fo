// Package textwidth measures the on-screen (visible) width of a string —
// ANSI escape sequences cost no columns and are excluded, wide (CJK, emoji)
// runes cost two. It exists so pkg/paint (column alignment) and pkg/scene
// (asciicast viewport sizing) share one yardstick instead of each keeping
// its own, differently-precise implementation (fo-d84 review: paint's was
// lipgloss.Width-based and CJK-aware, scene's was a regex-strip-plus-rune-
// count that explicitly wasn't — two functions named the same thing,
// silently disagreeing).
package textwidth

import "github.com/charmbracelet/lipgloss"

// Visible returns s's on-screen cell width: ANSI escapes are invisible and
// counted as zero columns; wide runes count as two.
func Visible(s string) int {
	return lipgloss.Width(s)
}
