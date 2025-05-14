// cmd/internal/design/config.go
package design

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

// BorderStyle defines the type of border to use for task output (remains as is)
type BorderStyle string

const (
	BorderLeftOnly   BorderStyle = "left_only"
	BorderLeftDouble BorderStyle = "left_double"
	BorderHeaderBox  BorderStyle = "header_box"
	BorderFull       BorderStyle = "full_box"
	BorderNone       BorderStyle = "none"  // For line-oriented themes or no task framing
	BorderAscii      BorderStyle = "ascii" // ASCII-only equivalent for boxed tasks
)

// --- NEW: ElementStyleDef defines generic style properties for an element ---
type ElementStyleDef struct {
	Prefix          string   `yaml:"prefix,omitempty"`           // e.g., "TARGET: ", "  -> CMD: "
	Suffix          string   `yaml:"suffix,omitempty"`           // e.g., " ..."
	TextContent     string   `yaml:"text_content,omitempty"`     // For status blocks like "SUCCESS", "FAILED"
	TextStyle       []string `yaml:"text_style,omitempty"`       // ["bold", "italic", "underline", "dim"]
	ColorFG         string   `yaml:"color_fg,omitempty"`         // ANSI code or color name
	ColorBG         string   `yaml:"color_bg,omitempty"`         // ANSI code or color name
	IconKey         string   `yaml:"icon_key,omitempty"`         // Key to look up in Icons map (e.g., "success", "start")
	LineChar        string   `yaml:"line_char,omitempty"`        // For H1/H2 lines
	LineLengthType  string   `yaml:"line_length_type,omitempty"` // "full_width", "dynamic_to_label", "fixed"
	FixedLength     int      `yaml:"fixed_length,omitempty"`
	TextCase        string   `yaml:"text_case,omitempty"` // "upper", "lower", "title", "none"
	DateTimeFormat  string   `yaml:"date_time_format,omitempty"`
	Alignment       string   `yaml:"alignment,omitempty"`        // "left", "center", "right" (for tables)
	PaddingChar     string   `yaml:"padding_char,omitempty"`     // For padding around labels in headers
	FillChar        string   `yaml:"fill_char,omitempty"`        // For filling lines in headers
	AdditionalChars string   `yaml:"additional_chars,omitempty"` // e.g., "  | " for stdout prefix
}

