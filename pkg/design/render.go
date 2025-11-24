// cmd/internal/design/render.go
package design

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// caserWrapper wraps a cases.Caser to allow pointer storage in sync.Pool.
type caserWrapper struct {
	caser cases.Caser
}

// titleCaserPool provides a pool of cases.Title instances for concurrent use.
// cases.Title is not safe for concurrent use, so we pool instances to avoid
// creating a new one on every call while maintaining thread safety.
var titleCaserPool = sync.Pool{
	New: func() interface{} {
		return &caserWrapper{caser: cases.Title(language.English)}
	},
}

// threadSafeTitle converts a string to title case using a thread-safe pool of casers.
func threadSafeTitle(s string) string {
	wrapper, ok := titleCaserPool.Get().(*caserWrapper)
	if !ok || wrapper == nil {
		// Fallback: create new caser if pool returns unexpected type
		caser := cases.Title(language.English)
		return caser.String(s)
	}
	defer titleCaserPool.Put(wrapper)
	return wrapper.caser.String(s)
}

// safeTitle converts a string to title case safely without panicking.
// This is a simple implementation that capitalizes the first letter of each word
// without relying on external libraries that may panic on certain inputs.
func safeTitle(s string) string {
	if s == "" {
		return s
	}

	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			// Convert first rune to upper case, rest to lower
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			for j := 1; j < len(runes); j++ {
				runes[j] = unicode.ToLower(runes[j])
			}
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// visualWidth returns the display width of a string in terminal cells.
// Uses go-runewidth for accurate handling of East Asian Wide characters,
// emojis, and other Unicode characters that occupy multiple cells.
func visualWidth(s string) int {
	return runewidth.StringWidth(s)
}

// RenderStartLine returns the formatted start line for the task.
func (t *Task) RenderStartLine() string {
	var sb strings.Builder

	if t.Config.IsMonochrome {
		icon := t.Config.GetIcon("Start")
		// Not using fmt.Sprintf to avoid any potential mangling of icon if it had %
		sb.WriteString(icon)
		sb.WriteString(" ")
		sb.WriteString(t.Label)
		sb.WriteString("...")
	} else {
		headerStyle := t.Config.GetElementStyle("Task_Label_Header")
		startIndicatorStyle := t.Config.GetElementStyle("Task_StartIndicator_Line")

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.TopCornerChar)
			sb.WriteString(t.Config.Border.HeaderChar)
			labelColor := t.Config.GetColor(headerStyle.ColorFG, "Task_Label_Header")
			sb.WriteString(" ")
			sb.WriteString(labelColor)
			if contains(headerStyle.TextStyle, "bold") {
				sb.WriteString(t.Config.GetColor("Bold"))
			}
			sb.WriteString(applyTextCase(t.Label, headerStyle.TextCase))
			sb.WriteString(t.Config.ResetColor())
			sb.WriteString(" ")

			labelRenderedLength := visualWidth(t.Label) + 2
			desiredHeaderContentVisualWidth := t.Config.Style.HeaderWidth
			if desiredHeaderContentVisualWidth <= 0 {
				desiredHeaderContentVisualWidth = 40 // Default fallback
			}
			repeatCount := desiredHeaderContentVisualWidth - labelRenderedLength
			if repeatCount < 0 {
				repeatCount = 0
			}
			sb.WriteString(strings.Repeat(t.Config.Border.HeaderChar, repeatCount))
			sb.WriteString("\n")
			sb.WriteString(t.Config.Border.VerticalChar + "\n")
			sb.WriteString(t.Config.Border.VerticalChar + " ")
			sb.WriteString(t.Config.GetIndentation(1))
		} else {
			h2Style := t.Config.GetElementStyle("H2_Target_Title")
			labelColor := t.Config.GetColor(h2Style.ColorFG, "H2_Target_Title")
			sb.WriteString(labelColor)
			if contains(h2Style.TextStyle, "bold") {
				sb.WriteString(t.Config.GetColor("Bold"))
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

		sb.WriteString(icon)
		sb.WriteString(" ")
		sb.WriteString(processColor)
		sb.WriteString(processLabelText)
		sb.WriteString("...")
		sb.WriteString(t.Config.ResetColor())
	}
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG RenderStartLine] Output: %q\n", sb.String())
	}
	return sb.String()
}

