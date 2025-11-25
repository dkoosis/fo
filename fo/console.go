package fo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dkoosis/fo/pkg/adapter"
	"github.com/dkoosis/fo/pkg/design"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// Constants for buffer sizes and limits.
const (
	// DefaultTerminalWidth is the fallback terminal width when detection fails.
	DefaultTerminalWidth = 80

	// DefaultHeaderWidth is the default header width when not specified in theme.
	DefaultHeaderWidth = 60

	// ReadBufferSize is the buffer size for reading from pipes (4KB).
	ReadBufferSize = 4096

	// AdapterDetectionLineCount is the number of lines to buffer for adapter detection.
	AdapterDetectionLineCount = 15

	// SignalTimeout is the timeout for graceful process termination before force kill.
	SignalTimeout = 2 * time.Second

	// StreamCount is the number of output streams (stdout and stderr).
	StreamCount = 2

	// BorderCornerDouble is the double-line box drawing corner character.
	BorderCornerDouble = "╔"

	// Box drawing corner characters.
	BoxCornerTopRightSingle    = "╮"
	BoxCornerTopRightDouble    = "╗"
	BoxCornerBottomRightSingle = "╯"
)

type ConsoleConfig struct {
	ThemeName      string
	UseBoxes       bool
	UseBoxesSet    bool
	InlineProgress bool
	InlineSet      bool
	Monochrome     bool
	ShowTimer      bool
	ShowTimerSet   bool
	ShowOutputMode string
	Stream         bool
	Pattern        string // Manual pattern selection hint (e.g., "test-table", "sparkline", "leaderboard")
	Debug          bool
	Profile        bool   // Enable performance profiling
	ProfileOutput  string // Profile output destination
	MaxBufferSize  int64
	MaxLineLength  int
	Design         *design.Config
	Out            io.Writer // Output writer, defaults to os.Stdout
	Err            io.Writer // Error writer, defaults to os.Stderr
}

// Line represents a classified line of command output.
// This is the public-facing type that doesn't leak internal design package types.
type Line struct {
	Content   string
	Type      string // "detail", "error", "warning", "success", "info", "progress"
	Timestamp time.Time
}

type TaskResult struct {
	Label    string
	Intent   string
	Status   string
	Duration time.Duration
	ExitCode int
	Lines    []Line
	Err      error
}

// ToJSON converts TaskResult to JSON format for structured output.
func (r *TaskResult) ToJSON() ([]byte, error) {
	type JSONLine struct {
		Content   string    `json:"content"`
		Type      string    `json:"type"`
		Timestamp time.Time `json:"timestamp"`
	}

	type JSONOutput struct {
		Version    string     `json:"version"`
		Label      string     `json:"label"`
		Intent     string     `json:"intent"`
		Status     string     `json:"status"`
		ExitCode   int        `json:"exit_code"`
		Duration   string     `json:"duration"`
		DurationMs int64      `json:"duration_ms"`
		Lines      []JSONLine `json:"lines"`
		Error      string     `json:"error,omitempty"`
	}

	jsonLines := make([]JSONLine, len(r.Lines))
	for i, line := range r.Lines {
		jsonLines[i] = JSONLine(line)
	}

	output := JSONOutput{
		Version:    "1.0",
		Label:      r.Label,
		Intent:     r.Intent,
		Status:     r.Status,
		ExitCode:   r.ExitCode,
		Duration:   r.Duration.String(),
		DurationMs: r.Duration.Milliseconds(),
		Lines:      jsonLines,
	}

	if r.Err != nil {
		output.Error = r.Err.Error()
	}

	return json.MarshalIndent(output, "", "  ")
}

type Console struct {
	cfg             ConsoleConfig
	designConf      *design.Config
	adapterRegistry *adapter.Registry
	profiler        *Profiler
	currentSummary  string // Summary message for current section being executed
	inSection       bool   // Whether we're currently executing a section (suppresses individual Run() outputs)
}

func DefaultConsole() *Console {
	return NewConsole(ConsoleConfig{})
}

func NewConsole(cfg ConsoleConfig) *Console {
	normalized := normalizeConfig(cfg)
	profiler := NewProfiler(normalized.Profile, normalized.ProfileOutput)
	return &Console{
		cfg:             normalized,
		designConf:      resolveDesignConfig(normalized),
		adapterRegistry: adapter.NewRegistry(),
		profiler:        profiler,
	}
}

// getPaleGrayColor returns a very pale gray ANSI color code.
func (c *Console) getPaleGrayColor() string {
	return "\033[38;5;252m"
}

// getVeryPaleGrayColor returns an even paler, thinner gray for orca theme borders.
func (c *Console) getVeryPaleGrayColor() string {
	return "\033[38;5;250m" // Lighter than 252, appears thinner
}

// getFaintDarkGrayColor returns a very faint darkish gray ANSI color code.
func (c *Console) getFaintDarkGrayColor() string {
	return "\033[38;5;238m"
}

// BoxLayout defines the dimensions and styling of a rendered box.
// This provides a single source of truth for all box rendering calculations.
type BoxLayout struct {
	TotalWidth   int    // Full terminal width
	ContentWidth int    // Available width for content (TotalWidth - borders - padding)
	LeftPadding  int    // Spaces after left border (typically 2)
	RightPadding int    // Spaces before right border (typically 1)
	BorderColor  string // ANSI color for borders
	BorderChars  BorderChars
}

// BorderChars encapsulates all border characters for consistent rendering.
type BorderChars struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
}

// ContentLine represents structured content for consistent rendering.
// This ensures icons and text align properly across all section lines.
type ContentLine struct {
	Icon      string // Optional icon (e.g., checkmark) - will be rendered at fixed position
	IconColor string // Optional color for icon (empty = no color)
	Text      string // Main text content
	TextColor string // Optional color for text (empty = no color)
}

// calculateBoxLayout computes the box layout dimensions once, providing a single source of truth.
func (c *Console) calculateBoxLayout() *BoxLayout {
	cfg := c.designConf
	totalWidth := c.getTerminalWidth()

	// Content area = total width - left border (1) - left padding (2) - right padding (1) - right border (1)
	// This ensures all content lines use the same width calculation
	contentWidth := totalWidth - 5

	// Determine border color based on theme
	borderColor := c.getFaintDarkGrayColor()
	if cfg.ThemeName == "orca" {
		borderColor = c.getVeryPaleGrayColor()
	}

	// Determine border characters based on theme
	borderChars := c.getBorderChars(cfg)

	return &BoxLayout{
		TotalWidth:   totalWidth,
		ContentWidth: contentWidth,
		LeftPadding:  2,
		RightPadding: 1,
		BorderColor:  borderColor,
		BorderChars:  borderChars,
	}
}

