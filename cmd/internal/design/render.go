// cmd/internal/design/render.go
package design

import (
	"fmt"
	"strings"
	"time"
)

// RenderStartLine returns the formatted start line for the task
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	if t.Config.IsMonochrome {
		// Simple monochrome output (for --no-color, --ci)
		// Tests expect: [START] <label>...
		icon := t.Config.GetIcon("Start") // Should be "[START]" from updated GetIcon
		sb.WriteString(fmt.Sprintf("%s %s...", icon, t.Label))
	} else { // Themed output
		headerStyle := t.Config.GetElementStyle("Task_Label_Header")
		startIndicatorStyle := t.Config.GetElementStyle("Task_StartIndicator_Line")

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.TopCornerChar)
			sb.WriteString(t.Config.Border.HeaderChar) // Start of top border line part

			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(labelColor) // Start color for label segment
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m")
			}
			sb.WriteString(" ") // Space before label text
			sb.WriteString(applyTextCase(t.Label, headerStyle.TextCase))
			sb.WriteString(" ") // Space after label text

			// Calculate remaining width for the header line.
			labelRenderedLength := len(t.Label) + 2 // Label + spaces
			// Assuming a max visual width for the colored part of the header to avoid overly long lines
			// This is a heuristic and might need refinement or access to terminal width.
			desiredHeaderContentVisualWidth := 40
			repeatCount := desiredHeaderContentVisualWidth - labelRenderedLength
			if repeatCount < 0 {
				repeatCount = 0
			}

			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, repeatCount))
			sb.WriteString(t.Config.ResetColor()) // Reset color AFTER all potentially colored header parts
			sb.WriteString("\n")

			sb.WriteString(t.Config.Border.VerticalChar + "\n") // Empty line with border
			sb.WriteString(t.Config.Border.VerticalChar + " ")  // Start of process line
			sb.WriteString(t.Config.GetIndentation(1))          // Indent actual start indicator text
		} else { // Line-oriented (non-boxed) themed output
			// For line-oriented, the label might be the "header" itself.
			h2Style := t.Config.GetElementStyle("H2_Target_Title") // Or Task_Label_Header if more appropriate
			labelColor := t.Config.GetColor(h2Style.ColorFG, "H2_Target_Title")
			sb.WriteString(labelColor)
			if contains(h2Style.TextStyle, "bold") {
				sb.WriteString("\033[1m")
			}
			sb.WriteString(h2Style.Prefix) // If any prefix defined for this style
			sb.WriteString(applyTextCase(t.Label, h2Style.TextCase))
			sb.WriteString(t.Config.ResetColor())
			sb.WriteString("\n\n")                     // Spacing after label/title
			sb.WriteString(t.Config.GetIndentation(1)) // Indent start indicator text
		}

		// Process indicator text (common for themed and line-oriented after label)
		processLabelText := getProcessLabel(t.Intent)
		processColor := t.Config.GetColor(startIndicatorStyle.ColorFG, "Task_StartIndicator_Line")
		if processColor == "" { // Fallback if element style doesn't specify color
			processColor = t.Config.GetColor("Process")
		}
		icon := t.Config.GetIcon(startIndicatorStyle.IconKey)
		if icon == "" { // Fallback if element style doesn't specify icon
			icon = t.Config.GetIcon("Start")
		}

		sb.WriteString(fmt.Sprintf("%s %s%s...%s",
			icon,
			processColor,
			processLabelText,
			t.Config.ResetColor()))
	}
	return sb.String()
}

