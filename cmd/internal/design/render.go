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
	// startStyle := t.Config.GetElementStyle("Task_StartIndicator_Line") // Correctly removed as per previous advice

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
			// Simplified for brevity - ensure these are implemented if used by a theme
			label := applyTextCase(t.Label, headerStyle.TextCase)
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(t.Config.Border.TopCornerChar) // Example
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, calculateHeaderWidth(t.Label, 30)))
			// ... more border chars ...
			sb.WriteString("\n")
			sb.WriteString(t.Config.Border.VerticalChar + " ")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m")
			}
			sb.WriteString(label)
			sb.WriteString(t.Config.ResetColor())
			// ...
			sb.WriteString("\n")
			sb.WriteString(t.Config.Border.VerticalChar + " ")

		case BorderNone, BorderAscii: // These are not box styles typically
			// Simplified header for non-boxed styles
			label := applyTextCase(t.Label, headerStyle.TextCase)
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m") // Bold
			}
			sb.WriteString(label)
			sb.WriteString(t.Config.ResetColor()) // Reset
			sb.WriteString(":\n\n")               // Added colon and newlines for non-boxed title
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
		sb.WriteString("\n\n") // Ensure spacing after title
	}

	// Process indicator
	processLabelText := getProcessLabel(t.Intent)                            // Ensure getProcessLabel is defined
	processColor := t.Config.GetColor("Process", "Task_StartIndicator_Line") // Use specific element if color defined
	icon := t.Config.GetIcon("Start")

	// Add the process indicator text
	sb.WriteString(fmt.Sprintf("%s %s%s...%s",
		icon,
		processColor,
		processLabelText,
		t.Config.ResetColor()))

	return sb.String()
}