// getBorderChars returns the appropriate border characters for the current theme.
func (c *Console) getBorderChars(cfg *design.Config) BorderChars {
	topCorner := cfg.Border.TopCornerChar
	bottomCorner := cfg.Border.BottomCornerChar
	headerChar := cfg.Border.HeaderChar
	verticalChar := cfg.Border.VerticalChar

	// Determine closing corners based on opening corners
	topRight := BoxCornerTopRightSingle
	switch topCorner {
	case "╔": // Double-line square corner
		topRight = BoxCornerTopRightDouble
	case "╒": // Double-line rounded corner
		topRight = BoxCornerTopRightDouble
	case "╭": // Single-line rounded corner
		topRight = BoxCornerTopRightSingle
	}

	bottomRight := BoxCornerBottomRightSingle
	switch bottomCorner {
	case "╚": // Double-line square corner
		bottomRight = "╝"
	case "╰": // Single-line rounded corner
		bottomRight = BoxCornerBottomRightSingle
	}

	return BorderChars{
		TopLeft:     topCorner,
		TopRight:    topRight,
		BottomLeft:  bottomCorner,
		BottomRight: bottomRight,
		Horizontal:  headerChar,
		Vertical:    verticalChar,
	}
}

// getTerminalWidth returns the terminal width, or a default if unavailable.
// This represents the total box width including corners.
// The horizontal line length will be this value minus 2 (for the corner characters).
func (c *Console) getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return DefaultTerminalWidth
	}
	return width - 3
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// stripANSICodes removes ANSI escape sequences from a string to calculate visual width.
func stripANSICodes(s string) string {
	var result strings.Builder
	inEscape := false
	for i := range len(s) {
		switch {
		case s[i] == '\033':
			inEscape = true
		case inEscape && s[i] == 'm':
			inEscape = false
		case !inEscape:
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// PrintSectionHeader prints a section header and starts a section box.
// Uses BoxLayout for consistent dimensions.
func (c *Console) PrintSectionHeader(name string) {
	cfg := c.designConf
	reset := cfg.ResetColor()

	var sb strings.Builder
	sb.WriteString("\n")

	if cfg.IsMonochrome {
		title := strings.ToUpper(name)
		sb.WriteString("--- ")
		sb.WriteString(title)
		sb.WriteString(" ---\n")
	} else {
		box := c.calculateBoxLayout()
		title := strings.ToUpper(name)
		headerStyle := cfg.GetElementStyle("Task_Label_Header")
		labelColor := cfg.GetColor(headerStyle.ColorFG, "Task_Label_Header")

		// Top border line
		// TotalWidth includes corners, so subtract 2 for the corner chars
		horizontalWidth := box.TotalWidth - 2
		sb.WriteString(box.BorderColor)
		sb.WriteString(box.BorderChars.TopLeft)
		sb.WriteString(strings.Repeat(box.BorderChars.Horizontal, horizontalWidth))
		sb.WriteString(box.BorderChars.TopRight)
		sb.WriteString(reset)
		sb.WriteString("\n")

		// Title line with consistent padding
		sb.WriteString(box.BorderColor)
		sb.WriteString(box.BorderChars.Vertical)
		sb.WriteString(reset)
		sb.WriteString(strings.Repeat(" ", box.LeftPadding))
		sb.WriteString(labelColor)
		if contains(headerStyle.TextStyle, "bold") {
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(title)
		sb.WriteString(reset)
		titleVisualLen := runewidth.StringWidth(stripANSICodes(title))
		remainingWidth := box.TotalWidth - titleVisualLen - box.RightPadding
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		sb.WriteString(strings.Repeat(" ", remainingWidth))
		sb.WriteString(box.BorderColor)
		sb.WriteString(box.BorderChars.Vertical)
		sb.WriteString(reset)
		sb.WriteString("\n")
	}

	_, _ = c.cfg.Out.Write([]byte(sb.String()))
}

// renderBoxLine is the unified function for rendering a line with box borders.
// It handles the border characters, padding, and content alignment consistently.
// The contentWidth parameter is the total visual width of the content including any left padding.
// The content string may include ANSI codes and should already have left padding if needed.
func (c *Console) renderBoxLine(box *BoxLayout, content string, contentWidth int) {
	cfg := c.designConf
	reset := cfg.ResetColor()

	// Calculate right padding to fill the box
	// TotalWidth is the total rendered width including border chars (terminal width - 3)
	// Total rendered = left border (1) + left padding (2) + content + right padding + right border (1) = TotalWidth
	// So: left padding (2) + content + right padding = TotalWidth - 2 (for the two border chars)
	// Therefore: right padding = (TotalWidth - 2) - contentWidth
	// where contentWidth already includes the left padding
	rightPadding := (box.TotalWidth - 2) - contentWidth
	if rightPadding < 0 {
		rightPadding = 0
	}

	var sb strings.Builder
	sb.WriteString(box.BorderColor)
	sb.WriteString(box.BorderChars.Vertical)
	sb.WriteString(reset)
	sb.WriteString(content)
	sb.WriteString(strings.Repeat(" ", rightPadding))
	sb.WriteString(box.BorderColor)
	sb.WriteString(box.BorderChars.Vertical)
	sb.WriteString(reset)
	sb.WriteString("\n")

	_, _ = c.cfg.Out.Write([]byte(sb.String()))
}

// PrintSectionLine prints a line of section content with side borders.
// Uses BoxLayout for consistent dimensions. Supports both plain strings and structured ContentLine.
func (c *Console) PrintSectionLine(line string) {
	cfg := c.designConf
	if cfg.IsMonochrome {
		_, _ = c.cfg.Out.Write([]byte(line + "\n"))
		return
	}

	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	// Clip content to fit within content width
	visualLine := stripANSICodes(line)
	if len(visualLine) > box.ContentWidth {
		clippedVisual := visualLine[:box.ContentWidth]
		ansiEnd := 0
		for i := 0; i < len(line); i++ {
			if line[i] == '\033' {
				for i < len(line) && line[i] != 'm' {
					i++
				}
				ansiEnd = i + 1
			} else {
				break
			}
		}
		if ansiEnd > 0 {
			line = line[:ansiEnd] + clippedVisual + reset
		} else {
			line = clippedVisual
		}
	}

	// Prepare content with left padding
	visualWidth := runewidth.StringWidth(stripANSICodes(line))
	contentWithPadding := strings.Repeat(" ", box.LeftPadding) + line

	// Use unified rendering function
	c.renderBoxLine(box, contentWithPadding, box.LeftPadding+visualWidth)
}

// PrintSectionContentLine renders a structured content line with guaranteed icon alignment.
// Icons are always rendered at the same horizontal position, ensuring vertical alignment.
func (c *Console) PrintSectionContentLine(content ContentLine) {
	cfg := c.designConf
	if cfg.IsMonochrome {
		line := content.Text
		if content.Icon != "" {
			line = content.Icon + " " + line
		}
		_, _ = c.cfg.Out.Write([]byte(line + "\n"))
		return
	}

	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	var sb strings.Builder
	sb.WriteString(box.BorderColor)
	sb.WriteString(box.BorderChars.Vertical)
	sb.WriteString(reset)
	sb.WriteString(strings.Repeat(" ", box.LeftPadding))

	// Render icon at fixed position (if present)
	if content.Icon != "" {
		if content.IconColor != "" {
			sb.WriteString(content.IconColor)
		}
		sb.WriteString(content.Icon)
		if content.IconColor != "" {
			sb.WriteString(reset)
		}
		sb.WriteString(" ") // Space after icon
	}

	// Render text
	if content.TextColor != "" {
		sb.WriteString(content.TextColor)
	}
	sb.WriteString(content.Text)
	if content.TextColor != "" {
		sb.WriteString(reset)
	}

	// Calculate padding: left padding + icon (if present) + space + text + right padding
	iconWidth := 0
	if content.Icon != "" {
		iconWidth = runewidth.StringWidth(content.Icon) + 1 // Icon + space
	}
	textWidth := runewidth.StringWidth(stripANSICodes(content.Text))
	totalContentWidth := iconWidth + textWidth

	minRightPadding := box.RightPadding
	dynamicPadding := (box.TotalWidth - 2) - box.LeftPadding - totalContentWidth - minRightPadding
	if dynamicPadding < 0 {
		dynamicPadding = 0
	}

	sb.WriteString(strings.Repeat(" ", minRightPadding+dynamicPadding))
	sb.WriteString(box.BorderColor)
	sb.WriteString(box.BorderChars.Vertical)
	sb.WriteString(reset)
	sb.WriteString("\n")

	_, _ = c.cfg.Out.Write([]byte(sb.String()))
}

// PrintSectionFooter closes the section box with a bottom border.
// Uses BoxLayout for consistent dimensions.
func (c *Console) PrintSectionFooter() {
	cfg := c.designConf
	if cfg.IsMonochrome {
		_, _ = c.cfg.Out.Write([]byte("\n"))
		return
	}

	box := c.calculateBoxLayout()
	reset := cfg.ResetColor()

	var sb strings.Builder
	// TotalWidth includes corners, so subtract 2 for the corner chars
	horizontalWidth := box.TotalWidth - 2
	sb.WriteString(box.BorderColor)
	sb.WriteString(box.BorderChars.BottomLeft)
	sb.WriteString(strings.Repeat(box.BorderChars.Horizontal, horizontalWidth))
	sb.WriteString(box.BorderChars.BottomRight)
	sb.WriteString(reset)
	sb.WriteString("\n\n")

	_, _ = c.cfg.Out.Write([]byte(sb.String()))
}

// SectionStatus represents the outcome status of a section execution.
type SectionStatus string

const (
	// SectionOK indicates the section completed successfully.
	SectionOK SectionStatus = "ok"
	// SectionWarning indicates the section completed with warnings.
	SectionWarning SectionStatus = "warning"
	// SectionError indicates the section failed.
	SectionError SectionStatus = "error"
)

// SectionWarningError is a special error type that signals a section completed
// with warnings but should not be treated as a fatal error.
type SectionWarningError struct {
	Err error
}

// Error implements the error interface.
func (e *SectionWarningError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error for error wrapping compatibility.
func (e *SectionWarningError) Unwrap() error {
	return e.Err
}

// NewSectionWarning wraps an error as a warning that should be displayed
// but not fail the section execution.
func NewSectionWarning(err error) error {
	if err == nil {
		return nil
	}
	return &SectionWarningError{Err: err}
}

// SectionFunc is a function that performs work for a section.
type SectionFunc func() error

// Section represents a unit of orchestration with a name and work function.
type Section struct {
	Name        string      // Human-readable section name, shown in header
	Description string      // Optional description
	Summary     string      // Optional summary message (shown instead of name/description on success)
	Run         SectionFunc // Work to perform for this section
}

// SectionResult contains the outcome of running a section.
type SectionResult struct {
	Name     string
	Status   SectionStatus
	Duration time.Duration
	Err      error
	Summary  string // Optional summary message from the section
}

// RunSection executes a single section and returns its result.
// It prints a section header, runs the work function, and prints
// a one-line summary with status icon and duration.
func (c *Console) RunSection(s Section) SectionResult {
	start := time.Now()

	// 1) Section header
	c.PrintSectionHeader(s.Name)

	// 2) Mark that we're in a section (suppresses individual Run() outputs)
	wasInSection := c.inSection
	c.inSection = true
	c.currentSummary = "" // Clear any previous summary

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG RunSection] Starting section '%s', inSection=%v\n", s.Name, c.inSection)
	}

	// 3) Run the actual work
	err := s.Run()

	// Restore previous section state
	c.inSection = wasInSection

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG RunSection] Completed section '%s', restoring inSection=%v\n", s.Name, wasInSection)
	}

	// 3) Derive status from error type
	status := SectionOK
	var displayErr error
	if err != nil {
		var warningErr *SectionWarningError
		if errors.As(err, &warningErr) {
			status = SectionWarning
			displayErr = warningErr.Err
		} else {
			status = SectionError
			displayErr = err
		}
	}

	// 4) Timing + trailing summary line
	duration := time.Since(start)
	roundedDuration := duration.Round(time.Second)
	seconds := int(roundedDuration.Seconds())

	// Format duration with muted color
	mutedColor := c.GetMutedColor()
	resetColor := c.designConf.ResetColor()
	durationStr := fmt.Sprintf("%s(%ds)%s", mutedColor, seconds, resetColor)

	// Get status icon and color from theme
	icon, iconColor := c.getStatusIcon(status)

	// Render status line with structured content
	content := ContentLine{
		Icon:      icon,
		IconColor: iconColor,
		TextColor: "", // Use default text color
	}

	// Determine what text to show
	// Priority: error message > section summary > current summary > description > name
	summary := s.Summary
	if summary == "" {
		summary = c.currentSummary
		c.currentSummary = "" // Clear after use
	}
	switch {
	case displayErr != nil:
		content.Text = fmt.Sprintf("%s %s: %v", s.Name, durationStr, displayErr)
	case summary != "":
		content.Text = fmt.Sprintf("%s %s", summary, durationStr)
	case s.Description != "":
		content.Text = fmt.Sprintf("%s %s", s.Description, durationStr)
	default:
		content.Text = fmt.Sprintf("%s %s", s.Name, durationStr)
	}

	c.PrintSectionContentLine(content)

	// Close the section box
	c.PrintSectionFooter()

	return SectionResult{
		Name:     s.Name,
		Status:   status,
		Duration: duration,
		Err:      displayErr,
	}
}

// RunSections executes multiple sections in sequence and returns all results
// plus an aggregated error. If any section fails (SectionError), the aggregated
// error will be non-nil, but all sections will still be executed. Warnings
// (SectionWarning) are displayed but do not contribute to the aggregated error.
func (c *Console) RunSections(sections ...Section) ([]SectionResult, error) {
	results := make([]SectionResult, 0, len(sections))
	var errs []error

	for _, s := range sections {
		res := c.RunSection(s)
		results = append(results, res)
		// Only aggregate actual errors, not warnings
		if res.Status == SectionError && res.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", res.Name, res.Err))
		}
	}

	return results, errors.Join(errs...)
}

