// cmd/internal/design/config.go
package design

import (
	"encoding/json"
	"fmt"
	"os" // For debug prints to stderr
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

// Config holds all resolved design system settings for rendering.
type Config struct {
	ThemeName    string `yaml:"-"`
	IsMonochrome bool   `yaml:"-"`
	CI           bool   `yaml:"-"` // Explicit CI mode flag (takes precedence over heuristics)

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
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
		White   string `yaml:"white,omitempty"`
		BlueFg  string `yaml:"blue_fg,omitempty"`
		BlueBg  string `yaml:"blue_bg,omitempty"`
		Bold    string `yaml:"bold,omitempty"`
		Italic  string `yaml:"italic,omitempty"`
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
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
		White   string `yaml:"white,omitempty"`
		BlueFg  string `yaml:"blue_fg,omitempty"`
		BlueBg  string `yaml:"blue_bg,omitempty"`
		Bold    string `yaml:"bold,omitempty"`
		Italic  string `yaml:"italic,omitempty"`
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

	cfg.Colors.Process = ANSIBrightWhite
	cfg.Colors.Success = ANSIBrightWhite
	cfg.Colors.Warning = "\033[0;33m"
	cfg.Colors.Error = "\033[0;31m"
	cfg.Colors.Detail = "\033[0m"
	cfg.Colors.Muted = "\033[2m"
	cfg.Colors.Reset = "\033[0m"
	cfg.Colors.White = "\033[0;97m"
	cfg.Colors.BlueFg = "\033[0;34m"
	cfg.Colors.BlueBg = "\033[44m"
	cfg.Colors.Bold = "\033[1m"
	cfg.Colors.Italic = "\033[3m"

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
		AdditionalChars: "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏",
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
		case StatusSuccess:
			return IconSuccess
		case StatusWarning:
			return IconWarning
		case StatusError:
			return IconFailed
		case TypeInfo:
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
	case StatusSuccess:
		return c.Icons.Success
	case StatusWarning:
		return c.Icons.Warning
	case StatusError:
		return c.Icons.Error
	case TypeInfo:
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

	if len(elementName) > 0 && elementName[0] != "" {
		if elemStyle, ok := c.Elements[elementName[0]]; ok {
			if elemStyle.ColorFG != "" {
				return c.resolveColorName(elemStyle.ColorFG)
			}
		}
	}
	return c.resolveColorName(colorKeyOrName)
}

func (c *Config) resolveColorName(name string) string {
	if c.IsMonochrome || name == "" {
		return ""
	}

	var codeToProcess string
	lowerName := strings.ToLower(name)

	switch lowerName {
	case "process", StatusSuccess:
		codeToProcess = c.Colors.Process
	case StatusWarning:
		codeToProcess = c.Colors.Warning
	case StatusError:
		codeToProcess = c.Colors.Error
	case "detail":
		codeToProcess = c.Colors.Detail
	case "muted":
		codeToProcess = c.Colors.Muted
	case "reset":
		codeToProcess = c.Colors.Reset
	case "white":
		codeToProcess = c.Colors.White
	case "bluefg":
		codeToProcess = c.Colors.BlueFg
	case "bluebg":
		codeToProcess = c.Colors.BlueBg
	case "bold":
		codeToProcess = c.Colors.Bold
	case "italic":
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
		case "process", StatusSuccess, "white":
			codeToProcess = escChar + "[0;97m"
		case StatusWarning:
			codeToProcess = escChar + "[0;33m"
		case StatusError:
			codeToProcess = escChar + "[0;31m"
		case "detail", "reset":
			codeToProcess = escChar + "[0m"
		case "muted":
			codeToProcess = escChar + "[2m"
		case "bluefg":
			codeToProcess = escChar + "[0;34m"
		case "bluebg":
			codeToProcess = escChar + "[44m"
		case "bold":
			codeToProcess = escChar + "[1m"
		case "italic":
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
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
		White   string `yaml:"white,omitempty"`
		BlueFg  string `yaml:"blue_fg,omitempty"`
		BlueBg  string `yaml:"blue_bg,omitempty"`
		Bold    string `yaml:"bold,omitempty"`
		Italic  string `yaml:"italic,omitempty"`
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
