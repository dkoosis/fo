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

	taskLabelForWidth := t.Label // Use the main task label for width calculations
	if taskLabelForWidth == "" { // Fallback if label is empty
		taskLabelForWidth = filepath.Base(t.Command)
	}

	if t.Config.Style.UseBoxes {
		width := calculateWidth(taskLabelForWidth)
		topLabel := strings.ToUpper(taskLabelForWidth)
		lineWidth := width - len(topLabel) - 3 // for "┌─ " and " ┐"
		if lineWidth < 0 {
			lineWidth = 0
		}
		sb.WriteString("┌─ ")
		sb.WriteString(topLabel)
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat("─", lineWidth))
		sb.WriteString("┐\n")

		sb.WriteString("│")
		// The empty line padding should span the calculated width + 1 for each border char
		emptyPadding := width + 1 // Adjusted to be simpler: width of content area + 1 right border
		if emptyPadding < 0 {
			emptyPadding = 0
		}
		sb.WriteString(strings.Repeat(" ", emptyPadding))
		sb.WriteString("│\n")
	}

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	intentLabel := t.formatIntentLabel() // This is "Running", "Building", etc.
	sb.WriteString(fmt.Sprintf("%s %s%s...%s",
		t.Config.Icons.Start,
		t.Config.Colors.Process,
		intentLabel,
		t.Config.Colors.Reset))

	if t.Config.Style.UseBoxes {
		// Calculate padding based on the visible length of the intentLabel line
		// Visible length: icon + space + intentLabel + "..."
		contentLen := len(stripANSI(t.Config.Icons.Start)) + 1 + len(stripANSI(intentLabel)) + 3
		paddingWidth := calculateWidth(taskLabelForWidth) - contentLen
		if paddingWidth < 0 {
			paddingWidth = 0
		}
		sb.WriteString(strings.Repeat(" ", paddingWidth))
		sb.WriteString("│")
	}
	return sb.String()
}

func (t *Task) formatIntentLabel() string {
	if t.Intent == "" {
		return filepath.Base(t.Command) // Default to command name if no intent
	}
	intent := t.Intent
	if len(intent) > 0 {
		// Ensure first letter is capitalized
		intent = strings.ToUpper(string(intent[0])) + intent[1:]
	}
	return intent
}

func (t *Task) RenderEndLine() string {
	var sb strings.Builder
	taskLabelForWidth := t.Label
	if taskLabelForWidth == "" {
		taskLabelForWidth = filepath.Base(t.Command)
	}

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	var icon, color, statusText string
	switch t.Status {
	case StatusSuccess:
		icon, color, statusText = t.Config.Icons.Success, t.Config.Colors.Success, "Complete"
	case StatusWarning:
		icon, color, statusText = t.Config.Icons.Warning, t.Config.Colors.Warning, "Completed with warnings"
	case StatusError:
		icon, color, statusText = t.Config.Icons.Error, t.Config.Colors.Error, "Failed"
	default:
		icon, color, statusText = t.Config.Icons.Info, t.Config.Colors.Process, "Done"
	}

	durationStr := ""
	if !t.Config.Style.NoTimer {
		durationStr = fmt.Sprintf(" (%s)", formatDuration(t.Duration))
	}

	// Strip ANSI for length calculation of the status line content
	statusLineContent := fmt.Sprintf("%s %s%s%s", icon, statusText, durationStr, "") // No reset for length calc
	visibleStatusLineLength := len(stripANSI(statusLineContent))

	sb.WriteString(fmt.Sprintf("%s %s%s%s%s", icon, color, statusText, durationStr, t.Config.Colors.Reset))

	if t.Config.Style.UseBoxes {
		width := calculateWidth(taskLabelForWidth)
		// contentLen should be the visible length of what's printed before padding
		paddingWidth := width - visibleStatusLineLength
		if paddingWidth < 0 {
			paddingWidth = 0
		}
		sb.WriteString(strings.Repeat(" ", paddingWidth))
		sb.WriteString("│")
	}

	if t.Config.Style.UseBoxes {
		width := calculateWidth(taskLabelForWidth)
		bottomLineWidth := width + 1
		if bottomLineWidth < 0 {
			bottomLineWidth = 0
		}
		sb.WriteString("\n└")
		sb.WriteString(strings.Repeat("─", bottomLineWidth))
		sb.WriteString("┘")
	}
	return sb.String()
}