// Config holds all resolved design system settings for rendering.
// This struct is populated based on the selected theme.
type Config struct {
	// Top-level theme information (optional, for reference)
	ThemeName    string `yaml:"-"` // Name of the theme this config represents
	IsMonochrome bool   `yaml:"-"` // True if colors should be stripped/ignored

	// General style properties
	Style struct {
		UseBoxes       bool   `yaml:"use_boxes"`       // Master switch for task container: true for boxed, false for line-oriented
		Indentation    string `yaml:"indentation"`     // Base indent unit (e.g., "  ")
		ShowTimestamps bool   `yaml:"show_timestamps"` // For Fo_Banner Start/End times
		NoTimer        bool   `yaml:"no_timer"`        // For individual task timers
		Density        string `yaml:"density"`         // "compact", "balanced", "relaxed" (affects blank lines, padding)
		// ReduceContrast, etc. can be added if implemented
	} `yaml:"style"`

	// Border characters - primarily for table rendering now if Style.UseBoxes is false,
	// or for specific Task_Container border styles if Style.UseBoxes is true.
	Border struct {
		// For Task Containers (if Style.UseBoxes=true and a specific BorderStyle is chosen)
		TaskStyle              BorderStyle `yaml:"task_style"` // e.g., "left_double", "ascii", "none"
		HeaderChar             string      `yaml:"header_char"`
		VerticalChar           string      `yaml:"vertical_char"`
		TopCornerChar          string      `yaml:"top_corner_char"`
		BottomCornerChar       string      `yaml:"bottom_corner_char"`
		FooterContinuationChar string      `yaml:"footer_continuation_char"` // e.g. "─" in "└─"

		// For Tables (always boxed)
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

	// Color palette (ANSI codes or color names to be resolved)
	Colors struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"` // Default text
		Muted   string `yaml:"muted"`  // Dimmed text
		Reset   string `yaml:"reset"`

		// Specific element colors can be added if they don't fit generic categories
		FoBannerFG       string `yaml:"fo_banner_fg,omitempty"`
		TargetTitleFG    string `yaml:"target_title_fg,omitempty"`
		CmdLineFG        string `yaml:"cmd_line_fg,omitempty"`
		StatusDurationFG string `yaml:"status_duration_fg,omitempty"`
		TableHeaderFG    string `yaml:"table_header_fg,omitempty"`
		TableHeaderBG    string `yaml:"table_header_bg,omitempty"`
	} `yaml:"colors"`

	// Icon symbols
	Icons struct {
		Start   string `yaml:"start"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Info    string `yaml:"info"`
		Bullet  string `yaml:"bullet"` // For summary items
		// Spinner Chars could go here if only one spinner style is supported
		// SpinnerChars []string `yaml:"spinner_chars,omitempty"`
	} `yaml:"icons"`

	// Granular styles for specific elements (keyed by element name from our list A-G)
	// This allows themes to define very specific looks for each part.
	Elements map[string]ElementStyleDef `yaml:"elements"`

	// --- Existing fields from your design.Config ---
	CognitiveLoad struct {
		AutoDetect bool                 `yaml:"auto_detect"`
		Default    CognitiveLoadContext `yaml:"default"`
	} `yaml:"cognitive_load"`

	Patterns struct { // For recognition - less about theme, more about fo's behavior
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	} `yaml:"patterns"`

	Tools map[string]*ToolConfig `yaml:"tools"` // Tool-specific parsing hints & style overrides
}

// ToolConfig remains as you have it, for tool-specific behaviors and pattern overrides
type ToolConfig struct {
	Label          string              `yaml:"label"`
	Intent         string              `yaml:"intent"`
	Stream         bool                `yaml:"stream"` // Execution override
	OutputPatterns map[string][]string `yaml:"output_patterns"`
	// Could add: DefaultThemeElementOverrides map[string]ElementStyleDef `yaml:"element_style_overrides"`
	Layout struct {
		GroupByType bool `yaml:"group_by_type"`
	} `yaml:"layout"`
}

// --- THEME CONSTRUCTORS ---

// baseTheme provides foundational settings common to many themes or as a fallback.
func baseTheme() *Config {
	cfg := &Config{
		ThemeName:    "base",
		IsMonochrome: false, // Will be set by theme resolver
		Style: struct {
			UseBoxes       bool   `yaml:"use_boxes"`
			Indentation    string `yaml:"indentation"`
			ShowTimestamps bool   `yaml:"show_timestamps"`
			NoTimer        bool   `yaml:"no_timer"`
			Density        string `yaml:"density"`
		}{
			Indentation:    "  ",
			ShowTimestamps: true,  // Show FO start/end times by default
			NoTimer:        false, // Show individual task timers by default
			Density:        "balanced",
		},
		Border: struct {
			TaskStyle              BorderStyle `yaml:"task_style"`
			HeaderChar             string      `yaml:"header_char"`
			VerticalChar           string      `yaml:"vertical_char"`
			TopCornerChar          string      `yaml:"top_corner_char"`
			BottomCornerChar       string      `yaml:"bottom_corner_char"`
			FooterContinuationChar string      `yaml:"footer_continuation_char"`
			Table_HChar            string      `yaml:"table_h_char"`
			Table_VChar            string      `yaml:"table_v_char"`
			Table_XChar            string      `yaml:"table_x_char"`
			Table_Corner_TL        string      `yaml:"table_corner_tl"`
			Table_Corner_TR        string      `yaml:"table_corner_tr"`
			Table_Corner_BL        string      `yaml:"table_corner_bl"`
			Table_Corner_BR        string      `yaml:"table_corner_br"`
			Table_T_Down           string      `yaml:"table_t_down"`
			Table_T_Up             string      `yaml:"table_t_up"`
			Table_T_Left           string      `yaml:"table_t_left"`
			Table_T_Right          string      `yaml:"table_t_right"`
		}{
			Table_HChar: "-", Table_VChar: "|", Table_XChar: "+",
			Table_Corner_TL: "+", Table_Corner_TR: "+", Table_Corner_BL: "+", Table_Corner_BR: "+",
			Table_T_Down: "+", Table_T_Up: "+", Table_T_Left: "+", Table_T_Right: "+",
		},
		Colors: struct { // Default to no colors unless overridden
			Process          string `yaml:"process"`
			Success          string `yaml:"success"`
			Warning          string `yaml:"warning"`
			Error            string `yaml:"error"`
			Detail           string `yaml:"detail"`
			Muted            string `yaml:"muted"`
			Reset            string `yaml:"reset"`
			FoBannerFG       string `yaml:"fo_banner_fg,omitempty"`
			TargetTitleFG    string `yaml:"target_title_fg,omitempty"`
			CmdLineFG        string `yaml:"cmd_line_fg,omitempty"`
			StatusDurationFG string `yaml:"status_duration_fg,omitempty"`
			TableHeaderFG    string `yaml:"table_header_fg,omitempty"`
			TableHeaderBG    string `yaml:"table_header_bg,omitempty"`
		}{Reset: ""}, // All others empty for base
		Icons: struct {
			Start   string `yaml:"start"`
			Success string `yaml:"success"`
			Warning string `yaml:"warning"`
			Error   string `yaml:"error"`
			Info    string `yaml:"info"`
			Bullet  string `yaml:"bullet"`
		}{Bullet: "*"}, // All others empty for base
		Elements: make(map[string]ElementStyleDef), // Initialize map
		// Patterns and Tools would be loaded from global config, not usually theme-specific
		Patterns: struct {
			Intent map[string][]string `yaml:"intent"`
			Output map[string][]string `yaml:"output"`
		}{Intent: make(map[string][]string), Output: make(map[string][]string)},
		Tools: make(map[string]*ToolConfig),
		CognitiveLoad: struct {
			AutoDetect bool                 `yaml:"auto_detect"`
			Default    CognitiveLoadContext `yaml:"default"`
		}{AutoDetect: true, Default: LoadMedium},
	}
	// Populate cfg.Elements with very basic defaults for all known element keys
	// This is important so render.go doesn't crash if a theme doesn't define an element.
	// Example for a few:
	cfg.Elements["Fo_Banner_Top_Line_FoProcessing"] = ElementStyleDef{Prefix: "FO: PROCESSING "}
	cfg.Elements["Fo_Timestamp_Start"] = ElementStyleDef{Prefix: "FO: STARTED AT ", DateTimeFormat: "2006-01-02 15:04:05"}
	// ... and so on for all elements from list A-G.
	// This can be a helper function initBaseElementStyles(cfg.Elements)
	initBaseElementStyles(cfg.Elements)

	return cfg
}

// AsciiMinimalTheme defines a pure ASCII, monochrome, line-oriented theme.
func AsciiMinimalTheme() *Config {
	cfg := baseTheme() // Start with base defaults
	cfg.ThemeName = "ascii_minimal"
	cfg.IsMonochrome = true

	cfg.Style.UseBoxes = false // Line-oriented tasks
	cfg.Style.Indentation = "  "
	cfg.Style.NoTimer = false // Theme prefers timers

	cfg.Icons = Icons{
		Start: "[>]", Success: "[OK]", Warning: "[WW]", Error: "[!!]", Info: "[ii]", Bullet: "*",
	}
	cfg.Colors = Colors{Reset: ""} // All colors are empty/none

	// Task borders are not used due to UseBoxes: false, but table borders are
	cfg.Border.TaskStyle = BorderNone
	cfg.Border.Table_HChar = "-"
	cfg.Border.Table_VChar = "|"
	cfg.Border.Table_XChar = "+"
	// ... set all table corner/T chars to "+" for pure ASCII

	// --- Define Element Styles for ascii_minimal ---
	elements := cfg.Elements // Get the map initialized by baseTheme()
	elements["Fo_Banner_Top"] = ElementStyleDef{LineChar: "=", Prefix: "FO: ", TextStyle: []string{"bold"}}
	elements["Fo_Banner_Bottom"] = ElementStyleDef{LineChar: "=", Prefix: "FO: ", TextStyle: []string{"bold"}}

	elements["Fo_OverallStatus_Success"] = ElementStyleDef{IconKey: "Success", TextContent: "SUCCESS", TextStyle: []string{"bold"}}
	elements["Fo_OverallStatus_Failed"] = ElementStyleDef{IconKey: "Error", TextContent: "FAILED", TextStyle: []string{"bold"}}
	elements["Fo_OverallStatus_Warnings"] = ElementStyleDef{IconKey: "Warning", TextContent: "WARNINGS", TextStyle: []string{"bold"}}

	elements["H2_Target_Header_Line"] = ElementStyleDef{LineChar: "-", LineLengthType: "full_width"}
	elements["H2_Target_Title"] = ElementStyleDef{Prefix: "TARGET: ", TextCase: "upper", TextStyle: []string{"bold"}}
	elements["H2_Target_Footer_Line"] = ElementStyleDef{
		FramingCharStart: "---- ", FramingCharEnd: " ----",
		InnerTextFormat: "{target_name}: {status_marker} {status_text} (Duration: {duration})", // render.go will parse this
	}

	elements["Command_Line_Prefix"] = ElementStyleDef{Text: "  -> CMD:"}
	elements["Stdout_Line_Prefix"] = ElementStyleDef{Text: "    | "}
	elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{Text: "    ! WARN: "}
	elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{Text: "    X ERROR: "}
	elements["Make_Info_Line_Prefix"] = ElementStyleDef{Text: "INFO: "}

	elements["Status_Label_Prefix"] = ElementStyleDef{Text: "  STAT:"}
	elements["Task_Status_Success_Block"] = ElementStyleDef{IconKey: "Success", TextContent: "PASSED"}
	elements["Task_Status_Failed_Block"] = ElementStyleDef{IconKey: "Error", TextContent: "FAILED"}
	elements["Task_Status_Warning_Block"] = ElementStyleDef{IconKey: "Warning", TextContent: "WARNINGS"}
	elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")"}

	elements["Table_Header_Cell_Text"] = ElementStyleDef{TextStyle: []string{"bold"}}
	// Any element not specified here will use the (very minimal) defaults from baseTheme()

	return cfg
}

// UnicodeVibrantTheme defines a richer theme with Unicode and colors.
// This would be similar to your current design.DefaultConfig().
func UnicodeVibrantTheme() *Config {
	cfg := baseTheme()
	cfg.ThemeName = "unicode_vibrant"
	cfg.IsMonochrome = false // This theme uses color

	cfg.Style.UseBoxes = true // Boxed tasks
	cfg.Style.Indentation = "  "
	cfg.Style.NoTimer = false

	cfg.Icons = Icons{
		Start: "▶️", Success: "✅", Warning: "⚠️", Error: "❌", Info: "ℹ️", Bullet: "•",
	}
	cfg.Colors = Colors{
		Process: "\033[0;34m", Success: "\033[0;32m", Warning: "\033[0;33m",
		Error: "\033[0;31m", Detail: "\033[0m", Muted: "\033[2m", Reset: "\033[0m",
		FoBannerFG: "\033[0;34m", TargetTitleFG: "\033[1;36m", // Bold Cyan for target titles
		StatusDurationFG: "\033[2m",    // Muted
		TableHeaderFG:    "\033[1;34m", // Bold Blue
	}

	cfg.Border.TaskStyle = BorderLeftDouble // Your current default
	cfg.Border.HeaderChar = "═"
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╒"
	cfg.Border.BottomCornerChar = "└"
	cfg.Border.FooterContinuationChar = "─"

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

	// --- Define Element Styles for unicode_vibrant ---
	elements := cfg.Elements
	elements["Fo_Banner_Top"] = ElementStyleDef{LineChar: "═", Prefix: "FO: ", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.FoBannerFG}
	elements["Fo_Banner_Bottom"] = ElementStyleDef{LineChar: "═", Prefix: "FO: ", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.FoBannerFG}

	elements["Fo_OverallStatus_Success"] = ElementStyleDef{IconKey: "Success", TextContent: "SUCCESS", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.Success}
	elements["Fo_OverallStatus_Failed"] = ElementStyleDef{IconKey: "Error", TextContent: "FAILED", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.Error}
	elements["Fo_OverallStatus_Warnings"] = ElementStyleDef{IconKey: "Warning", TextContent: "WARNINGS", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.Warning}

	// Task_Label_Header will be rendered by render.go using the Border settings and specific label content.
	// The color for the label can be implicitly theme.Colors.Detail or a specific ElementStyleDef for Task_Label_Header_Text.
	elements["Task_Label_Header"] = ElementStyleDef{TextCase: "upper", TextStyle: []string{"bold"}, ColorFG: cfg.Colors.TargetTitleFG}
	elements["Task_StartIndicator_Line"] = ElementStyleDef{IconKey: "Start" /* text_process_label handled by getProcessLabel */, TextStyle: []string{}, ColorFG: cfg.Colors.Process}

	// For boxed themes, prefixes often just an indent or nothing after the vertical border char.
	// The render function will combine border.VerticalChar with indent + element prefix.
	elements["Stdout_Line_Prefix"] = ElementStyleDef{AdditionalChars: "  "}                             // Added after border char
	elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{IconKey: "Warning", AdditionalChars: "  "} // Icon here + color on text
	elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{IconKey: "Error", AdditionalChars: "  "}     // Icon here + color on text
	elements["Make_Info_Line_Prefix"] = ElementStyleDef{IconKey: "Info", Text: " "}                     // Icon + space

	elements["Task_Content_Stderr_Warning_Text"] = ElementStyleDef{ColorFG: cfg.Colors.Warning}
	elements["Task_Content_Stderr_Error_Text"] = ElementStyleDef{ColorFG: cfg.Colors.Error} // render.go might add italics

	elements["Status_Label_Prefix"] = ElementStyleDef{Text: ""} // No explicit "STAT:" in boxed mode usually
	elements["Task_Status_Success_Block"] = ElementStyleDef{IconKey: "Success", TextContent: "Complete", ColorFG: cfg.Colors.Success}
	elements["Task_Status_Failed_Block"] = ElementStyleDef{IconKey: "Error", TextContent: "Failed", ColorFG: cfg.Colors.Error}
	elements["Task_Status_Warning_Block"] = ElementStyleDef{IconKey: "Warning", TextContent: "Completed with warnings", ColorFG: cfg.Colors.Warning}
	elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")", ColorFG: cfg.Colors.Muted}

	elements["Table_Header_Cell_Text"] = ElementStyleDef{TextStyle: []string{"bold"}, ColorFG: cfg.Colors.TableHeaderFG}
	elements["Table_Body_Cell_Text"] = ElementStyleDef{ColorFG: cfg.Colors.Detail}

	// Specific patterns/tools from your existing design.Config
	cfg.Patterns = yourCurrentDesignConfigPatterns()
	cfg.Tools = yourCurrentDesignConfigTools()
	cfg.CognitiveLoad = yourCurrentDesignConfigCognitiveLoad()

	return cfg
}

// Helper to initialize base element styles to avoid nil map access
// This should define ALL known element keys from our list (A-G) with minimal defaults.
func initBaseElementStyles(elements map[string]ElementStyleDef) {
	// A. Global / fo Meta Elements
	elements["Fo_Banner_Top"] = ElementStyleDef{LineChar: "=", Prefix: "FO: "}
	elements["Fo_Banner_Top_Line_FoProcessing"] = ElementStyleDef{Prefix: "FO: PROCESSING "}
	elements["Fo_Timestamp_Start"] = ElementStyleDef{Prefix: "FO: STARTED AT ", DateTimeFormat: "2006-01-02 15:04:05"}
	elements["Fo_Banner_Bottom"] = ElementStyleDef{LineChar: "=", Prefix: "FO: "}
	elements["Fo_Banner_Bottom_Line_FoBuildSummaryTitle"] = ElementStyleDef{Prefix: "FO: BUILD SUMMARY"}
	elements["Fo_Banner_Bottom_Line_OverallStatusLabel"] = ElementStyleDef{TextContent: "Overall Status:", Suffix: " "}
	elements["Fo_Banner_Bottom_OverallStatus_Success"] = ElementStyleDef{IconKey: "Success", TextContent: "SUCCESS"}
	elements["Fo_Banner_Bottom_OverallStatus_Failed"] = ElementStyleDef{IconKey: "Error", TextContent: "FAILED"}
	elements["Fo_Banner_Bottom_OverallStatus_Warnings"] = ElementStyleDef{IconKey: "Warning", TextContent: "WARNINGS"}
	elements["Fo_Banner_Bottom_Summary_ItemLabel"] = ElementStyleDef{}
	elements["Fo_Banner_Bottom_Summary_ItemValue"] = ElementStyleDef{}
	elements["Fo_Banner_Bottom_Line_EndTime"] = ElementStyleDef{Prefix: "FO: COMPLETED AT ", DateTimeFormat: "2006-01-02 15:04:05"}

	// B. Task Block Elements
	elements["Task_Container"] = ElementStyleDef{} // Border style name will be in cfg.Border.TaskStyle
	elements["Task_Label_Header"] = ElementStyleDef{TextCase: "upper"}
	elements["Task_StartIndicator_Line"] = ElementStyleDef{IconKey: "Start"}
	elements["H2_Target_Header_Line"] = ElementStyleDef{LineChar: "-"} // For line-oriented themes
	elements["H2_Target_Title"] = ElementStyleDef{Prefix: "TARGET: "}
	elements["H2_Target_Footer_Line"] = ElementStyleDef{FramingCharStart: "---- ", FramingCharEnd: " ----"}

	// C. Task Content Line Elements
	elements["Command_Line_Prefix"] = ElementStyleDef{Text: "  -> CMD:"}
	elements["Stdout_Line_Prefix"] = ElementStyleDef{Text: "    | "}
	elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{IconKey: "Warning", Text: "    ! WARN: "}
	elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{IconKey: "Error", Text: "    X ERROR: "}
	elements["Make_Info_Line_Prefix"] = ElementStyleDef{IconKey: "Info", Text: "INFO: "}
	elements["Task_Content_Stdout_Text"] = ElementStyleDef{}
	elements["Task_Content_Stderr_Warning_Text"] = ElementStyleDef{}
	elements["Task_Content_Stderr_Error_Text"] = ElementStyleDef{}
	elements["Task_Content_Summary_Heading"] = ElementStyleDef{TextContent: "SUMMARY:", TextStyle: []string{"bold"}}
	elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{BulletChar: "*"}
	elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{BulletChar: "*"}

	// D. Task Status & Footer Elements
	elements["Status_Label_Prefix"] = ElementStyleDef{Text: "  STAT:"}
	elements["Task_Status_Success_Block"] = ElementStyleDef{IconKey: "Success", TextContent: "PASSED"}
	elements["Task_Status_Failed_Block"] = ElementStyleDef{IconKey: "Error", TextContent: "FAILED"}
	elements["Task_Status_Warning_Block"] = ElementStyleDef{IconKey: "Warning", TextContent: "WARNINGS"}
	elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")"}

	// E. Tool-Specific (placeholders, actual definitions come from theme or tool config)
	elements["ToolSpecific_Generic_Error"] = ElementStyleDef{}
	elements["ToolSpecific_Generic_Warning"] = ElementStyleDef{}

	// F. Progress Indicator Elements
	elements["ProgressIndicator_Spinner_Chars"] = ElementStyleDef{Text: "|/-\\"} // Default spinner chars
	elements["ProgressIndicator_Text_Style"] = ElementStyleDef{}

	// G. Tabular Elements
	elements["Table_Header_Cell_Text"] = ElementStyleDef{TextStyle: []string{"bold"}}
	elements["Table_Body_Cell_Text"] = ElementStyleDef{}
	// (Table border chars are in cfg.Border)
}

// Helper to get an element's style, falling back to a default if not defined in theme
func (cfg *Config) GetElementStyle(elementName string) ElementStyleDef {
	if style, ok := cfg.Elements[elementName]; ok {
		return style
	}
	// Fallback to a very basic, empty style if not found to prevent panics
	// This shouldn't happen if initBaseElementStyles is comprehensive.
	// fmt.Fprintf(os.Stderr, "Warning: Style element '%s' not found in theme '%s'. Using empty default.\n", elementName, cfg.ThemeName)
	return ElementStyleDef{}
}

// Helper to get an icon string, considering monochrome mode
func (cfg *Config) GetIcon(iconKey string) string {
	if cfg.IsMonochrome { // Use ASCII fallbacks for all icons in monochrome
		switch iconKey {
		case "Start":
			return "[>]"
		case "Success":
			return "[OK]"
		case "Warning":
			return "[WW]"
		case "Error":
			return "[!!]"
		case "Info":
			return "[ii]"
		case "Bullet":
			return "*"
		default:
			return ""
		}
	}
	// Return theme-defined icon
	switch iconKey {
	case "Start":
		return cfg.Icons.Start
	case "Success":
		return cfg.Icons.Success
	case "Warning":
		return cfg.Icons.Warning
	case "Error":
		return cfg.Icons.Error
	case "Info":
		return cfg.Icons.Info
	case "Bullet":
		return cfg.Icons.Bullet
	default:
		return ""
	}
}

// Helper to get a color string, considering monochrome mode
func (cfg *Config) GetColor(colorKey string, forElement ...string) string {
	if cfg.IsMonochrome || cfg.Colors.Reset == "" { // Also treat as monochrome if Reset is not set
		return "" // No color codes in monochrome
	}
	// This can be made more sophisticated to check cfg.Elements[forElement...].ColorFG first
	switch colorKey {
	case "Process":
		return cfg.Colors.Process
	case "Success":
		return cfg.Colors.Success
	case "Warning":
		return cfg.Colors.Warning
	case "Error":
		return cfg.Colors.Error
	case "Detail":
		return cfg.Colors.Detail
	case "Muted":
		return cfg.Colors.Muted
	case "FoBannerFG":
		return cfg.Colors.FoBannerFG
	case "TargetTitleFG":
		return cfg.Colors.TargetTitleFG
		// ... add other specific color lookups as needed
	default:
		return ""
	}
}

// ResetColor returns the reset code, empty if monochrome
func (cfg *Config) ResetColor() string {
	if cfg.IsMonochrome || cfg.Colors.Reset == "" {
		return ""
	}
	return cfg.Colors.Reset
}

// getIndentation returns the appropriate indentation string (remains as is)
func (c *Config) GetIndentation(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(c.Style.Indentation, level)
}

// --- Functions to load your existing patterns and tool configs ---
// These are placeholders; you'd copy your actual initialization for these from your
// current design.DefaultConfig() or wherever they are defined.
func yourCurrentDesignConfigPatterns() struct {
	Intent map[string][]string `yaml:"intent"`
	Output map[string][]string `yaml:"output"`
} {
	return struct {
		Intent map[string][]string `yaml:"intent"`
		Output map[string][]string `yaml:"output"`
	}{
		Intent: map[string][]string{
			"building": {"go build", "make", "gcc"}, "testing": {"go test"}, "linting": {"golangci-lint"},
		},
		Output: map[string][]string{
			"error": {"^Error:", "^ERROR:", "failed", "panic:"}, "warning": {"^Warning:", "^WARNING:", "deprecated"},
		},
	}
}
func yourCurrentDesignConfigTools() map[string]*ToolConfig {
	return make(map[string]*ToolConfig)
}
func yourCurrentDesignConfigCognitiveLoad() struct {
	AutoDetect bool
	Default    CognitiveLoadContext
} {
	return struct {
		AutoDetect bool
		Default    CognitiveLoadContext
	}{AutoDetect: true, Default: LoadMedium}
}

// (Your existing CognitiveLoadContext, LineType, TaskStatus constants would remain here)

// AppConfig holds the application-level configuration, including theme management.
// This struct is what's populated from .fo.yaml, environment variables, and CLI flags.
type AppConfig struct {
	// Execution behavior settings (can be influenced by CLI flags and presets)
	Label         string `yaml:"-"` // Set by CLI flag or preset, not directly in top-level fo.yaml usually
	Stream        bool   `yaml:"stream"`
	ShowOutput    string `yaml:"show_output"` // "on-fail", "always", "never"
	NoTimer       bool   `yaml:"no_timer"`    // Global override for timer display
	NoColor       bool   `yaml:"no_color"`    // Global override for color display
	CI            bool   `yaml:"ci"`          // Global override for CI mode (implies no_color, no_timer, simpler theme)
	Debug         bool   `yaml:"debug"`
	MaxBufferSize int64  `yaml:"max_buffer_size"`
	MaxLineLength int    `yaml:"max_line_length"`

	// Theme management
	ActiveThemeName string                    `yaml:"active_theme"` // Name of the theme to use by default from the themes map
	Themes          map[string]*design.Config `yaml:"themes"`       // All themes defined in the config file
	EffectiveTheme  string                    `yaml:"-"`            // The actual theme name to be used after CLI/env override

	// Command-specific presets
	Presets map[string]Preset `yaml:"presets"`
}

// Preset represents command-specific configuration overrides.
type Preset struct {
	Label      string `yaml:"label,omitempty"`
	Stream     *bool  `yaml:"stream,omitempty"`
	ShowOutput string `yaml:"show_output,omitempty"`
	NoTimer    *bool  `yaml:"no_timer,omitempty"`
	// Presets could also suggest a theme_name if desired:
	// ThemeName  string `yaml:"theme_name,omitempty"`
}

// Default values for buffer sizes
const (
	DefaultMaxBufferSize int64 = 10 * 1024 * 1024 // 10MB
	DefaultMaxLineLength int   = 1 * 1024 * 1024  // 1MB
)

// defaultThemeName is used if no theme is specified or found.
const defaultThemeName = "unicode_vibrant" // Or "ascii_minimal" if you prefer that as absolute default

// NewDefaultAppConfig returns a new AppConfig with sensible defaults,
// including definitions for built-in themes.
func NewDefaultAppConfig() *AppConfig {
	cfg := &AppConfig{
		ShowOutput:      "on-fail",
		MaxBufferSize:   DefaultMaxBufferSize,
		MaxLineLength:   DefaultMaxLineLength,
		ActiveThemeName: defaultThemeName,
		Themes:          make(map[string]*design.Config),
		Presets:         make(map[string]Preset),
	}

	// Populate with built-in themes. These functions must exist in the design package.
	// These will be used if a config file isn't found or doesn't define them.
	// The keys here MUST match the theme names used in active_theme or by CLI/env.
	cfg.Themes["ascii_minimal"] = design.AsciiMinimalTheme()
	cfg.Themes["unicode_vibrant"] = design.UnicodeVibrantTheme()
	// Add a CI-specific theme if you have one, or CI mode will modify a base theme.
	// cfg.Themes["ci_theme"] = design.CITheme()

	return cfg
}

// LoadGlobalConfig loads configuration from standard file locations and environment variables.
// It starts with defaults, then layers file config, then environment overrides.
// CLI flag overrides are handled separately by the MergeCliWithAppConfig function.
func LoadGlobalConfig() *AppConfig {
	// Start with a configuration that includes built-in default themes
	appCfg := NewDefaultAppConfig()

	configLocations := []string{
		".fo.yaml",
		".fo.yml",
		filepath.Join(os.UserHomeDir(), ".config", "fo", "config.yaml"),
		filepath.Join(os.UserHomeDir(), ".config", "fo", ".fo.yaml"), // Common alternative
		filepath.Join(os.UserHomeDir(), ".fo.yaml"),
	}

	loadedFromFile := false
	for _, location := range configLocations {
		expandedPath := expandPath(location)
		if _, err := os.Stat(expandedPath); err == nil {
			data, errFile := os.ReadFile(expandedPath)
			if errFile == nil {
				// Create a temporary config to unmarshal into, so we don't overwrite defaults partially on error
				tempCfg := NewDefaultAppConfig() // Start with fresh defaults for this attempt
				if errYaml := yaml.Unmarshal(data, tempCfg); errYaml == nil {
					// Successfully unmarshalled, now merge.
					// File's top-level settings override initial defaults.
					appCfg.Stream = tempCfg.Stream
					appCfg.ShowOutput = tempCfg.ShowOutput
					appCfg.NoTimer = tempCfg.NoTimer
					appCfg.NoColor = tempCfg.NoColor
					appCfg.CI = tempCfg.CI
					appCfg.Debug = tempCfg.Debug
					if tempCfg.MaxBufferSize > 0 {
						appCfg.MaxBufferSize = tempCfg.MaxBufferSize
					}
					if tempCfg.MaxLineLength > 0 {
						appCfg.MaxLineLength = tempCfg.MaxLineLength
					}
					if tempCfg.ActiveThemeName != "" {
						appCfg.ActiveThemeName = tempCfg.ActiveThemeName
					}

					// Merge presets (file presets add to or override default presets)
					for k, v := range tempCfg.Presets {
						appCfg.Presets[k] = v
					}
					// Merge themes (file themes add to or override built-in themes)
					for k, v := range tempCfg.Themes {
						appCfg.Themes[k] = v
					}
					loadedFromFile = true
					break // Stop after first successful load
				} else {
					fmt.Fprintf(os.Stderr, "fo: warning: could not parse config file %s: %v\n", expandedPath, errYaml)
				}
			}
		}
	}

	if !loadedFromFile {
		// This message can be helpful for users wondering where config comes from
		// fmt.Fprintln(os.Stderr, "fo: notice: No .fo.yaml configuration file found or usable. Using internal default settings and themes.")
	}

	applyEnvironmentOverrides(appCfg) // Environment variables override file/defaults
	return appCfg
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[1:])
		}
		// If home dir can't be found, return path as is, os.Stat will fail later
	}
	return path
}

// applyEnvironmentOverrides applies configuration from environment variables.
// These override settings from the config file or defaults.
func applyEnvironmentOverrides(config *AppConfig) {
	if val := os.Getenv("FO_STREAM"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.Stream = b
		}
	}
	if val := os.Getenv("FO_SHOW_OUTPUT"); val != "" {
		if val == "on-fail" || val == "always" || val == "never" {
			config.ShowOutput = val
		}
	}
	if val := os.Getenv("FO_NO_TIMER"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.NoTimer = b
		}
	}
	if val := os.Getenv("FO_NO_COLOR"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.NoColor = b
		}
	}
	if val := os.Getenv("FO_DEBUG"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			config.Debug = b
		}
	}
	if val := os.Getenv("FO_MAX_BUFFER_SIZE"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil && i > 0 {
			config.MaxBufferSize = i
		}
	}
	if val := os.Getenv("FO_MAX_LINE_LENGTH"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 32); err == nil && i > 0 {
			config.MaxLineLength = int(i)
		}
	}
	if val := os.Getenv("FO_THEME"); val != "" {
		config.ActiveThemeName = val // Env var overrides active_theme from file
	}

	// CI environment variable implies --ci behavior
	if val := os.Getenv("CI"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil && b {
			config.CI = true
		}
	}
	// If CI is true (from env or flag later), it will also imply NoColor and NoTimer
}

// CliFlags represents the values passed via command-line flags.
// It also tracks if a flag was explicitly set by the user.
type CliFlags struct {
	Label         string
	Stream        bool
	StreamSet     bool
	ShowOutput    string
	ShowOutputSet bool
	NoTimer       bool
	NoTimerSet    bool
	NoColor       bool
	NoColorSet    bool
	CI            bool
	CISet         bool
	Debug         bool
	DebugSet      bool
	ThemeName     string // From --theme
	MaxBufferSize int64  // Value from flag (MB, converted to bytes)
	MaxLineLength int    // Value from flag (KB, converted to bytes)
}

// MergeCliWithAppConfig merges CLI flags into the AppConfig.
// CLI flags generally take the highest precedence.
func MergeCliWithAppConfig(appCfg *AppConfig, cli CliFlags) {
	// Label is handled directly in main.go from CLI if provided, or preset, or inferred.
	// appCfg.Label is not directly set here; it's more about effective settings.

	if cli.StreamSet {
		appCfg.Stream = cli.Stream
	}
	if cli.ShowOutputSet {
		appCfg.ShowOutput = cli.ShowOutput
	}
	if cli.NoTimerSet {
		appCfg.NoTimer = cli.NoTimer
	}
	if cli.NoColorSet {
		appCfg.NoColor = cli.NoColor
	}
	if cli.CISet {
		appCfg.CI = cli.CI
	}
	if cli.DebugSet {
		appCfg.Debug = cli.Debug
	}
	if cli.ThemeName != "" {
		appCfg.ActiveThemeName = cli.ThemeName // CLI flag overrides theme from env/file
	}
	if cli.MaxBufferSize > 0 { // Assume flag parsing already converted MB to bytes
		appCfg.MaxBufferSize = cli.MaxBufferSize
	}
	if cli.MaxLineLength > 0 { // Assume flag parsing already converted KB to bytes
		appCfg.MaxLineLength = cli.MaxLineLength
	}

	// CI mode implies NoColor and NoTimer, overriding other settings for these
	if appCfg.CI {
		appCfg.NoColor = true
		appCfg.NoTimer = true
	}
}

// ApplyCommandPreset modifies the AppConfig based on presets for the given command.
// This should be called *after* CLI flags are merged if CLI flags for these
// specific fields (Label, Stream, ShowOutput, NoTimer) should override presets.
// Or, call before merging CLI flags if presets should be overridden by CLI.
// Current fo logic: CLI overrides presets. Presets override file/default.
// So, this function would typically be called on the config derived from file/default,
// and then CLI flags are merged on top.
// For simplicity here, we'll assume it modifies the passed config.
func ApplyCommandPreset(config *AppConfig, cmdName string, cliDidSetLabel bool) {
	if len(cmdName) == 0 {
		return
	}
	baseName := filepath.Base(cmdName)
	preset, ok := config.Presets[baseName]
	if !ok {
		// Try with ".sh" suffix if it's a script
		if strings.HasSuffix(cmdName, ".sh") {
			preset, ok = config.Presets[cmdName]
		}
		if !ok {
			return
		}
	}

	// Only apply preset label if CLI did not provide one AND config.Label is still empty
	if !cliDidSetLabel && config.Label == "" && preset.Label != "" {
		config.Label = preset.Label
	}
	// Apply other preset values if they exist
	// These will be overridden by CLI flags if MergeCliWithAppConfig is called later.
	if preset.Stream != nil {
		config.Stream = *preset.Stream
	}
	if preset.ShowOutput != "" {
		config.ShowOutput = preset.ShowOutput
	}
	if preset.NoTimer != nil {
		config.NoTimer = *preset.NoTimer
	}
	// if preset.ThemeName != "" { // If presets could suggest themes
	// 	config.ActiveThemeName = preset.ThemeName
	// }
}

// GetResolvedDesignConfig selects the active theme from the AppConfig,
// applies global overrides (like NoColor, NoTimer from AppConfig which reflect CLI flags),
// and returns the final *design.Config to be used for rendering.
func (ac *AppConfig) GetResolvedDesignConfig() *design.Config {
	themeToLoad := ac.ActiveThemeName
	if themeToLoad == "" { // Should have been set by LoadGlobalConfig or MergeCli
		themeToLoad = defaultThemeName
		fmt.Fprintf(os.Stderr, "fo: warning: no active theme specified, defaulting to '%s'.\n", themeToLoad)
	}

	// Attempt to get the selected theme; fallback to a known default if not found
	baseDesignConfig, themeFound := ac.Themes[themeToLoad]
	if !themeFound {
		fmt.Fprintf(os.Stderr, "fo: warning: theme '%s' not found in configuration. Falling back to internal default theme '%s'.\n", themeToLoad, defaultThemeName)
		baseDesignConfig, themeFound = ac.Themes[defaultThemeName]
		if !themeFound { // Should not happen if NewDefaultAppConfig populates defaults
			fmt.Fprintf(os.Stderr, "fo: critical error: default theme '%s' also not found. Using emergency minimal theme.\n", defaultThemeName)
			baseDesignConfig = design.AsciiMinimalTheme() // Absolute fallback
		}
	}

	// Create a mutable copy to apply global overrides
	// This needs to be a deep enough copy if design.Config has nested structs that will be modified
	finalDesignCfg := *baseDesignConfig // Start with a shallow copy

	// Apply global overrides (NoColor, NoTimer, CI) from AppConfig
	// These AppConfig fields (ac.NoColor, ac.NoTimer, ac.CI) should already
	// reflect the highest precedence settings (CLI > Env > File default).

	isMonochrome := ac.NoColor // This NoColor field in AppConfig is the final say after CLI/env/CI
	showTimer := !ac.NoTimer   // This NoTimer field in AppConfig is the final say

	if isMonochrome {
		// If a theme explicitly named (e.g.) "ascii_minimal_ci" or "selected_theme_monochrome" exists,
		// we could try to load that. For now, transform the loaded theme.

		// Create a truly monochrome config based on the structure of design.NoColorConfig()
		monoDesign := design.NoColorConfig() // This provides the color/icon/style settings for monochrome

		finalDesignCfg.Colors = monoDesign.Colors
		finalDesignCfg.Icons = monoDesign.Icons
		// Decide if monochrome should always force non-boxed style for tasks, or respect theme's box choice
		// For fo, --no-color typically also means simpler ASCII icons and often simpler structure.
		finalDesignCfg.Style.UseBoxes = monoDesign.Style.UseBoxes // Usually false for NoColorConfig
		// Border chars should also come from the monochrome/ASCII set
		finalDesignCfg.Border.HeaderChar = monoDesign.Border.HeaderChar
		finalDesignCfg.Border.VerticalChar = monoDesign.Border.VerticalChar
		finalDesignCfg.Border.TopCornerChar = monoDesign.Border.TopCornerChar
		finalDesignCfg.Border.BottomCornerChar = monoDesign.Border.BottomCornerChar
		// ... and table chars if they differ
	}

	finalDesignCfg.Style.NoTimer = !showTimer

	if ac.CI {
		// Apply any other CI-specific structural simplifications to finalDesignCfg
		// For example, many CI themes prefer no boxes for tasks.
		// design.CITheme() could return a *design.Config with these settings.
		// If Style.UseBoxes is part of the theme, a CI theme would set it to false.
		// Or, as a simpler override:
		// finalDesignCfg.Style.UseBoxes = false // Example direct override for CI
	}

	return &finalDesignCfg
}
