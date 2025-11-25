// cmd/internal/design/config.go
package design

import (
	"encoding/json"
	"fmt"
	"os" // For debug prints to stderr
	"reflect"
	"strings"
)

// ToolConfig defines specific settings for a command/tool preset for design purposes.
type ToolConfig struct {
	Label          string              `yaml:"label,omitempty"`
	Intent         string              `yaml:"intent,omitempty"`
	OutputPatterns map[string][]string `yaml:"output_patterns,omitempty"`
}

// BorderStyle defines the type of border to use for task output.
type BorderStyle string

const (
	BorderLeftOnly   BorderStyle = "left_only"
	BorderLeftDouble BorderStyle = "left_double"
	BorderHeaderBox  BorderStyle = "header_box"
	BorderFull       BorderStyle = "full_box"
	BorderNone       BorderStyle = "none"
	BorderASCII      BorderStyle = "ascii"
)

// Icon constants for monochrome/ASCII mode.
const (
	IconStart   = "[START]"
	IconSuccess = "[SUCCESS]"
	IconWarning = "[WARNING]"
	IconFailed  = "[FAILED]"
	IconInfo    = "[INFO]"
	IconBullet  = "*"
)

// Color key constants for theme color lookups.
const (
	ColorKeyProcess = "Process"
	ColorKeySuccess = "Success"
	ColorKeyWarning = "Warning"
	ColorKeyError   = "Error"
	ColorKeyMuted   = "Muted"
)

// Color name constants for string comparisons (lowercase).
const (
	colorNameProcess = "process"
	colorNameSuccess = "success"
	colorNameWarning = "warning"
	colorNameError   = "error"
	colorNameDetail  = "detail"
	colorNameMuted   = "muted"
	colorNameReset   = "reset"
	colorNameWhite   = "white"
	colorNameGreenFg = "greenfg"
	colorNameBlueFg  = "bluefg"
	colorNameBlueBg  = "bluebg"
	colorNameBold    = "bold"
	colorNameItalic  = "italic"
)

// Default spinner characters for ASCII mode.
const DefaultSpinnerChars = "-\\|/"

// ANSIReset is the escape code to reset all styling.
const ANSIReset = "\033[0m"

// Color wraps an ANSI escape code for safer and more readable color handling.
// Use Sprint to wrap text in color codes, which automatically resets styling.
type Color struct {
	code string
}

// NewColor creates a Color from an ANSI escape code.
func NewColor(code string) Color {
	return Color{code: code}
}

// Sprint wraps text in this color and automatically resets styling.
func (c Color) Sprint(s string) string {
	if c.code == "" {
		return s
	}
	return c.code + s + ANSIReset
}

// Code returns the raw ANSI escape code for manual use.
func (c Color) Code() string {
	return c.code
}

// IsEmpty returns true if this color has no escape code.
func (c Color) IsEmpty() bool {
	return c.code == ""
}

// MessageType constant for legacy support.
const MessageTypeHeader = "header"

// ANSI color code constant.
const ANSIBrightWhite = "\033[0;97m"

// ElementStyleDef defines visual styling properties for a specific UI element.
type ElementStyleDef struct {
	Text             string   `yaml:"text,omitempty"`
	Prefix           string   `yaml:"prefix,omitempty"`
	Suffix           string   `yaml:"suffix,omitempty"`
	TextContent      string   `yaml:"text_content,omitempty"`
	TextCase         string   `yaml:"text_case,omitempty"`
	TextStyle        []string `yaml:"text_style,omitempty"`
	ColorFG          string   `yaml:"color_fg,omitempty"`
	ColorBG          string   `yaml:"color_bg,omitempty"`
	IconKey          string   `yaml:"icon_key,omitempty"`
	BulletChar       string   `yaml:"bullet_char,omitempty"`
	LineChar         string   `yaml:"line_char,omitempty"`
	LineLengthType   string   `yaml:"line_length_type,omitempty"`
	FramingCharStart string   `yaml:"framing_char_start,omitempty"`
	FramingCharEnd   string   `yaml:"framing_char_end,omitempty"`
	AdditionalChars  string   `yaml:"additional_chars,omitempty"`
	DateTimeFormat   string   `yaml:"date_time_format,omitempty"`
}