// SetSectionSummary sets a summary message for the current section being executed.
// This should be called by section work functions to provide a summary message
// that will be shown instead of the section name on success.
func (c *Console) SetSectionSummary(summary string) {
	c.currentSummary = summary
}

// getStatusIcon returns the icon and color for a section status from the theme.
// Returns (icon, color) where color is empty string if no color should be applied.
func (c *Console) getStatusIcon(status SectionStatus) (string, string) {
	cfg := c.designConf
	switch status {
	case SectionOK:
		icon := cfg.GetIcon("Success")
		color := cfg.GetColor("Success")
		return icon, color
	case SectionWarning:
		icon := cfg.GetIcon("Warning")
		color := cfg.GetColor("Warning")
		return icon, color
	case SectionError:
		icon := cfg.GetIcon("Error")
		color := cfg.GetColor("Error")
		return icon, color
	default:
		// Fallback for unknown status
		return "", ""
	}
}

// PrintH1Header prints a major headline (H1) using the console's theme.
func (c *Console) PrintH1Header(name string) {
	cfg := c.designConf
	headerWidth := cfg.Style.HeaderWidth
	if headerWidth <= 0 {
		headerWidth = 70
	}

	title := strings.ToUpper(name)
	paleGray := c.getPaleGrayColor()
	reset := cfg.ResetColor()

	var sb strings.Builder

	if cfg.IsMonochrome {
		sb.WriteString("=== ")
		sb.WriteString(title)
		sb.WriteString(" ===\n")
	} else {
		headerStyle := cfg.GetElementStyle("H1_Major_Header")
		labelColor := cfg.GetColor(headerStyle.ColorFG, "H1_Major_Header")

		topCorner := cfg.Border.TopCornerChar
		headerChar := cfg.Border.HeaderChar
		closingCorner := BoxCornerTopRightSingle
		if topCorner == BorderCornerDouble {
			closingCorner = BoxCornerTopRightDouble
		}
		sb.WriteString(paleGray)
		sb.WriteString(topCorner)
		sb.WriteString(strings.Repeat(headerChar, headerWidth))
		sb.WriteString(closingCorner)
		sb.WriteString(reset)
		sb.WriteString("\n")

		sb.WriteString(paleGray)
		sb.WriteString(cfg.Border.VerticalChar)
		sb.WriteString(reset)
		sb.WriteString("  ")
		sb.WriteString(labelColor)
		if contains(headerStyle.TextStyle, "bold") {
			sb.WriteString(cfg.GetColor("Bold"))
		}
		sb.WriteString(title)
		sb.WriteString(reset)
		titleLen := len(title) + 3
		remainingWidth := headerWidth + 2 - titleLen - 1
		if remainingWidth < 0 {
			remainingWidth = 0
		}
		sb.WriteString(strings.Repeat(" ", remainingWidth))
		sb.WriteString(paleGray)
		sb.WriteString(cfg.Border.VerticalChar)
		sb.WriteString(reset)
		sb.WriteString("\n")

		bottomCorner := cfg.Border.BottomCornerChar
		bottomClosingCorner := BoxCornerBottomRightSingle
		if bottomCorner == "╚" {
			bottomClosingCorner = "╝"
		}
		sb.WriteString(paleGray)
		sb.WriteString(bottomCorner)
		sb.WriteString(strings.Repeat(headerChar, headerWidth))
		sb.WriteString(bottomClosingCorner)
		sb.WriteString(reset)
		sb.WriteString("\n")
	}

	_, _ = c.cfg.Out.Write([]byte(sb.String()))
}

