// cmd/internal/design/config.go
package design

import (
	"strings"
)

// BorderStyle defines the type of border to use for task output
type BorderStyle string

const (
	BorderLeftOnly   BorderStyle = "left_only"
	BorderLeftDouble BorderStyle = "left_double"
	BorderHeaderBox  BorderStyle = "header_box"
	BorderFull       BorderStyle = "full_box"
	BorderNone       BorderStyle = "none"  // For line-oriented themes or no task framing
	BorderAscii      BorderStyle = "ascii" // ASCII-only equivalent for boxed tasks
)

// ElementStyleDef defines visual styling properties for a specific UI element
type ElementStyleDef struct {
	// Text content and formatting
	Text        string   `yaml:"text,omitempty"`         // Fixed text content
	Prefix      string   `yaml:"prefix,omitempty"`       // Text before content
	Suffix      string   `yaml:"suffix,omitempty"`       // Text after content
	TextContent string   `yaml:"text_content,omitempty"` // Default content, e.g. "SUCCESS", "FAILED"
	TextCase    string   `yaml:"text_case,omitempty"`    // "upper", "lower", "title", "none"
	TextStyle   []string `yaml:"text_style,omitempty"`   // ["bold", "italic", "underline", "dim"]

	// Colors
	ColorFG string `yaml:"color_fg,omitempty"` // Foreground color name or ANSI code
	ColorBG string `yaml:"color_bg,omitempty"` // Background color name or ANSI code

	// Icons and symbols
	IconKey    string `yaml:"icon_key,omitempty"`    // Key to lookup in Icons map
	BulletChar string `yaml:"bullet_char,omitempty"` // Character for bullet points

	// Line formatting
	LineChar       string `yaml:"line_char,omitempty"`        // Character for horizontal lines
	LineLengthType string `yaml:"line_length_type,omitempty"` // "full_width", "dynamic_to_label", "fixed"

	// Border formatting
	FramingCharStart string `yaml:"framing_char_start,omitempty"` // Start char for framing (e.g. "====[ ")
	FramingCharEnd   string `yaml:"framing_char_end,omitempty"`   // End char for framing (e.g. " ]====")

	// Additional layout controls
	AdditionalChars string `yaml:"additional_chars,omitempty"` // For extra spacing or symbols
	DateTimeFormat  string `yaml:"date_time_format,omitempty"` // For timestamp formatting
}

// Config holds all resolved design system settings for rendering
type Config struct {
	// Theme metadata
	ThemeName    string `yaml:"-"` // Name of the theme this config represents
	IsMonochrome bool   `yaml:"-"` // True if colors should be stripped/ignored

	// General style properties
	Style struct {
		UseBoxes       bool   `yaml:"use_boxes"`       // Master switch for task container (boxed vs line-oriented)
		Indentation    string `yaml:"indentation"`     // Base indent unit (e.g., "  ")
		ShowTimestamps bool   `yaml:"show_timestamps"` // For overall start/end times
		NoTimer        bool   `yaml:"no_timer"`        // For individual task timers
		Density        string `yaml:"density"`         // "compact", "balanced", "relaxed" for spacing
	} `yaml:"style"`

	// Border characters
	Border struct {
		// Task container border style
		TaskStyle              BorderStyle `yaml:"task_style"` // One of the BorderStyle constants
		HeaderChar             string      `yaml:"header_char"`
		VerticalChar           string      `yaml:"vertical_char"`
		TopCornerChar          string      `yaml:"top_corner_char"`
		BottomCornerChar       string      `yaml:"bottom_corner_char"`
		FooterContinuationChar string      `yaml:"footer_continuation_char"` // e.g. "─" in "└─"

		// Table border characters
		Table_HChar     string `yaml:"table_h_char"`
		Table_VChar     string `yaml:"table_v_char"`
		Table_XChar     string `yaml:"table_x_char"` // Cross intersection
		Table_Corner_TL string `yaml:"table_corner_tl"`
		Table_Corner_TR string `yaml:"table_corner_tr"`
		Table_Corner_BL string `yaml:"table_corner_bl"`
		Table_Corner_BR string `yaml:"table_corner_br"`
		Table_T_Down    string `yaml:"table_t_down"`
		Table_T_Up      string `yaml:"table_t_up"`
		Table_T_Left    string `yaml:"table_t_left"`
		Table_T_Right   string `yaml:"table_t_right"`
	} `yaml:"border"`

	// Color palette
	Colors struct {
		Process string `yaml:"process"` // Blue by default
		Success string `yaml:"success"` // Green by default
		Warning string `yaml:"warning"` // Yellow by default
		Error   string `yaml:"error"`   // Red by default
		Detail  string `yaml:"detail"`  // Default text
		Muted   string `yaml:"muted"`   // Dimmed text
		Reset   string `yaml:"reset"`   // Reset all styling
	} `yaml:"colors"`

	// Icon symbols
	Icons struct {
		Start   string `yaml:"start"`   // Process indicator
		Success string `yaml:"success"` // Success indicator
		Warning string `yaml:"warning"` // Warning indicator
		Error   string `yaml:"error"`   // Error indicator
		Info    string `yaml:"info"`    // Information indicator
		Bullet  string `yaml:"bullet"`  // For lists
	} `yaml:"icons"`

	// Element-specific styles
	Elements map[string]ElementStyleDef `yaml:"elements"`

	// Pattern recognition rules (existing fields preserved)
	Patterns struct {
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	} `yaml:"patterns"`

	// Tool-specific configuration (existing fields preserved)
	Tools map[string]*ToolConfig `yaml:"tools"`

	// Cognitive load settings (existing fields preserved)
	CognitiveLoad struct {
		AutoDetect bool                 `yaml:"auto_detect"`
		Default    CognitiveLoadContext `yaml:"default"`
	} `yaml:"cognitive_load"`
}

