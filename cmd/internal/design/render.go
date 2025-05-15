// cmd/internal/design/render.go
package design

import (
	"fmt"
	"strings"
	"time"
)

// RenderStartLine returns the formatted start line for the task
// RenderStartLine returns the formatted start line for the task
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	if t.Config.IsMonochrome {
		// Simple monochrome output (for --no-color, --ci)
		icon := t.Config.GetIcon("Start") // Should be "[START]" from GetIcon's monochrome path
		sb.WriteString(fmt.Sprintf("%s %s...", icon, t.Label))
	} else { // Themed output
		headerStyle := t.Config.GetElementStyle("Task_Label_Header")
		startIndicatorStyle := t.Config.GetElementStyle("Task_StartIndicator_Line")

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.TopCornerChar)
			sb.WriteString(t.Config.Border.HeaderChar)

			// Apply color only to the label text, not the border
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(" ")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString("\033[1m") // ANSI bold
			}
			sb.WriteString(applyTextCase(t.Label, headerStyle.TextCase))
			sb.WriteString(t.Config.ResetColor())
			sb.WriteString(" ")

			// Calculate border length - rendered after color reset
			labelRenderedLength := len(t.Label) + 2
			desiredHeaderContentVisualWidth := 40
			repeatCount := desiredHeaderContentVisualWidth - labelRenderedLength
			if repeatCount < 0 {
				repeatCount = 0
			}

			// Border characters without color
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, repeatCount))
			sb.WriteString("\n")

			sb.WriteString(t.Config.Border.VerticalChar + "\n")
			sb.WriteString(t.Config.Border.VerticalChar + " ")
			sb.WriteString(t.Config.GetIndentation(1))
		} else { // Line-oriented (non-boxed) themed output
			h2Style := t.Config.GetElementStyle("H2_Target_Title")
			labelColor := t.Config.GetColor(h2Style.ColorFG, "H2_Target_Title")
			sb.WriteString(labelColor)
			if contains(h2Style.TextStyle, "bold") {
				sb.WriteString("\033[1m") // ANSI bold
			}
			sb.WriteString(h2Style.Prefix)
			sb.WriteString(applyTextCase(t.Label, h2Style.TextCase))
			sb.WriteString(t.Config.ResetColor())
			sb.WriteString("\n\n")
			sb.WriteString(t.Config.GetIndentation(1))
		}

		processLabelText := getProcessLabel(t.Intent)
		processColor := t.Config.GetColor(startIndicatorStyle.ColorFG, "Task_StartIndicator_Line")
		if processColor == "" {
			processColor = t.Config.GetColor("Process")
		}
		icon := t.Config.GetIcon(startIndicatorStyle.IconKey)
		if icon == "" {
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

	if !t.Config.Style.NoTimer {
		durationStyle := t.Config.GetElementStyle("Task_Status_Duration")
		prefix := durationStyle.Prefix
		suffix := durationStyle.Suffix

		if t.Config.IsMonochrome {
			if prefix == "" {
				prefix = "("
			}
			if suffix == "" {
				suffix = ")"
			}
		} else {
			if prefix == "" && suffix == "" {
				prefix = "("
				suffix = ")"
			}
		}

		durationColorName := durationStyle.ColorFG
		if durationColorName == "" {
			durationColorName = "Muted"
		}
		durationAnsiColor := t.Config.GetColor(durationColorName)

		space := " "
		if prefix != "" && (strings.HasSuffix(prefix, " ") || strings.HasPrefix(suffix, " ")) {
			space = ""
		}

		durationStr = fmt.Sprintf("%s%s%s%s%s%s",
			space, durationAnsiColor, prefix, formatDuration(t.Duration), suffix, t.Config.ResetColor())
	}

	if t.Config.IsMonochrome {
		var icon string
		switch t.Status {
		case StatusSuccess:
			icon = t.Config.GetIcon("Success")
		case StatusWarning:
			icon = t.Config.GetIcon("Warning")
		case StatusError:
			icon = t.Config.GetIcon("Error")
		default:
			icon = t.Config.GetIcon("Info")
		}
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
		default:
			statusStyle = t.Config.GetElementStyle("Task_Status_Info_Block")
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
			}
		}

		colorCode := t.Config.GetColor(colorKey)
		statusTextWithStyle := statusText
		if !t.Config.IsMonochrome && contains(statusStyle.TextStyle, "bold") { // Check IsMonochrome
			statusTextWithStyle = "\033[1m" + statusText + "\033[22m" // ANSI bold on/off
		}

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.VerticalChar + " ")
			sb.WriteString(t.Config.GetIndentation(1))
		} else if statusStyle.Prefix != "" {
			sb.WriteString(statusStyle.Prefix)
		}

		sb.WriteString(fmt.Sprintf("%s %s%s%s%s",
			icon, colorCode, statusTextWithStyle, t.Config.ResetColor(), durationStr))
		sb.WriteString("\n")

		if t.Config.Style.UseBoxes {
			footerChar := t.Config.Border.FooterContinuationChar
			if footerChar == "" {
				footerChar = t.Config.Border.HeaderChar
			}
			if footerChar == "" {
				footerChar = "─"
			}
			sb.WriteString(t.Config.Border.BottomCornerChar)
			sb.WriteString(footerChar)
			sb.WriteString("\n")
		} else {
			footerStyle := t.Config.GetElementStyle("H2_Target_Footer_Line")
			if footerStyle.LineChar != "" {
				sb.WriteString(strings.Repeat(footerStyle.LineChar, calculateHeaderWidth(t.Label, 40)))
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

// RenderOutputLine formats an output line according to the design system
func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	isFoInternalMessage := strings.HasPrefix(line.Content, "[fo] ") ||
		(line.Type == TypeError && (strings.HasPrefix(line.Content, "Error starting command") ||
			strings.HasPrefix(line.Content, "Error creating stdout pipe") ||
			strings.HasPrefix(line.Content, "Error creating stderr pipe")))

	if t.Config.IsMonochrome {
		sb.WriteString(t.Config.GetIndentation(1))
		if line.Indentation > 0 {
			sb.WriteString(strings.Repeat(t.Config.GetIndentation(1), line.Indentation))
		}

		if isFoInternalMessage {
			sb.WriteString(line.Content)
		} else {
			var prefixText string
			switch line.Type {
			case TypeError:
				prefixStyle := t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
				prefixText = prefixStyle.Text
			case TypeWarning:
				prefixStyle := t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
				prefixText = prefixStyle.Text
			default:
				prefixStyle := t.Config.GetElementStyle("Stdout_Line_Prefix")
				prefixText = prefixStyle.Text
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
		var contentElementStyleKey string

		switch line.Type {
		case TypeError:
			if isFoInternalMessage {
				prefixStyle = ElementStyleDef{}
				contentColorKey = "Error"
				contentElementStyleKey = "Task_Content_Stderr_Error_Text"
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
			prefixStyle = t.Config.GetElementStyle("Make_Info_Line_Prefix")
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = "Process"
			}
			contentElementStyleKey = "Task_Content_Info_Text"
		default:
			prefixStyle = t.Config.GetElementStyle("Stdout_Line_Prefix")
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = "Detail"
			}
			contentElementStyleKey = "Task_Content_Stdout_Text"
		}

		prefixRenderedColor := t.Config.GetColor(prefixStyle.ColorFG)
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
		}

		finalContentColor := t.Config.GetColor(contentColorKey)
		contentStyleDef := t.Config.GetElementStyle(contentElementStyleKey)
		styleStart, styleEnd := "", ""

		if !t.Config.IsMonochrome {
			if line.Context.CognitiveLoad == LoadHigh && line.Type == TypeError && !isFoInternalMessage {
				styleStart += "\033[3m"
			}
			if contains(contentStyleDef.TextStyle, "bold") {
				styleStart += "\033[1m"
			}
			if contains(contentStyleDef.TextStyle, "italic") && !strings.Contains(styleStart, "\033[3m") {
				styleStart += "\033[3m"
			}
			if styleStart != "" {
				styleEnd = t.Config.ResetColor()
			}
		}
		sb.WriteString(fmt.Sprintf("%s%s%s%s", finalContentColor, styleStart, line.Content, styleEnd))
	}
	return sb.String()
}

// RenderSummary creates a summary section for the output
func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0
	t.OutputLinesLock()
	for _, line := range t.OutputLines {
		isFoInternalError := (line.Type == TypeError &&
			(strings.HasPrefix(line.Content, "Error starting command") ||
				strings.HasPrefix(line.Content, "Error creating stdout pipe") ||
				strings.HasPrefix(line.Content, "Error creating stderr pipe") ||
				strings.HasPrefix(line.Content, "[fo] ")))
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
	t.OutputLinesUnlock()

	if errorCount == 0 && warningCount == 0 {
		return ""
	}

	var sb strings.Builder
	summaryHeadingStyle := t.Config.GetElementStyle("Task_Content_Summary_Heading")
	errorItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Error")
	warningItemStyle := t.Config.GetElementStyle("Task_Content_Summary_Item_Warning")

	baseIndentForSummaryItems := ""
	if t.Config.Style.UseBoxes && !t.Config.IsMonochrome { // Only box if not monochrome
		sb.WriteString(t.Config.Border.VerticalChar + "\n")
		baseIndentForSummaryItems = t.Config.Border.VerticalChar + " " + t.Config.GetIndentation(1)
	} else {
		sb.WriteString("\n")
		baseIndentForSummaryItems = t.Config.GetIndentation(1)
	}
	sb.WriteString(baseIndentForSummaryItems)

	headingText := summaryHeadingStyle.TextContent
	if headingText == "" {
		headingText = "SUMMARY:"
	}
	headingColor := t.Config.GetColor(summaryHeadingStyle.ColorFG, "Task_Content_Summary_Heading")
	if headingColor == "" && !t.Config.IsMonochrome {
		headingColor = t.Config.GetColor("Process")
	}

	hStyleStart, hStyleEnd := "", ""
	// Apply bold styling for summary heading only if not monochrome
	if !t.Config.IsMonochrome && contains(summaryHeadingStyle.TextStyle, "bold") {
		hStyleStart = "\033[1m" // ANSI bold
		hStyleEnd = t.Config.ResetColor()
	}
	sb.WriteString(fmt.Sprintf("%s%s%s%s%s\n", headingColor, hStyleStart, headingText, hStyleEnd, t.Config.ResetColor()))

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

	if t.Config.Style.UseBoxes && !t.Config.IsMonochrome {
		sb.WriteString(t.Config.Border.VerticalChar + "\n")
	} else {
		sb.WriteString("\n")
	}

	return sb.String()
}

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
	const maxLabelContribution = 30
	effectiveLabelLength := len(label)
	if effectiveLabelLength > maxLabelContribution {
		effectiveLabelLength = maxLabelContribution
	}
	width := effectiveLabelLength + 10
	if width < defaultWidth {
		width = defaultWidth
	}
	maxWidth := 60
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
	}
	if len(intent) > 0 {
		return strings.ToUpper(string(intent[0])) + strings.ToLower(intent[1:])
	}
	return "Running"
}
