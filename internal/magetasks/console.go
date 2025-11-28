// Package magetasks provides mage build tasks for fo.
//
// This file provides a thin wrapper around fo for project-specific needs.
// Most functionality is now provided by fo directly.
package magetasks

import (
	"io"

	"github.com/dkoosis/fo/fo"
)

// getProjectTheme returns the theme name from the project configuration.
func getProjectTheme() string {
	cfg := fo.LoadProjectConfig()
	return cfg.Theme
}

// console is the shared fo instance for all magetasks.
var console = fo.NewConsoleFromProject()

// streamConsole shows output in real-time (for tests, long-running commands).
var streamConsole = fo.NewConsole(fo.ConsoleConfig{
	LiveStreamOutput: true,
	ThemeName:        getProjectTheme(),
})

// Console returns the shared fo instance.
func Console() *fo.Console {
	return console
}

// StreamConsole returns the fo instance for live streaming output.
func StreamConsole() *fo.Console {
	return streamConsole
}

// SetConsoleWriter sets a custom writer for the console (for testing).
func SetConsoleWriter(w io.Writer) {
	console = fo.NewConsole(fo.ConsoleConfig{
		ShowOutputMode: "on-fail",
		ThemeName:      getProjectTheme(),
		Out:            w,
	})
}

// ResetConsole resets the console to default settings.
func ResetConsole() {
	console = fo.NewConsoleFromProject()
}

// Convenience wrappers - these delegate to fo

// SuccessMsg returns a themed success message.
func SuccessMsg(msg string) string {
	return Console().SuccessMsg(msg)
}

// InfoMsg returns a themed info message.
func InfoMsg(msg string) string {
	return Console().InfoMsg(msg)
}

// WarnMsg returns a themed warning message.
func WarnMsg(msg string) string {
	return Console().WarnMsg(msg)
}

// FormatPath formats a file path with themed colors.
func FormatPath(path string) string {
	return Console().FormatPath(path)
}

// Run executes a command with captured output (shown on failure).
func Run(label, command string, args ...string) error {
	_, err := console.Run(label, command, args...)
	return err
}

// RunCapture executes a command and returns combined output for parsing.
func RunCapture(label, command string, args ...string) (string, error) {
	return console.RunCapture(label, command, args...)
}

// RunStream executes a command with live output.
func RunStream(label, command string, args ...string) error {
	_, err := streamConsole.Run(label, command, args...)
	return err
}

// Re-export types from fo for convenience.
type (
	SectionStatus       = fo.SectionStatus
	SectionWarningError = fo.SectionWarningError
	SectionFunc         = fo.SectionFunc
	Section             = fo.Section
	SectionResult       = fo.SectionResult
)

// Re-export constants from fo.
const (
	SectionOK      = fo.SectionOK
	SectionWarning = fo.SectionWarning
	SectionError   = fo.SectionError
)

// Re-export functions from fo.
var (
	NewSectionWarning = fo.NewSectionWarning
)

// RunSection executes a single section using the default console.
func RunSection(s Section) SectionResult {
	return Console().RunSection(s)
}

// RunSections executes multiple sections using the default console.
func RunSections(sections ...Section) ([]SectionResult, error) {
	return Console().RunSections(sections...)
}

// RunSingleSection runs a single section by name, useful for development testing.
func RunSingleSection(name string, run SectionFunc) error {
	result := RunSection(Section{Name: name, Run: run})
	if result.Status == SectionError {
		return result.Err
	}
	return nil
}

// SetSectionSummary sets a summary message for the current section.
func SetSectionSummary(summary string) {
	Console().SetSectionSummary(summary)
}

// Legacy convenience functions for backward compatibility

// PrintH1Header prints a top-level header using fo.
func PrintH1Header(title string) {
	Console().PrintH1Header(title)
}

// PrintH2Header prints a section header using fo.
func PrintH2Header(title string) {
	Console().PrintSectionHeader(title)
}

// PrintSuccess prints a success message using fo.
func PrintSuccess(msg string) {
	Console().PrintText(SuccessMsg(msg))
}

// PrintWarning prints a warning message using fo.
func PrintWarning(msg string) {
	Console().PrintText(WarnMsg(msg))
}

// PrintError prints an error message using fo.
func PrintError(msg string) {
	Console().PrintText(Console().ErrorMsg(msg))
}

// PrintInfo prints an info message using fo.
func PrintInfo(msg string) {
	Console().PrintText(InfoMsg(msg))
}