// RenderEndLine returns the formatted end line for the task
func (t *Task) RenderEndLine() string {
	var sb strings.Builder
	durationStr := ""

	// Effective NoTimer state comes from resolved t.Config.Style.NoTimer
	if !t.Config.Style.NoTimer {
		durationStyle := t.Config.GetElementStyle("Task_Status_Duration")
		prefix := durationStyle.Prefix
		suffix := durationStyle.Suffix

		if t.Config.IsMonochrome { // Ensure simple parentheses for monochrome if not styled
			if prefix == "" {
				prefix = "("
			}
			if suffix == "" {
				suffix = ")"
			}
		} else { // For themed, allow theme to specify empty prefix/suffix
			if prefix == "" && suffix == "" { // Add default parens if both are empty for themed
				prefix = "("
				suffix = ")"
			}
		}

		durationColorName := durationStyle.ColorFG
		if durationColorName == "" {
			durationColorName = "Muted"
		} // Default color for duration
		durationAnsiColor := t.Config.GetColor(durationColorName) // Will be "" if monochrome

		space := " "
		if prefix != "" && (strings.HasSuffix(prefix, " ") || strings.HasPrefix(suffix, " ")) {
			space = "" // Avoid double space if prefix/suffix already includes it
		}

		durationStr = fmt.Sprintf("%s%s%s%s%s%s",
			space, durationAnsiColor, prefix, formatDuration(t.Duration), suffix, t.Config.ResetColor())
	}

	if t.Config.IsMonochrome {
		// Simple monochrome output: [ICON] Label (duration) or [ICON] Label
		var icon string
		switch t.Status {
		case StatusSuccess:
			icon = t.Config.GetIcon("Success") // e.g., "[SUCCESS]"
		case StatusWarning:
			icon = t.Config.GetIcon("Warning") // e.g., "[WARNING]"
		case StatusError:
			icon = t.Config.GetIcon("Error") // e.g., "[FAILED]"
		default:
			icon = t.Config.GetIcon("Info") // e.g., "[INFO]"
		}
		// durationStr will be empty if t.Config.Style.NoTimer is true (e.g. CI mode)
		sb.WriteString(fmt.Sprintf("%s %s%s", icon, t.Label, durationStr))

	} else { // Themed output
		var statusStyle ElementStyleDef
		var icon, statusText, colorKey string

		switch t.Status {
		case StatusSuccess:
			statusStyle = t.Config.GetElementStyle("Task_Status_Success_Block")
			icon = t.Config.GetIcon(statusStyle.IconKey)
			if icon == "" {
				icon = t.Config.GetIcon("Success")
			}
			statusText = statusStyle.TextContent
			if statusText == "" {
				statusText = "Complete"
			}
			colorKey = statusStyle.ColorFG
			if colorKey == "" {
				colorKey = "Success"
			}
		case StatusWarning:
			statusStyle = t.Config.GetElementStyle("Task_Status_Warning_Block")
			icon = t.Config.GetIcon(statusStyle.IconKey)
			if icon == "" {
				icon = t.Config.GetIcon("Warning")
			}
			statusText = statusStyle.TextContent
			if statusText == "" {
				statusText = "Completed with warnings"
			}
			colorKey = statusStyle.ColorFG
			if colorKey == "" {
				colorKey = "Warning"
			}
		case StatusError:
			statusStyle = t.Config.GetElementStyle("Task_Status_Failed_Block")
			icon = t.Config.GetIcon(statusStyle.IconKey)
			if icon == "" {
				icon = t.Config.GetIcon("Error")
			}
			statusText = statusStyle.TextContent
			if statusText == "" {
				statusText = "Failed"
			}
			colorKey = statusStyle.ColorFG
			if colorKey == "" {
				colorKey = "Error"
			}
		default: // Should ideally not happen if Status is always set
			statusStyle = t.Config.GetElementStyle("Task_Status_Info_Block") // Assuming an Info_Block
			icon = t.Config.GetIcon(statusStyle.IconKey)
			if icon == "" {
				icon = t.Config.GetIcon("Info")
			}
			statusText = statusStyle.TextContent
			if statusText == "" {
				statusText = "Done"
			}
			colorKey = statusStyle.ColorFG
			if colorKey == "" {
				colorKey = "Process"
			} // Default color
		}

		colorCode := t.Config.GetColor(colorKey)
		statusTextWithStyle := statusText
		if contains(statusStyle.TextStyle, "bold") {
			statusTextWithStyle = "\033[1m" + statusText + "\033[22m" // Specific off-bold
		}

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.VerticalChar + " ")
			sb.WriteString(t.Config.GetIndentation(1)) // Indent status line for boxed themes
		} else if statusStyle.Prefix != "" { // For line-oriented themes that might have a status prefix
			sb.WriteString(statusStyle.Prefix)
		}

		sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
			icon, colorCode, statusTextWithStyle, t.Config.ResetColor(), durationStr))
		sb.WriteString("\n") // Newline after status text

		if t.Config.Style.UseBoxes {
			footerChar := t.Config.Border.FooterContinuationChar
			if footerChar == "" {
				footerChar = t.Config.Border.HeaderChar
			} // Fallback to header char
			if footerChar == "" {
				footerChar = "─"
			} // Absolute fallback
			sb.WriteString(t.Config.Border.BottomCornerChar)
			sb.WriteString(footerChar) // This creates "└─" or similar
			// Could extend the line: strings.Repeat(footerChar, width)
			sb.WriteString("\n")
		} else { // Line-oriented themes might have a final horizontal rule
			footerStyle := t.Config.GetElementStyle("H2_Target_Footer_Line")
			if footerStyle.LineChar != "" {
				sb.WriteString(strings.Repeat(footerStyle.LineChar, calculateHeaderWidth(t.Label, 40))) // Use consistent width
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	// Check for fo's own internal messages that should be rendered plainly
	isFoInternalMessage := strings.HasPrefix(line.Content, "[fo] ") ||
		(line.Type == TypeError && (strings.HasPrefix(line.Content, "Error starting command") ||
			strings.HasPrefix(line.Content, "Error creating stdout pipe") ||
			strings.HasPrefix(line.Content, "Error creating stderr pipe")))

	if t.Config.IsMonochrome {
		sb.WriteString(t.Config.GetIndentation(1)) // Base indent for all content
		if line.Indentation > 0 {                  // Additional script-level indent
			sb.WriteString(strings.Repeat(t.Config.GetIndentation(1), line.Indentation))
		}

		if isFoInternalMessage { // Print fo's own errors/messages plainly after initial indent
			sb.WriteString(line.Content)
		} else { // Regular script output
			var prefixText string
			switch line.Type {
			case TypeError:
				prefixStyle := t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
				prefixText = prefixStyle.Text // e.g., "  > " from AsciiMinimalTheme
			case TypeWarning:
				prefixStyle := t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
				prefixText = prefixStyle.Text // e.g., "  > "
			default: // TypeDetail, TypeInfo, TypeSuccess etc.
				prefixStyle := t.Config.GetElementStyle("Stdout_Line_Prefix")
				prefixText = prefixStyle.Text // e.g., "  "
			}
			sb.WriteString(prefixText)
			sb.WriteString(line.Content)
		}
	} else { // Themed rendering
		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
		sb.WriteString(t.Config.GetIndentation(1))
		if line.Indentation > 0 {
			sb.WriteString(strings.Repeat(t.Config.GetIndentation(1), line.Indentation))
		}

		var prefixStyle ElementStyleDef
		var contentColorKey string
		var contentElementStyleKey string // Key to get style for content text (bold, italic)

		switch line.Type {
		case TypeError:
			if isFoInternalMessage {
				prefixStyle = ElementStyleDef{}                           // No icon/prefix from element style for fo's internal error line
				contentColorKey = "Error"                                 // Just color the text red
				contentElementStyleKey = "Task_Content_Stderr_Error_Text" // For potential bold/italic
			} else {
				prefixStyle = t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
				contentColorKey = prefixStyle.ColorFG
				if contentColorKey == "" {
					contentColorKey = "Error"
				}
				contentElementStyleKey = "Task_Content_Stderr_Error_Text"
			}
		case TypeWarning:
			prefixStyle = t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = "Warning"
			}
			contentElementStyleKey = "Task_Content_Stderr_Warning_Text"
		case TypeInfo:
			prefixStyle = t.Config.GetElementStyle("Make_Info_Line_Prefix") // Or a generic "Info_Line_Prefix"
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = "Process"
			} // Default info color
			contentElementStyleKey = "Task_Content_Info_Text"
		default: // TypeDetail, TypeSuccess, etc.
			prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix") // Default prefix for stdout-like content
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = "Detail"
			}
			contentElementStyleKey = "Task_Content_Stdout_Text"
		}

		// Render prefix part (icon, prefix text from style)
		prefixRenderedColor := t.Config.GetColor(prefixStyle.ColorFG) // Color for the prefix itself
		sb.WriteString(prefixRenderedColor)
		if prefixStyle.IconKey != "" {
			sb.WriteString(t.Config.GetIcon(prefixStyle.IconKey) + " ")
		}
		if prefixStyle.Text != "" {
			sb.WriteString(prefixStyle.Text)
		}
		if prefixStyle.AdditionalChars != "" {
			sb.WriteString(prefixStyle.AdditionalChars)
		}
		if prefixRenderedColor != "" {
			sb.WriteString(t.Config.ResetColor())
		} // Reset color if prefix had one

		// Content styling
		finalContentColor := t.Config.GetColor(contentColorKey) // Color for the content text
		contentStyleDef := t.Config.GetElementStyle(contentElementStyleKey)
		styleStart, styleEnd := "", ""

		// Apply cognitive load italics for critical application errors (not fo's own)
		if line.Context.CognitiveLoad == LoadHigh && line.Type == TypeError && !isFoInternalMessage {
			styleStart += "\033[3m" // Italics
		}
		// Apply text styles from definition
		if contains(contentStyleDef.TextStyle, "bold") {
			styleStart += "\033[1m"
		}
		if contains(contentStyleDef.TextStyle, "italic") && !strings.Contains(styleStart, "\033[3m") {
			styleStart += "\033[3m"
		}
		// ... (add other styles like underline, dim if defined in ElementStyleDef.TextStyle)

		if styleStart != "" {
			styleEnd = t.Config.ResetColor()
		} // Reset if any style was applied

		sb.WriteString(fmt.Sprintf("%s%s%s%s", finalContentColor, styleStart, line.Content, styleEnd))
	}
	return sb.String()
}