// GetMutedColor returns the Muted color code from the theme.
func (c *Console) GetMutedColor() string {
	return c.designConf.GetColor("Muted")
}

// ResetColor returns the reset color code from the theme.
func (c *Console) ResetColor() string {
	return c.designConf.ResetColor()
}

// GetSuccessColor returns the Success color code from the theme.
func (c *Console) GetSuccessColor() string {
	return c.designConf.GetColor("Success")
}

// GetGreenFgColor returns the light green color code from the theme.
func (c *Console) GetGreenFgColor() string {
	return c.designConf.GetColor("GreenFg")
}

// GetBlueFgColor returns the light blue color code from the theme.
func (c *Console) GetBlueFgColor() string {
	return c.designConf.GetColor("BlueFg")
}

// GetWarningColor returns the Warning color code from the theme.
func (c *Console) GetWarningColor() string {
	return c.designConf.GetColor("Warning")
}

// GetErrorColor returns the Error color code from the theme.
func (c *Console) GetErrorColor() string {
	return c.designConf.GetColor("Error")
}

// GetIcon returns an icon from the theme by key.
func (c *Console) GetIcon(iconKey string) string {
	return c.designConf.GetIcon(iconKey)
}

// GetColor returns a color code from the theme by key.
func (c *Console) GetColor(colorKey string) string {
	return c.designConf.GetColor(colorKey)
}

// FormatStatusIcon returns a fully styled icon string for the given status.
// Status can be "PASS", "FAIL", "SKIP", "SUCCESS", "ERROR", "WARNING", or "MUTED".
// Returns the icon with appropriate color and reset code.
func (c *Console) FormatStatusIcon(status string) string {
	statusUpper := strings.ToUpper(status)
	var icon string
	var color string

	switch statusUpper {
	case "PASS", "SUCCESS":
		icon = c.GetIcon("Success")
		if icon == "" {
			icon = defaultSuccessIcon
		}
		color = c.GetSuccessColor()
	case "FAIL", "ERROR":
		icon = c.GetIcon("Error")
		if icon == "" {
			icon = "✗"
		}
		color = c.GetErrorColor()
	case "WARNING":
		icon = c.GetIcon("Warning")
		if icon == "" {
			icon = "⚠"
		}
		color = c.GetWarningColor()
	case "SKIP", "MUTED":
		icon = "▫"
		color = c.GetMutedColor()
	default:
		icon = "▫"
		color = c.GetMutedColor()
	}

	if color == "" {
		return icon
	}
	return color + icon + c.ResetColor()
}

// FormatStatusText returns a fully styled text string for the given status.
// Status can be "PASS", "FAIL", "SKIP", "SUCCESS", "ERROR", "WARNING", or "MUTED".
// Returns the text with appropriate color and reset code.
func (c *Console) FormatStatusText(text, status string) string {
	statusUpper := strings.ToUpper(status)
	var color string

	switch statusUpper {
	case "PASS", "SUCCESS":
		color = c.GetSuccessColor()
	case "FAIL", "ERROR":
		color = c.GetErrorColor()
	case "WARNING":
		color = c.GetWarningColor()
	case "SKIP", "MUTED":
		color = c.GetMutedColor()
	default:
		color = c.GetMutedColor()
	}

	if color == "" {
		return text
	}
	return color + text + c.ResetColor()
}

// FormatTestName returns a fully styled test name for the given status.
// This is a convenience method that combines FormatStatusIcon and FormatStatusText
// for common test rendering patterns.
func (c *Console) FormatTestName(name, status string) string {
	icon := c.FormatStatusIcon(status)
	humanName := HumanizeTestName(name)
	text := c.FormatStatusText(humanName, status)
	return icon + " " + text
}

// SuccessMsg returns a themed success message with icon and color.
func (c *Console) SuccessMsg(msg string) string {
	icon := c.GetIcon("Success")
	color := c.GetColor("Success")
	reset := c.designConf.ResetColor()
	return fmt.Sprintf("%s%s%s %s", color, icon, reset, msg)
}