// RenderEndLine returns the formatted end line for the task
func (t *Task) RenderEndLine() string {
	var sb strings.Builder

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

	durationStyle := t.Config.GetElementStyle("Task_Status_Duration")

	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
	} else {
		if statusStyle.Prefix != "" { // For line-oriented themes, prefix might be indent or symbol
			sb.WriteString(statusStyle.Prefix)
		}
	}

	var icon, statusText string
	var colorKey string // To fetch from t.Config.Colors directly

	switch t.Status {
	case StatusSuccess:
		icon = t.Config.GetIcon("Success")
		statusText = statusStyle.TextContent // Use text from element style if defined
		if statusText == "" {
			statusText = "Complete"
		}
		colorKey = statusStyle.ColorFG // Prefer color from element style
		if colorKey == "" {
			colorKey = "Success" // Fallback to direct color key
		}
	case StatusWarning:
		icon = t.Config.GetIcon("Warning")
		statusText = statusStyle.TextContent
		if statusText == "" {
			statusText = "Completed with warnings"
		}
		colorKey = statusStyle.ColorFG
		if colorKey == "" {
			colorKey = "Warning"
		}
	case StatusError:
		icon = t.Config.GetIcon("Error")
		statusText = statusStyle.TextContent
		if statusText == "" {
			statusText = "Failed"
		}
		colorKey = statusStyle.ColorFG
		if colorKey == "" {
			colorKey = "Error"
		}
	default:
		icon = t.Config.GetIcon("Info") // Should not happen if status is set
		statusText = "Done"
		colorKey = statusStyle.ColorFG
		if colorKey == "" {
			colorKey = "Process"
		}
	}

	colorCode := t.Config.GetColor(colorKey) // Get resolved color

	durationStr := ""
	if !t.Config.Style.NoTimer {
		prefix := durationStyle.Prefix
		if prefix == "" && !t.Config.IsMonochrome { // Add default parens only if not monochrome (monochrome has its own format)
			prefix = "("
		}
		suffix := durationStyle.Suffix
		if suffix == "" && !t.Config.IsMonochrome {
			suffix = ")"
		}

		durationColorName := durationStyle.ColorFG
		if durationColorName == "" {
			durationColorName = "Muted" // Default duration color
		}
		durationAnsiColor := t.Config.GetColor(durationColorName)

		// Ensure space before duration only if prefix is not already providing it or is empty
		space := " "
		if prefix != "" && strings.HasSuffix(prefix, " ") {
			space = ""
		}

		durationStr = fmt.Sprintf("%s%s%s%s%s%s",
			space, // Ensure space before duration if needed
			durationAnsiColor,
			prefix,
			formatDuration(t.Duration),
			suffix,
			t.Config.ResetColor())
	}

	statusTextWithStyle := statusText
	if contains(statusStyle.TextStyle, "bold") {
		statusTextWithStyle = "\033[1m" + statusText + "\033[22m" // Using 22m for bold off to be specific
	}

	sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
		icon, colorCode, statusTextWithStyle, t.Config.ResetColor(), durationStr))

	sb.WriteString("\n") // Newline after status

	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly:
			footerChar := t.Config.Border.FooterContinuationChar
			if footerChar == "" {
				footerChar = "─" // Default
			}
			sb.WriteString(t.Config.Border.BottomCornerChar)
			sb.WriteString(footerChar) // This creates "└─" or similar
			// Optional: extend footer line further
			// sb.WriteString(strings.Repeat(footerChar, calculateWidth(t.Label)-1))
			sb.WriteString("\n")

		case BorderHeaderBox: // Needs full bottom border
			width := calculateWidth(t.Label) // Or a relevant width
			sb.WriteString(t.Config.Border.BottomCornerChar)
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width)) // Match header char
			// This needs adjustment based on how BorderHeaderBox is defined visually
			// For a box, it might be BottomLeftCorner + HorizontalRepeat + BottomRightCorner
			sb.WriteString(t.Config.Border.BottomCornerChar) // Example placeholder
			sb.WriteString("\n")

		case BorderFull:
			width := calculateHeaderWidth(t.Label, 30)                        // Use consistent width calculation
			sb.WriteString(t.Config.Border.BottomCornerChar)                  // Typically bottom-left
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, width)) // Horizontal line
			// This depends on the full box definition, might need a right corner char if defined
			// For simplicity, assuming BottomCornerChar is the bottom-left, and line extends.
			// If a specific right corner is part of BorderFull theme:
			// sb.WriteString(t.Config.Border.BottomRightCornerChar)
			sb.WriteString("\n")

		case BorderAscii: // ASCII equivalent of a bottom border
			sb.WriteString(strings.Repeat("-", calculateHeaderWidth(t.Label, 30)))
			sb.WriteString("\n")
		case BorderNone:
			// No border for BorderNone
			break
		}
	} else { // Line-oriented themes
		footerStyle := t.Config.GetElementStyle("H2_Target_Footer_Line")
		if footerStyle.FramingCharStart != "" || footerStyle.LineChar != "" {
			char := footerStyle.LineChar
			if char == "" {
				char = "-" // Default line char
			}

			if footerStyle.FramingCharStart != "" { // e.g. "---- FAILED ----"
				finalStatusText := t.Status // Use raw status like "success", "error"
				if t.Status == StatusSuccess {
					finalStatusText = "PASSED"
				}
				if t.Status == StatusError {
					finalStatusText = "FAILED"
				}

				sb.WriteString(footerStyle.FramingCharStart)
				sb.WriteString(strings.ToUpper(finalStatusText)) // Example: make it upper case
				sb.WriteString(footerStyle.FramingCharEnd)
			} else { // Just a line
				sb.WriteString(strings.Repeat(char, calculateHeaderWidth(t.Label, 40)))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
	}

	indentLevel := line.Indentation
	if indentLevel > 0 {
		sb.WriteString(t.Config.GetIndentation(indentLevel))
	}

	var prefixStyle ElementStyleDef
	// var contentStyle ElementStyleDef // REMOVED as it was not used effectively

	switch line.Type {
	case TypeError:
		prefixStyle = t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
		// contentStyle = t.Config.GetElementStyle("Task_Content_Stderr_Error_Text") // REMOVED
	case TypeWarning:
		prefixStyle = t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
		// contentStyle = t.Config.GetElementStyle("Task_Content_Stderr_Warning_Text") // REMOVED
	case TypeInfo:
		prefixStyle = t.Config.GetElementStyle("Make_Info_Line_Prefix")
	case TypeDetail:
		if t.Config.Style.UseBoxes { // Boxed themes might have a stdout prefix
			prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix")
		} else { // Line themes might use a command line prefix
			prefixStyle = t.Config.GetElementStyle("Command_Line_Prefix")
		}
	default: // Includes TypeSuccess, TypeProgress, TypeSummary, etc. or unknown
		// Default prefix usually for regular stdout or unclassified lines
		if t.Config.Style.UseBoxes {
			prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix")
		} else {
			// For line-oriented themes, TypeDetail might not have a prefix by default from here
			// but specific line types like TypeSuccess could define one.
			// Let's assume a generic or no prefix if not explicitly handled.
		}
	}

	if prefixStyle.Text != "" { // Text from prefix definition
		sb.WriteString(prefixStyle.Text)
	}
	if prefixStyle.IconKey != "" {
		sb.WriteString(t.Config.GetIcon(prefixStyle.IconKey) + " ")
	}
	if prefixStyle.AdditionalChars != "" {
		sb.WriteString(prefixStyle.AdditionalChars)
	}

	content := line.Content
	var finalColor, finalStyleStart, finalStyleEnd string

	// Determine color and style based on line type and element definitions
	// This section now directly uses colors and styles based on line.Type
	// rather than attempting to use the 'contentStyle' variable.

	currentElementStyle := ElementStyleDef{} // To store text style for the content itself
	switch line.Type {
	case TypeError:
		finalColor = t.Config.GetColor("Error")
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Stderr_Error_Text")
		if line.Context.CognitiveLoad == LoadHigh { // Specific research-backed rule
			finalStyleStart += "\033[3m" // Italics
		}
	case TypeWarning:
		finalColor = t.Config.GetColor("Warning")
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Stderr_Warning_Text")
	case TypeSuccess:
		finalColor = t.Config.GetColor("Success")
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Stdout_Success_Text") // Example new element
	case TypeInfo:
		finalColor = t.Config.GetColor("Process")                                // Or a specific "Info" color if defined
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Info_Text") // Example new element
	case TypeSummary:
		finalColor = t.Config.GetColor("Process")                                   // Or a specific "Summary" color
		finalStyleStart += "\033[1m"                                                // Bold for summary lines
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Summary_Text") // Example new element
	case TypeProgress:
		finalColor = t.Config.GetColor("Muted")
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Progress_Text") // Example new element
	case TypeDetail:
		finalColor = t.Config.GetColor("Detail") // Default text color
		currentElementStyle = t.Config.GetElementStyle("Task_Content_Stdout_Text")
	default:
		finalColor = t.Config.GetColor("Detail") // Fallback
	}

	// Apply text styles from the fetched element definition for the content
	if contains(currentElementStyle.TextStyle, "bold") && !strings.Contains(finalStyleStart, "\033[1m") {
		finalStyleStart += "\033[1m"
	}
	if contains(currentElementStyle.TextStyle, "italic") && !strings.Contains(finalStyleStart, "\033[3m") {
		finalStyleStart += "\033[3m"
	}
	// Add other styles like underline, dim if needed

	if finalStyleStart != "" {
		finalStyleEnd = t.Config.ResetColor() // Ensure reset if any style was applied
	}

	sb.WriteString(fmt.Sprintf("%s%s%s%s", finalColor, finalStyleStart, content, finalStyleEnd))
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
	summaryHeadingStyle := t.Config.GetElementStyle("Task_Content_Summary_Heading")
	errorItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Error")
	warningItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Warning")

	// Border and spacing
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + "\n") // Empty line before summary
			sb.WriteString(t.Config.Border.VerticalChar + " ")  // Start summary heading line
		}
	} else {
		sb.WriteString("\n") // Spacing for line themes
	}

	// Heading text and color
	headingText := summaryHeadingStyle.TextContent
	if headingText == "" {
		headingText = "SUMMARY:"
	}
	headingColorName := summaryHeadingStyle.ColorFG
	if headingColorName == "" {
		headingColorName = "Process" // Default color for summary heading
	}
	headingColor := t.Config.GetColor(headingColorName)
	headingStyleStart, headingStyleEnd := "", ""
	if contains(summaryHeadingStyle.TextStyle, "bold") {
		headingStyleStart = "\033[1m"
		headingStyleEnd = t.Config.ResetColor() // Reset after bold
	}

	sb.WriteString(fmt.Sprintf("%s%s%s%s%s\n",
		headingColor,
		headingStyleStart,
		headingText,
		headingStyleEnd,
		t.Config.ResetColor())) // Overall reset for the line

	// Error items
	if errorCount > 0 {
		if t.Config.Style.UseBoxes {
			switch t.Config.Border.TaskStyle {
			case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
				sb.WriteString(t.Config.Border.VerticalChar + " ")
			}
		}
		sb.WriteString(t.Config.GetIndentation(1)) // Indent summary items

		bulletChar := errorItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet") // Fallback to theme icon
			if bulletChar == "" {
				bulletChar = "•" // Hardcoded fallback
			}
		}
		itemColorName := errorItemStyle.ColorFG
		if itemColorName == "" {
			itemColorName = "Error"
		}
		itemColor := t.Config.GetColor(itemColorName)

		sb.WriteString(fmt.Sprintf("%s%s %d %s%s%s\n",
			itemColor, bulletChar, errorCount, "error", pluralSuffix(errorCount), t.Config.ResetColor()))
	}

	// Warning items
	if warningCount > 0 {
		if t.Config.Style.UseBoxes {
			switch t.Config.Border.TaskStyle {
			case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
				sb.WriteString(t.Config.Border.VerticalChar + " ")
			}
		}
		sb.WriteString(t.Config.GetIndentation(1))

		bulletChar := warningItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
			if bulletChar == "" {
				bulletChar = "•"
			}
		}
		itemColorName := warningItemStyle.ColorFG
		if itemColorName == "" {
			itemColorName = "Warning"
		}
		itemColor := t.Config.GetColor(itemColorName)

		sb.WriteString(fmt.Sprintf("%s%s %d %s%s%s\n",
			itemColor, bulletChar, warningCount, "warning", pluralSuffix(warningCount), t.Config.ResetColor()))
	}

	// Spacing after summary
	if t.Config.Style.UseBoxes {
		switch t.Config.Border.TaskStyle {
		case BorderLeftDouble, BorderLeftOnly, BorderHeaderBox, BorderFull:
			sb.WriteString(t.Config.Border.VerticalChar + "\n")
		default: // BorderNone, BorderAscii (which are not really UseBoxes true by default)
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n")
	}

	return sb.String()
}