// DesignTokens centralizes all design values with semantic naming.
// This provides a single source of truth for design values and enables
// extensibility without code changes.
type DesignTokens struct {
	Colors struct {
		// Status colors
		Process string `yaml:"process"` // Primary process/task color
		Success string `yaml:"success"` // Success state
		Warning string `yaml:"warning"` // Warning state
		Error   string `yaml:"error"`   // Error state
		Detail  string `yaml:"detail"`  // Detail text
		Muted   string `yaml:"muted"`   // Muted/secondary text
		Reset   string `yaml:"reset"`   // Reset/clear formatting

		// Base colors
		White   string `yaml:"white,omitempty"`
		GreenFg string `yaml:"green_fg,omitempty"`
		BlueFg  string `yaml:"blue_fg,omitempty"`
		BlueBg  string `yaml:"blue_bg,omitempty"`

		// Component-specific colors (semantic naming)
		Spinner string `yaml:"spinner,omitempty"` // Spinner active state (was PaleBlue)

		// Text styling
		Bold   string `yaml:"bold,omitempty"`
		Italic string `yaml:"italic,omitempty"`
	} `yaml:"colors"`

	Spacing struct {
		Progress int `yaml:"progress,omitempty"` // Progress indicator spacing
		Indent   int `yaml:"indent,omitempty"`   // Indentation level spacing
	} `yaml:"spacing"`

	Typography struct {
		HeaderWidth int `yaml:"header_width,omitempty"` // Visual width of headers
	} `yaml:"typography"`
}

// Get returns the ANSI color code for a color token.
// This provides type-safe access to color values.
func (dt *DesignTokens) GetColor(name string) string {
	// Use reflection to access color fields dynamically
	colorsValue := reflect.ValueOf(dt.Colors)
	field := colorsValue.FieldByName(name)
	if !field.IsValid() {
		return ""
	}
	return field.String()
}