// InfoMsg returns a themed info message with icon and color.
func (c *Console) InfoMsg(msg string) string {
	icon := c.GetIcon("Info")
	color := c.GetColor("BlueFg")
	reset := c.designConf.ResetColor()
	return fmt.Sprintf("%s%s%s %s", color, icon, reset, msg)
}

// WarnMsg returns a themed warning message with icon and color.
func (c *Console) WarnMsg(msg string) string {
	icon := c.GetIcon("Warning")
	color := c.GetColor("Warning")
	reset := c.designConf.ResetColor()
	return fmt.Sprintf("%s%s%s %s", color, icon, reset, msg)
}

// ErrorMsg returns a themed error message with icon and color.
func (c *Console) ErrorMsg(msg string) string {
	icon := c.GetIcon("Error")
	color := c.GetColor("Error")
	reset := c.designConf.ResetColor()
	return fmt.Sprintf("%s%s%s %s", color, icon, reset, msg)
}

// FormatPath formats a file path with directory in muted color and filename in white.
func (c *Console) FormatPath(path string) string {
	if path == "" {
		return path
	}

	// Split path into directory and filename
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	mutedColor := c.GetMutedColor()
	whiteColor := c.GetColor("White")
	reset := c.designConf.ResetColor()

	if dir == "." || dir == "" {
		// No directory, just filename
		return fmt.Sprintf("%s%s%s", whiteColor, filename, reset)
	}

	// Directory in muted color, filename in white
	return fmt.Sprintf("%s%s%s%s%s%s", mutedColor, dir+string(filepath.Separator), reset, whiteColor, filename, reset)
}

// GetBorderChars returns the border characters from the theme.
func (c *Console) GetBorderChars() (string, string, string, string) {
	return c.designConf.Border.TopCornerChar,
		c.designConf.Border.BottomCornerChar,
		c.designConf.Border.HeaderChar,
		c.designConf.Border.VerticalChar
}

// GetHeaderWidth returns the header width from the theme.
func (c *Console) GetHeaderWidth() int {
	width := c.designConf.Style.HeaderWidth
	if width <= 0 {
		return DefaultHeaderWidth
	}
	return width
}

// Run executes a command and returns the result.
//
// Error semantics:
//   - Returns (result, nil) when the command runs successfully (exit code 0)
//   - Returns (result, error) when the command runs but exits non-zero;
//     the error wraps the underlying exec.ExitError
//   - Returns (result, error) for infrastructure failures (command not found,
//     IO errors, context cancelled)
//
// Note: TaskResult is always non-nil. Even for infrastructure failures, the
// result contains useful information like duration, label, and any captured
// internal error messages. Use TaskResult.ExitCode (127 for command not found,
// 1 for other failures) and TaskResult.Err for failure details.
//
// Use errors.Is(err, exec.ErrNotFound) to check for missing commands.
func (c *Console) Run(label, command string, args ...string) (*TaskResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, getInterruptSignals()...)
	// Note: signal.Stop is called in the signal handler goroutine (see runContext)

	return c.runContext(ctx, cancel, sigChan, label, command, args)
}

// ErrNonZeroExit is returned when a command completes but exits with a non-zero code.
// Use errors.Is(err, ErrNonZeroExit) to check for this condition.
var ErrNonZeroExit = errors.New("command exited with non-zero code")

// ExitCodeError wraps an exit code for programmatic access.
// Use errors.As(err, &ExitCodeError{}) to extract the exit code from RunSimple errors.
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// RunSimple executes a command and returns only an error.
// This is a convenience wrapper around Run for simple use cases where you
// only need to know success vs failure.
//
// Returns nil on success (exit code 0).
// Returns ErrNonZeroExit (wrapped with ExitCodeError) if the command exits
// with non-zero code.
// Returns other errors for infrastructure failures.
//
// To check for non-zero exit and extract the code:
//
//	if errors.Is(err, ErrNonZeroExit) {
//	    var exitErr ExitCodeError
//	    if errors.As(err, &exitErr) {
//	        fmt.Printf("Exit code: %d\n", exitErr.Code)
//	    }
//	}
//
// For detailed results including captured output, use Run() instead.
func (c *Console) RunSimple(command string, args ...string) error {
	_, err := c.Run("", command, args...)
	if err == nil {
		return nil
	}

	// Map exec.ExitError to our wrapper error with extractable exit code
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := getExitCode(err, c.cfg.Debug)
		return fmt.Errorf("%w: %w", ErrNonZeroExit, ExitCodeError{Code: code})
	}
	return err // Infrastructure error, pass through
}