// RenderEndLine returns the formatted end line for the task.
// renderDuration returns the formatted duration string for the task.
func (t *Task) renderDuration() string {
	if t.Config.Style.NoTimer {
		return ""
	}

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
	} else if prefix == "" && suffix == "" {
		prefix = "("
		suffix = ")"
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
	var sb strings.Builder
	sb.WriteString(space)
	sb.WriteString(durationAnsiColor)
	sb.WriteString(prefix)
	sb.WriteString(formatDuration(t.Duration))
	sb.WriteString(suffix)
	sb.WriteString(t.Config.ResetColor())
	return sb.String()
}

// statusBlockData holds the computed values for rendering a status block.
type statusBlockData struct {
	style    ElementStyleDef
	icon     string
	text     string
	colorKey string
}

// getStatusBlockData returns the styling data for the current task status.
func (t *Task) getStatusBlockData() statusBlockData {
	var data statusBlockData
	switch t.Status {
	case StatusSuccess:
		data.style = t.Config.GetElementStyle("Task_Status_Success_Block")
		data.icon = t.Config.GetIcon(data.style.IconKey)
		if data.icon == "" {
			data.icon = t.Config.GetIcon("Success")
		}
		data.text = data.style.TextContent
		if data.text == "" {
			data.text = "Complete"
		}
		data.colorKey = data.style.ColorFG
		if data.colorKey == "" {
			data.colorKey = ColorKeySuccess
		}
	case StatusWarning:
		data.style = t.Config.GetElementStyle("Task_Status_Warning_Block")
		data.icon = t.Config.GetIcon(data.style.IconKey)
		if data.icon == "" {
			data.icon = t.Config.GetIcon("Warning")
		}
		data.text = data.style.TextContent
		if data.text == "" {
			data.text = "Completed with warnings"
		}
		data.colorKey = data.style.ColorFG
		if data.colorKey == "" {
			data.colorKey = ColorKeyWarning
		}
	case StatusError:
		data.style = t.Config.GetElementStyle("Task_Status_Failed_Block")
		data.icon = t.Config.GetIcon(data.style.IconKey)
		if data.icon == "" {
			data.icon = t.Config.GetIcon("Error")
		}
		data.text = data.style.TextContent
		if data.text == "" {
			data.text = "Failed"
		}
		data.colorKey = data.style.ColorFG
		if data.colorKey == "" {
			data.colorKey = ColorKeyError
		}
	default:
		data.style = t.Config.GetElementStyle("Task_Status_Info_Block")
		data.icon = t.Config.GetIcon(data.style.IconKey)
		if data.icon == "" {
			data.icon = t.Config.GetIcon("Info")
		}
		data.text = data.style.TextContent
		if data.text == "" {
			data.text = "Done"
		}
		data.colorKey = data.style.ColorFG
		if data.colorKey == "" {
			data.colorKey = ColorKeyProcess
		}
	}
	return data
}

// RenderEndLine returns the formatted end line for the task.
func (t *Task) RenderEndLine() string {
	var sb strings.Builder
	durationStr := t.renderDuration()

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
		sb.WriteString(icon)
		sb.WriteString(" ")
		sb.WriteString(t.Label)
		sb.WriteString(durationStr)
	} else {
		data := t.getStatusBlockData()
		statusStyle := data.style
		icon := data.icon
		statusText := data.text
		colorCode := t.Config.GetColor(data.colorKey)

		var styledStatusTextBuilder strings.Builder
		if !t.Config.IsMonochrome && contains(statusStyle.TextStyle, "bold") {
			styledStatusTextBuilder.WriteString(t.Config.GetColor("Bold"))
		}
		styledStatusTextBuilder.WriteString(statusText)
		if !t.Config.IsMonochrome && contains(statusStyle.TextStyle, "bold") {
			styledStatusTextBuilder.WriteString(t.Config.ResetColor())
		}
		statusTextWithStyle := styledStatusTextBuilder.String()

		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
		if t.Config.Style.UseBoxes || statusStyle.Prefix == "" {
			sb.WriteString(t.Config.GetIndentation(1))
		}
		if statusStyle.Prefix != "" && !t.Config.Style.UseBoxes {
			sb.WriteString(statusStyle.Prefix)
		}

		sb.WriteString(icon)
		sb.WriteString(" ")
		sb.WriteString(colorCode)
		sb.WriteString(statusTextWithStyle)
		sb.WriteString(t.Config.ResetColor())
		sb.WriteString(durationStr)
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
				headerWidth := t.Config.Style.HeaderWidth
				if headerWidth <= 0 {
					headerWidth = 40
				}
				sb.WriteString(strings.Repeat(footerStyle.LineChar, calculateHeaderWidth(t.Label, headerWidth)))
				sb.WriteString("\n")
			}
		}
	}
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG RenderEndLine] Output: %q\n", sb.String())
	}
	return sb.String()
}

