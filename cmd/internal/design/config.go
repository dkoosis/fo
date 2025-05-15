// cmd/internal/design/config.go
package design

import (
	"fmt"
	"os" // For debug prints to stderr
	"strings"
)

// ToolConfig defines specific settings for a command/tool preset for design purposes
type ToolConfig struct {
	Label          string              `yaml:"label,omitempty"`
	Intent         string              `yaml:"intent,omitempty"`
	OutputPatterns map[string][]string `yaml:"output_patterns,omitempty"`
}

// BorderStyle defines the type of border to use for task output
type BorderStyle string

const (
	BorderLeftOnly   BorderStyle = "left_only"
	BorderLeftDouble BorderStyle = "left_double"
	BorderHeaderBox  BorderStyle = "header_box"
	BorderFull       BorderStyle = "full_box"
	BorderNone       BorderStyle = "none"
	BorderAscii      BorderStyle = "ascii"
)

// ElementStyleDef defines visual styling properties for a specific UI element
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

// Config holds all resolved design system settings for rendering
type Config struct {
	ThemeName    string `yaml:"-"`
	IsMonochrome bool   `yaml:"-"`

	Style struct {
		UseBoxes       bool   `yaml:"use_boxes"`
		Indentation    string `yaml:"indentation"`
		ShowTimestamps bool   `yaml:"show_timestamps"`
		NoTimer        bool   `yaml:"no_timer"`
		Density        string `yaml:"density"`
	} `yaml:"style"`

	Border struct {
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
	} `yaml:"border"`

	Colors struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
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
}

type PatternsRepo struct {
	Intent map[string][]string `yaml:"intent"`
	Output map[string][]string `yaml:"output"`
}

// ensureEscapePrefix checks if a string starts with "33[" and prepends "\033" if so.
func ensureEscapePrefix(s string) string {
	// Debug print to see what this function receives and returns.
	// This should only be active if FO_DEBUG is set.
	if os.Getenv("FO_DEBUG") != "" {
		originalS := s
		defer func() {
			// This defer will execute after the return, showing the final state.
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Input: '%s' (Hex: %x), Output: '%s' (Hex: %x)\n",
				originalS, originalS, s, s)
		}()
	}

	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "\033") { // Already has correct ESCAPE char
		return s
	}
	if strings.HasPrefix(s, "33[") { // Missing initial \0 part of \033
		s = "\033" + s // Prepend the ESCAPE character
		return s
	}
	return s // Return as is if no known malformation
}

func DefaultConfig() *Config {
	return UnicodeVibrantTheme()
}

func NoColorConfig() *Config {
	cfg := AsciiMinimalTheme()
	ApplyMonochromeDefaults(cfg)
	cfg.ThemeName = "no_color_derived_from_ascii"
	return cfg
}

