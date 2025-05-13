package design

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RenderStartLine returns the formatted start line for the task
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	// Top border if using boxes
	if t.Config.Style.UseBoxes {
		width := calculateWidth(t.Label)
		sb.WriteString("┌─ ")
		sb.WriteString(strings.ToUpper(t.Label))
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat("─", width-len(t.Label)-3))
		sb.WriteString("┐\n")

		// Empty line for visual padding
		sb.WriteString("│")
		sb.WriteString(strings.Repeat(" ", width))
		sb.WriteString("│\n")
	}

	// Start line - process state
	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	// Icon and colored label
	sb.WriteString(fmt.Sprintf("%s %s%s...%s",
		t.Config.Icons.Start,
		t.Config.Colors.Process,
		t.formatIntentLabel(),
		t.Config.Colors.Reset))

	if t.Config.Style.UseBoxes {
		paddingWidth := calculateWidth(t.Label) - len(t.formatIntentLabel()) - 4 // Adjust for icon and ellipsis
		if paddingWidth > 0 {                                                    // Ensure padding is not negative
			sb.WriteString(strings.Repeat(" ", paddingWidth))
		}
		sb.WriteString("│")
	}

	return sb.String()
}

// formatIntentLabel creates a proper label from the intent
func (t *Task) formatIntentLabel() string {
	if t.Intent == "" {
		return filepath.Base(t.Command)
	}

	// Capitalize first letter
	intent := t.Intent
	if len(intent) > 0 {
		intent = strings.ToUpper(intent[:1]) + intent[1:]
	}

	return intent
}

// RenderEndLine returns the formatted end line for the task
func (t *Task) RenderEndLine() string {
	var sb strings.Builder

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
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
		icon = t.Config.Icons.Info      // Fallback icon
		color = t.Config.Colors.Process // Fallback color
		statusText = "Done"             // Fallback text
	}

	// Format duration
	durationStr := ""
	// Access NoTimer from t.Config.Style.NoTimer
	if !t.Config.Style.NoTimer { // Check the NoTimer field from the Style struct
		durationStr = formatDuration(t.Duration) // formatDuration is now local to this file
	}

	// Status line
	sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
		icon, color, statusText, durationStr, t.Config.Colors.Reset))

	if t.Config.Style.UseBoxes {
		// Add padding to align with right border
		width := calculateWidth(t.Label)
		// Calculate current line length more accurately
		// Length of icon (often 2 for emoji) + space + statusText + durationStr
		currentLength := len(icon) + 1 + len(statusText) + len(durationStr)
		paddingWidth := width - currentLength
		if paddingWidth > 0 {
			sb.WriteString(strings.Repeat(" ", paddingWidth))
		}
		sb.WriteString("│")
	}

	// Bottom border if using boxes
	if t.Config.Style.UseBoxes {
		width := calculateWidth(t.Label)
		sb.WriteString("\n└")
		sb.WriteString(strings.Repeat("─", width+1)) // +1 to account for the left box char
		sb.WriteString("┘")
	}

	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	// Add indentation
	indentLevel := 1
	if line.Indentation > 0 {
		indentLevel = line.Indentation
	}
	sb.WriteString(t.Config.getIndentation(indentLevel))

	// Add timestamp if enabled
	if t.Config.Style.ShowTimestamps {
		elapsed := line.Timestamp.Sub(t.StartTime)
		sb.WriteString(fmt.Sprintf("[%s] ", formatDuration(elapsed)))
	}

	// Style based on line type and context
	content := line.Content

	switch line.Type {
	case TypeError:
		// Research: Red italics reduce cognitive load for critical info (Zhou et al.)
		if line.Context.CognitiveLoad == LoadHigh && !t.Config.Accessibility.ScreenReaderFriendly {
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
		sb.WriteString(fmt.Sprintf("%s%s %s%s",
			t.Config.Colors.Warning,
			t.Config.Icons.Warning,
			content,
			t.Config.Colors.Reset))
	case TypeSuccess:
		sb.WriteString(fmt.Sprintf("%s%s%s",
			t.Config.Colors.Success,
			content,
			t.Config.Colors.Reset))
	case TypeInfo:
		sb.WriteString(fmt.Sprintf("%s%s %s%s",
			t.Config.Colors.Process, // Using process color for info
			t.Config.Icons.Info,
			content,
			t.Config.Colors.Reset))
	case TypeSummary:
		// Bold formatting for summary
		if !t.Config.Accessibility.ScreenReaderFriendly {
			sb.WriteString(fmt.Sprintf("%s%s%s%s",
				t.Config.Colors.Process, // Using process color for summary
				"\033[1m",               // Bold
				content,
				"\033[0m"+t.Config.Colors.Reset)) // Reset bold and color
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s",
				t.Config.Colors.Process,
				content,
				t.Config.Colors.Reset))
		}
	case TypeProgress:
		sb.WriteString(fmt.Sprintf("%s%s%s",
			t.Config.Colors.Muted,
			content,
			t.Config.Colors.Reset))
	default: // TypeDetail
		sb.WriteString(fmt.Sprintf("%s%s%s", // Apply detail color
			t.Config.Colors.Detail,
			content,
			t.Config.Colors.Reset))
	}

	// Add right border padding if using boxes
	if t.Config.Style.UseBoxes {
		visibleContentLength := len(stripANSI(content)) // Basic ANSI stripping for length
		// Calculate current line length more accurately
		currentLineLength := len(t.Config.getIndentation(indentLevel)) + visibleContentLength
		if t.Config.Style.ShowTimestamps {
			// A more precise calculation for timestamp length would be better
			currentLineLength += len(formatDuration(line.Timestamp.Sub(t.StartTime))) + 3 // for "[ts] "
		}
		if line.Type == TypeWarning || line.Type == TypeInfo {
			currentLineLength += len(t.Config.Icons.Warning) + 1 // Icon and space
		}

		paddingWidth := calculateWidth(t.Label) - currentLineLength
		if paddingWidth > 0 {
			sb.WriteString(strings.Repeat(" ", paddingWidth))
		}
		sb.WriteString("│")
	}

	return sb.String()
}