// DefaultConfig returns a Config with standard values (for backward compatibility)
func DefaultConfig() *Config {
	return UnicodeVibrantTheme()
}

// NoColorConfig returns a monochrome Config for --no-color mode
func NoColorConfig() *Config {
	cfg := AsciiMinimalTheme()
	cfg.IsMonochrome = true
	return cfg
}

// AsciiMinimalTheme creates a theme using only ASCII characters and no colors
func AsciiMinimalTheme() *Config {
	cfg := &Config{
		ThemeName:    "ascii_minimal",
		IsMonochrome: true,
	}

	// Style settings
	cfg.Style.UseBoxes = false // Line-oriented, not boxed
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "compact"

	// Icons (ASCII only)
	cfg.Icons.Start = "[>]"
	cfg.Icons.Success = "[OK]"
	cfg.Icons.Warning = "[!!]"
	cfg.Icons.Error = "[XX]"
	cfg.Icons.Info = "[i]"
	cfg.Icons.Bullet = "*"

	// Colors (empty for monochrome)
	cfg.Colors.Process = ""
	cfg.Colors.Success = ""
	cfg.Colors.Warning = ""
	cfg.Colors.Error = ""
	cfg.Colors.Detail = ""
	cfg.Colors.Muted = ""
	cfg.Colors.Reset = ""

	// Border characters (ASCII only)
	cfg.Border.TaskStyle = BorderNone
	cfg.Border.HeaderChar = "-"
	cfg.Border.VerticalChar = "|"
	cfg.Border.TopCornerChar = "+"
	cfg.Border.BottomCornerChar = "+"
	cfg.Border.FooterContinuationChar = "-"

	// Table borders (ASCII)
	cfg.Border.Table_HChar = "-"
	cfg.Border.Table_VChar = "|"
	cfg.Border.Table_XChar = "+"
	cfg.Border.Table_Corner_TL = "+"
	cfg.Border.Table_Corner_TR = "+"
	cfg.Border.Table_Corner_BL = "+"
	cfg.Border.Table_Corner_BR = "+"
	cfg.Border.Table_T_Down = "+"
	cfg.Border.Table_T_Up = "+"
	cfg.Border.Table_T_Left = "+"
	cfg.Border.Table_T_Right = "+"

	// Initialize Elements map with all known element styles
	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements)

	// Override specific element styles for this theme

	// A. Global banner elements
	cfg.Elements["Fo_Banner_Top"] = ElementStyleDef{
		LineChar:  "=",
		Prefix:    "FO: ",
		TextStyle: []string{"bold"},
	}
	cfg.Elements["Fo_Banner_Bottom"] = ElementStyleDef{
		LineChar:  "=",
		Prefix:    "FO: ",
		TextStyle: []string{"bold"},
	}

	// B. Task block elements (line-oriented style)
	cfg.Elements["H2_Target_Header_Line"] = ElementStyleDef{
		LineChar:       "-",
		LineLengthType: "full_width",
	}
	cfg.Elements["H2_Target_Title"] = ElementStyleDef{
		Prefix:    "TARGET: ",
		TextCase:  "upper",
		TextStyle: []string{"bold"},
	}
	cfg.Elements["H2_Target_Footer_Line"] = ElementStyleDef{
		FramingCharStart: "---- ",
		FramingCharEnd:   " ----",
	}

	// C. Task content line elements
	cfg.Elements["Command_Line_Prefix"] = ElementStyleDef{
		Text: "  -> CMD: ",
	}
	cfg.Elements["Stdout_Line_Prefix"] = ElementStyleDef{
		Text: "    | ",
	}
	cfg.Elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{
		Text: "    ! WARN: ",
	}
	cfg.Elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{
		Text: "    X ERROR: ",
	}
	cfg.Elements["Make_Info_Line_Prefix"] = ElementStyleDef{
		Text: "INFO: ",
	}

	// D. Task status elements
	cfg.Elements["Status_Label_Prefix"] = ElementStyleDef{
		Text: "  STAT: ",
	}
	cfg.Elements["Task_Status_Success_Block"] = ElementStyleDef{
		TextContent: "PASSED",
	}
	cfg.Elements["Task_Status_Failed_Block"] = ElementStyleDef{
		TextContent: "FAILED",
	}
	cfg.Elements["Task_Status_Warning_Block"] = ElementStyleDef{
		TextContent: "WARNINGS",
	}
	cfg.Elements["Task_Status_Duration"] = ElementStyleDef{
		Prefix: "(",
		Suffix: ")",
	}

	// E. Summary elements
	cfg.Elements["Task_Content_Summary_Heading"] = ElementStyleDef{
		TextContent: "SUMMARY:",
		TextStyle:   []string{"bold"},
	}
	cfg.Elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{
		BulletChar: "*",
	}
	cfg.Elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{
		BulletChar: "*",
	}

	// Pattern recognition (for backward compatibility)
	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium

	return cfg
}

