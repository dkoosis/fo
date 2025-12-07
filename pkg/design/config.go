// cmd/internal/design/config.go
package design

import (
	"encoding/json"
	"fmt"
	"os" // For debug prints to stderr
	"reflect"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// defaultColorValues maps color names to their default 256-color palette values.
// This centralizes color defaults to avoid duplication across resolution functions.
var defaultColorValues = map[string]lipgloss.Color{
	"process": "231", // Bright white
	"white":   "231", // Bright white
	"success": "120", // Light green
	"warning": "214", // Orange
	"error":   "196", // Red
	"detail":  "",    // No color (reset)
	"reset":   "",    // No color (reset)
	"muted":   "242", // Dark gray
	"greenfg": "120", // Light green
	"bluefg":  "39",  // Bright blue
	"bluebg":  "4",   // Blue
	"bold":    "",    // Style, not color
	"italic":  "",    // Style, not color
}

// getDefaultColor returns the default color value for a color name.
// Returns the color and true if found, or empty string and false if not.
func getDefaultColor(name string) (lipgloss.Color, bool) {
	color, ok := defaultColorValues[strings.ToLower(name)]
	return color, ok
}

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

// Icon character constants.
const (
	IconCharError   = "✗"
	IconCharWarning = "⚠"
	BorderCharDash  = "─"
)

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
// Uses Lip Gloss types for proper color/style handling.
type DesignTokens struct {
	Colors struct {
		// Status colors (semantic naming)
		Process lipgloss.Color `yaml:"process"` // Primary process/task color
		Success lipgloss.Color `yaml:"success"` // Success state
		Warning lipgloss.Color `yaml:"warning"` // Warning state
		Error   lipgloss.Color `yaml:"error"`   // Error state
		Detail  lipgloss.Color `yaml:"detail"`  // Detail text
		Muted   lipgloss.Color `yaml:"muted"`   // Muted/secondary text
		Reset   lipgloss.Color `yaml:"reset"`   // Reset/clear formatting

		// Base colors
		White   lipgloss.Color `yaml:"white,omitempty"`
		GreenFg lipgloss.Color `yaml:"green_fg,omitempty"`
		BlueFg  lipgloss.Color `yaml:"blue_fg,omitempty"`
		BlueBg  lipgloss.Color `yaml:"blue_bg,omitempty"`

		// Component-specific colors (semantic naming)
		Spinner lipgloss.Color `yaml:"spinner,omitempty"` // Spinner active state (was PaleBlue)

		// Text styling
		Bold   lipgloss.Color `yaml:"bold,omitempty"`
		Italic lipgloss.Color `yaml:"italic,omitempty"`
	} `yaml:"colors"`

	Styles struct {
		Box     lipgloss.Style `yaml:"-"` // Pre-configured box style
		Header  lipgloss.Style `yaml:"-"` // Pre-configured header style
		Content lipgloss.Style `yaml:"-"` // Pre-configured content style
	} `yaml:"styles"`

	Spacing struct {
		Progress int `yaml:"progress,omitempty"` // Progress indicator spacing
		Indent   int `yaml:"indent,omitempty"`   // Indentation level spacing
	} `yaml:"spacing"`

	Typography struct {
		HeaderWidth int `yaml:"header_width,omitempty"` // Visual width of headers
	} `yaml:"typography"`
}

// GetColor returns the lipgloss.Color for a color token by name.
// This provides type-safe access to color values.
func (dt *DesignTokens) GetColor(name string) lipgloss.Color {
	// Use reflection to access color fields dynamically
	colorsValue := reflect.ValueOf(dt.Colors)
	field := colorsValue.FieldByName(name)
	if !field.IsValid() {
		return lipgloss.Color("")
	}
	if color, ok := field.Interface().(lipgloss.Color); ok {
		return color
	}
	return lipgloss.Color("")
}

// GetColorString returns the ANSI color code string for a color token.
// This is a convenience method for backwards compatibility.
func (dt *DesignTokens) GetColorString(name string) string {
	color := dt.GetColor(name)
	if color == "" {
		return ""
	}
	// Convert lipgloss.Color to ANSI string
	// lipgloss.Color is a string type, so we can cast it
	return string(color)
}

// Config holds all resolved design system settings for rendering.
// styleCache holds pre-computed lipgloss styles for performance.
type styleCache struct {
	colors   map[string]lipgloss.Style // Cached styles by color key
	elements map[string]lipgloss.Style // Cached styles by element name
}

// Config holds theme configuration.
type Config struct {
	ThemeName    string `yaml:"-"`
	IsMonochrome bool   `yaml:"-"`
	CI           bool   `yaml:"-"` // Explicit CI mode flag (takes precedence over heuristics)

	// DesignTokens provides centralized, semantic design values
	// This is the new extensible system (Phase 1)
	Tokens *DesignTokens `yaml:"-"`

	// styleCache holds pre-computed styles (lazy-initialized)
	cache *styleCache `yaml:"-"`

	Style struct {
		UseBoxes        bool   `yaml:"use_boxes"`
		Indentation     string `yaml:"indentation"`
		ShowTimestamps  bool   `yaml:"show_timestamps"`
		NoTimer         bool   `yaml:"no_timer"`
		Density         string `yaml:"density"`
		NoSpinner       bool   `yaml:"no_spinner"`
		SpinnerInterval int    `yaml:"spinner_interval"`
		HeaderWidth     int    `yaml:"header_width"` // Visual width of header content (default: 40)
	} `yaml:"style"`

	Border struct {
		TaskStyle              BorderStyle `yaml:"task_style"`
		HeaderChar             string      `yaml:"header_char"`
		VerticalChar           string      `yaml:"vertical_char"`
		TopCornerChar          string      `yaml:"top_corner_char"`
		TopRightChar           string      `yaml:"top_right_char"`
		BottomCornerChar       string      `yaml:"bottom_corner_char"`
		BottomRightChar        string      `yaml:"bottom_right_char"`
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
		Process  lipgloss.Color `yaml:"process"`
		Success  lipgloss.Color `yaml:"success"`
		Warning  lipgloss.Color `yaml:"warning"`
		Error    lipgloss.Color `yaml:"error"`
		Detail   lipgloss.Color `yaml:"detail"`
		Muted    lipgloss.Color `yaml:"muted"`
		Reset    lipgloss.Color `yaml:"reset"`
		White    lipgloss.Color `yaml:"white,omitempty"`
		GreenFg  lipgloss.Color `yaml:"green_fg,omitempty"`
		BlueFg   lipgloss.Color `yaml:"blue_fg,omitempty"`
		BlueBg   lipgloss.Color `yaml:"blue_bg,omitempty"`
		PaleBlue lipgloss.Color `yaml:"pale_blue,omitempty"`
		Bold     lipgloss.Color `yaml:"bold,omitempty"`
		Italic   lipgloss.Color `yaml:"italic,omitempty"`
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

	cfg.Style.NoSpinner = false
	cfg.Style.SpinnerInterval = 80

	// ASCII theme is monochrome, so all colors are empty
	cfg.Colors.Process = lipgloss.Color("")
	cfg.Colors.Success = lipgloss.Color("")
	cfg.Colors.Warning = lipgloss.Color("")
	cfg.Colors.Error = lipgloss.Color("")
	cfg.Colors.Detail = lipgloss.Color("")
	cfg.Colors.Muted = lipgloss.Color("")
	cfg.Colors.Reset = lipgloss.Color("")
	cfg.Colors.White = lipgloss.Color("")
	cfg.Colors.GreenFg = lipgloss.Color("")
	cfg.Colors.BlueFg = lipgloss.Color("")
	cfg.Colors.BlueBg = lipgloss.Color("")
	cfg.Colors.PaleBlue = lipgloss.Color("")
	cfg.Colors.Bold = lipgloss.Color("")
	cfg.Colors.Italic = lipgloss.Color("")

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
	// lipgloss.Color takes color values: names ("red"), hex ("#ff0000"), or 256-color numbers ("111")
	cfg.Tokens = &DesignTokens{}
	cfg.Tokens.Colors.Process = lipgloss.Color("231") // Bright white
	cfg.Tokens.Colors.Success = lipgloss.Color("120") // Light green
	cfg.Tokens.Colors.Warning = lipgloss.Color("214") // Orange
	cfg.Tokens.Colors.Error = lipgloss.Color("196")   // Red
	cfg.Tokens.Colors.Detail = lipgloss.Color("252")  // Light gray
	cfg.Tokens.Colors.Muted = lipgloss.Color("242")   // Dark gray
	cfg.Tokens.Colors.Reset = lipgloss.Color("")      // No color (reset)
	cfg.Tokens.Colors.White = lipgloss.Color("231")   // Bright white
	cfg.Tokens.Colors.GreenFg = lipgloss.Color("120") // Light green
	cfg.Tokens.Colors.BlueFg = lipgloss.Color("39")   // Bright blue
	cfg.Tokens.Colors.BlueBg = lipgloss.Color("4")    // Blue (for background)
	cfg.Tokens.Colors.Spinner = lipgloss.Color("111") // Pale blue
	cfg.Tokens.Colors.Bold = lipgloss.Color("")       // TODO: Bold is a style, not a color
	cfg.Tokens.Colors.Italic = lipgloss.Color("")     // TODO: Italic is a style, not a color
	cfg.Tokens.Spacing.Progress = 0
	cfg.Tokens.Spacing.Indent = 2
	cfg.Tokens.Typography.HeaderWidth = 40

	// Sync Tokens to old Colors struct for backwards compatibility
	cfg.syncTokensToColors()

	cfg.Border.TaskStyle = BorderLeftDouble
	cfg.Border.HeaderChar = "═"
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╒"
	cfg.Border.TopRightChar = "╕"
	cfg.Border.BottomCornerChar = "└"
	cfg.Border.BottomRightChar = "╛"
	cfg.Border.FooterContinuationChar = BorderCharDash

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

func OrcaTheme() *Config {
	cfg := &Config{
		ThemeName:    "orca",
		IsMonochrome: false,
	}
	cfg.Style.UseBoxes = true
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "balanced"
	cfg.Style.NoTimer = false
	cfg.Style.HeaderWidth = 50

	cfg.Icons.Start = "▶"
	cfg.Icons.Success = "✓"
	cfg.Icons.Warning = IconCharWarning
	cfg.Icons.Error = IconCharError
	cfg.Icons.Info = "ℹ"
	cfg.Icons.Bullet = "•"

	// Initialize Design Tokens (Phase 1: centralized, semantic values)
	// Using Lip Gloss Color types for proper color handling
	// lipgloss.Color takes color values: names ("red"), hex ("#ff0000"), or 256-color numbers ("111")
	cfg.Tokens = &DesignTokens{}
	cfg.Tokens.Colors.Process = lipgloss.Color("111") // Pale blue for process/task/headings
	cfg.Tokens.Colors.Success = lipgloss.Color("120") // Pale green for success
	cfg.Tokens.Colors.Warning = lipgloss.Color("214") // Orange for warnings
	cfg.Tokens.Colors.Error = lipgloss.Color("196")   // Red for errors
	cfg.Tokens.Colors.Detail = lipgloss.Color("252")  // Light gray for detail text
	cfg.Tokens.Colors.Muted = lipgloss.Color("242")   // Dark gray for muted text
	cfg.Tokens.Colors.Reset = lipgloss.Color("")      // No color (reset)
	cfg.Tokens.Colors.White = lipgloss.Color("231")   // Bright white
	cfg.Tokens.Colors.GreenFg = lipgloss.Color("120") // Light green
	cfg.Tokens.Colors.BlueFg = lipgloss.Color("39")   // Bright blue
	cfg.Tokens.Colors.BlueBg = lipgloss.Color("4")    // Blue (for background use)
	cfg.Tokens.Colors.Spinner = lipgloss.Color("111") // Pale blue for spinner
	cfg.Tokens.Colors.Bold = lipgloss.Color("")       // TODO: Bold is a style, not a color
	cfg.Tokens.Colors.Italic = lipgloss.Color("")     // TODO: Italic is a style, not a color
	cfg.Tokens.Spacing.Progress = 0
	cfg.Tokens.Spacing.Indent = 2
	cfg.Tokens.Typography.HeaderWidth = 50

	// Sync Tokens to old Colors struct for backwards compatibility
	cfg.syncTokensToColors()

	cfg.Border.TaskStyle = BorderHeaderBox
	cfg.Border.HeaderChar = BorderCharDash
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╭"
	cfg.Border.TopRightChar = "╮"
	cfg.Border.BottomCornerChar = "╰"
	cfg.Border.BottomRightChar = "╯"
	cfg.Border.FooterContinuationChar = BorderCharDash

	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements)
	cfg.Elements["Fo_Banner_Top"] = ElementStyleDef{LineChar: "═", Prefix: "ORCA: ", TextStyle: []string{"bold"}, ColorFG: "Process"}
	cfg.Elements["Fo_Banner_Bottom"] = ElementStyleDef{LineChar: "═", Prefix: "ORCA: ", TextStyle: []string{"bold"}, ColorFG: "Process"}
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
		case MessageTypeSuccess:
			return IconSuccess
		case MessageTypeWarning:
			return IconWarning
		case MessageTypeError:
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
	case MessageTypeSuccess:
		return c.Icons.Success
	case MessageTypeWarning:
		return c.Icons.Warning
	case MessageTypeError:
		return c.Icons.Error
	case "info":
		return c.Icons.Info
	case "bullet":
		return c.Icons.Bullet
	default:
		return ""
	}
}

func (c *Config) GetColor(colorKeyOrName string, elementName ...string) lipgloss.Color {
	if c.IsMonochrome {
		return lipgloss.Color("")
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

func (c *Config) resolveColorName(name string) lipgloss.Color {
	if c.IsMonochrome || name == "" {
		return lipgloss.Color("")
	}

	var color lipgloss.Color
	lowerName := strings.ToLower(name)

	switch lowerName {
	case colorNameProcess:
		color = c.Colors.Process
	case colorNameSuccess:
		color = c.Colors.Success
	case colorNameWarning:
		color = c.Colors.Warning
	case colorNameError:
		color = c.Colors.Error
	case colorNameDetail:
		color = c.Colors.Detail
	case colorNameMuted:
		color = c.Colors.Muted
	case colorNameReset:
		color = c.Colors.Reset
	case colorNameWhite:
		color = c.Colors.White
	case colorNameGreenFg:
		color = c.Colors.GreenFg
	case colorNameBlueFg:
		color = c.Colors.BlueFg
	case colorNameBlueBg:
		color = c.Colors.BlueBg
	case "paleblue":
		color = c.Colors.PaleBlue
	case colorNameBold:
		color = c.Colors.Bold
	case colorNameItalic:
		color = c.Colors.Italic
	default:
		// If name looks like a color value (hex, ANSI code, or color name), use it directly
		// lipgloss.Color accepts: hex codes (#ffffff), color names (red, blue), or ANSI codes
		if name != "" {
			color = lipgloss.Color(name)
		}
	}

	// If color is empty, use default color values (256-color palette)
	if color == "" {
		if defaultColor, ok := getDefaultColor(lowerName); ok {
			color = defaultColor
		} else if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Color key/name '%s' not found in theme or defaults.\n", name)
		}
	}
	return color
}

// useReflectionForColors determines if we should use reflection-based color resolution.
// Controlled by environment variable FO_USE_REFLECTION_COLORS (default: false for now).
func (c *Config) useReflectionForColors() bool {
	return os.Getenv("FO_USE_REFLECTION_COLORS") != ""
}

// resolveColorNameByReflection uses reflection to dynamically access color fields.
// This is the new extensible approach that doesn't require hardcoded switch statements.
func (c *Config) resolveColorNameByReflection(name string) lipgloss.Color {
	if c.IsMonochrome || name == "" {
		return lipgloss.Color("")
	}

	// Normalize name: convert to field name format (e.g., "paleblue" -> "PaleBlue")
	lowerName := strings.ToLower(name)

	// Handle special cases and status constants
	var fieldName string
	switch lowerName {
	case colorNameProcess:
		fieldName = "Process"
	case colorNameSuccess:
		fieldName = "Success"
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
		// Try to construct field name manually
		// Convert "somecolor" -> "Somecolor" (simple capitalization)
		if len(lowerName) > 0 {
			fieldName = strings.ToUpper(lowerName[:1]) + lowerName[1:]
		} else {
			fieldName = ""
		}
		// If field name construction fails, we'll try to use the name directly as a lipgloss color
	}

	// Use reflection to get the field value from Colors struct
	colorsValue := reflect.ValueOf(c.Colors)
	colorsType := colorsValue.Type()

	field := colorsValue.FieldByName(fieldName)
	if !field.IsValid() {
		// Field not found, try fallback defaults
		if defaultColor, ok := getDefaultColor(lowerName); ok {
			return defaultColor
		}
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr,
				"[DEBUG resolveColorNameByReflection] Color field '%s' (from '%s') not found in Colors struct (type: %s).\n",
				fieldName, name, colorsType.Name())
		}
		// Try to use the name directly as a lipgloss color (supports hex, color names, etc.)
		return lipgloss.Color(name)
	}

	// Get the color value - field is now lipgloss.Color which is a string type
	colorValue := lipgloss.Color(field.String())
	if colorValue == "" {
		// Empty color, use fallback defaults
		if defaultColor, ok := getDefaultColor(lowerName); ok {
			return defaultColor
		}
		return lipgloss.Color("")
	}

	return colorValue
}