func (t *Task) RenderOutputLine(line OutputLine) string {
	var sb strings.Builder

	// Use the IsInternal flag from context to identify fo-generated errors
	isFoInternalMessage := line.Context.IsInternal

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
				prefixText = t.Config.GetElementStyle("Stderr_Error_Line_Prefix").Text
			case TypeWarning:
				prefixText = t.Config.GetElementStyle("Stderr_Warning_Line_Prefix").Text
			default:
				prefixText = t.Config.GetElementStyle("Stdout_Line_Prefix").Text
			}
			if prefixText == "" && (line.Type == TypeError || line.Type == TypeWarning) {
				prefixText = "  > "
			}
			if prefixText == "" && line.Type == TypeDetail {
				prefixText = "  "
			}
			sb.WriteString(prefixText)
			sb.WriteString(line.Content)
		}
	} else {
		if t.Config.Style.UseBoxes {
			sb.WriteString(t.Config.Border.VerticalChar + " ")
		}
		sb.WriteString(t.Config.GetIndentation(1))
		if line.Indentation > 0 {
			sb.WriteString(strings.Repeat(t.Config.GetIndentation(1), line.Indentation))
		}

		var prefixStyle ElementStyleDef
		var contentColorKey, contentElementStyleKey string
		switch line.Type {
		case TypeError:
			if isFoInternalMessage {
				prefixStyle = ElementStyleDef{}
				contentColorKey = ColorKeyError
				contentElementStyleKey = "Task_Content_Stderr_Error_Text"
			} else {
				prefixStyle = t.Config.GetElementStyle("Stderr_Error_Line_Prefix")
				contentColorKey = prefixStyle.ColorFG
				if contentColorKey == "" {
					contentColorKey = ColorKeyError
				}
				contentElementStyleKey = "Task_Content_Stderr_Error_Text"
			}
		case TypeWarning:
			prefixStyle = t.Config.GetElementStyle("Stderr_Warning_Line_Prefix")
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = ColorKeyWarning
			}
			contentElementStyleKey = "Task_Content_Stderr_Warning_Text"
		case TypeInfo:
			prefixStyle = t.Config.GetElementStyle("Make_Info_Line_Prefix")
			contentColorKey = prefixStyle.ColorFG
			if contentColorKey == "" {
				contentColorKey = ColorKeyProcess
			}
			contentElementStyleKey = "Task_Content_Info_Text"
		default: // TypeDetail
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

		var styleBuilder strings.Builder
		if !t.Config.IsMonochrome {
			// Cognitive-load aware formatting:
			// - High-load error lines get italic emphasis
			// - Low-importance detail lines get muted when task has high cognitive load
			if line.Context.CognitiveLoad == LoadHigh && line.Type == TypeError && !isFoInternalMessage {
				styleBuilder.WriteString(t.Config.GetColor("Italic"))
			} else if t.Context.CognitiveLoad == LoadHigh && line.Type == TypeDetail && line.Context.Importance <= 2 {
				// Dim low-importance details to reduce noise when cognitive load is high
				finalContentColor = t.Config.GetColor("Muted")
			}
			if contains(contentStyleDef.TextStyle, "bold") {
				styleBuilder.WriteString(t.Config.GetColor("Bold"))
			}
			if contains(contentStyleDef.TextStyle, "italic") && !strings.Contains(styleBuilder.String(), t.Config.GetColor("Italic")) {
				styleBuilder.WriteString(t.Config.GetColor("Italic"))
			}
		}

		sb.WriteString(finalContentColor)
		sb.WriteString(styleBuilder.String())
		sb.WriteString(line.Content)
		if styleBuilder.Len() > 0 || finalContentColor != "" {
			sb.WriteString(t.Config.ResetColor())
		}
	}
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG RenderOutputLine] Output: %q\n", sb.String())
	}
	return sb.String()
}