// Config holds all resolved design system settings for rendering.
type Config struct {
	ThemeName    string `yaml:"-"`
	IsMonochrome bool   `yaml:"-"`
	CI           bool   `yaml:"-"` // Explicit CI mode flag (takes precedence over heuristics)

	// DesignTokens provides centralized, semantic design values
	// This is the new extensible system (Phase 1)
	Tokens *DesignTokens `yaml:"-"`

	Style struct {
		UseBoxes          bool   `yaml:"use_boxes"`
		Indentation       string `yaml:"indentation"`
		ShowTimestamps    bool   `yaml:"show_timestamps"`
		NoTimer           bool   `yaml:"no_timer"`
		Density           string `yaml:"density"`
		UseInlineProgress bool   `yaml:"use_inline_progress"`
		NoSpinner         bool   `yaml:"no_spinner"`
		SpinnerInterval   int    `yaml:"spinner_interval"`
		HeaderWidth       int    `yaml:"header_width"` // Visual width of header content (default: 40)
	} `yaml:"style"`

	Border struct {
		TaskStyle              BorderStyle `yaml:"task_style"`
		HeaderChar             string      `yaml:"header_char"`
		VerticalChar           string      `yaml:"vertical_char"`
		TopCornerChar          string      `yaml:"top_corner_char"`
		BottomCornerChar       string      `yaml:"bottom_corner_char"`
		FooterContinuationChar string      `yaml:"footer_continuation_char"`
		TableHChar             string      `yaml:"table_h_char"`
		TableVChar             string      `yaml:"table_v_char"`
		TableXChar             string      `yaml:"table_x_char"`
		TableCornerTL          string      `yaml:"table_corner_tl"`
		TableCornerTR          string      `yaml:"table_corner_tr"`
		TableCornerBL          string      `yaml:"table_corner_bl"`
		TableCornerBR          string      `yaml:"table_corner_br"`
		TableTDown             string      `yaml:"table_t_down"`
		TableTUp               string      `yaml:"table_t_up"`
		TableTLeft             string      `yaml:"table_t_left"`
		TableTRight            string      `yaml:"table_t_right"`
	} `yaml:"border"`

	Colors struct {
		Process  string `yaml:"process"`
		Success  string `yaml:"success"`
		Warning  string `yaml:"warning"`
		Error    string `yaml:"error"`
		Detail   string `yaml:"detail"`
		Muted    string `yaml:"muted"`
		Reset    string `yaml:"reset"`
		White    string `yaml:"white,omitempty"`
		GreenFg  string `yaml:"green_fg,omitempty"`
		BlueFg   string `yaml:"blue_fg,omitempty"`
		BlueBg   string `yaml:"blue_bg,omitempty"`
		PaleBlue string `yaml:"pale_blue,omitempty"`
		Bold     string `yaml:"bold,omitempty"`
		Italic   string `yaml:"italic,omitempty"`
	} `yaml:"colors"`

	Icons struct {
		Start   string `yaml:"start"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Info    string `yaml:"info"`
		Bullet  string `yaml:"bullet"`
	} `yaml:"icons"`

	Elements      map[string]ElementStyleDef `yaml:"elements"`
	Patterns      PatternsRepo               `yaml:"patterns"`
	Tools         map[string]*ToolConfig     `yaml:"tools"`
	CognitiveLoad struct {
		AutoDetect bool                 `yaml:"auto_detect"`
		Default    CognitiveLoadContext `yaml:"default"`
	} `yaml:"cognitive_load"`
	ComplexityThresholds struct {
		VeryHigh           int `yaml:"very_high"`            // Output lines threshold for complexity level 5
		High               int `yaml:"high"`                 // Output lines threshold for complexity level 4
		Medium             int `yaml:"medium"`               // Output lines threshold for complexity level 3
		ErrorCountHigh     int `yaml:"error_count_high"`     // Error count threshold for high cognitive load
		WarningCountMedium int `yaml:"warning_count_medium"` // Warning count threshold for medium cognitive load
	} `yaml:"complexity_thresholds"`
	Tests struct {
		SparkbarFilled     string  `yaml:"sparkbar_filled"`
		SparkbarEmpty      string  `yaml:"sparkbar_empty"`
		SparkbarLength     int     `yaml:"sparkbar_length"`
		ShowPercentage     bool    `yaml:"show_percentage"`
		NoTestIcon         string  `yaml:"no_test_icon"`
		NoTestColor        string  `yaml:"no_test_color"`
		CoverageGoodMin    float64 `yaml:"coverage_good_min"`    // Minimum coverage for "good" (green)
		CoverageWarningMin float64 `yaml:"coverage_warning_min"` // Minimum coverage for "warning" (yellow)
	} `yaml:"tests"`
}

type PatternsRepo struct {
	Intent map[string][]string `yaml:"intent"`
	Output map[string][]string `yaml:"output"`
}

// NormalizeANSIEscape normalizes ANSI escape sequences to ensure they start with
// the ESC character (0x1b). YAML should handle escape sequences correctly, but
// this function normalizes them to handle edge cases and ensure consistency.
func NormalizeANSIEscape(s string) string {
	if s == "" {
		return ""
	}

	escChar := string([]byte{27}) // The actual ESC character (\x1b)

	// If it already starts with the ESC character, it's correct.
	if strings.HasPrefix(s, escChar) {
		return s
	}

	// Handle octal escape sequence \033 (which is ESC in octal)
	if strings.HasPrefix(s, "\033") {
		return s
	}

	// Handle literal backslash sequences that might come from YAML
	// YAML string like "\x1b[32m" should be unmarshaled as actual ESC, but
	// double-escaped like "\\x1b[32m" comes through as literal "\x1b[32m"
	if strings.HasPrefix(s, `\x1b`) {
		// Replace literal "\x1b" with actual ESC character
		return escChar + strings.TrimPrefix(s, `\x1b`)
	}
	if strings.HasPrefix(s, `\033`) {
		// Replace literal "\033" with actual ESC character
		return escChar + strings.TrimPrefix(s, `\033`)
	}

	// If we don't recognize it as an ANSI sequence, return as-is
	// (might be a color name or non-ANSI string)
	return s
}

func DefaultConfig() *Config {
	return UnicodeVibrantTheme()
}

func NoColorConfig() *Config {
	cfg := ASCIIMinimalTheme()
	ApplyMonochromeDefaults(cfg)
	cfg.ThemeName = "no_color_derived_from_ascii"
	return cfg
}

func ASCIIMinimalTheme() *Config {
	cfg := &Config{
		ThemeName:    "ascii_minimal",
		IsMonochrome: true,
	}
	cfg.Style.UseBoxes = false
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "compact"
	cfg.Style.NoTimer = false
	cfg.Style.HeaderWidth = 40

	cfg.Icons.Start = IconStart
	cfg.Icons.Success = IconSuccess
	cfg.Icons.Warning = IconWarning
	cfg.Icons.Error = IconFailed
	cfg.Icons.Info = IconInfo
	cfg.Icons.Bullet = IconBullet

	cfg.Style.UseInlineProgress = true
	cfg.Style.NoSpinner = false
	cfg.Style.SpinnerInterval = 80

	cfg.Colors = struct {
		Process  string `yaml:"process"`
		Success  string `yaml:"success"`
		Warning  string `yaml:"warning"`
		Error    string `yaml:"error"`
		Detail   string `yaml:"detail"`
		Muted    string `yaml:"muted"`
		Reset    string `yaml:"reset"`
		White    string `yaml:"white,omitempty"`
		GreenFg  string `yaml:"green_fg,omitempty"`
		BlueFg   string `yaml:"blue_fg,omitempty"`
		BlueBg   string `yaml:"blue_bg,omitempty"`
		PaleBlue string `yaml:"pale_blue,omitempty"`
		Bold     string `yaml:"bold,omitempty"`
		Italic   string `yaml:"italic,omitempty"`
	}{}

	cfg.Border.TaskStyle = BorderNone

	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements)

	cfg.Elements["Task_Label_Header"] = ElementStyleDef{}
	cfg.Elements["Task_StartIndicator_Line"] = ElementStyleDef{}
	cfg.Elements["H2_Target_Title"] = ElementStyleDef{Prefix: "", TextCase: "none"}
	cfg.Elements["Task_Status_Success_Block"] = ElementStyleDef{TextContent: "Success"}
	cfg.Elements["Task_Status_Failed_Block"] = ElementStyleDef{TextContent: "Failed"}
	cfg.Elements["Task_Status_Warning_Block"] = ElementStyleDef{TextContent: "Warnings"}
	cfg.Elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")"}
	cfg.Elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{Text: "  > "}
	cfg.Elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{Text: "  > "}
	cfg.Elements["Stdout_Line_Prefix"] = ElementStyleDef{Text: "  "}
	cfg.Elements["Task_Content_Summary_Heading"] = ElementStyleDef{TextContent: "SUMMARY:"}
	cfg.Elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{BulletChar: "*"}
	cfg.Elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{BulletChar: "*"}
	cfg.Elements["Print_Header_Highlight"] = ElementStyleDef{TextCase: "none", TextStyle: []string{"bold"}}
	cfg.Elements["Print_Success_Style"] = ElementStyleDef{}

	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = false
	cfg.CognitiveLoad.Default = LoadLow
	cfg.ComplexityThresholds.VeryHigh = 100
	cfg.ComplexityThresholds.High = 50
	cfg.ComplexityThresholds.Medium = 20
	cfg.ComplexityThresholds.ErrorCountHigh = 5
	cfg.ComplexityThresholds.WarningCountMedium = 2

	cfg.Tests.SparkbarFilled = "▮"
	cfg.Tests.SparkbarEmpty = "▯"
	cfg.Tests.SparkbarLength = 10
	cfg.Tests.ShowPercentage = false
	cfg.Tests.NoTestIcon = "○"
	cfg.Tests.NoTestColor = ColorKeyWarning
	cfg.Tests.CoverageGoodMin = 70
	cfg.Tests.CoverageWarningMin = 40

	return cfg
}

func UnicodeVibrantTheme() *Config {
	cfg := &Config{
		ThemeName:    "unicode_vibrant",
		IsMonochrome: false,
	}
	cfg.Style.UseBoxes = true
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "balanced"
	cfg.Style.NoTimer = false
	cfg.Style.HeaderWidth = 40

	cfg.Icons.Start = "▶️"
	cfg.Icons.Success = "✅"
	cfg.Icons.Warning = "⚠️"
	cfg.Icons.Error = "❌"
	cfg.Icons.Info = "ℹ️"
	cfg.Icons.Bullet = "•"

	// Initialize Design Tokens (Phase 1: centralized, semantic values)
	cfg.Tokens = &DesignTokens{}
	cfg.Tokens.Colors.Process = ANSIBrightWhite
	cfg.Tokens.Colors.Success = ANSIBrightWhite
	cfg.Tokens.Colors.Warning = "\033[0;33m"
	cfg.Tokens.Colors.Error = "\033[0;31m"
	cfg.Tokens.Colors.Detail = "\033[0m"
	cfg.Tokens.Colors.Muted = "\033[2m"
	cfg.Tokens.Colors.Reset = "\033[0m"
	cfg.Tokens.Colors.White = "\033[0;97m"
	cfg.Tokens.Colors.GreenFg = "\033[38;5;120m"
	cfg.Tokens.Colors.BlueFg = "\033[0;34m"
	cfg.Tokens.Colors.BlueBg = "\033[44m"
	cfg.Tokens.Colors.Spinner = "\033[38;5;111m" // Semantic: Spinner (was PaleBlue)
	cfg.Tokens.Colors.Bold = "\033[1m"
	cfg.Tokens.Colors.Italic = "\033[3m"
	cfg.Tokens.Spacing.Progress = 0
	cfg.Tokens.Spacing.Indent = 2
	cfg.Tokens.Typography.HeaderWidth = 40

	// Sync Tokens to old Colors struct for backwards compatibility
	cfg.syncTokensToColors()

	cfg.Border.TaskStyle = BorderLeftDouble
	cfg.Border.HeaderChar = "═"
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╒"
	cfg.Border.BottomCornerChar = "└"
	cfg.Border.FooterContinuationChar = "─"

	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements)
	cfg.Elements["Fo_Banner_Top"] = ElementStyleDef{LineChar: "═", Prefix: "FO: ", TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Fo_Banner_Bottom"] = ElementStyleDef{LineChar: "═", Prefix: "FO: ", TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Task_Label_Header"] = ElementStyleDef{TextCase: "upper", TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Task_StartIndicator_Line"] = ElementStyleDef{IconKey: "Start", ColorFG: "Process"}
	cfg.Elements["Stdout_Line_Prefix"] = ElementStyleDef{AdditionalChars: "  "}
	cfg.Elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{IconKey: "Warning", AdditionalChars: "  ", ColorFG: "Warning"}
	cfg.Elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{IconKey: "Error", AdditionalChars: "  ", ColorFG: "Error"}
	cfg.Elements["Make_Info_Line_Prefix"] = ElementStyleDef{IconKey: "Info", Text: " "}
	cfg.Elements["Task_Content_Stderr_Warning_Text"] = ElementStyleDef{ColorFG: "Warning"}
	cfg.Elements["Task_Content_Stderr_Error_Text"] = ElementStyleDef{ColorFG: "Error"}
	cfg.Elements["Task_Status_Success_Block"] = ElementStyleDef{IconKey: "Success", TextContent: "Complete", ColorFG: "Success"}
	cfg.Elements["Task_Status_Failed_Block"] = ElementStyleDef{IconKey: "Error", TextContent: "Failed", ColorFG: "Error"}
	cfg.Elements["Task_Status_Warning_Block"] = ElementStyleDef{IconKey: "Warning", TextContent: "Completed with warnings", ColorFG: "Warning"}
	cfg.Elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")", ColorFG: "Muted"}
	cfg.Elements["Task_Content_Summary_Heading"] = ElementStyleDef{TextContent: "SUMMARY:", TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{BulletChar: "•", ColorFG: "Error"}
	cfg.Elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{BulletChar: "•", ColorFG: "Warning"}
	cfg.Elements["Table_Header_Cell_Text"] = ElementStyleDef{TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Print_Header_Highlight"] = ElementStyleDef{
		TextCase: "none", TextStyle: []string{"bold"}, ColorFG: "White", ColorBG: "BlueBg",
	}
	cfg.Elements["Print_Success_Style"] = ElementStyleDef{ColorFG: "Success"}

	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium
	cfg.ComplexityThresholds.VeryHigh = 100
	cfg.ComplexityThresholds.High = 50
	cfg.ComplexityThresholds.Medium = 20
	cfg.ComplexityThresholds.ErrorCountHigh = 5
	cfg.ComplexityThresholds.WarningCountMedium = 2

	cfg.Tests.SparkbarFilled = "▮"
	cfg.Tests.SparkbarEmpty = "▯"
	cfg.Tests.SparkbarLength = 10
	cfg.Tests.ShowPercentage = false
	cfg.Tests.NoTestIcon = "○"
	cfg.Tests.NoTestColor = ColorKeyWarning
	cfg.Tests.CoverageGoodMin = 70
	cfg.Tests.CoverageWarningMin = 40

	return cfg
}

func initBaseElementStyles(elements map[string]ElementStyleDef) {
	knownElements := []string{
		"Fo_Banner_Top", "Fo_Banner_Top_Line_FoProcessing", "Fo_Timestamp_Start",
		"Fo_Banner_Bottom", "Fo_OverallStatus_Success", "Fo_OverallStatus_Failed", "Fo_OverallStatus_Warnings",
		"Task_Label_Header", "Task_StartIndicator_Line",
		"H2_Target_Header_Line", "H2_Target_Title", "H2_Target_Footer_Line",
		"Command_Line_Prefix", "Stdout_Line_Prefix", "Stderr_Warning_Line_Prefix", "Stderr_Error_Line_Prefix",
		"Make_Info_Line_Prefix",
		"Task_Content_Stdout_Text", "Task_Content_Stderr_Warning_Text", "Task_Content_Stderr_Error_Text",
		"Task_Content_Stdout_Success_Text", "Task_Content_Info_Text", "Task_Content_Summary_Text", "Task_Content_Progress_Text",
		"Status_Label_Prefix", "Task_Status_Success_Block", "Task_Status_Failed_Block",
		"Task_Status_Warning_Block", "Task_Status_Duration",
		"Task_Content_Summary_Heading", "Task_Content_Summary_Item_Error", "Task_Content_Summary_Item_Warning",
		"Table_Header_Cell_Text", "Table_Body_Cell_Text",
		"ProgressIndicator_Spinner_Chars", "ProgressIndicator_Text",
		"Task_Progress_Line",
		"Print_Header_Highlight", "Print_Success_Style", "Print_Warning_Style", "Print_Error_Style", "Print_Info_Style",
	}
	elements["Task_Progress_Line"] = ElementStyleDef{
		AdditionalChars: "·✻✽✶✳✢",
		Text:            "{verb}ing {subject}...",
		TextContent:     "{verb}ing {subject} complete",
	}
	for _, elKey := range knownElements {
		if _, exists := elements[elKey]; !exists {
			elements[elKey] = ElementStyleDef{}
		}
	}
}

func defaultPatterns() PatternsRepo {
	return PatternsRepo{
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
				"\\[ERROR\\]", "^FAIL\\t", "failure",
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

func (c *Config) GetElementStyle(elementName string) ElementStyleDef {
	if style, ok := c.Elements[elementName]; ok {
		return style
	}
	return ElementStyleDef{}
}

func (c *Config) GetIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	baseIndent := c.Style.Indentation
	if baseIndent == "" {
		baseIndent = "  "
	}
	return strings.Repeat(baseIndent, level)
}

func (c *Config) GetIcon(iconKey string) string {
	if c.IsMonochrome {
		switch strings.ToLower(iconKey) {
		case "start":
			return IconStart
		case "success":
			return IconSuccess
		case "warning":
			return IconWarning
		case "error":
			return IconFailed
		case "info":
			return IconInfo
		case "bullet":
			return IconBullet
		default:
			return "?"
		}
	}
	switch strings.ToLower(iconKey) {
	case "start":
		return c.Icons.Start
	case "success":
		return c.Icons.Success
	case "warning":
		return c.Icons.Warning
	case "error":
		return c.Icons.Error
	case "info":
		return c.Icons.Info
	case "bullet":
		return c.Icons.Bullet
	default:
		return ""
	}
}

func (c *Config) GetColor(colorKeyOrName string, elementName ...string) string {
	if c.IsMonochrome {
		return ""
	}

	// Use reflection-based resolution if enabled, otherwise use switch-based
	resolveFunc := c.resolveColorName
	if c.useReflectionForColors() {
		resolveFunc = c.resolveColorNameByReflection
	}

	if len(elementName) > 0 && elementName[0] != "" {
		if elemStyle, ok := c.Elements[elementName[0]]; ok {
			if elemStyle.ColorFG != "" {
				return resolveFunc(elemStyle.ColorFG)
			}
		}
	}
	return resolveFunc(colorKeyOrName)
}

func (c *Config) resolveColorName(name string) string {
	if c.IsMonochrome || name == "" {
		return ""
	}

	var codeToProcess string
	lowerName := strings.ToLower(name)

	switch lowerName {
	case colorNameProcess, colorNameSuccess:
		codeToProcess = c.Colors.Process
	case colorNameWarning:
		codeToProcess = c.Colors.Warning
	case colorNameError:
		codeToProcess = c.Colors.Error
	case colorNameDetail:
		codeToProcess = c.Colors.Detail
	case colorNameMuted:
		codeToProcess = c.Colors.Muted
	case colorNameReset:
		codeToProcess = c.Colors.Reset
	case colorNameWhite:
		codeToProcess = c.Colors.White
	case colorNameGreenFg:
		codeToProcess = c.Colors.GreenFg
	case colorNameBlueFg:
		codeToProcess = c.Colors.BlueFg
	case colorNameBlueBg:
		codeToProcess = c.Colors.BlueBg
	case "paleblue":
		codeToProcess = c.Colors.PaleBlue
	case colorNameBold:
		codeToProcess = c.Colors.Bold
	case colorNameItalic:
		codeToProcess = c.Colors.Italic
	default:
		// If name contains '[' and starts with an escape sequence, treat it as a raw ANSI code
		hasEscPrefix := strings.HasPrefix(name, "\033") || strings.HasPrefix(name, "\x1b") ||
			strings.HasPrefix(name, "\\033") || strings.HasPrefix(name, "\\x1b")
		if strings.Contains(name, "[") && hasEscPrefix {
			codeToProcess = name
		}
	}

	escChar := string([]byte{27})
	if codeToProcess == "" {
		switch lowerName {
		case colorNameProcess, colorNameSuccess, colorNameWhite:
			codeToProcess = escChar + "[0;97m"
		case colorNameWarning:
			codeToProcess = escChar + "[0;33m"
		case colorNameError:
			codeToProcess = escChar + "[0;31m"
		case colorNameDetail, colorNameReset:
			codeToProcess = escChar + "[0m"
		case colorNameMuted:
			codeToProcess = escChar + "[2m"
		case colorNameGreenFg:
			codeToProcess = escChar + "[38;5;120m"
		case colorNameBlueFg:
			codeToProcess = escChar + "[0;34m"
		case colorNameBlueBg:
			codeToProcess = escChar + "[44m"
		case colorNameBold:
			codeToProcess = escChar + "[1m"
		case colorNameItalic:
			codeToProcess = escChar + "[3m"
		default:
			codeToProcess = escChar + "[0m"
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Color key/name '%s' not found in theme or defaults, using reset.\n", name)
			}
		}
	}
	return NormalizeANSIEscape(codeToProcess)
}

// useReflectionForColors determines if we should use reflection-based color resolution.
// Controlled by environment variable FO_USE_REFLECTION_COLORS (default: false for now).
func (c *Config) useReflectionForColors() bool {
	return os.Getenv("FO_USE_REFLECTION_COLORS") != ""
}

// resolveColorNameByReflection uses reflection to dynamically access color fields.
// This is the new extensible approach that doesn't require hardcoded switch statements.
func (c *Config) resolveColorNameByReflection(name string) string {
	if c.IsMonochrome || name == "" {
		return ""
	}

	// Normalize name: convert to field name format (e.g., "paleblue" -> "PaleBlue")
	lowerName := strings.ToLower(name)

	// Handle special cases and status constants
	var fieldName string
	switch lowerName {
	case colorNameProcess, colorNameSuccess:
		fieldName = "Process"
	case colorNameWarning:
		fieldName = "Warning"
	case colorNameError:
		fieldName = "Error"
	case colorNameDetail:
		fieldName = "Detail"
	case colorNameMuted:
		fieldName = "Muted"
	case colorNameReset:
		fieldName = "Reset"
	case colorNameWhite:
		fieldName = "White"
	case colorNameGreenFg:
		fieldName = "GreenFg"
	case colorNameBlueFg:
		fieldName = "BlueFg"
	case colorNameBlueBg:
		fieldName = "BlueBg"
	case "paleblue":
		fieldName = "PaleBlue"
	case colorNameBold:
		fieldName = "Bold"
	case colorNameItalic:
		fieldName = "Italic"
	default:
		// Try to convert to field name (capitalize first letter, handle camelCase)
		// For now, fall back to checking if it's a raw ANSI code
		hasEscPrefix := strings.HasPrefix(name, "\033") || strings.HasPrefix(name, "\x1b") ||
			strings.HasPrefix(name, "\\033") || strings.HasPrefix(name, "\\x1b")
		if strings.Contains(name, "[") && hasEscPrefix {
			return name
		}
		// If we can't map it, try to construct field name manually
		// Convert "somecolor" -> "Somecolor" (simple capitalization)
		if len(lowerName) > 0 {
			fieldName = strings.ToUpper(lowerName[:1]) + lowerName[1:]
		} else {
			fieldName = ""
		}
	}

	// Use reflection to get the field value from Colors struct
	colorsValue := reflect.ValueOf(c.Colors)
	colorsType := colorsValue.Type()

	field := colorsValue.FieldByName(fieldName)
	if !field.IsValid() {
		// Field not found, try fallback defaults
		escChar := string([]byte{27})
		switch lowerName {
		case "process", "success", "white":
			return escChar + "[0;97m"
		case "warning":
			return escChar + "[0;33m"
		case "error":
			return escChar + "[0;31m"
		case "detail", "reset":
			return escChar + "[0m"
		case "muted":
			return escChar + "[2m"
		case "greenfg":
			return escChar + "[38;5;120m"
		case "bluefg":
			return escChar + "[0;34m"
		case "bluebg":
			return escChar + "[44m"
		case "bold":
			return escChar + "[1m"
		case "italic":
			return escChar + "[3m"
		default:
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr,
					"[DEBUG resolveColorNameByReflection] Color field '%s' (from '%s') not found in Colors struct (type: %s), using reset.\n",
					fieldName, name, colorsType.Name())
			}
			return escChar + "[0m"
		}
	}

	colorValue := field.String()
	if colorValue == "" {
		// Empty color, use fallback defaults (same as above)
		escChar := string([]byte{27})
		switch lowerName {
		case "process", "success", "white":
			return escChar + "[0;97m"
		case "warning":
			return escChar + "[0;33m"
		case "error":
			return escChar + "[0;31m"
		case "detail", "reset":
			return escChar + "[0m"
		case "muted":
			return escChar + "[2m"
		case "greenfg":
			return escChar + "[38;5;120m"
		case "bluefg":
			return escChar + "[0;34m"
		case "bluebg":
			return escChar + "[44m"
		case "bold":
			return escChar + "[1m"
		case "italic":
			return escChar + "[3m"
		default:
			return escChar + "[0m"
		}
	}

	return NormalizeANSIEscape(colorValue)
}

// syncTokensToColors syncs DesignTokens to the old Colors struct for backwards compatibility.
// This allows code using the old Colors struct to continue working during the migration.
// Will be removed in Phase 5.
func (c *Config) syncTokensToColors() {
	if c.Tokens == nil {
		return
	}
	// Sync color values from Tokens to Colors
	c.Colors.Process = c.Tokens.Colors.Process
	c.Colors.Success = c.Tokens.Colors.Success
	c.Colors.Warning = c.Tokens.Colors.Warning
	c.Colors.Error = c.Tokens.Colors.Error
	c.Colors.Detail = c.Tokens.Colors.Detail
	c.Colors.Muted = c.Tokens.Colors.Muted
	c.Colors.Reset = c.Tokens.Colors.Reset
	c.Colors.White = c.Tokens.Colors.White
	c.Colors.GreenFg = c.Tokens.Colors.GreenFg
	c.Colors.BlueFg = c.Tokens.Colors.BlueFg
	c.Colors.BlueBg = c.Tokens.Colors.BlueBg
	c.Colors.PaleBlue = c.Tokens.Colors.Spinner // Map Spinner token to PaleBlue for compatibility
	c.Colors.Bold = c.Tokens.Colors.Bold
	c.Colors.Italic = c.Tokens.Colors.Italic
}

// GetColorObj returns a Color wrapper for the given color key.
// This provides a safer interface for color handling with automatic reset.
// Example: cfg.GetColorObj("Error").Sprint("Error message").
func (c *Config) GetColorObj(colorKeyOrName string) Color {
	return NewColor(c.GetColor(colorKeyOrName))
}

func (c *Config) ResetColor() string {
	if c.IsMonochrome {
		return ""
	}
	resetCode := c.Colors.Reset
	if resetCode == "" {
		resetCode = string([]byte{27}) + "[0m"
	}
	return NormalizeANSIEscape(resetCode)
}

// DeepCopyConfig creates a deep copy of the Config using JSON marshal/unmarshal.
// This automatically handles all fields, preventing bugs when new fields are added.
// The small overhead of JSON encoding is acceptable since this is not in a hot path.
func DeepCopyConfig(original *Config) *Config {
	if original == nil {
		return nil
	}

	// Use JSON round-trip for automatic deep copy of all fields
	//nolint:musttag // Config fields have yaml tags; json marshal works for deep copy
	data, err := json.Marshal(original)
	if err != nil {
		// Fallback to shallow copy if marshal fails (shouldn't happen with valid configs)
		copied := *original
		return &copied
	}

	var copied Config
	//nolint:musttag // Config fields have yaml tags; json marshal works for deep copy
	if err := json.Unmarshal(data, &copied); err != nil {
		// Fallback to shallow copy if unmarshal fails
		shallow := *original
		return &shallow
	}

	return &copied
}

// DefaultThemes returns a map of all built-in themes.
// This is the single source of truth for default theme definitions.
func DefaultThemes() map[string]*Config {
	return map[string]*Config{
		"unicode_vibrant": UnicodeVibrantTheme(),
		"ascii_minimal":   ASCIIMinimalTheme(),
	}
}

func ApplyMonochromeDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.IsMonochrome = true
	cfg.Style.UseBoxes = false

	cfg.Colors = struct {
		Process  string `yaml:"process"`
		Success  string `yaml:"success"`
		Warning  string `yaml:"warning"`
		Error    string `yaml:"error"`
		Detail   string `yaml:"detail"`
		Muted    string `yaml:"muted"`
		Reset    string `yaml:"reset"`
		White    string `yaml:"white,omitempty"`
		GreenFg  string `yaml:"green_fg,omitempty"`
		BlueFg   string `yaml:"blue_fg,omitempty"`
		BlueBg   string `yaml:"blue_bg,omitempty"`
		PaleBlue string `yaml:"pale_blue,omitempty"`
		Bold     string `yaml:"bold,omitempty"`
		Italic   string `yaml:"italic,omitempty"`
	}{}

	asciiMinimalElements := ASCIIMinimalTheme().Elements
	if cfg.Elements == nil {
		cfg.Elements = make(map[string]ElementStyleDef)
		initBaseElementStyles(cfg.Elements)
	}

	for key := range cfg.Elements {
		elDef := cfg.Elements[key]
		elDef.ColorFG = ""
		elDef.ColorBG = ""
		if asciiStyle, ok := asciiMinimalElements[key]; ok {
			if asciiStyle.Text != "" {
				elDef.Text = asciiStyle.Text
			}
			if asciiStyle.Prefix != "" {
				elDef.Prefix = asciiStyle.Prefix
			}
			if asciiStyle.Suffix != "" {
				elDef.Suffix = asciiStyle.Suffix
			}
			if asciiStyle.TextContent != "" {
				elDef.TextContent = asciiStyle.TextContent
			}
		}
		elDef.TextStyle = nil
		cfg.Elements[key] = elDef
	}
	cfg.Icons.Start = IconStart
	cfg.Icons.Success = IconSuccess
	cfg.Icons.Warning = IconWarning
	cfg.Icons.Error = IconFailed
	cfg.Icons.Info = IconInfo
	cfg.Icons.Bullet = IconBullet
}
