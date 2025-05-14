package design

import (
	"fmt"
	"strings"
	"time"
)

// RenderStartLine returns the formatted start line for the task
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	// Use the border style from config
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly:
		// Top line with corner and header
		sb.WriteString(t.Config.Border.TopCornerChar)
		sb.WriteString(t.Config.Border.HeaderChar + " ")
		label := strings.ToUpper(t.Label)
		sb.WriteString(label)
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, 30))
		sb.WriteString("\n")

		// Vertical bar with space (empty line)
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

		// Process state line with left border
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderHeaderBox:
		// Top line with box around header
		width := calculateWidth(t.Label)
		sb.WriteString(t.Config.Border.TopCornerChar)
		sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width))
		sb.WriteString(t.Config.Border.TopCornerChar + "\n")

		// Header line
		sb.WriteString(t.Config.Border.VerticalChar + " ")
		label := strings.ToUpper(t.Label)
		sb.WriteString(label)
		paddingWidth := width - len(label) - 2
		sb.WriteString(strings.Repeat(" ", paddingWidth))
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

		// Bottom of header box
		sb.WriteString(t.Config.Border.BottomCornerChar)
		sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width))
		sb.WriteString(t.Config.Border.BottomCornerChar + "\n")

		// Empty line
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

		// Process state line
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderFull:
		// Full box (all sides)
		width := calculateWidth(t.Label)
		sb.WriteString(t.Config.Border.TopCornerChar)
		sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width))
		sb.WriteString(t.Config.Border.TopCornerChar + "\n")

		// Empty line
		sb.WriteString(t.Config.Border.VerticalChar)
		sb.WriteString(strings.Repeat(" ", width))
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

		// Process state line
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderNone:
		// Simple label with no border
		label := strings.ToUpper(t.Label)
		sb.WriteString(label + ":\n\n")

	case BorderAscii:
		// ASCII-only version
		label := strings.ToUpper(t.Label)
		sb.WriteString("=" + strings.Repeat("=", len(label)+4) + "\n")
		sb.WriteString("  " + label + "  \n")
		sb.WriteString("\n")
	}

	// Process label based on intent
	processLabel := getProcessLabel(t.Intent)

	// Add the process indicator (same for all styles)
	sb.WriteString(fmt.Sprintf("%s %s%s...%s",
		t.Config.Icons.Start,
		t.Config.Colors.Process,
		processLabel,
		t.Config.Colors.Reset))

	return sb.String()
}

// Helper to generate better process labels
func getProcessLabel(intent string) string {
	// Map intents to better activity labels
	intentMap := map[string]string{
		"building":    "Building",
		"testing":     "Running tests",
		"linting":     "Linting",
		"checking":    "Checking",
		"running":     "Running",
		"installing":  "Installing",
		"downloading": "Downloading",
	}

	if label, ok := intentMap[intent]; ok {
		return label
	}

	// Default label based on intent
	if intent != "" {
		// Capitalize first letter
		return strings.ToUpper(intent[:1]) + intent[1:]
	}

	return "Running" // Fallback
}

// RenderEndLine returns the formatted end line for the task
func (t *Task) RenderEndLine() string {
	var sb strings.Builder

	// Status line depends on border style
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox:
		// Left border for status line
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderFull:
		// Left border for status in full box
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderNone, BorderAscii:
		// No border for status line
		// Just indentation for alignment
		sb.WriteString("")
	}

	// Select icon and color based on status
	var icon, color, statusText string
	switch t.Status {
	case StatusSuccess:
		icon = t.Config.Icons.Success
		color = t.Config.Colors.Success
		statusText = "Complete"
	case StatusWarning:
		icon = t.Config.Icons.Warning
		color = t.Config.Colors.Warning
		statusText = "Completed with warnings"
	case StatusError:
		icon = t.Config.Icons.Error
		color = t.Config.Colors.Error
		statusText = "Failed"
	default:
		icon = t.Config.Icons.Info
		color = t.Config.Colors.Process
		statusText = "Done"
	}

	// Format duration
	durationStr := ""
	if !t.Config.Style.NoTimer {
		durationStr = fmt.Sprintf(" (%s)", formatDuration(t.Duration))
	}

	// Status with duration
	sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
		icon, color, statusText, durationStr, t.Config.Colors.Reset))
	sb.WriteString("\n")

	// Bottom border depends on style
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox:
		// Bottom left corner with dash
		sb.WriteString(t.Config.Border.BottomCornerChar + "─")

	case BorderFull:
		// Full bottom border
		width := calculateWidth(t.Label)
		sb.WriteString(t.Config.Border.BottomCornerChar)
		sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width))
		sb.WriteString(t.Config.Border.BottomCornerChar)

	case BorderNone:
		// No border, just newline

	case BorderAscii:
		// ASCII bottom border
		sb.WriteString(strings.Repeat("-", calculateWidth(t.Label)))
	}

	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	// Add left border depending on style
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderNone, BorderAscii:
		// No left border, just indentation
		sb.WriteString("")
	}

	// Add indentation
	indentLevel := line.Indentation
	if indentLevel > 0 {
		sb.WriteString(t.Config.getIndentation(indentLevel))
	}

	// Content styling based on line type
	content := line.Content

	switch line.Type {
	case TypeError:
		// Red for errors, with italics for high cognitive load
		if line.Context.CognitiveLoad == LoadHigh {
			sb.WriteString(fmt.Sprintf("%s%s%s%s%s",
				t.Config.Colors.Error,
				"\033[3m", // Italics
				content,
				"\033[0m", // Reset italics
				t.Config.Colors.Reset))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s",
				t.Config.Colors.Error,
				content,
				t.Config.Colors.Reset))
		}
	case TypeWarning:
		// Warning with icon
		sb.WriteString(fmt.Sprintf("%s%s %s%s",
			t.Config.Colors.Warning,
			t.Config.Icons.Warning,
			content,
			t.Config.Colors.Reset))
	case TypeSuccess:
		// Green for success
		sb.WriteString(fmt.Sprintf("%s%s%s",
			t.Config.Colors.Success,
			content,
			t.Config.Colors.Reset))
	case TypeInfo:
		// Info with icon
		sb.WriteString(fmt.Sprintf("%s%s %s%s",
			t.Config.Colors.Process,
			t.Config.Icons.Info,
			content,
			t.Config.Colors.Reset))
	case TypeSummary:
		// Bold formatting for summary
		sb.WriteString(fmt.Sprintf("%s%s%s%s",
			t.Config.Colors.Process,
			"\033[1m", // Bold
			content,
			"\033[0m"+t.Config.Colors.Reset)) // Reset bold and color
	case TypeProgress:
		// Muted for progress
		sb.WriteString(fmt.Sprintf("%s%s%s",
			t.Config.Colors.Muted,
			content,
			t.Config.Colors.Reset))
	default: // TypeDetail
		sb.WriteString(content)
	}

	// No right border needed for any style

	return sb.String()
}

