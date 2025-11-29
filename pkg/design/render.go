// cmd/internal/design/render.go
package design

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
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
// Uses lipgloss for accurate handling of East Asian Wide characters,
// emojis, and other Unicode characters that occupy multiple cells.
// This function is kept for backward compatibility but now uses
// the VisualWidth function from style.go which wraps lipgloss.
func visualWidth(s string) int {
	return VisualWidth(s)
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

// BoxLayout is the single source of truth for box dimensions.
// It centralizes all box-related calculations to eliminate magic numbers
// and ensure consistent rendering across all box functions.
type BoxLayout struct {
	TotalWidth   int            // Total terminal width
	ContentWidth int            // Available width for content (TotalWidth - borders - padding)
	LeftPadding  int            // Left padding inside box
	RightPadding int            // Right padding inside box
	BorderColor  lipgloss.Color // Color for borders
	BorderChars  BorderChars    // Border characters
	BorderStyle  lipgloss.Style // Lip Gloss style for borders
	Config       *Config        // Reference to config for border chars
}

// NewBoxLayout creates a BoxLayout with single-point dimension calculation.
// termWidth is the terminal width. If 0 or negative, defaults to 80.
func (c *Config) NewBoxLayout(termWidth int) *BoxLayout {
	if termWidth <= 0 {
		termWidth = 80 // Default fallback
	}

	leftPad := 2
	rightPad := 1
	borderWidth := 2 // left + right border chars

	contentWidth := termWidth - borderWidth - leftPad - rightPad
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Resolve border characters with proper corner matching
	borderChars := c.ResolveBorderChars()

	// Determine border color based on theme
	borderColor := c.resolveBorderColor()

	// Create Lip Gloss border style
	border := lipgloss.Border{
		Top:         borderChars.Horizontal,
		Bottom:      borderChars.Horizontal,
		Left:        borderChars.Vertical,
		Right:       borderChars.Vertical,
		TopLeft:     borderChars.TopLeft,
		TopRight:    borderChars.TopRight,
		BottomLeft:  borderChars.BottomLeft,
		BottomRight: borderChars.BottomRight,
	}

	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Width(termWidth - 2). // Content width; borders add 2 for total
		PaddingLeft(leftPad).
		PaddingRight(rightPad).
		PaddingTop(0).
		PaddingBottom(0)

	return &BoxLayout{
		TotalWidth:   termWidth,
		ContentWidth: contentWidth,
		LeftPadding:  leftPad,
		RightPadding: rightPad,
		BorderColor:  borderColor,
		BorderChars:  borderChars,
		BorderStyle:  style,
		Config:       c,
	}
}

// ResolveBorderChars returns the appropriate border characters with matching corners.
func (c *Config) ResolveBorderChars() BorderChars {
	topCorner := c.Border.TopCornerChar
	bottomCorner := c.Border.BottomCornerChar
	headerChar := c.Border.HeaderChar
	verticalChar := c.Border.VerticalChar

	// Determine closing corners based on opening corners
	topRight := "┐" // Default single-line
	switch topCorner {
	case "╔": // Double-line square corner
		topRight = "╗"
	case "╒": // Mixed double/single
		topRight = "╕"
	case "╭": // Single-line rounded corner
		topRight = "╮"
	case "┌": // Single-line square corner
		topRight = "┐"
	}

	bottomRight := "┘" // Default single-line
	switch bottomCorner {
	case "╚": // Double-line square corner
		bottomRight = "╝"
	case "╰": // Single-line rounded corner
		bottomRight = "╯"
	case "└": // Single-line square corner
		bottomRight = "┘"
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

// resolveBorderColor returns the appropriate border color based on theme.
func (c *Config) resolveBorderColor() lipgloss.Color {
	if c.IsMonochrome {
		return lipgloss.Color("")
	}
	if c.ThemeName == "orca" {
		return lipgloss.Color("250") // Very pale gray for orca
	}
	return lipgloss.Color("238") // Faint dark gray for others
}

// RenderTopBorder renders the top border line with optional title.
// Uses lipgloss for consistent border rendering.
func (b *BoxLayout) RenderTopBorder(title string) string {
	if !b.Config.Style.UseBoxes {
		return ""
	}

	// Use lipgloss to render top border with optional title
	// Create a style that shows top border with left/right borders for corners
	topBorderStyle := b.BorderStyle.
		BorderTop(true).
		BorderBottom(false).
		BorderLeft(true).  // Need left border for top-left corner
		BorderRight(true). // Need right border for top-right corner
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(0).
		PaddingRight(0).
		Width(b.TotalWidth - 2) // Content width; borders add 2 for total

	if title != "" {
		// Render title line with borders on all sides except bottom
		// Calculate desired header width
		desiredWidth := b.Config.Style.HeaderWidth
		if desiredWidth <= 0 {
			desiredWidth = 40
		}

		// Build title content with padding
		titleWidth := VisualWidth(title)
		remainingWidth := desiredWidth - titleWidth - 1 // -1 for space after title
		if remainingWidth < 0 {
			remainingWidth = 0
		}

		titleContent := " " + title + strings.Repeat(b.BorderChars.Horizontal, remainingWidth)

		// Render with side borders (no top border, as it's rendered separately)
		titleLineStyle := b.BorderStyle.
			BorderTop(false).  // Top border rendered separately above
			BorderBottom(false).
			BorderLeft(true).
			BorderRight(true).
			PaddingTop(0).
			PaddingBottom(0).
			Width(b.TotalWidth - 2)

		// First render the top border line (empty content)
		topBorder := topBorderStyle.Render("")
		// Then render the title line with side borders
		titleLine := titleLineStyle.Render(titleContent)

		// Return both lines joined with newline (no trailing newline - caller handles that)
		return topBorder + "\n" + titleLine
	}

	// No title: just render top border (no trailing newline - caller handles that)
	return topBorderStyle.Render("")
}

// RenderContentLine renders a single content line with proper padding and borders.
// Uses lipgloss for consistent rendering with automatic width handling.
func (b *BoxLayout) RenderContentLine(content string) string {
	if !b.Config.Style.UseBoxes {
		return content
	}

	// Use lipgloss to render content line with side borders
	// BorderStyle already has proper padding and width configured
	contentLineStyle := b.BorderStyle.
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(true).
		BorderRight(true).
		PaddingTop(0).
		PaddingBottom(0)
		// Width and Padding are inherited from BorderStyle

	return contentLineStyle.Render(content)
}

// RenderBottomBorder renders the bottom border line.
// Uses lipgloss for consistent border rendering.
func (b *BoxLayout) RenderBottomBorder() string {
	if !b.Config.Style.UseBoxes {
		return ""
	}

	// Use lipgloss to render bottom border
	// Create a style that shows bottom border with left/right borders for corners
	bottomBorderStyle := b.BorderStyle.
		BorderTop(false).
		BorderBottom(true).
		BorderLeft(true).  // Need left border for bottom-left corner
		BorderRight(true). // Need right border for bottom-right corner
		PaddingTop(0).
		PaddingBottom(0).
		PaddingLeft(0).
		PaddingRight(0).
		Width(b.TotalWidth - 2) // Content width; borders add 2 for total

	return bottomBorderStyle.Render("")
}

// getTerminalWidth returns the terminal width, defaulting to 80 if unavailable.
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80
	}
	return width
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

		if t.Config.Style.UseBoxes {
			// Use BoxLayout for consistent dimension calculations
			box := t.Config.NewBoxLayout(getTerminalWidth())

			// Render top border with label using unified style helper
			labelStyle := t.Config.BuildStyle("Task_Label_Header")
			labelText := applyTextCase(t.Label, headerStyle.TextCase)
			styledLabel := labelStyle.Render(labelText)

			topBorder := box.RenderTopBorder(styledLabel)
			sb.WriteString(topBorder)
			sb.WriteString("\n")
			sb.WriteString(box.Config.Border.VerticalChar + "\n")
			sb.WriteString(box.Config.Border.VerticalChar + " ")
			sb.WriteString(t.Config.GetIndentation(1))
		} else {
			h2Style := t.Config.GetElementStyle("H2_Target_Title")
			labelStyle := t.Config.BuildStyle("H2_Target_Title")
			labelText := h2Style.Prefix + applyTextCase(t.Label, h2Style.TextCase)
			sb.WriteString(labelStyle.Render(labelText))
			sb.WriteString("\n\n")
			sb.WriteString(t.Config.GetIndentation(1))
		}

		processLabelText := getProcessLabel(t.Intent)
		processStyle := t.Config.BuildStyle("Task_StartIndicator_Line", "Process")
		startIndicatorStyle := t.Config.GetElementStyle("Task_StartIndicator_Line")
		icon := t.Config.GetIcon(startIndicatorStyle.IconKey)
		if icon == "" {
			icon = t.Config.GetIcon("Start")
		}

		processText := icon + " " + processLabelText + "..."
		sb.WriteString(processStyle.Render(processText))
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
	durationStyleLipgloss := t.Config.GetStyle(durationColorName)
	space := " "
	if prefix != "" && (strings.HasSuffix(prefix, " ") || strings.HasPrefix(suffix, " ")) {
		space = ""
	}
	durationText := prefix + formatDuration(t.Duration) + suffix
	return space + durationStyleLipgloss.Render(durationText)
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
			styledStatusTextBuilder.WriteString(string(t.Config.GetColor("Bold")))
		}
		styledStatusTextBuilder.WriteString(statusText)
		if !t.Config.IsMonochrome && contains(statusStyle.TextStyle, "bold") {
			styledStatusTextBuilder.WriteString(string(t.Config.ResetColor()))
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
		sb.WriteString(string(colorCode))
		sb.WriteString(statusTextWithStyle)
		sb.WriteString(string(t.Config.ResetColor()))
		sb.WriteString(durationStr)
		sb.WriteString("\n")

		if t.Config.Style.UseBoxes {
			// Use BoxLayout for consistent bottom border rendering
			box := t.Config.NewBoxLayout(getTerminalWidth())
			bottomBorder := box.RenderBottomBorder()
			sb.WriteString(bottomBorder)
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
		// Use BoxLayout for consistent border character access
		var box *BoxLayout
		if t.Config.Style.UseBoxes {
			box = t.Config.NewBoxLayout(getTerminalWidth())
			sb.WriteString(box.Config.Border.VerticalChar + " ")
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

		prefixStyleLipgloss := t.Config.GetStyleFromElement(prefixStyle, "")
		var prefixText strings.Builder
		if prefixStyle.IconKey != "" {
			prefixText.WriteString(t.Config.GetIcon(prefixStyle.IconKey) + " ")
		}
		if prefixStyle.Text != "" {
			prefixText.WriteString(prefixStyle.Text)
		}
		if prefixStyle.AdditionalChars != "" {
			prefixText.WriteString(prefixStyle.AdditionalChars)
		}
		if prefixText.Len() > 0 {
			sb.WriteString(prefixStyleLipgloss.Render(prefixText.String()))
		}

		contentStyleDef := t.Config.GetElementStyle(contentElementStyleKey)

		// Build content style (Phase 2: using lipgloss.Style)
		contentStyle := t.Config.GetStyleFromElement(contentStyleDef, contentColorKey)

		if !t.Config.IsMonochrome {
			// Cognitive-load aware formatting:
			// - High-load error lines get italic emphasis
			// - Low-importance detail lines get muted when task has high cognitive load
			if line.Context.CognitiveLoad == LoadHigh && line.Type == TypeError && !isFoInternalMessage {
				contentStyle = contentStyle.Italic(true)
			} else if t.Context.CognitiveLoad == LoadHigh && line.Type == TypeDetail && line.Context.Importance <= 2 {
				// Dim low-importance details to reduce noise when cognitive load is high
				contentStyle = t.Config.GetStyle("Muted")
			}
		}

		sb.WriteString(contentStyle.Render(line.Content))
	}
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG RenderOutputLine] Output: %q\n", sb.String())
	}
	return sb.String()
}