// syncTokensToColors syncs DesignTokens to Colors struct.
// Both now use lipgloss.Color, so this is a direct assignment.
func (c *Config) syncTokensToColors() {
	if c.Tokens == nil {
		return
	}
	// Both are lipgloss.Color (which is a string type), so we can assign directly
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
	// Convert lipgloss.Color to string for NewColor
	return NewColor(string(c.GetColor(colorKeyOrName)))
}

func (c *Config) ResetColor() lipgloss.Color {
	if c.IsMonochrome {
		return lipgloss.Color("")
	}
	resetColor := c.Colors.Reset
	if resetColor == "" {
		// lipgloss doesn't need explicit reset - styles handle this automatically
		// Return empty color which means "no color"
		return lipgloss.Color("")
	}
	return resetColor
}

// GetStyleWithFallback returns the first available lipgloss.Style from the
// provided color keys. If all color keys are empty or unavailable, an empty
// style is returned. This helper keeps color fallback logic centralized for
// callers that want a preferred color but need a graceful secondary option.
func (c *Config) GetStyleWithFallback(colorKeys ...string) lipgloss.Style {
	if c.IsMonochrome {
		return lipgloss.NewStyle()
	}

	for _, key := range colorKeys {
		if key == "" {
			continue
		}
		color := c.GetColor(key)
		if color == "" {
			continue
		}
		return lipgloss.NewStyle().Foreground(color)
	}

	return lipgloss.NewStyle()
}

