// cmd/internal/design/config.go
package design

import (
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
	ColorFG          string   `yaml:"color_fg,omitempty"` // Use color names like "Process", "Error"
	ColorBG          string   `yaml:"color_bg,omitempty"` // Use color names
	IconKey          string   `yaml:"icon_key,omitempty"` // Refers to keys in Config.Icons or GetIcon
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
		NoTimer        bool   `yaml:"no_timer"` // This is the effective value after flags
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

	Icons struct { // These are for themed (non-monochrome) icons
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

func DefaultConfig() *Config {
	return UnicodeVibrantTheme()
}

func NoColorConfig() *Config {
	// Start with ASCII minimal as a base for structure, then ensure all color is stripped
	// and specific monochrome settings (like UseBoxes=false) are enforced.
	cfg := AsciiMinimalTheme()   // This already sets IsMonochrome = true, UseBoxes = false
	ApplyMonochromeDefaults(cfg) // Further enforce monochrome properties
	cfg.ThemeName = "no_color_derived_from_ascii"
	return cfg
}

func AsciiMinimalTheme() *Config {
	cfg := &Config{
		ThemeName:    "ascii_minimal",
		IsMonochrome: true, // This theme IS monochrome by definition
	}
	cfg.Style.UseBoxes = false // Critical for test expectations: line-oriented
	cfg.Style.Indentation = "  "
	cfg.Style.ShowTimestamps = false
	cfg.Style.Density = "compact"
	cfg.Style.NoTimer = false // Default state for this theme, CLI flags can override

	// Icons for this theme are actually defined by GetIcon's monochrome path.
	// This section can be empty or align with GetIcon's monochrome output for clarity.
	cfg.Icons.Start = "[START]"
	cfg.Icons.Success = "[SUCCESS]"
	cfg.Icons.Warning = "[WARNING]"
	cfg.Icons.Error = "[FAILED]"
	cfg.Icons.Info = "[INFO]"
	cfg.Icons.Bullet = "*"

	// Colors are all empty for a true monochrome theme.
	cfg.Colors = struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
	}{}

	cfg.Border.TaskStyle = BorderNone // No boxes
	// Other border fields are mostly irrelevant if UseBoxes is false and TaskStyle is None.

	cfg.Elements = make(map[string]ElementStyleDef)
	initBaseElementStyles(cfg.Elements) // Ensure all element keys exist

	// Define element styles specific to ASCII minimal / monochrome behavior.
	// These ensure that RenderStartLine/EndLine/OutputLine behave as expected for tests.
	cfg.Elements["Task_Label_Header"] = ElementStyleDef{}                               // No specific text, Render* funcs handle
	cfg.Elements["Task_StartIndicator_Line"] = ElementStyleDef{}                        // RenderStartLine handles
	cfg.Elements["H2_Target_Title"] = ElementStyleDef{Prefix: "", TextCase: "none"}     // No "TARGET:"
	cfg.Elements["Task_Status_Success_Block"] = ElementStyleDef{TextContent: "Success"} // Plain text used by RenderEndLine
	cfg.Elements["Task_Status_Failed_Block"] = ElementStyleDef{TextContent: "Failed"}
	cfg.Elements["Task_Status_Warning_Block"] = ElementStyleDef{TextContent: "Warnings"}
	cfg.Elements["Task_Status_Duration"] = ElementStyleDef{Prefix: "(", Suffix: ")"}
	cfg.Elements["Stderr_Error_Line_Prefix"] = ElementStyleDef{Text: "  > "}   // Simple prefix for errors
	cfg.Elements["Stderr_Warning_Line_Prefix"] = ElementStyleDef{Text: "  > "} // Simple prefix for warnings
	cfg.Elements["Stdout_Line_Prefix"] = ElementStyleDef{Text: "  "}           // Just indent for plain stdout lines
	cfg.Elements["Task_Content_Summary_Heading"] = ElementStyleDef{TextContent: "SUMMARY:"}
	cfg.Elements["Task_Content_Summary_Item_Error"] = ElementStyleDef{BulletChar: "*"}
	cfg.Elements["Task_Content_Summary_Item_Warning"] = ElementStyleDef{BulletChar: "*"}

	cfg.Patterns = defaultPatterns()
	cfg.Tools = make(map[string]*ToolConfig)
	cfg.CognitiveLoad.AutoDetect = false // No colors to vary
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

	cfg.Colors.Process = "\033[0;34m"
	cfg.Colors.Success = "\033[0;32m"
	cfg.Colors.Warning = "\033[0;33m"
	cfg.Colors.Error = "\033[0;31m"
	cfg.Colors.Detail = "\033[0m"
	cfg.Colors.Muted = "\033[2m"
	cfg.Colors.Reset = "\033[0m"

	cfg.Border.TaskStyle = BorderLeftDouble
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
				"^E!", "^panic:", "^fatal:", "^Failed", // General "Failed" might be too broad
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

func (c *Config) resolveColorName(name string) string {
	if c.IsMonochrome {
		return ""
	}
	switch strings.ToLower(name) {
	case "process":
		return c.Colors.Process
	case "success":
		return c.Colors.Success
	case "warning":
		return c.Colors.Warning
	case "error":
		return c.Colors.Error
	case "detail":
		return c.Colors.Detail
	case "muted":
		return c.Colors.Muted
	default:
		if strings.HasPrefix(name, "\033[") {
			return name
		}
		return c.Colors.Detail // Fallback for unknown names
	}
}

func (c *Config) ResetColor() string {
	if c.IsMonochrome {
		return ""
	}
	return c.Colors.Reset
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
	cfg.Style.UseBoxes = false            // Force line-oriented for monochrome
	cfg.Style.NoTimer = cfg.Style.NoTimer // Preserve NoTimer state, as it can be set by --ci separately

	cfg.Colors = struct {
		Process string `yaml:"process"`
		Success string `yaml:"success"`
		Warning string `yaml:"warning"`
		Error   string `yaml:"error"`
		Detail  string `yaml:"detail"`
		Muted   string `yaml:"muted"`
		Reset   string `yaml:"reset"`
	}{}

	// Use ASCII minimal element definitions as a base for monochrome elements
	asciiMinimalElements := AsciiMinimalTheme().Elements
	if cfg.Elements == nil {
		cfg.Elements = make(map[string]ElementStyleDef)
		initBaseElementStyles(cfg.Elements) // Ensure all keys exist
	}
	for key := range cfg.Elements {
		elDef := cfg.Elements[key]
		elDef.ColorFG = ""
		elDef.ColorBG = ""
		if asciiStyle, ok := asciiMinimalElements[key]; ok {
			// Override with simpler text/prefixes from ASCII theme if they exist
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
			// IconKey in element styles is less relevant if GetIcon handles monochrome centrally
		}
		cfg.Elements[key] = elDef
	}
}