// UnicodeVibrantTheme creates a rich theme with Unicode characters and colors
func UnicodeVibrantTheme() *Config {
	cfg := &Config{
		ThemeName:    "unicode_vibrant",
		IsMonochrome: false,
	}

	// Style settings
	cfg.Style.UseBoxes = true // Use box-drawing for tasks
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "balanced"

	// Icons (Unicode/Emoji)
	cfg.Icons.Start = "▶️"
	cfg.Icons.Success = "✅"
	cfg.Icons.Warning = "⚠️"
	cfg.Icons.Error = "❌"
	cfg.Icons.Info = "ℹ️"
	cfg.Icons.Bullet = "•"

	// Colors (ANSI codes)
	cfg.Colors.Process = "\033[0;34m" // Blue
	cfg.Colors.Success = "\033[0;32m" // Green
	cfg.Colors.Warning = "\033[0;33m" // Yellow
	cfg.Colors.Error = "\033[0;31m"   // Red
	cfg.Colors.Detail = "\033[0m"     // Default
	cfg.Colors.Muted = "\033[2m"      // Dim
	cfg.Colors.Reset = "\033[0m"      // Reset

	// Border characters (Unicode)
	cfg.Border.TaskStyle = BorderLeftDouble
	cfg.Border.HeaderChar = "═"
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╒"
	cfg.Border.BottomCornerChar = "└"
	cfg.Border.FooterContinuationChar = "─"

	// Table borders (Unicode)
	cfg.Border.Table_HChar = "─"
	cfg.Border.Table_VChar = "│"
	cfg.Border.Table_XChar = "┼"
	cfg.Border.Table_Corner_TL = "┌"
	cfg.Border.Table_Corner_TR = "┐"
	cfg.Border.Table_Corner_BL = "└"
	cfg.Border.Table_Corner_BR = "┘"
	cfg.Border.Table_T_Down = "┬"
	cfg.Border.Table_T_Up = "┴"
	cfg.Border.Table_T_Left = "├"
	cfg.Border.Table_T_Right = "┤"

	// Initialize Elements map with base styles
	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements)

	// Override specific element styles for this theme

	// A. Global banner elements
	cfg.Elements["Fo_Banner_Top"] = ElementStyleDef{
		LineChar:  "═",
		Prefix:    "FO: ",
		TextStyle: []string{"bold"},
		ColorFG:   "Process",
	}
	cfg.Elements["Fo_Banner_Bottom"] = ElementStyleDef{
		LineChar:  "═",
		Prefix:    "FO: ",
		TextStyle: []string{"bold"},
		ColorFG:   "Process",
	}

	// B. Task block elements (boxed style)
	cfg.Elements["Task_Label_Header"] = ElementStyleDef{
		TextCase:  "upper",
		TextStyle: []string{"bold"},
		ColorFG:   "Process",
	}
	cfg.Elements["Task_StartIndicator_Line"] = ElementStyleDef{
		IconKey: "Start",
		ColorFG: "Process",
	}

	// C. Task content line elements
	cfg.Elements["Stdout_Line_Prefix"] = ElementStyleDef{
		AdditionalChars: "  ",
	}
	cfg.Elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{
		IconKey:         "Warning",
		AdditionalChars: "  ",
	}
	cfg.Elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{
		IconKey:         "Error",
		AdditionalChars: "  ",
	}
	cfg.Elements["Make_Info_Line_Prefix"] = ElementStyleDef{
		IconKey: "Info",
		Text:    " ",
	}

	// Styling for line content
	cfg.Elements["Task_Content_Stderr_Warning_Text"] = ElementStyleDef{
		ColorFG: "Warning",
	}
	cfg.Elements["Task_Content_Stderr_Error_Text"] = ElementStyleDef{
		ColorFG: "Error",
	}

	// D. Task status elements
	cfg.Elements["Task_Status_Success_Block"] = ElementStyleDef{
		IconKey:     "Success",
		TextContent: "Complete",
		ColorFG:     "Success",
	}
	cfg.Elements["Task_Status_Failed_Block"] = ElementStyleDef{
		IconKey:     "Error",
		TextContent: "Failed",
		ColorFG:     "Error",
	}
	cfg.Elements["Task_Status_Warning_Block"] = ElementStyleDef{
		IconKey:     "Warning",
		TextContent: "Completed with warnings",
		ColorFG:     "Warning",
	}
	cfg.Elements["Task_Status_Duration"] = ElementStyleDef{
		Prefix:  "(",
		Suffix:  ")",
		ColorFG: "Muted",
	}

	// E. Summary elements
	cfg.Elements["Task_Content_Summary_Heading"] = ElementStyleDef{
		TextContent: "SUMMARY:",
		TextStyle:   []string{"bold"},
		ColorFG:     "Process",
	}
	cfg.Elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{
		BulletChar: "•",
		ColorFG:    "Error",
	}
	cfg.Elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{
		BulletChar: "•",
		ColorFG:    "Warning",
	}

	// F. Table elements
	cfg.Elements["Table_Header_Cell_Text"] = ElementStyleDef{
		TextStyle: []string{"bold"},
		ColorFG:   "Process",
	}

	// Pattern recognition (for backward compatibility)
	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium

	return cfg
}