func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder
	taskLabelForWidth := t.Label
	if taskLabelForWidth == "" {
		taskLabelForWidth = filepath.Base(t.Command)
	}

	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ")
	}

	indentLevel := 1
	if line.Indentation > 0 {
		indentLevel = line.Indentation
	}
	indentStr := t.Config.getIndentation(indentLevel)
	sb.WriteString(indentStr)

	timestampStr := ""
	if t.Config.Style.ShowTimestamps {
		elapsed := line.Timestamp.Sub(t.StartTime)
		timestampStr = fmt.Sprintf("[%s] ", formatDuration(elapsed))
		sb.WriteString(timestampStr)
	}

	content := line.Content
	formattedContent := ""

	switch line.Type {
	case TypeError:
		if line.Context.CognitiveLoad == LoadHigh && !t.Config.Accessibility.ScreenReaderFriendly {
			formattedContent = fmt.Sprintf("%s%s%s%s%s", t.Config.Colors.Error, "\033[3m", content, "\033[0m", t.Config.Colors.Reset)
		} else {
			formattedContent = fmt.Sprintf("%s%s%s", t.Config.Colors.Error, content, t.Config.Colors.Reset)
		}
	case TypeWarning:
		formattedContent = fmt.Sprintf("%s%s %s%s", t.Config.Colors.Warning, t.Config.Icons.Warning, content, t.Config.Colors.Reset)
	case TypeSuccess:
		formattedContent = fmt.Sprintf("%s%s%s", t.Config.Colors.Success, content, t.Config.Colors.Reset)
	case TypeInfo:
		formattedContent = fmt.Sprintf("%s%s %s%s", t.Config.Colors.Process, t.Config.Icons.Info, content, t.Config.Colors.Reset)
	case TypeSummary:
		if !t.Config.Accessibility.ScreenReaderFriendly {
			formattedContent = fmt.Sprintf("%s%s%s%s", t.Config.Colors.Process, "\033[1m", content, "\033[0m"+t.Config.Colors.Reset)
		} else {
			formattedContent = fmt.Sprintf("%s%s%s", t.Config.Colors.Process, content, t.Config.Colors.Reset)
		}
	case TypeProgress:
		formattedContent = fmt.Sprintf("%s%s%s", t.Config.Colors.Muted, content, t.Config.Colors.Reset)
	default: // TypeDetail
		formattedContent = fmt.Sprintf("%s%s%s", t.Config.Colors.Detail, content, t.Config.Colors.Reset)
	}
	sb.WriteString(formattedContent)

	if t.Config.Style.UseBoxes {
		// Calculate visible length of the entire printed part of the line so far
		// This includes indentation, timestamp (if any), and the styled content itself.
		// stripANSI is used on formattedContent to get its visible length.
		currentLineVisibleLength := len(indentStr) + len(timestampStr) + len(stripANSI(formattedContent))

		paddingWidth := calculateWidth(taskLabelForWidth) - currentLineVisibleLength
		if paddingWidth < 0 {
			paddingWidth = 0
		}
		sb.WriteString(strings.Repeat(" ", paddingWidth))
		sb.WriteString("│")
	}
	return sb.String()
}