//nolint:funlen,gocognit // Complex function handling command execution, signal handling, and output rendering
func (c *Console) runContext(
	ctx context.Context, cancel context.CancelFunc, sigChan chan os.Signal,
	label, command string, args []string,
) (*TaskResult, error) {
	labelToUse := label
	if labelToUse == "" {
		labelToUse = filepath.Base(command)
	}

	designCfg := design.DeepCopyConfig(c.designConf)

	patternMatcher := design.NewPatternMatcher(designCfg)
	intent := patternMatcher.DetectCommandIntent(command, args)
	task := design.NewTask(labelToUse, intent, command, args, designCfg)

	useInlineProgress := designCfg.Style.UseInlineProgress && c.cfg.InlineProgress && !c.cfg.Stream

	progress := design.NewInlineProgress(task, c.cfg.Debug, c.cfg.Out)

	// Set up cursor restoration at the outermost level for inline progress
	if useInlineProgress {
		enableSpinner := !designCfg.Style.NoSpinner
		if enableSpinner && design.IsInteractiveTerminal() && !designCfg.IsMonochrome {
			// Hide cursor at start, restore on any exit path
			_, _ = c.cfg.Out.Write([]byte("\033[?25l"))
			defer func() {
				_, _ = c.cfg.Out.Write([]byte("\033[?25h"))
			}()
		}
		progress.Start(ctx, enableSpinner)
	} else if !c.inSection {
		// Suppress start line when in a section - section header already shows context
		_, _ = c.cfg.Out.Write([]byte(task.RenderStartLine() + "\n"))
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = os.Environ()
	setProcessGroup(cmd)

	cmdDone := make(chan struct{})

	// Goroutine: Handle signals
	signalHandlerDone := make(chan struct{})
	go func() {
		defer func() {
			signal.Stop(sigChan)
			close(signalHandlerDone)
		}()
		select {
		case sig := <-sigChan:
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v\n", sig)
				processStateStr := "nil"
				if cmd.ProcessState != nil {
					processStateStr = fmt.Sprintf("%+v", cmd.ProcessState)
				}
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Process state: %s\n", processStateStr)
			}
			if cmd.Process == nil {
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Process is nil, canceling context.")
				}
				cancel()
				return
			}
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Received signal %v for PID %d. Forwarding...\n", sig, cmd.Process.Pid)
			}
			if err := killProcessGroup(cmd, sig); err != nil && c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG sigChan] Error killing process group: %v\n", err)
			}
			select {
			case <-cmdDone:
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received after signal forwarding.")
				}
			case <-time.After(SignalTimeout):
				if c.cfg.Debug {
					fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Timeout after signal, ensuring process is killed.")
				}
				if cmd.Process != nil && cmd.ProcessState == nil {
					_ = killProcessGroupWithSIGKILL(cmd)
				}
				cancel()
			}
		case <-ctx.Done():
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] Context done, ensuring process is killed if running.")
			}
			if cmd.Process != nil && cmd.ProcessState == nil {
				_ = killProcessGroupWithSIGKILL(cmd)
			}
		case <-cmdDone:
			if c.cfg.Debug {
				fmt.Fprintln(os.Stderr, "[DEBUG sigChan] cmdDone received, command finished naturally.")
			}
		}
	}()

	var exitCode int
	var cmdRunError error
	var isActualFoStartupFailure bool

	// Execute command (these functions call cmd.Start() and cmd.Wait())
	// They will close cmdDone when cmd.Wait() completes
	if c.cfg.Stream {
		exitCode, cmdRunError = c.executeStreamMode(cmd, task, cmdDone)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	} else {
		exitCode, cmdRunError = c.executeCaptureMode(cmd, task, patternMatcher, cmdDone)
		if cmdRunError != nil {
			var exitErr *exec.ExitError
			if !errors.As(cmdRunError, &exitErr) {
				isActualFoStartupFailure = true
			}
		}
	}

	// Wait for signal handler to finish
	<-signalHandlerDone

	task.Complete(exitCode)

	// Write profile data if enabled
	if c.profiler != nil {
		_ = c.profiler.Write()
	}

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr,
			"[DEBUG executeCommand] CI=%t, exitCode=%d, task.Status=%s, isActualFoStartupFailure=%t\n",
			c.cfg.Monochrome && !c.cfg.ShowTimer, exitCode, task.Status, isActualFoStartupFailure)
	}

	if useInlineProgress {
		status := design.StatusSuccess
		if exitCode != 0 {
			status = design.StatusError
		} else if task.Status == design.StatusWarning {
			status = design.StatusWarning
		}
		progress.Complete(status)
	}

	if !c.cfg.Stream {
		c.renderCapturedOutput(task, exitCode, isActualFoStartupFailure)
	} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
		summary := task.RenderSummary()
		if summary != "" {
			_, _ = c.cfg.Out.Write([]byte(summary))
		}
	}

	if !useInlineProgress {
		// Suppress individual task end lines when we're in a section
		// The section summary will be printed instead at the end of RunSection()
		if c.inSection {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG] Suppressing RenderEndLine for '%s' (inSection=true)\n", task.Label)
			}
			// Don't render - section summary will be shown instead
		} else {
			_, _ = c.cfg.Out.Write([]byte(task.RenderEndLine() + "\n"))
		}
	}

	// Convert design.OutputLine to Line
	lines := make([]Line, len(task.OutputLines))
	for i, ol := range task.OutputLines {
		lines[i] = Line{
			Content:   ol.Content,
			Type:      ol.Type,
			Timestamp: ol.Timestamp,
		}
	}

	return &TaskResult{
		Label:    task.Label,
		Intent:   task.Intent,
		Status:   task.Status,
		Duration: task.Duration,
		ExitCode: exitCode,
		Lines:    lines,
		Err:      cmdRunError,
	}, cmdRunError
}

// RunCapture executes a command and returns the combined output as a string.
// This is useful when you need to parse command output.
func (c *Console) RunCapture(label, command string, args ...string) (string, error) {
	result, err := c.Run(label, command, args...)
	if err != nil {
		return "", err
	}

	var output strings.Builder
	for _, line := range result.Lines {
		output.WriteString(line.Content)
		output.WriteString("\n")
	}
	return output.String(), nil
}

func (c *Console) renderCapturedOutput(task *design.Task, exitCode int, isActualFoStartupFailure bool) {
	showCaptured := false
	switch c.cfg.ShowOutputMode {
	case "always":
		showCaptured = true
	case "on-fail":
		if exitCode != 0 {
			showCaptured = true
		}
	}

	if showCaptured && !isActualFoStartupFailure {
		summary := task.RenderSummary()
		if summary != "" {
			_, _ = c.cfg.Out.Write([]byte(summary))
		}

		hasActualRenderableOutput := false
		task.OutputLinesLock()
		for _, l := range task.OutputLines {
			// Check IsInternal flag first, fall back to string prefix for backwards compatibility
			isInternal := l.Context.IsInternal ||
				(l.Type == design.TypeError && (strings.HasPrefix(l.Content, "Error starting command") ||
					strings.HasPrefix(l.Content, "Error creating stdout pipe") ||
					strings.HasPrefix(l.Content, "Error creating stderr pipe") ||
					strings.HasPrefix(l.Content, "[fo] ")))
			if !isInternal {
				hasActualRenderableOutput = true
				break
			}
		}
		task.OutputLinesUnlock()

		if hasActualRenderableOutput {
			_, _ = c.cfg.Out.Write([]byte(task.Config.GetColor("Muted") + "--- Captured output: ---" + task.Config.ResetColor() + "\n"))
			task.OutputLinesLock()
			for _, line := range task.OutputLines {
				_, _ = c.cfg.Out.Write([]byte(task.RenderOutputLine(line) + "\n"))
			}
			task.OutputLinesUnlock()
		} else if (task.Status == design.StatusError || task.Status == design.StatusWarning) && summary == "" {
			summary = task.RenderSummary()
			if summary != "" {
				_, _ = c.cfg.Out.Write([]byte(summary))
			}
		}
	} else if !showCaptured && (task.Status == design.StatusError || task.Status == design.StatusWarning) && !isActualFoStartupFailure {
		summary := task.RenderSummary()
		if summary != "" {
			_, _ = c.cfg.Out.Write([]byte(summary))
		}
	}
}