// Helper function to initialize the Elements map with base styles
// This ensures every known element has at least an empty definition
func initBaseElementStyles(elements map[string]ElementStyleDef) {
	// A. Global banner elements
	elements["Fo_Banner_Top"] = ElementStyleDef{}
	elements["Fo_Banner_Top_Line_FoProcessing"] = ElementStyleDef{}
	elements["Fo_Timestamp_Start"] = ElementStyleDef{DateTimeFormat: "2006-01-02 15:04:05"}
	elements["Fo_Banner_Bottom"] = ElementStyleDef{}
	elements["Fo_OverallStatus_Success"] = ElementStyleDef{}
	elements["Fo_OverallStatus_Failed"] = ElementStyleDef{}
	elements["Fo_OverallStatus_Warnings"] = ElementStyleDef{}

	// B. Task block elements
	elements["Task_Label_Header"] = ElementStyleDef{}
	elements["Task_StartIndicator_Line"] = ElementStyleDef{}
	elements["H2_Target_Header_Line"] = ElementStyleDef{}
	elements["H2_Target_Title"] = ElementStyleDef{}
	elements["H2_Target_Footer_Line"] = ElementStyleDef{}

	// C. Task content line elements
	elements["Command_Line_Prefix"] = ElementStyleDef{}
	elements["Stdout_Line_Prefix"] = ElementStyleDef{}
	elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{}
	elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{}
	elements["Make_Info_Line_Prefix"] = ElementStyleDef{}
	elements["Task_Content_Stdout_Text"] = ElementStyleDef{}
	elements["Task_Content_Stderr_Warning_Text"] = ElementStyleDef{}
	elements["Task_Content_Stderr_Error_Text"] = ElementStyleDef{}

	// D. Task status elements
	elements["Status_Label_Prefix"] = ElementStyleDef{}
	elements["Task_Status_Success_Block"] = ElementStyleDef{}
	elements["Task_Status_Failed_Block"] = ElementStyleDef{}
	elements["Task_Status_Warning_Block"] = ElementStyleDef{}
	elements["Task_Status_Duration"] = ElementStyleDef{}

	// E. Summary elements
	elements["Task_Content_Summary_Heading"] = ElementStyleDef{}
	elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{}
	elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{}

	// F. Table elements
	elements["Table_Header_Cell_Text"] = ElementStyleDef{}
	elements["Table_Body_Cell_Text"] = ElementStyleDef{}

	// G. Progress indicator elements
	elements["ProgressIndicator_Spinner_Chars"] = ElementStyleDef{}
	elements["ProgressIndicator_Text"] = ElementStyleDef{}
}

