package design

import (
	"fmt"
	"strings"
	"time"
)

// RenderStartLine returns the formatted start line for the task
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	// Get the task header element style
	headerStyle := t.Config.GetElementStyle("Task_Label_Header")
	startStyle := t.Config.GetElementStyle("Task_StartIndicator_Line")

	// Determine the border behavior based on the Style.UseBoxes setting
	if t.Config.Style.UseBoxes {
		// Box-oriented themes (left borders, full box, etc.)
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly:
			// Top corner with header line
			sb.WriteString(t.Config.Border.TopCornerChar)
			sb.WriteString(t.Config.Border.HeaderChar + " ")

			// Apply styling to the label based on header style
			label := applyTextCase(t.Label, headerStyle.TextCase)
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m") // Bold
			}
			sb.WriteString(label)
			sb.WriteString(t.Config.ResetColor()) // Reset

			// Continue the header line
			sb.WriteString(" ")
			headerWidth := calculateHeaderWidth(t.Label, 30) // Default 30 if not specified
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, headerWidth))
			sb.WriteString("\n")

			// Empty line with vertical border
			sb.WriteString(t.Config.Border.VerticalChar + "\n")

			// Start the process line with left border
			sb.WriteString(t.Config.Border.VerticalChar + " ")

		case BorderHeaderBox, BorderFull:
			// Similar implementation for these border styles...
			// This would follow the same pattern but with the specific border layout
			// Implementation would be based on the existing code for these styles
			// ...

		case BorderNone, BorderAscii:
			// Simplified header for non-boxed styles
			label := applyTextCase(t.Label, headerStyle.TextCase)
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m") // Bold
			}
			sb.WriteString(label)
			sb.WriteString(t.Config.ResetColor()) // Reset
			sb.WriteString(":\n\n")
		}
	} else {
		// Line-oriented themes (no boxes)
		h2Style := t.Config.GetElementStyle("H2_Target_Title")
		headerLineStyle := t.Config.GetElementStyle("H2_Target_Header_Line")

		// Header line if specified
		if headerLineStyle.LineChar != "" {
			sb.WriteString(strings.Repeat(headerLineStyle.LineChar, calculateHeaderWidth(t.Label, 40)))
			sb.WriteString("\n")
		}

		// Task title
		labelColor := t.Config.GetColor(h2Style.ColorFG, "H2_Target_Title")
		sb.WriteString(labelColor)
		if contains(h2Style.TextStyle, "bold") {
			sb.WriteString("\033[1m") // Bold
		}
		sb.WriteString(h2Style.Prefix)
		sb.WriteString(applyTextCase(t.Label, h2Style.TextCase))
		sb.WriteString(t.Config.ResetColor())
		sb.WriteString("\n\n")
	}

	// Process indicator
	processLabel := getProcessLabel(t.Intent)
	processColor := t.Config.GetColor("Process", "Task_StartIndicator_Line")
	icon := t.Config.GetIcon("Start")

	// Add the process indicator text
	sb.WriteString(fmt.Sprintf("%s %s%s...%s",
		icon,
		processColor,
		processLabel,
		t.Config.ResetColor()))

	return sb.String()
}