func (c *Console) executeStreamMode(cmd *exec.Cmd, task *design.Task, cmdDone chan struct{}) (int, error) {
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode] Error creating stderr pipe, fallback to direct os.Stderr:", err)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr := cmd.Run()
		close(cmdDone) // Signal that command has finished
		errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true}
		task.AddOutputLine(
			formatInternalError("Error setting up stderr pipe for stream mode: %v", err),
			design.TypeError, errCtx)
		exitCode := getExitCode(runErr, c.cfg.Debug)
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return exitCode, runErr
		}
		return exitCode, runErr
	}
	cmd.Stdout = os.Stdout

	// Connect stdin for interactive input support in stream mode
	// Check if stdin is a terminal to support interactive commands
	if term.IsTerminal(int(os.Stdin.Fd())) {
		cmd.Stdin = os.Stdin
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		scanner := bufio.NewScanner(stderrPipe)
		buffer := make([]byte, 0, bufio.MaxScanTokenSize)
		scanner.Buffer(buffer, c.cfg.MaxLineLength)

		for scanner.Scan() {
			line := scanner.Text()
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanned line: %s\n", line)
			}
			fmt.Fprintln(c.cfg.Err, line)
			task.AddOutputLine(line, design.TypeDetail, design.LineContext{CognitiveLoad: design.LoadMedium, Importance: 2})
		}
		if scanErr := scanner.Err(); scanErr != nil {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner error: %v\n", scanErr)
			}
			isIgnorable := errors.Is(scanErr, io.EOF) ||
				strings.Contains(scanErr.Error(), "file already closed") ||
				strings.Contains(scanErr.Error(), "broken pipe")
			if !isIgnorable {
				errCtx := design.LineContext{
					CognitiveLoad: design.LoadMedium, Importance: 3, IsInternal: true,
				}
				task.AddOutputLine(
					formatInternalError("Error reading stderr in stream mode: %v", scanErr),
					design.TypeError, errCtx)
			}
		} else if c.cfg.Debug {
			fmt.Fprintln(os.Stderr, "[DEBUG executeStreamMode STDERR] Scanner finished without error.")
		}
	}()

	startErr := cmd.Start()
	if startErr != nil {
		errMsg := fmt.Sprintf("Error starting command '%s': %v", strings.Join(cmd.Args, " "), startErr)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)

		_ = stderrPipe.Close()
		waitGroup.Wait()
		close(cmdDone) // Signal that command has finished (failed to start)

		return getExitCode(startErr, c.cfg.Debug), startErr
	}

	runErr := cmd.Wait()
	waitGroup.Wait()
	close(cmdDone) // Signal that command has finished

	exitCode := getExitCode(runErr, c.cfg.Debug)
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return exitCode, runErr
		}
	}

	return exitCode, runErr
}

func (c *Console) executeCaptureMode(
	cmd *exec.Cmd, task *design.Task, patternMatcher *design.PatternMatcher, cmdDone chan struct{},
) (int, error) {
	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG executeCaptureMode] Starting in CAPTURE mode\n")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		errMsg := formatInternalError("Error creating stdout pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		close(cmdDone) // Signal that command has finished (failed to create pipe)
		return 1, err
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		errMsg := formatInternalError("Error creating stderr pipe: %v", err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		if closeErr := stdoutPipe.Close(); closeErr != nil && c.cfg.Debug {
			fmt.Fprintf(c.cfg.Err, "[DEBUG] Error closing stdout pipe: %v\n", closeErr)
		}
		close(cmdDone) // Signal that command has finished (failed to create pipe)
		return 1, err
	}

	if err := cmd.Start(); err != nil {
		errMsg := formatInternalError("Error starting command '%s': %v", strings.Join(cmd.Args, " "), err)
		task.AddOutputLine(errMsg, design.TypeError, design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true})
		fmt.Fprintln(c.cfg.Err, errMsg)
		if closeErr := stdoutPipe.Close(); closeErr != nil && c.cfg.Debug {
			fmt.Fprintf(c.cfg.Err, "[DEBUG] Error closing stdout pipe: %v\n", closeErr)
		}
		if closeErr := stderrPipe.Close(); closeErr != nil && c.cfg.Debug {
			fmt.Fprintf(c.cfg.Err, "[DEBUG] Error closing stderr pipe: %v\n", closeErr)
		}
		close(cmdDone) // Signal that command has finished (failed to start)
		return getExitCode(err, c.cfg.Debug), err
	}

	// Try adapter-based parsing first
	exitCode, runErr := c.tryAdapterMode(cmd, task, stdoutPipe, stderrPipe, patternMatcher, cmdDone)

	return exitCode, runErr
}

// tryAdapterMode attempts to use a stream adapter to parse structured output.
// If no adapter is detected or adapter parsing fails, falls back to line-by-line classification.
func (c *Console) tryAdapterMode(
	cmd *exec.Cmd,
	task *design.Task,
	stdoutPipe io.ReadCloser,
	stderrPipe io.ReadCloser,
	patternMatcher *design.PatternMatcher,
	cmdDone chan struct{},
) (int, error) {
	captureStart := c.profiler.StartStage("capture")

	// Buffer output from both streams
	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer
	var wgRead sync.WaitGroup
	wgRead.Add(StreamCount)

	var errStdoutCopy, errStderrCopy error
	var totalBytesRead int64 // Use atomic operations for thread-safe access
	maxTotalBytes := c.cfg.MaxBufferSize * 2

	// Goroutine to capture stdout
	go func() {
		defer wgRead.Done()
		buf := make([]byte, ReadBufferSize)
		for {
			n, readErr := stdoutPipe.Read(buf)
			if n > 0 {
				// Atomically check and update buffer size to prevent race conditions
				for {
					current := atomic.LoadInt64(&totalBytesRead)
					newTotal := current + int64(n)
					if newTotal > maxTotalBytes {
						break // Exceeded limit, skip this chunk
					}
					if atomic.CompareAndSwapInt64(&totalBytesRead, current, newTotal) {
						stdoutBuffer.Write(buf[:n])
						break
					}
					// CAS failed, retry (another goroutine updated totalBytesRead)
				}
			}
			if readErr != nil {
				if readErr != io.EOF &&
					!strings.Contains(readErr.Error(), "file already closed") &&
					!strings.Contains(readErr.Error(), "broken pipe") {
					errStdoutCopy = readErr
				}
				break
			}
		}
	}()

	// Goroutine to capture stderr
	go func() {
		defer wgRead.Done()
		buf := make([]byte, ReadBufferSize)
		for {
			n, readErr := stderrPipe.Read(buf)
			if n > 0 {
				// Atomically check and update buffer size to prevent race conditions
				for {
					current := atomic.LoadInt64(&totalBytesRead)
					newTotal := current + int64(n)
					if newTotal > maxTotalBytes {
						break // Exceeded limit, skip this chunk
					}
					if atomic.CompareAndSwapInt64(&totalBytesRead, current, newTotal) {
						stderrBuffer.Write(buf[:n])
						break
					}
					// CAS failed, retry (another goroutine updated totalBytesRead)
				}
			}
			if readErr != nil {
				if readErr != io.EOF &&
					!strings.Contains(readErr.Error(), "file already closed") &&
					!strings.Contains(readErr.Error(), "broken pipe") {
					errStderrCopy = readErr
				}
				break
			}
		}
	}()

	// Wait for command and output capture to complete
	runErr := cmd.Wait()
	wgRead.Wait()
	close(cmdDone)

	// Get combined output for detection
	combinedOutput := append(stdoutBuffer.Bytes(), stderrBuffer.Bytes()...)

	// Extract first N lines for adapter detection
	firstLines := extractFirstLines(string(combinedOutput), AdapterDetectionLineCount)

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG tryAdapterMode] Captured %d bytes, first %d lines for detection\n",
			len(combinedOutput), len(firstLines))
	}

	// Try to detect a suitable adapter
	detectedAdapter := c.adapterRegistry.Detect(firstLines)

	if detectedAdapter != nil {
		if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG tryAdapterMode] Detected adapter: %s\n", detectedAdapter.Name())
		}

		// Parse with adapter
		pattern, parseErr := detectedAdapter.Parse(bytes.NewReader(combinedOutput))
		if parseErr == nil && pattern != nil {
			if c.cfg.Debug {
				fmt.Fprintf(os.Stderr, "[DEBUG tryAdapterMode] Successfully parsed with adapter: %s\n", detectedAdapter.Name())
			}

			// Render the pattern using the design config
			rendered := pattern.Render(task.Config)
			if rendered != "" {
				// Add the rendered pattern as output
				task.AddOutputLine(rendered, design.TypeDetail, design.LineContext{
					CognitiveLoad: design.LoadLow,
					Importance:    4,
					IsInternal:    false,
				})
			}

			exitCode := getExitCode(runErr, c.cfg.Debug)
			return exitCode, runErr
		}

		if c.cfg.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG tryAdapterMode] Adapter parsing failed: %v, falling back to line-by-line\n", parseErr)
		}
	} else if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG tryAdapterMode] No adapter detected, using line-by-line classification\n")
	}

	// Fall back to line-by-line classification
	processStart := c.profiler.StartStage("process")
	c.processBufferedOutputLineByLine(task, string(combinedOutput), patternMatcher)
	c.profiler.EndStage("process", processStart, map[string]interface{}{
		"line_count":  len(strings.Split(string(combinedOutput), "\n")),
		"buffer_size": int64(len(combinedOutput)),
	})

	// Report any capture errors
	errCtx := design.LineContext{CognitiveLoad: design.LoadHigh, Importance: 5, IsInternal: true}
	if errStdoutCopy != nil {
		task.AddOutputLine(
			formatInternalError("Error reading stdout: %v", errStdoutCopy),
			design.TypeError, errCtx)
	}
	if errStderrCopy != nil {
		task.AddOutputLine(
			formatInternalError("Error reading stderr: %v", errStderrCopy),
			design.TypeError, errCtx)
	}

	c.profiler.EndStage("capture", captureStart, map[string]interface{}{
		"buffer_size": int64(len(combinedOutput)),
	})

	exitCode := getExitCode(runErr, c.cfg.Debug)
	task.UpdateTaskContext()

	return exitCode, runErr
}