// defaultPatterns returns the standard pattern recognition rules
func defaultPatterns() struct {
	Intent map[string][]string `yaml:"intent"`
	Output map[string][]string `yaml:"output"`
} {
	return struct {
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	}{
		Intent: map[string][]string{
			"building":   {"go build", "make", "gcc", "g++"},
			"testing":    {"go test", "pytest", "jest", "jasmine"},
			"linting":    {"golangci-lint", "eslint", "pylint", "flake8"},
			"checking":   {"go vet", "check", "verify"},
			"installing": {"go install", "npm install", "pip install"},
			"formatting": {"go fmt", "prettier", "black", "gofmt"},
		},
		Output: map[string][]string{
			"error": {
				"^Error:", "^ERROR:", "^ERRO[R]?\\[",
				"^E!", "^panic:", "^fatal:", "^Failed",
				"\\[ERROR\\]", "^FAIL\\t",
			},
			"warning": {
				"^Warning:", "^WARNING:", "^WARN\\[",
				"^W!", "^deprecated:", "^\\[warn\\]",
				"\\[WARNING\\]", "^Warn:",
			},
			"success": {
				"^Success:", "^SUCCESS:", "^PASS\\t",
				"^ok\\t", "^Done!", "^Completed",
				"^✓", "^All tests passed!",
			},
			"info": {
				"^Info:", "^INFO:", "^INFO\\[",
				"^I!", "^\\[info\\]", "^Running",
			},
		},
	}
}

// Helper methods for accessing theme elements

// GetElementStyle retrieves the style for a specific element
// Returns an empty ElementStyleDef if not found to avoid nil panics
func (c *Config) GetElementStyle(elementName string) ElementStyleDef {
	if style, ok := c.Elements[elementName]; ok {
		return style
	}

	// Return empty style if not found
	return ElementStyleDef{}
}

// GetIndentation returns the appropriate indentation string
func (c *Config) GetIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(c.Style.Indentation, level)
}

// GetIcon returns the icon for the given key, respecting monochrome mode
func (c *Config) GetIcon(iconKey string) string {
	if c.IsMonochrome {
		// ASCII fallbacks in monochrome mode
		switch iconKey {
		case "Start":
			return "[>]"
		case "Success":
			return "[OK]"
		case "Warning":
			return "[!!]"
		case "Error":
			return "[XX]"
		case "Info":
			return "[i]"
		case "Bullet":
			return "*"
		default:
			return ""
		}
	}

	// Return the configured icon
	switch iconKey {
	case "Start":
		return c.Icons.Start
	case "Success":
		return c.Icons.Success
	case "Warning":
		return c.Icons.Warning
	case "Error":
		return c.Icons.Error
	case "Info":
		return c.Icons.Info
	case "Bullet":
		return c.Icons.Bullet
	default:
		return ""
	}
}

// GetColor returns the color for the given key, empty string in monochrome mode
func (c *Config) GetColor(colorKey string, elementName ...string) string {
	if c.IsMonochrome {
		return "" // No colors in monochrome mode
	}

	// First check if we were given a specific element to check
	if len(elementName) > 0 && elementName[0] != "" {
		elemStyle := c.GetElementStyle(elementName[0])
		if elemStyle.ColorFG != "" {
			// The element has a specific color definition
			return getColorByName(elemStyle.ColorFG, c)
		}
	}

	// Otherwise use the color key directly
	return getColorByName(colorKey, c)
}

// getColorByName resolves a color name to its ANSI code
func getColorByName(name string, c *Config) string {
	switch name {
	case "Process":
		return c.Colors.Process
	case "Success":
		return c.Colors.Success
	case "Warning":
		return c.Colors.Warning
	case "Error":
		return c.Colors.Error
	case "Detail":
		return c.Colors.Detail
	case "Muted":
		return c.Colors.Muted
	default:
		// If the name itself looks like an ANSI code, return it directly
		if strings.HasPrefix(name, "\033[") {
			return name
		}
		return ""
	}
}

// ResetColor returns the ANSI reset code, or empty string in monochrome mode
func (c *Config) ResetColor() string {
	if c.IsMonochrome {
		return ""
	}
	return c.Colors.Reset
}