// RenderEndLine returns the formatted end line for the task
func (t *Task) RenderEndLine() string {
	var sb strings.Builder

	// Determine which task status element to use based on the task's status
	var statusStyle ElementStyleDef
	switch t.Status {
	case StatusSuccess:
		statusStyle = t.Config.GetElementStyle("Task_Status_Success_Block")
	case StatusWarning:
		statusStyle = t.Config.GetElementStyle("Task_Status_Warning_Block")
	case StatusError:
		statusStyle = t.Config.GetElementStyle("Task_Status_Failed_Block")
	default:
		statusStyle = t.Config.GetElementStyle("Task_Status_Success_Block") // Default
	}

	// Duration style
	durationStyle := t.Config.GetElementStyle("Task_Status_Duration")

	// Border alignment based on theme style
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
	} else {
		// For line-oriented themes, possibly add some indent
		if statusStyle.Prefix != "" {
			sb.WriteString(statusStyle.Prefix)
		}
	}

	// Status icon
	var icon, statusText string
	var colorKey string

	switch t.Status {
	case StatusSuccess:
		icon = t.Config.GetIcon("Success")
		statusText = statusStyle.TextContent
		if statusText == "" {
			statusText = "Complete"
		}
		colorKey = "Success"
	case StatusWarning:
		icon = t.Config.GetIcon("Warning")
		statusText = statusStyle.TextContent
		if statusText == "" {
			statusText = "Completed with warnings"
		}
		colorKey = "Warning"
	case StatusError:
		icon = t.Config.GetIcon("Error")
		statusText = statusStyle.TextContent
		if statusText == "" {
			statusText = "Failed"
		}
		colorKey = "Error"
	default:
		icon = t.Config.GetIcon("Info")
		statusText = "Done"
		colorKey = "Process"
	}

	// Get color for the status text
	colorCode := t.Config.GetColor(colorKey)

	// Format duration if timer is enabled
	durationStr := ""
	if !t.Config.Style.NoTimer {
		prefix := durationStyle.Prefix
		if prefix == "" {
			prefix = "("
		}
		suffix := durationStyle.Suffix
		if suffix == "" {
			suffix = ")"
		}

		durationColor := t.Config.GetColor(durationStyle.ColorFG, "StatusDurationFG")
		if durationColor == "" {
			durationColor = t.Config.GetColor("Muted")
		}

		durationStr = fmt.Sprintf(" %s%s%s%s%s",
			durationColor,
			prefix,
			formatDuration(t.Duration),
			suffix,
			t.Config.ResetColor())
	}

	// Add status and duration
	sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
		icon, colorCode, statusText, t.Config.ResetColor(), durationStr))

	sb.WriteString("\n")

	// Bottom border based on theme style
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly:
			footerChar := t.Config.Border.FooterContinuationChar
			if footerChar == "" {
				footerChar = "─"
			}
			sb.WriteString(t.Config.Border.BottomCornerChar + footerChar)

		case BorderFull:
			width := calculateWidth(t.Label)
			sb.WriteString(t.Config.Border.BottomCornerChar)
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width))
			sb.WriteString(t.Config.Border.BottomCornerChar)

		case BorderAscii:
			sb.WriteString(strings.Repeat("-", calculateWidth(t.Label)))
		}
	} else {
		// Get the footer line style for line-oriented themes
		footerStyle := t.Config.GetElementStyle("H2_Target_Footer_Line")
		if footerStyle.FramingCharStart != "" || footerStyle.LineChar != "" {
			char := footerStyle.LineChar
			if char == "" {
				char = "-"
			}

			if footerStyle.FramingCharStart != "" {
				sb.WriteString(footerStyle.FramingCharStart)
				sb.WriteString(t.Status) // Simplified - would need more logic for complex formats
				sb.WriteString(footerStyle.FramingCharEnd)
			} else {
				sb.WriteString(strings.Repeat(char, calculateWidth(t.Label)))
			}
		}
	}

	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	// Add left border or line prefix based on style
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
	}

	// Add appropriate indentation
	indentLevel := line.Indentation
	if indentLevel > 0 {
		sb.WriteString(t.Config.GetIndentation(indentLevel))
	}

	// Determine the line prefix and styling based on line type
	var prefixStyle ElementStyleDef
	var contentStyle ElementStyleDef

	switch line.Type {
	case TypeError:
		prefixStyle = t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
		contentStyle = t.Config.GetElementStyle("Task_Content_Stderr_Error_Text")
	case TypeWarning:
		prefixStyle = t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
		contentStyle = t.Config.GetElementStyle("Task_Content_Stderr_Warning_Text")
	case TypeInfo:
		prefixStyle = t.Config.GetElementStyle("Make_Info_Line_Prefix")
	case TypeDetail:
		if t.Config.Style.UseBoxes {
			prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix")
		} else {
			prefixStyle = t.Config.GetElementStyle("Command_Line_Prefix")
		}
	default:
		// For other types, use stdout prefix as default
		prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix")
	}

	// Add prefix if specified
	if prefixStyle.Text != "" {
		sb.WriteString(prefixStyle.Text)
	}

	// Add the icon if specified for this line type
	if prefixStyle.IconKey != "" {
		sb.WriteString(t.Config.GetIcon(prefixStyle.IconKey) + " ")
	}

	// Add any additional chars specified (spaces, symbols, etc.)
	if prefixStyle.AdditionalChars != "" {
		sb.WriteString(prefixStyle.AdditionalChars)
	}

	// Content styling based on line type and cognitive load
	content := line.Content

	// Apply appropriate styling based on line type
	switch line.Type {
	case TypeError:
		colorCode := t.Config.GetColor("Error")

		// Use red italics for errors in high cognitive load as per research
		if line.Context.CognitiveLoad == LoadHigh {
			sb.WriteString(fmt.Sprintf("%s\033[3m%s\033[0m%s", // Italics
				colorCode,
				content,
				t.Config.ResetColor()))
		} else {
			sb.WriteString(fmt.Sprintf("%s%s%s",
				colorCode,
				content,
				t.Config.ResetColor()))
		}

	case TypeWarning:
		colorCode := t.Config.GetColor("Warning")
		sb.WriteString(fmt.Sprintf("%s%s%s",
			colorCode,
			content,
			t.Config.ResetColor()))

	case TypeSuccess:
		colorCode := t.Config.GetColor("Success")
		sb.WriteString(fmt.Sprintf("%s%s%s",
			colorCode,
			content,
			t.Config.ResetColor()))

	case TypeInfo:
		colorCode := t.Config.GetColor("Process")
		sb.WriteString(fmt.Sprintf("%s%s%s",
			colorCode,
			content,
			t.Config.ResetColor()))

	case TypeSummary:
		// Bold formatting for summary
		colorCode := t.Config.GetColor("Process")
		sb.WriteString(fmt.Sprintf("%s\033[1m%s\033[0m%s", // Bold
			colorCode,
			content,
			t.Config.ResetColor()))

	case TypeProgress:
		// Muted for progress
		colorCode := t.Config.GetColor("Muted")
		sb.WriteString(fmt.Sprintf("%s%s%s",
			colorCode,
			content,
			t.Config.ResetColor()))

	default: // TypeDetail
		sb.WriteString(content)
	}

	return sb.String()
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
		return "" // No summary needed
	}

	var sb strings.Builder

	// Get summary style elements
	summaryHeadingStyle := t.Config.GetElementStyle("Task_Content_Summary_Heading")
	errorItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Error")
	warningItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Warning")

	// Add border and spacing based on style
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			// Empty line before summary
			sb.WriteString(t.Config.Border.VerticalChar + "\n")

			// Summary heading with border
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
	} else {
		// Just add a newline for non-boxed themes
		sb.WriteString("\n")
	}

	// Add summary heading with styling
	headingColor := t.Config.GetColor("Process")
	headingText := summaryHeadingStyle.TextContent
	if headingText == "" {
		headingText = "SUMMARY:"
	}

	sb.WriteString(fmt.Sprintf("%s\033[1m%s\033[0m%s\n",
		headingColor,
		headingText,
		t.Config.ResetColor()))

	// Error count
	if errorCount > 0 {
		// Border if applicable
		if t.Config.Style.UseBoxes {
			switch t.Config.Border.TaskStyle {
			case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
				sb.WriteString(t.Config.Border.VerticalChar + " ")
			}
		}

		// Indentation
		sb.WriteString(t.Config.GetIndentation(1))

		// Bullet character
		bulletChar := errorItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
			if bulletChar == "" {
				bulletChar = "•"
			}
		}

		// Error count text with styling
		sb.WriteString(fmt.Sprintf("%s%s %d %s%s%s\n",
			t.Config.GetColor("Error"),
			bulletChar,
			errorCount,
			"error",
			pluralSuffix(errorCount),
			t.Config.ResetColor()))
	}

	// Warning count
	if warningCount > 0 {
		// Border if applicable
		if t.Config.Style.UseBoxes {
			switch t.Config.Border.TaskStyle {
			case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
				sb.WriteString(t.Config.Border.VerticalChar + " ")
			}
		}

		// Indentation
		sb.WriteString(t.Config.GetIndentation(1))

		// Bullet character
		bulletChar := warningItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
			if bulletChar == "" {
				bulletChar = "•"
			}
		}

		// Warning count text with styling
		sb.WriteString(fmt.Sprintf("%s%s %d %s%s%s\n",
			t.Config.GetColor("Warning"),
			bulletChar,
			warningCount,
			"warning",
			pluralSuffix(warningCount),
			t.Config.ResetColor()))
	}

	// Empty line after summary for boxed themes
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + "\n")
		case BorderNone, BorderAscii:
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n") // Always add spacing for non-boxed themes
	}

	return sb.String()
}

// Helper functions

// applyTextCase applies the specified text case transformation to a string
func applyTextCase(text, caseType string) string {
	switch strings.ToLower(caseType) {
	case "upper":
		return strings.ToUpper(text)
	case "lower":
		return strings.ToLower(text)
	case "title":
		return strings.Title(text)
	default:
		return text // No transformation
	}
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// calculateHeaderWidth determines the width for header formatting
func calculateHeaderWidth(label string, defaultWidth int) int {
	width := len(label) + 10 // Add some padding

	if width < defaultWidth {
		return defaultWidth
	}

	// Cap maximum width to avoid excessive lines
	maxWidth := 60
	if width > maxWidth {
		return maxWidth
	}

	return width
}

// calculateWidth is a simplified version that uses default values
func calculateWidth(label string) int {
	return calculateHeaderWidth(label, 30)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	// Implementation kept the same
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

// pluralSuffix returns "s" for counts not equal to 1
func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