func (t *Task) RenderSummary() string {
	errorCount, warningCount := 0, 0
	t.ProcessOutputLines(func(lines []OutputLine) {
		for _, line := range lines {
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
	})
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
	headingStyle := t.Config.GetStyleFromElement(summaryHeadingStyle, "Process")
	sb.WriteString(headingStyle.Render(headingText))
	sb.WriteString("\n")

	itemFurtherIndent := baseIndentForSummaryItems + t.Config.GetIndentation(1)
	if errorCount > 0 {
		sb.WriteString(itemFurtherIndent)
		bulletChar := errorItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
		}
		itemStyle := t.Config.GetStyleFromElement(errorItemStyle, "Error")
		itemText := bulletChar + fmt.Sprintf(" %d error%s", errorCount, pluralSuffix(errorCount))
		sb.WriteString(itemStyle.Render(itemText))
		sb.WriteString("\n")
	}
	if warningCount > 0 {
		sb.WriteString(itemFurtherIndent)
		bulletChar := warningItemStyle.BulletChar
		if bulletChar == "" {
			bulletChar = t.Config.GetIcon("Bullet")
		}
		itemStyle := t.Config.GetStyleFromElement(warningItemStyle, "Warning")
		itemText := bulletChar + fmt.Sprintf(" %d warning%s", warningCount, pluralSuffix(warningCount))
		sb.WriteString(itemStyle.Render(itemText))
		sb.WriteString("\n")
	}

	// Add complexity context for high cognitive load tasks
	if t.Context.CognitiveLoad == LoadHigh && t.Context.Complexity >= 4 {
		sb.WriteString(itemFurtherIndent)
		mutedStyle := t.Config.GetStyle("Muted")
		snapshot := t.GetOutputLinesSnapshot()
		lineCount := len(snapshot)
		contextText := fmt.Sprintf("(%d lines - see above for details)", lineCount)
		sb.WriteString(mutedStyle.Render(contextText))
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

// JoinVertical composes multiple rendered strings vertically using lipgloss.
// This is useful for combining multiple patterns or sections into a single output.
// The components are joined with proper alignment and spacing.
//
// Example:
//   summary := renderSummary()
//   testTable := renderTestTable()
//   combined := JoinVertical(summary, testTable)
func JoinVertical(components ...string) string {
	if len(components) == 0 {
		return ""
	}
	if len(components) == 1 {
		return components[0]
	}
	return lipgloss.JoinVertical(lipgloss.Left, components...)
}

// RenderDirectMessage creates the formatted status line.
func RenderDirectMessage(cfg *Config, messageType, customIcon, message string, indentLevel int) string {
	var sb strings.Builder
	var iconToUse string
	var rawFgColor, rawBgColor string
	var finalFgColor, finalBgColor lipgloss.Color
	var finalTextStyle string
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
				styleParts = append(styleParts, string(stylePart))
			}
		}
		finalTextStyle = strings.Join(styleParts, "")
	}

	sb.WriteString(strings.Repeat(cfg.GetIndentation(1), indentLevel))

	// Phase 2: Build lipgloss.Style instead of manual concatenation
	if lowerMessageType != MessageTypeRaw {
		style := lipgloss.NewStyle()
		if finalBgColor != "" {
			style = style.Background(finalBgColor)
		}
		if finalFgColor != "" {
			style = style.Foreground(finalFgColor)
		}
		// Apply text styles
		if strings.Contains(finalTextStyle, string(cfg.GetColor("Bold"))) {
			style = style.Bold(true)
		}
		if strings.Contains(finalTextStyle, string(cfg.GetColor("Italic"))) {
			style = style.Italic(true)
		}

		messageWithIcon := message
		if iconToUse != "" {
			messageWithIcon = iconToUse + " " + message
		}
		sb.WriteString(style.Render(messageWithIcon))
	} else {
		// Raw mode: no styling
		if iconToUse != "" {
			sb.WriteString(iconToUse)
			sb.WriteString(" ")
		}
		sb.WriteString(message)
	}

	sb.WriteString("\n")
	if os.Getenv("FO_DEBUG_RENDER") != "" {
		fmt.Fprintf(os.Stderr,
			"[DEBUG RenderDirectMessage] FG: %q->%q, BG: %q->%q, Style: %q, Msg: %q\n",
			rawFgColor, finalFgColor, rawBgColor, finalBgColor, finalTextStyle, message)
	}
	return sb.String()
}