// Helper functions

func applyTextCase(text, caseType string) string {
	switch strings.ToLower(caseType) {
	case "upper":
		return strings.ToUpper(text)
	case "lower":
		return strings.ToLower(text)
	case "title":
		// strings.Title is deprecated. A common alternative:
		words := strings.Fields(text)
		for i, word := range words {
			if len(word) > 0 {
				words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
			}
		}
		return strings.Join(words, " ")
	default:
		return text
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func calculateHeaderWidth(label string, defaultWidth int) int {
	// Ensure label does not cause excessively wide headers if it's very long
	const maxLabelContribution = 40
	effectiveLabelLength := len(label)
	if effectiveLabelLength > maxLabelContribution {
		effectiveLabelLength = maxLabelContribution
	}

	// Base width on label length plus some padding, or use default.
	// This is a bit arbitrary and might need tweaking based on desired visual output.
	width := effectiveLabelLength + 10
	if width < defaultWidth {
		width = defaultWidth
	}

	// Max width constraint
	maxWidth := 80
	if width > maxWidth {
		return maxWidth
	}
	return width
}

func calculateWidth(label string) int {
	return calculateHeaderWidth(label, 30) // Default to 30 for generic width
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond { // Add microsecond precision for very short durations
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds()) // Use integer ms
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds()) // Seconds with one decimal place
	}
	// For longer durations, use a more detailed format like M:SS.ms
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000
	return fmt.Sprintf("%d:%02d.%03ds", minutes, seconds, milliseconds)
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// getProcessLabel is expected to be defined, ensure it is.
func getProcessLabel(intent string) string {
	if intent == "" {
		return "Processing"
	}
	return strings.Title(strings.ToLower(intent))
}