// initCache initializes the style cache if not already initialized.
func (c *Config) initCache() {
	if c.cache == nil {
		c.cache = &styleCache{
			colors:   make(map[string]lipgloss.Style),
			elements: make(map[string]lipgloss.Style),
		}
	}
}

// GetStyle returns a lipgloss.Style for the given color key.
// Styles are cached for performance - subsequent calls return the same Style object.
// Returns a style with foreground color set, or empty style if monochrome or color not found.
func (c *Config) GetStyle(colorKey string) lipgloss.Style {
	if c.IsMonochrome {
		return lipgloss.NewStyle()
	}

	// Check cache first
	c.initCache()
	if cached, ok := c.cache.colors[colorKey]; ok {
		return cached
	}

	// Build and cache the style
	color := c.GetColor(colorKey)
	var style lipgloss.Style
	if color == "" {
		style = lipgloss.NewStyle()
	} else {
		style = lipgloss.NewStyle().Foreground(color)
	}
	c.cache.colors[colorKey] = style
	return style
}

// GetStyleWithBold returns a lipgloss.Style with color and bold text.
func (c *Config) GetStyleWithBold(colorKey string) lipgloss.Style {
	style := c.GetStyle(colorKey)
	if c.IsMonochrome {
		return style
	}
	return style.Bold(true)
}