// RenderSummary creates a summary section for the output
func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0
	for _, line := range t.OutputLines {
		// Exclude fo's internal/startup errors from the user-facing summary of command output issues.
		isFoInternalError := (line.Type == TypeError &&
			(strings.HasPrefix(line.Content, "Error starting command") ||
				strings.HasPrefix(line.Content, "Error creating stdout pipe") ||
				strings.HasPrefix(line.Content, "Error creating stderr pipe") ||
				strings.HasPrefix(line.Content, "[fo] "))) // Tag for other fo internal messages
		if isFoInternalError {
			continue
		}

		switch line.Type {
		case TypeError:
			errorCount++
		case TypeWarning:
			warningCount++
		}
	}

	if errorCount == 0 && warningCount == 0 {
		return "" // No summary needed if no relevant errors/warnings from the command
	}

	var sb strings.Builder
	summaryHeadingStyle := t.Config.GetElementStyle("Task_Content_Summary_Heading")
	errorItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Error")
	warningItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Warning")

	// Determine base indentation for summary lines (respecting boxing)
	baseIndentForSummaryItems := ""
	if t.Config.Style.UseBoxes {
		sb.WriteString(t.Config.Border.VerticalChar + "\n") // Empty line before summary for boxed
		baseIndentForSummaryItems = t.Config.Border.VerticalChar + " " + t.Config.GetIndentation(1)
	} else {
		sb.WriteString("\n") // Spacing for line themes before summary
		baseIndentForSummaryItems = t.Config.GetIndentation(1)
	}
	sb.WriteString(baseIndentForSummaryItems) // Indent summary heading

	// Heading text and color
	headingText := summaryHeadingStyle.TextContent
	if headingText == "" {
		headingText = "SUMMARY:"
	}
	headingColor := t.Config.GetColor(summaryHeadingStyle.ColorFG, "Task_Content_Summary_Heading")
	if headingColor == "" && !t.Config.IsMonochrome {
		headingColor = t.Config.GetColor("Process")
	} // Default for themed

	hStyleStart, hStyleEnd := "", ""
	if contains(summaryHeadingStyle.TextStyle, "bold") {
		hStyleStart = "\033[1m"
		hStyleEnd = t.Config.GetColor("Reset") // Ensure reset after bold specifically
	}
	sb.WriteString(fmt.Sprintf("%s%s%s%s%s\n", headingColor, hStyleStart, headingText, hStyleEnd, t.Config.ResetColor()))

	// Item indentation is one level deeper than the summary heading's base indent
	itemFurtherIndent := baseIndentForSummaryItems + t.Config.GetIndentation(1)

	if errorCount > 0 {
		sb.WriteString(itemFurtherIndent)
		bulletChar := errorItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
		}
		itemColor := t.Config.GetColor(errorItemStyle.ColorFG, "Task_Content_Summary_Item_Error")
		if itemColor == "" && !t.Config.IsMonochrome {
			itemColor = t.Config.GetColor("Error")
		}
		sb.WriteString(fmt.Sprintf("%s%s %d error%s%s\n", itemColor, bulletChar, errorCount, pluralSuffix(errorCount), t.Config.ResetColor()))
	}

	if warningCount > 0 {
		sb.WriteString(itemFurtherIndent)
		bulletChar := warningItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
		}
		itemColor := t.Config.GetColor(warningItemStyle.ColorFG, "Task_Content_Summary_Item_Warning")
		if itemColor == "" && !t.Config.IsMonochrome {
			itemColor = t.Config.GetColor("Warning")
		}
		sb.WriteString(fmt.Sprintf("%s%s %d warning%s%s\n", itemColor, bulletChar, warningCount, pluralSuffix(warningCount), t.Config.ResetColor()))
	}

	// Spacing after summary items
	if t.Config.Style.UseBoxes {
		sb.WriteString(t.Config.Border.VerticalChar + "\n")
	} else {
		sb.WriteString("\n") // Ensure a blank line after summary for line themes
	}

	return sb.String()
}

// Helper functions (assumed to be mostly correct from previous versions)
func applyTextCase(text, caseType string) string {
	switch strings.ToLower(caseType) {
	case "upper":
		return strings.ToUpper(text)
	case "lower":
		return strings.ToLower(text)
	case "title":
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
	const maxLabelContribution = 30 // Max length of label to consider for width calculation
	effectiveLabelLength := len(label)
	if effectiveLabelLength > maxLabelContribution {
		effectiveLabelLength = maxLabelContribution
	}
	width := effectiveLabelLength + 10 // Base width on label length plus some padding
	if width < defaultWidth {
		width = defaultWidth
	}
	maxWidth := 60 // Absolute max width for header line to prevent very wide outputs
	if width > maxWidth {
		return maxWidth
	}
	return width
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
	// For M:SS.ms format
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

func getProcessLabel(intent string) string {
	if intent == "" {
		return "Running"
	} // Default if intent is empty
	// Capitalize the first letter of the intent
	if len(intent) > 0 {
		return strings.ToUpper(string(intent[0])) + strings.ToLower(intent[1:])
	}
	return "Running" // Fallback
}