func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0
	t.OutputLinesLock()
	for _, line := range t.OutputLines {
		// Skip internal fo errors - use IsInternal flag only for clean encapsulation
		if line.Context.IsInternal {
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
	var baseIndentForSummaryItems string
	if t.Config.Style.UseBoxes && !t.Config.IsMonochrome {
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

	var headingStyleBuilder strings.Builder
	if !t.Config.IsMonochrome && contains(summaryHeadingStyle.TextStyle, "bold") {
		headingStyleBuilder.WriteString(t.Config.GetColor("Bold"))
	}

	sb.WriteString(headingColor)
	sb.WriteString(headingStyleBuilder.String())
	sb.WriteString(headingText)
	if headingStyleBuilder.Len() > 0 || headingColor != "" {
		sb.WriteString(t.Config.ResetColor())
	}
	sb.WriteString("\n")

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
		sb.WriteString(itemColor)
		sb.WriteString(bulletChar)
		sb.WriteString(fmt.Sprintf(" %d error%s", errorCount, pluralSuffix(errorCount)))
		sb.WriteString(t.Config.ResetColor())
		sb.WriteString("\n")
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
		sb.WriteString(itemColor)
		sb.WriteString(bulletChar)
		sb.WriteString(fmt.Sprintf(" %d warning%s", warningCount, pluralSuffix(warningCount)))
		sb.WriteString(t.Config.ResetColor())
		sb.WriteString("\n")
	}

	// Add complexity context for high cognitive load tasks
	if t.Context.CognitiveLoad == LoadHigh && t.Context.Complexity >= 4 {
		sb.WriteString(itemFurtherIndent)
		mutedColor := t.Config.GetColor("Muted")
		sb.WriteString(mutedColor)
		t.OutputLinesLock()
		lineCount := len(t.OutputLines)
		t.OutputLinesUnlock()
		sb.WriteString(fmt.Sprintf("(%d lines - see above for details)", lineCount))
		sb.WriteString(t.Config.ResetColor())
		sb.WriteString("\n")
	}

	if t.Config.Style.UseBoxes && !t.Config.IsMonochrome {
		sb.WriteString(t.Config.Border.VerticalChar + "\n")
	} else {
		sb.WriteString("\n")
	}
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG RenderSummary] Output: %q\n", sb.String())
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
		return safeTitle(text)
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
	effectiveLabelLength := visualWidth(label)
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

func formatDuration(duration time.Duration) string {
	if duration < time.Millisecond {
		return fmt.Sprintf("%dµs", duration.Microseconds())
	}
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	if duration < time.Minute {
		return fmt.Sprintf("%.1fs", duration.Seconds())
	}
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	milliseconds := int(duration.Milliseconds()) % 1000
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
	return strings.ToUpper(string(intent[0])) + strings.ToLower(intent[1:])
}

// RenderDirectMessage creates the formatted status line.
func RenderDirectMessage(cfg *Config, messageType, customIcon, message string, indentLevel int) string {
	var sb strings.Builder
	var iconToUse string
	var rawFgColor, rawBgColor string
	var finalFgColor, finalBgColor, finalTextStyle string
	var elementStyle ElementStyleDef

	lowerMessageType := strings.ToLower(messageType)
	styleKey := ""

	// Determine the styleKey based on messageType to fetch ElementStyleDef
	switch lowerMessageType {
	case MessageTypeH1, MessageTypeH2, MessageTypeH3, StatusSuccess, StatusWarning, StatusError, TypeInfo:
		// Direct mapping from type to element style key (properly capitalized)
		styleKey = threadSafeTitle(lowerMessageType)
	case MessageTypeHeader: // Legacy support for MessageTypeHeader type
		styleKey = "H1"
	}

	if styleKey != "" {
		elementStyle = cfg.GetElementStyle(styleKey)
	}

	// Determine Icon
	switch {
	case customIcon != "":
		iconToUse = customIcon
	case elementStyle.IconKey != "":
		iconToUse = cfg.GetIcon(elementStyle.IconKey)
	default:
		// Fallback icon logic based on message type
		// Note: MessageType* constants are aliases with same values as Status* and Type* constants
		switch lowerMessageType {
		case MessageTypeH1, MessageTypeHeader:
			iconToUse = cfg.GetIcon("Start")
		case MessageTypeH2:
			iconToUse = cfg.GetIcon("Info")
		case MessageTypeH3:
			iconToUse = cfg.GetIcon("Bullet")
		case StatusSuccess: // Also matches MessageTypeSuccess (same value: "success")
			iconToUse = cfg.GetIcon("Success")
		case StatusWarning: // Also matches MessageTypeWarning (same value: "warning")
			iconToUse = cfg.GetIcon("Warning")
		case StatusError: // Also matches MessageTypeError (same value: "error")
			iconToUse = cfg.GetIcon("Error")
		case TypeInfo: // Also matches MessageTypeInfo (same value: "info")
			iconToUse = cfg.GetIcon("Info")
		default:
			iconToUse = cfg.GetIcon("Info")
		}
	}

	// Determine colors from ElementStyleDef
	if elementStyle.ColorFG != "" {
		rawFgColor = elementStyle.ColorFG
	} else {
		// Fallback color logic based on message type
		// Note: MessageType* constants are aliases with same values as Status* and Type* constants
		switch lowerMessageType {
		case MessageTypeH1, MessageTypeH2, MessageTypeHeader:
			rawFgColor = "Process"
		case StatusSuccess: // Also matches MessageTypeSuccess (same value: "success")
			rawFgColor = "Success"
		case StatusWarning: // Also matches MessageTypeWarning (same value: "warning")
			rawFgColor = "Warning"
		case StatusError: // Also matches MessageTypeError (same value: "error")
			rawFgColor = "Error"
		case TypeInfo: // Also matches MessageTypeInfo (same value: "info")
			rawFgColor = "Process"
		default:
			rawFgColor = "Detail"
		}
	}

	// BG Color from ElementStyleDef
	if elementStyle.ColorBG != "" {
		rawBgColor = elementStyle.ColorBG
	}

	if lowerMessageType == MessageTypeRaw {
		rawFgColor = ""
		rawBgColor = ""
	}

	// Resolve raw color names/keys to actual ANSI codes
	if rawFgColor != "" {
		finalFgColor = cfg.GetColor(rawFgColor)
	}
	if rawBgColor != "" {
		finalBgColor = cfg.GetColor(rawBgColor)
	}

	if !cfg.IsMonochrome && elementStyle.TextStyle != nil {
		var styleParts []string
		for _, styleName := range elementStyle.TextStyle {
			stylePart := cfg.GetColor(threadSafeTitle(strings.ToLower(styleName)))
			if stylePart != "" {
				styleParts = append(styleParts, stylePart)
			}
		}
		finalTextStyle = strings.Join(styleParts, "")
	}

	sb.WriteString(strings.Repeat(cfg.GetIndentation(1), indentLevel))

	needsReset := false
	if lowerMessageType != MessageTypeRaw {
		// Apply all styling at once to ensure proper ordering
		// Background must come before foreground for proper display
		fullStyle := ""
		if finalBgColor != "" {
			fullStyle += finalBgColor
			needsReset = true
		}
		if finalFgColor != "" {
			fullStyle += finalFgColor
			needsReset = true
		}
		if finalTextStyle != "" {
			fullStyle += finalTextStyle
			needsReset = true
		}

		if fullStyle != "" {
			sb.WriteString(fullStyle)
		}

		if iconToUse != "" {
			sb.WriteString(iconToUse)
			sb.WriteString(" ")
		}
	}

	sb.WriteString(message)

	if needsReset {
		sb.WriteString(cfg.ResetColor())
	}

	sb.WriteString("\n")
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr,
			"[DEBUG RenderDirectMessage] FG: %q->%q, BG: %q->%q, Style: %q, Msg: %q\n",
			rawFgColor, finalFgColor, rawBgColor, finalBgColor, finalTextStyle, message)
	}
	return sb.String()
}