// GetStyleWithBackground returns a lipgloss.Style with foreground and background colors.
func (c *Config) GetStyleWithBackground(fgKey, bgKey string) lipgloss.Style {
	if c.IsMonochrome {
		return lipgloss.NewStyle()
	}
	fgColor := c.GetColor(fgKey)
	bgColor := c.GetColor(bgKey)
	style := lipgloss.NewStyle()
	if fgColor != "" {
		style = style.Foreground(fgColor)
	}
	if bgColor != "" {
		style = style.Background(bgColor)
	}
	return style
}

// GetStyleFromElement builds a lipgloss.Style from an ElementStyleDef.
// This is the unified way to convert element styles to lipgloss styles.
// Parameters:
//   - element: The ElementStyleDef containing style configuration
//   - fallbackColorKey: Used if element.ColorFG is empty
//
// Handles: foreground color, background color, bold, italic, and underline.
func (c *Config) GetStyleFromElement(element ElementStyleDef, fallbackColorKey string) lipgloss.Style {
	if c.IsMonochrome {
		return lipgloss.NewStyle()
	}

	style := lipgloss.NewStyle()

	// Apply foreground color
	colorKey := element.ColorFG
	if colorKey == "" {
		colorKey = fallbackColorKey
	}
	if colorKey != "" {
		if color := c.GetColor(colorKey); color != "" {
			style = style.Foreground(color)
		}
	}

	// Apply background color
	if element.ColorBG != "" {
		if bgColor := c.GetColor(element.ColorBG); bgColor != "" {
			style = style.Background(bgColor)
		}
	}

	// Apply text styles
	for _, textStyle := range element.TextStyle {
		switch textStyle {
		case "bold":
			style = style.Bold(true)
		case "italic":
			style = style.Italic(true)
		case "underline":
			style = style.Underline(true)
		}
	}

	return style
}