func AsciiMinimalTheme() *Config {
	cfg := &Config{
		ThemeName:    "ascii_minimal",
		IsMonochrome: true,
	}
	cfg.Style.UseBoxes = false
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "compact"
	cfg.Style.NoTimer = false

	cfg.Icons.Start = "[START]"
	cfg.Icons.Success = "[SUCCESS]"
	cfg.Icons.Warning = "[WARNING]"
	cfg.Icons.Error = "[FAILED]"
	cfg.Icons.Info = "[INFO]"
	cfg.Icons.Bullet = "*"

	cfg.Colors = struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
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

	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = false
	cfg.CognitiveLoad.Default = LoadLow

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

	cfg.Icons.Start = "▶️"
	cfg.Icons.Success = "✅"
	cfg.Icons.Warning = "⚠️"
	cfg.Icons.Error = "❌"
	cfg.Icons.Info = "ℹ️"
	cfg.Icons.Bullet = "•"

	cfg.Colors.Process = "\033[0;34m" // Blue
	cfg.Colors.Success = "\033[0;32m" // Green
	cfg.Colors.Warning = "\033[0;33m" // Yellow
	cfg.Colors.Error = "\033[0;31m"   // Red
	cfg.Colors.Detail = "\033[0m"     // Default/Reset
	cfg.Colors.Muted = "\033[2m"      // Dim
	cfg.Colors.Reset = "\033[0m"      // ANSI Reset

	cfg.Border.TaskStyle = BorderLeftDouble
	cfg.Border.HeaderChar = "═"
	cfg.Border.VerticalChar = "│"
	cfg.Border.TopCornerChar = "╒"
	cfg.Border.BottomCornerChar = "└"
	cfg.Border.FooterContinuationChar = "─"
	// ... (other border characters)

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

	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = true
	cfg.CognitiveLoad.Default = LoadMedium
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
	}
	for _, elKey := range knownElements {
		elements[elKey] = ElementStyleDef{}
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
			return "[START]"
		case "success":
			return "[SUCCESS]"
		case "warning":
			return "[WARNING]"
		case "error":
			return "[FAILED]"
		case "info":
			return "[INFO]"
		case "bullet":
			return "*"
		default:
			return ""
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

func (c *Config) GetColor(colorKey string, elementName ...string) string {
	if c.IsMonochrome {
		return ""
	}
	if len(elementName) > 0 && elementName[0] != "" {
		if elemStyle, ok := c.Elements[elementName[0]]; ok && elemStyle.ColorFG != "" {
			return c.resolveColorName(elemStyle.ColorFG)
		}
	}
	return c.resolveColorName(colorKey)
}

// resolveColorName translates a color name or a direct ANSI code string.
// It applies ensureEscapePrefix to the final code.
func (c *Config) resolveColorName(name string) string {
	if c.IsMonochrome { // Should be handled by GetColor, but defensive check.
		return ""
	}

	// Debug print for input to resolveColorName
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Input name: '%s' (Hex: %x)\n", name, name)
	}

	var colorCode string
	switch strings.ToLower(name) {
	case "process":
		colorCode = c.Colors.Process
	case "success":
		colorCode = c.Colors.Success
	case "warning":
		colorCode = c.Colors.Warning
	case "error":
		colorCode = c.Colors.Error
	case "detail":
		colorCode = c.Colors.Detail
	case "muted":
		colorCode = c.Colors.Muted
	default:
		// If 'name' itself is an ANSI code (e.g., "\033[1;31m") or potentially malformed ("33[1;31m"),
		// ensureEscapePrefix will handle it. Otherwise, it's an unknown symbolic name.
		if strings.HasPrefix(name, "\033[") || strings.HasPrefix(name, "33[") {
			colorCode = name // Pass potentially malformed or correct codes to ensureEscapePrefix
		} else {
			colorCode = c.Colors.Detail // Fallback for unknown symbolic names
			if os.Getenv("FO_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Unknown color name '%s', falling back to Detail: '%s'\n", name, colorCode)
			}
		}
	}

	// Debug print for the code chosen by the switch statement.
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Code from switch: '%s' (Hex: %x)\n", colorCode, colorCode)
	}

	finalCode := ensureEscapePrefix(colorCode)

	// Debug print for the final code after ensureEscapePrefix.
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG resolveColorName] Final code after ensureEscapePrefix: '%s' (Hex: %x)\n", finalCode, finalCode)
	}
	return finalCode
}

func (c *Config) ResetColor() string {
	if c.IsMonochrome {
		return ""
	}
	// Debug print for ResetColor input
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG ResetColor] Input c.Colors.Reset: '%s' (Hex: %x)\n", c.Colors.Reset, c.Colors.Reset)
	}
	finalReset := ensureEscapePrefix(c.Colors.Reset)
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG ResetColor] Output finalReset: '%s' (Hex: %x)\n", finalReset, finalReset)
	}
	return finalReset
}

func DeepCopyConfig(original *Config) *Config {
	if original == nil {
		return nil
	}
	copied := *original

	if original.Elements != nil {
		copied.Elements = make(map[string]ElementStyleDef)
		for k, v := range original.Elements {
			copiedTextStyle := make([]string, len(v.TextStyle))
			copy(copiedTextStyle, v.TextStyle)
			v.TextStyle = copiedTextStyle
			copied.Elements[k] = v
		}
	}
	copied.Patterns.Intent = make(map[string][]string)
	for k, v := range original.Patterns.Intent {
		s := make([]string, len(v))
		copy(s, v)
		copied.Patterns.Intent[k] = s
	}
	copied.Patterns.Output = make(map[string][]string)
	for k, v := range original.Patterns.Output {
		s := make([]string, len(v))
		copy(s, v)
		copied.Patterns.Output[k] = s
	}
	if original.Tools != nil {
		copied.Tools = make(map[string]*ToolConfig)
		for k, v := range original.Tools {
			if v == nil {
				copied.Tools[k] = nil
				continue
			}
			toolCfgCopy := *v
			if v.OutputPatterns != nil {
				toolCfgCopy.OutputPatterns = make(map[string][]string)
				for pk, pv := range v.OutputPatterns {
					s := make([]string, len(pv))
					copy(s, pv)
					toolCfgCopy.OutputPatterns[pk] = s
				}
			}
			copied.Tools[k] = &toolCfgCopy
		}
	}
	return &copied
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
	}{}

	asciiMinimalElements := AsciiMinimalTheme().Elements
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
		cfg.Elements[key] = elDef
	}
}
