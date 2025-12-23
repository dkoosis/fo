package dashboard

import "github.com/charmbracelet/lipgloss"

// FormatterStyles contains shared styles for all dashboard formatters.
// Centralizes the 62+ duplicated style definitions across formatters.
type FormatterStyles struct {
	Error   lipgloss.Style
	Warn    lipgloss.Style
	Success lipgloss.Style
	Header  lipgloss.Style
	File    lipgloss.Style
	Muted   lipgloss.Style
}

// DefaultFormatterStyles returns the standard formatter style set.
func DefaultFormatterStyles() *FormatterStyles {
	return &FormatterStyles{
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56")).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FFBD2E")).Bold(true),
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Bold(true),
		Header:  lipgloss.NewStyle().Foreground(lipgloss.Color("#0077B6")).Bold(true),
		File:    lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")),
		Muted:   lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")),
	}
}

// defaultStyles is the singleton instance.
var defaultStyles = DefaultFormatterStyles()

// Styles returns the default formatter styles.
func Styles() *FormatterStyles {
	return defaultStyles
}