// BuildStyle is a convenience method that gets an element style by name and builds
// a lipgloss.Style from it. Styles are cached for performance.
// This combines GetElementStyle and GetStyleFromElement.
func (c *Config) BuildStyle(elementName string, fallbackColorKey ...string) lipgloss.Style {
	if c.IsMonochrome {
		return lipgloss.NewStyle()
	}

	// Build cache key including fallback
	fallback := ""
	if len(fallbackColorKey) > 0 {
		fallback = fallbackColorKey[0]
	}
	cacheKey := elementName
	if fallback != "" {
		cacheKey = elementName + ":" + fallback
	}

	// Check cache first
	c.initCache()
	if cached, ok := c.cache.elements[cacheKey]; ok {
		return cached
	}

	// Build and cache the style
	element := c.GetElementStyle(elementName)
	style := c.GetStyleFromElement(element, fallback)
	c.cache.elements[cacheKey] = style
	return style
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
		"orca":            OrcaTheme(),
	}
}

func ApplyMonochromeDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	cfg.IsMonochrome = true
	cfg.Style.UseBoxes = false

	// Set all colors to empty (monochrome mode)
	cfg.Colors.Process = lipgloss.Color("")
	cfg.Colors.Success = lipgloss.Color("")
	cfg.Colors.Warning = lipgloss.Color("")
	cfg.Colors.Error = lipgloss.Color("")
	cfg.Colors.Detail = lipgloss.Color("")
	cfg.Colors.Muted = lipgloss.Color("")
	cfg.Colors.Reset = lipgloss.Color("")
	cfg.Colors.White = lipgloss.Color("")
	cfg.Colors.GreenFg = lipgloss.Color("")
	cfg.Colors.BlueFg = lipgloss.Color("")
	cfg.Colors.BlueBg = lipgloss.Color("")
	cfg.Colors.PaleBlue = lipgloss.Color("")
	cfg.Colors.Bold = lipgloss.Color("")
	cfg.Colors.Italic = lipgloss.Color("")

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
