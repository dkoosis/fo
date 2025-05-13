package design

import (
	"fmt"
	"path/filepath"
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
		sb.WriteString(strings.Repeat(" ", paddingWidth))
		sb.WriteString("│")
	}

	return sb.String()
}

// formatIntentLabel creates a proper label from the intent
func (t *Task) formatIntentLabel() string {
	if t.Intent == "" {
		return filepath.Base(t.Command)
	}

	// Capitalize first letter, ensure it ends with "ing"
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
		icon = t.Config.Icons.Info
		color = t.Config.Colors.Process
		statusText = "Done"
	}

	// Format duration
	durationStr := formatDuration(t.Duration)

	// Status line
	sb.WriteString(fmt.Sprintf("%s %s%s (%s)%s",
		icon, color, statusText, durationStr, t.Config.Colors.Reset))

	if t.Config.Style.UseBoxes {
		// Add padding to align with right border
		width := calculateWidth(t.Label)
		paddingWidth := width - len(statusText) - len(durationStr) - 4 // Adjust for icon and parentheses
		if paddingWidth > 0 {
			sb.WriteString(strings.Repeat(" ", paddingWidth))
		}
		sb.WriteString("│")
	}

	// Bottom border if using boxes
	if t.Config.Style.UseBoxes {
		width := calculateWidth(t.Label)
		sb.WriteString("\n└")
		sb.WriteString(strings.Repeat("─", width+1))
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
		sb.WriteString(fmt.Sprintf("%s%s%s",
			t.Config.Colors.Muted,
			content,
			t.Config.Colors.Reset))
	default: // TypeDetail
		sb.WriteString(content)
	}

	// Add right border padding if using boxes
	if t.Config.Style.UseBoxes {
		paddingWidth := calculateWidth(t.Label) - len(content) - t.Config.getIndentation(indentLevel).Length()
		if t.Config.Style.ShowTimestamps {
			// Adjust for timestamp
			paddingWidth -= 10 // Approximate timestamp width
		}
		if line.Type == TypeWarning || line.Type == TypeInfo {
			// Adjust for icon
			paddingWidth -= 2
		}
		if paddingWidth > 0 {
			sb.WriteString(strings.Repeat(" ", paddingWidth))
		}
		sb.WriteString("│")
	}

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

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	// Summary heading
	sb.WriteString(fmt.Sprintf("%s%s%s%s\n",
		t.Config.Colors.Process,
		"\033[1m", // Bold
		"SUMMARY:",
		"\033[0m"+t.Config.Colors.Reset)) // Reset bold and color

	if errorCount > 0 {
		if t.Config.Style.UseBoxes {
			sb.WriteString("│ ")
		}
		sb.WriteString(t.Config.getIndentation(1))
		sb.WriteString(fmt.Sprintf("%s• %d error%s detected%s\n",
			t.Config.Colors.Error,
			errorCount,
			pluralSuffix(errorCount),
			t.Config.Colors.Reset))
	}

	if warningCount > 0 {
		if t.Config.Style.UseBoxes {
			sb.WriteString("│ ")
		}
		sb.WriteString(t.Config.getIndentation(1))
		sb.WriteString(fmt.Sprintf("%s• %d warning%s present%s\n",
			t.Config.Colors.Warning,
			warningCount,
			pluralSuffix(warningCount),
			t.Config.Colors.Reset))
	}

	// Empty line after summary
	if t.Config.Style.UseBoxes {
		sb.WriteString("│")
		sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
		sb.WriteString("│\n")
	} else {
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderCompleteOutput creates the fully formatted output
func (t *Task) RenderCompleteOutput(showOutput string) string {
	var sb strings.Builder

	// Start line
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
		// Empty line for visual separation
		if t.Config.Style.UseBoxes {
			sb.WriteString("│")
			sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
			sb.WriteString("│\n")
		}

		// Summary section
		if summary := t.RenderSummary(); summary != "" {
			sb.WriteString(summary)
		}

		// Group lines by type if the cognitive load is high
		var renderedLines []string

		if t.Context.CognitiveLoad == LoadHigh && t.Config.Output.SummarizeSimilar {
			// Group similar lines and summarize if there are many
			similarGroups := t.SummarizeOutput()
			for _, groupedLines := range similarGroups {
				for _, line := range groupedLines {
					renderedLines = append(renderedLines, t.RenderOutputLine(line))
				}
			}
		} else {
			// Regular output rendering
			for _, line := range t.OutputLines {
				renderedLines = append(renderedLines, t.RenderOutputLine(line))
			}
		}

		// Add all rendered lines with newlines
		sb.WriteString(strings.Join(renderedLines, "\n"))
		sb.WriteString("\n")

		// Empty line for visual separation
		if t.Config.Style.UseBoxes {
			sb.WriteString("│")
			sb.WriteString(strings.Repeat(" ", calculateWidth(t.Label)))
			sb.WriteString("│\n")
		}
	}

	// End line
	sb.WriteString(t.RenderEndLine())

	return sb.String()
}

// SummarizeOutput groups similar output lines for better readability
func (t *Task) SummarizeOutput() [][]OutputLine {
	pm := NewPatternMatcher(t.Config)
	groups := pm.FindSimilarLines(t.OutputLines)

	// Convert groups to a slice for ordering
	var result [][]OutputLine

	// First add error groups
	for key, lines := range groups {
		if strings.HasPrefix(key, TypeError) {
			// If many similar errors, just show a sample
			if len(lines) > t.Config.Output.MaxErrorSamples && t.Config.Output.SummarizeSimilar {
				sampleLines := lines[:t.Config.Output.MaxErrorSamples]
				// Add a summary line
				summaryLine := OutputLine{
					Content:     fmt.Sprintf("... %d similar errors", len(lines)-t.Config.Output.MaxErrorSamples),
					Type:        TypeSummary,
					Timestamp:   time.Now(),
					Indentation: 1,
					Context:     LineContext{CognitiveLoad: t.Context.CognitiveLoad, Importance: 3},
				}
				sampleLines = append(sampleLines, summaryLine)
				result = append(result, sampleLines)
			} else {
				result = append(result, lines)
			}
		}
	}

	// Then add warning groups
	for key, lines := range groups {
		if strings.HasPrefix(key, TypeWarning) {
			if len(lines) > t.Config.Output.MaxErrorSamples && t.Config.Output.SummarizeSimilar {
				sampleLines := lines[:t.Config.Output.MaxErrorSamples]
				summaryLine := OutputLine{
					Content:     fmt.Sprintf("... %d similar warnings", len(lines)-t.Config.Output.MaxErrorSamples),
					Type:        TypeSummary,
					Timestamp:   time.Now(),
					Indentation: 1,
					Context:     LineContext{CognitiveLoad: t.Context.CognitiveLoad, Importance: 3},
				}
				sampleLines = append(sampleLines, summaryLine)
				result = append(result, sampleLines)
			} else {
				result = append(result, lines)
			}
		}
	}

	// Finally add other groups
	for key, lines := range groups {
		if !strings.HasPrefix(key, TypeError) && !strings.HasPrefix(key, TypeWarning) {
			result = append(result, lines)
		}
	}

	return result
}

// Helper functions

// calculateWidth determines the appropriate width for task display
func calculateWidth(label string) int {
	// Base width on label length, with minimum and maximum values
	minWidth := 50
	maxWidth := 80

	width := len(label) + 40 // Add space for formatting

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