// stripANSI removes ANSI escape codes from a string for length calculation.
// This is a basic implementation and might not cover all ANSI sequences.
func stripANSI(s string) string {
	// This regex matches typical SGR (Select Graphic Rendition) escape sequences.
	// \x1b is the ESC character.
	// \[ is the literal '['.
	// [0-9;]* matches zero or more digits or semicolons (parameters for the SGR command).
	// [mKHF] matches common SGR terminators (m for SGR, K for EL, H for CUP, F for CPL - though m is most relevant for color/style).
	re := regexp.MustCompile(`\x1b\[[0-9;]*[mKHF]`)
	return re.ReplaceAllString(s, "")
}

// RenderSummary creates a summary section for the output
func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0

	for _, line := range t.OutputLines {
		switch line.Type {
		case TypeError:
			errorCount++
		case TypeWarning:
			warningCount++
		}
	}

	if errorCount == 0 && warningCount == 0 {
		return "" // No summary needed if no issues
	}

	var sb strings.Builder

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ") // Indent summary within box
	}

	// Summary heading
	summaryHeading := "SUMMARY:"
	if !t.Config.Accessibility.ScreenReaderFriendly {
		sb.WriteString(fmt.Sprintf("%s%s%s%s\n",
			t.Config.Colors.Process, // Use process color for summary heading
			"\033[1m",               // Bold
			summaryHeading,
			"\033[0m"+t.Config.Colors.Reset)) // Reset bold and then color
	} else {
		sb.WriteString(fmt.Sprintf("%s%s%s\n",
			t.Config.Colors.Process,
			summaryHeading,
			t.Config.Colors.Reset))
	}

	indentStr := ""
	if t.Config.Style.UseBoxes {
		indentStr = "│ " + t.Config.getIndentation(1) // Box prefix + one level of indentation
	} else {
		indentStr = t.Config.getIndentation(1) // Just one level of indentation
	}

	if errorCount > 0 {
		sb.WriteString(indentStr)
		sb.WriteString(fmt.Sprintf("%s• %d error%s detected%s\n",
			t.Config.Colors.Error,
			errorCount,
			pluralSuffix(errorCount),
			t.Config.Colors.Reset))
	}

	if warningCount > 0 {
		sb.WriteString(indentStr)
		sb.WriteString(fmt.Sprintf("%s• %d warning%s present%s\n",
			t.Config.Colors.Warning,
			warningCount,
			pluralSuffix(warningCount),
			t.Config.Colors.Reset))
	}

	// Empty line after summary for visual separation, only if using boxes
	if t.Config.Style.UseBoxes {
		sb.WriteString("│")
		sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
		sb.WriteString("│\n")
	} else if errorCount > 0 || warningCount > 0 { // Add newline if not using boxes and summary was printed
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderCompleteOutput creates the fully formatted output.
// This function assumes that cmd/main.go will call RenderStartLine, then conditionally
// print the summary and output lines, and finally call RenderEndLine.
// This function itself might not be directly called if main.go orchestrates parts.
func (t *Task) RenderCompleteOutput(showOutput string) string {
	var sb strings.Builder

	sb.WriteString(t.RenderStartLine())
	sb.WriteString("\n")

	showDetailedOutput := false
	switch showOutput {
	case "always":
		showDetailedOutput = true
	case "on-fail":
		showDetailedOutput = (t.Status == StatusError || t.Status == StatusWarning)
		// "never" case means showDetailedOutput remains false
	}

	if showDetailedOutput && len(t.OutputLines) > 0 {
		if t.Config.Style.UseBoxes {
			sb.WriteString("│") // Empty line with borders
			sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
			sb.WriteString("│\n")
		}

		if summary := t.RenderSummary(); summary != "" {
			sb.WriteString(summary) // RenderSummary already adds newlines appropriately
		}

		var renderedLines []string
		if t.Context.CognitiveLoad == LoadHigh && t.Config.Output.SummarizeSimilar {
			similarGroups := t.SummarizeOutputGroups()
			for _, groupedLines := range similarGroups {
				for _, line := range groupedLines {
					renderedLines = append(renderedLines, t.RenderOutputLine(line))
				}
			}
		} else {
			for _, line := range t.OutputLines {
				renderedLines = append(renderedLines, t.RenderOutputLine(line))
			}
		}
		if len(renderedLines) > 0 {
			sb.WriteString(strings.Join(renderedLines, "\n"))
			sb.WriteString("\n") // Add a final newline after all output lines
		}

		if t.Config.Style.UseBoxes {
			sb.WriteString("│") // Empty line with borders
			sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
			sb.WriteString("│\n")
		}
	}

	sb.WriteString(t.RenderEndLine())
	return sb.String()
}

// SummarizeOutputGroups groups similar output lines for better readability.
func (t *Task) SummarizeOutputGroups() [][]OutputLine {
	pm := NewPatternMatcher(t.Config)
	groups := pm.FindSimilarLines(t.OutputLines)

	var result [][]OutputLine
	// Process groups by type to control order (e.g., errors first)
	processGroupType := func(targetType string, groupNameForSummary string) {
		for key, lines := range groups {
			if strings.HasPrefix(key, targetType) {
				result = append(result, t.sampleAndSummarizeGroup(lines, groupNameForSummary))
				delete(groups, key) // Remove processed group
			}
		}
	}

	processGroupType(TypeError, "errors")
	processGroupType(TypeWarning, "warnings")

	// Process remaining groups
	for _, lines := range groups {
		result = append(result, lines) // Or apply generic sampling
	}
	return result
}

// sampleAndSummarizeGroup is a helper to sample lines and add a summary line.
func (t *Task) sampleAndSummarizeGroup(lines []OutputLine, groupType string) []OutputLine {
	if len(lines) > t.Config.Output.MaxErrorSamples && t.Config.Output.SummarizeSimilar {
		sampleLines := lines[:t.Config.Output.MaxErrorSamples]
		summaryLine := OutputLine{
			Content:     fmt.Sprintf("... %d similar %s", len(lines)-t.Config.Output.MaxErrorSamples, groupType),
			Type:        TypeSummary,
			Timestamp:   time.Now(), // Or consider timestamp of last sampled line
			Indentation: 1,          // Consistent indentation for summary line
			Context:     LineContext{CognitiveLoad: t.Context.CognitiveLoad, Importance: 3},
		}
		return append(sampleLines, summaryLine)
	}
	return lines
}

// Helper functions

// calculateWidth determines the appropriate width for task display.
func calculateWidth(label string) int {
	minWidth := 50
	maxWidth := 80
	// Base width on label length, with added space for icons, status, timer.
	// This is an estimate; precise calculation is complex due to variable content.
	width := len(label) + 25 // Increased padding for better fitting

	if width < minWidth {
		return minWidth
	}
	if width > maxWidth {
		return maxWidth
	}
	return width
}

// pluralSuffix returns "s" for counts not equal to 1.
func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// formatDuration converts a time.Duration to a human-readable string.
// This function is now defined only in render.go.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		secondsFraction := d.Seconds() - float64(minutes*60)
		return fmt.Sprintf("%dm%.1fs", minutes, secondsFraction)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