// RenderSummary creates a summary section for the output
func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0

	for _, line := range t.OutputLines {
		if line.Type == TypeError {
			errorCount++
		} else if line.Type == TypeWarning {
			warningCount++
		}
	}

	if errorCount == 0 && warningCount == 0 {
		return ""
	}

	var sb strings.Builder

	// Add border based on style
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
		// Empty line before summary
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

		// Summary heading
		sb.WriteString(t.Config.Border.VerticalChar + " ")

	case BorderNone, BorderAscii:
		// Just add a newline
		sb.WriteString("\n")
	}

	// Summary heading
	sb.WriteString(fmt.Sprintf("%s%s%s%s\n",
		t.Config.Colors.Process,
		"\033[1m", // Bold
		"SUMMARY:",
		"\033[0m"+t.Config.Colors.Reset)) // Reset bold and color

	// Error count
	if errorCount > 0 {
		switch t.Config.Border.Style {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			// Left border
			sb.WriteString(t.Config.Border.VerticalChar + " ")

		case BorderNone, BorderAscii:
			// No border
		}

		// Indentation
		sb.WriteString(t.Config.getIndentation(1))

		// Format count
		sb.WriteString(fmt.Sprintf("%s• %d %s%s%s\n",
			t.Config.Colors.Error,
			errorCount,
			"error",
			pluralSuffix(errorCount),
			t.Config.Colors.Reset))
	}

	// Warning count
	if warningCount > 0 {
		switch t.Config.Border.Style {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			// Left border
			sb.WriteString(t.Config.Border.VerticalChar + " ")

		case BorderNone, BorderAscii:
			// No border
		}

		// Indentation
		sb.WriteString(t.Config.getIndentation(1))

		// Format count
		sb.WriteString(fmt.Sprintf("%s• %d %s%s%s\n",
			t.Config.Colors.Warning,
			warningCount,
			"warning",
			pluralSuffix(warningCount),
			t.Config.Colors.Reset))
	}

	// Empty line after summary
	switch t.Config.Border.Style {
	case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
		sb.WriteString(t.Config.Border.VerticalChar + "\n")

	case BorderNone, BorderAscii:
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderCompleteOutput creates the fully formatted task output
func (t *Task) RenderCompleteOutput(showOutput string) string {
	var sb strings.Builder

	// Start line (task header)
	sb.WriteString(t.RenderStartLine())
	sb.WriteString("\n")

	// Determine if we should show output
	showDetailedOutput := false
	switch showOutput {
	case "always":
		showDetailedOutput = true
	case "on-fail":
		showDetailedOutput = (t.Status == StatusError || t.Status == StatusWarning)
	case "never":
		showDetailedOutput = false
	}

	// Add output lines if we should show them
	if showDetailedOutput && len(t.OutputLines) > 0 {
		// Add summary section
		if summary := t.RenderSummary(); summary != "" {
			sb.WriteString(summary)
		}

		// Output lines
		for _, line := range t.OutputLines {
			if line.Type != TypeSummary { // Skip summary lines as they're handled above
				sb.WriteString(t.RenderOutputLine(line))
				sb.WriteString("\n")
			}
		}

		// Empty line before status, with border if needed
		switch t.Config.Border.Style {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + "\n")

		case BorderNone, BorderAscii:
			sb.WriteString("\n")
		}
	}

	// End line (status and bottom border)
	sb.WriteString(t.RenderEndLine())

	return sb.String()
}

// calculateWidth determines the appropriate width for formatting
func calculateWidth(label string) int {
	// Base width on label length, with minimum and maximum values
	minWidth := 30
	maxWidth := 60

	width := len(label) + 10 // Add some space

	if width < minWidth {
		return minWidth
	} else if width > maxWidth {
		return maxWidth
	}

	return width
}

// pluralSuffix returns "s" for counts not equal to 1
func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// formatDuration formats a duration as a human-readable string
func formatDuration(d time.Duration) string {
	// For durations less than a second, use milliseconds
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
	}

	// For durations less than a minute, use seconds with decimal
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", float64(d)/float64(time.Second))
	}

	// For longer durations, use minutes and seconds
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}