// stripANSI removes ANSI escape codes from a string for length calculation.
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[mKHF]`)
	return re.ReplaceAllString(s, "")
}

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
		return ""
	}

	var sb strings.Builder
	taskLabelForWidth := t.Label
	if taskLabelForWidth == "" {
		taskLabelForWidth = filepath.Base(t.Command)
	}

	boxLinePrefix := ""
	if t.Config.Style.UseBoxes {
		sb.WriteString("│ ") // Initial indent for the SUMMARY line itself if in a box
		boxLinePrefix = "│ "
	}

	summaryHeading := "SUMMARY:"
	if !t.Config.Accessibility.ScreenReaderFriendly {
		sb.WriteString(fmt.Sprintf("%s%s%s%s\n", t.Config.Colors.Process, "\033[1m", summaryHeading, "\033[0m"+t.Config.Colors.Reset))
	} else {
		sb.WriteString(fmt.Sprintf("%s%s%s\n", t.Config.Colors.Process, summaryHeading, t.Config.Colors.Reset))
	}

	itemIndentStr := t.Config.getIndentation(1)
	if errorCount > 0 {
		sb.WriteString(boxLinePrefix) // Prefix for box drawing
		sb.WriteString(itemIndentStr)
		sb.WriteString(fmt.Sprintf("%s• %d error%s detected%s\n", t.Config.Colors.Error, errorCount, pluralSuffix(errorCount), t.Config.Colors.Reset))
	}
	if warningCount > 0 {
		sb.WriteString(boxLinePrefix) // Prefix for box drawing
		sb.WriteString(itemIndentStr)
		sb.WriteString(fmt.Sprintf("%s• %d warning%s present%s\n", t.Config.Colors.Warning, warningCount, pluralSuffix(warningCount), t.Config.Colors.Reset))
	}

	if t.Config.Style.UseBoxes {
		sb.WriteString("│")
		emptyPadding := calculateWidth(taskLabelForWidth) + 1
		if emptyPadding < 0 {
			emptyPadding = 0
		}
		sb.WriteString(strings.Repeat(" ", emptyPadding))
		sb.WriteString("│\n")
	} else if errorCount > 0 || warningCount > 0 { // Only add extra newline if summary was printed and not in box
		sb.WriteString("\n")
	}
	return sb.String()
}

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
	}

	if showDetailedOutput && len(t.OutputLines) > 0 {
		taskLabelForWidth := t.Label
		if taskLabelForWidth == "" {
			taskLabelForWidth = filepath.Base(t.Command)
		}
		if t.Config.Style.UseBoxes {
			sb.WriteString("│")
			emptyPadding := calculateWidth(taskLabelForWidth) + 1
			if emptyPadding < 0 {
				emptyPadding = 0
			}
			sb.WriteString(strings.Repeat(" ", emptyPadding))
			sb.WriteString("│\n")
		}
		if summary := t.RenderSummary(); summary != "" {
			sb.WriteString(summary)
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
			sb.WriteString("\n")
		}
		if t.Config.Style.UseBoxes {
			sb.WriteString("│")
			emptyPadding := calculateWidth(taskLabelForWidth) + 1
			if emptyPadding < 0 {
				emptyPadding = 0
			}
			sb.WriteString(strings.Repeat(" ", emptyPadding))
			sb.WriteString("│\n")
		}
	}
	sb.WriteString(t.RenderEndLine())
	return sb.String()
}

func (t *Task) SummarizeOutputGroups() [][]OutputLine {
	pm := NewPatternMatcher(t.Config)
	groups := pm.FindSimilarLines(t.OutputLines)
	var result [][]OutputLine
	processGroupType := func(targetType string, groupNameForSummary string) {
		for key, lines := range groups {
			if strings.HasPrefix(key, targetType) {
				result = append(result, t.sampleAndSummarizeGroup(lines, groupNameForSummary))
				delete(groups, key)
			}
		}
	}
	processGroupType(TypeError, "errors")
	processGroupType(TypeWarning, "warnings")
	for _, lines := range groups {
		result = append(result, lines)
	}
	return result
}

func (t *Task) sampleAndSummarizeGroup(lines []OutputLine, groupType string) []OutputLine {
	if len(lines) > t.Config.Output.MaxErrorSamples && t.Config.Output.SummarizeSimilar {
		sampleLines := lines[:t.Config.Output.MaxErrorSamples]
		summaryLine := OutputLine{
			Content: fmt.Sprintf("... %d similar %s", len(lines)-t.Config.Output.MaxErrorSamples, groupType),
			Type:    TypeSummary, Timestamp: time.Now(), Indentation: 1,
			Context: LineContext{CognitiveLoad: t.Context.CognitiveLoad, Importance: 3},
		}
		return append(sampleLines, summaryLine)
	}
	return lines
}

func calculateWidth(label string) int {
	// Ensure label is not excessively long for width calculation
	// This helps prevent extreme widths if a very long path becomes a label.
	const maxLabelLengthForWidthCalc = 40
	effectiveLabel := label
	if len(effectiveLabel) > maxLabelLengthForWidthCalc {
		effectiveLabel = effectiveLabel[:maxLabelLengthForWidthCalc] + "..."
	}

	minWidth, maxWidth, basePad := 40, 80, 15 // Adjusted basePad
	width := len(effectiveLabel) + basePad
	if width < minWidth {
		return minWidth
	}
	if width > maxWidth {
		return maxWidth
	}
	return width
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

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
		m := int(d.Minutes())
		s := d.Seconds() - float64(m*60)
		return fmt.Sprintf("%dm%.1fs", m, s)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