// extractFirstLines extracts the first N lines from the output for adapter detection.
func extractFirstLines(output string, count int) []string {
	lines := strings.Split(output, "\n")
	if len(lines) > count {
		lines = lines[:count]
	}
	return lines
}

// processBufferedOutputLineByLine processes buffered output with line-by-line classification.
func (c *Console) processBufferedOutputLineByLine(
	task *design.Task,
	output string,
	patternMatcher *design.PatternMatcher,
) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	buf := make([]byte, 0, bufio.MaxScanTokenSize)
	scanner.Buffer(buf, c.cfg.MaxLineLength)

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineType, lineContext := patternMatcher.ClassifyOutputLine(line, task.Command, task.Args)
		if c.cfg.Debug && lineCount < 5 {
			fmt.Fprintf(os.Stderr, "[DEBUG processBufferedOutput] Line classified as %s: %s\n", lineType, line)
		}
		task.AddOutputLine(line, lineType, lineContext)
		lineCount++
	}

	if c.cfg.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG processBufferedOutput] Processed %d lines with line-by-line classification\n", lineCount)
	}
}

// formatInternalError formats an internal fo error message with consistent prefix.
// All internal fo errors should use this function to ensure clear distinction
// from command output errors.
func formatInternalError(format string, args ...interface{}) string {
	return fmt.Sprintf("[fo] "+format, args...)
}

func getExitCode(err error, debug bool) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code, ok := getExitCodeFromError(exitErr); ok {
			return code
		}
		if debug {
			fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] ExitError.Sys() not WaitStatus: %T\n", exitErr.Sys())
		}
		return 1
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[DEBUG getExitCode] Non-ExitError type: %T, error: %v\n", err, err)
	}

	if isCommandNotFoundError(err) {
		return 127
	}
	return 1
}

// isCommandNotFoundError checks if the error indicates the command was not found.
// This handles the standard exec.ErrNotFound and platform-specific string fallbacks
// for older Go versions or edge cases.
func isCommandNotFoundError(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Fallback string matching for edge cases
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return true
	}
	if runtime.GOOS != "windows" && strings.Contains(errStr, "no such file or directory") {
		return true
	}
	return false
}

func normalizeConfig(cfg ConsoleConfig) ConsoleConfig {
	normalized := cfg
	if normalized.ShowOutputMode == "" {
		normalized.ShowOutputMode = "on-fail"
	}
	if normalized.MaxBufferSize == 0 {
		normalized.MaxBufferSize = 10 * 1024 * 1024
	}
	if normalized.MaxLineLength == 0 {
		normalized.MaxLineLength = 1 * 1024 * 1024
	}
	if cfg.ShowTimerSet {
		normalized.ShowTimer = cfg.ShowTimer
	} else {
		normalized.ShowTimer = true
	}
	switch {
	case cfg.InlineSet:
		normalized.InlineProgress = cfg.InlineProgress
	case cfg.Design != nil:
		normalized.InlineProgress = cfg.Design.Style.UseInlineProgress
	default:
		normalized.InlineProgress = true
	}
	if normalized.Out == nil {
		normalized.Out = os.Stdout
	}
	if normalized.Err == nil {
		normalized.Err = os.Stderr
	}
	return normalized
}

func resolveDesignConfig(cfg ConsoleConfig) *design.Config {
	if cfg.Design != nil {
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] Using provided Design config (ThemeName: %s)\n", cfg.Design.ThemeName)
		}
		return design.DeepCopyConfig(cfg.Design)
	}

	var base *design.Config

	// Check if theme name is provided and exists in default themes
	if cfg.ThemeName != "" {
		themes := design.DefaultThemes()
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] Requested theme: %q\n", cfg.ThemeName)
			fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] Available themes: %v\n", getThemeNames(themes))
		}
		if theme, ok := themes[cfg.ThemeName]; ok {
			base = design.DeepCopyConfig(theme)
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] ✓ Found theme %q: TopCorner=%q, HeaderChar=%q, ProcessColor=%q\n",
					cfg.ThemeName, base.Border.TopCornerChar, base.Border.HeaderChar, base.GetColor("Process"))
			}
		} else {
			// Theme not found in defaults, fall back to unicode_vibrant
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] ✗ Theme %q not found, falling back to unicode_vibrant\n", cfg.ThemeName)
			}
			base = design.UnicodeVibrantTheme()
		}
	} else {
		// No theme name specified, use default
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] No theme name specified, using default unicode_vibrant\n")
		}
		base = design.UnicodeVibrantTheme()
	}

	if cfg.Monochrome {
		design.ApplyMonochromeDefaults(base)
	}

	if cfg.UseBoxesSet {
		base.Style.UseBoxes = cfg.UseBoxes
	}
	// Only override theme's UseInlineProgress if explicitly set in console config
	if cfg.InlineSet {
		base.Style.UseInlineProgress = cfg.InlineProgress
	}
	if cfg.ShowTimerSet {
		base.Style.NoTimer = !cfg.ShowTimer
	}

	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG resolveDesignConfig] Final theme: %s (TopCorner=%q, HeaderChar=%q)\n",
			base.ThemeName, base.Border.TopCornerChar, base.Border.HeaderChar)
	}
	return base
}

// getThemeNames returns a slice of theme names from the themes map.
func getThemeNames(themes map[string]*design.Config) []string {
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	return names
}
