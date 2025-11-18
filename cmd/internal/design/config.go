// cmd/internal/design/config.go
package design

import (
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
	BorderAscii      BorderStyle = "ascii"
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

// Config holds all resolved design system settings for rendering
type Config struct {
	ThemeName    string `yaml:"-"`
	IsMonochrome bool   `yaml:"-"`

	Style struct {
		UseBoxes          bool   `yaml:"use_boxes"`
		Indentation       string `yaml:"indentation"`
		ShowTimestamps    bool   `yaml:"show_timestamps"`
		NoTimer           bool   `yaml:"no_timer"`
		Density           string `yaml:"density"`
		UseInlineProgress bool   `yaml:"use_inline_progress"`
		NoSpinner         bool   `yaml:"no_spinner"`
		SpinnerInterval   int    `yaml:"spinner_interval"`
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
}

type PatternsRepo struct {
	Intent map[string][]string `yaml:"intent"`
	Output map[string][]string `yaml:"output"`
}

func ensureEscapePrefix(s string) string {
	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] INPUT: '%s' (len %d, GoStr: %q, hex: %x)\n", s, len(s), s, s)
	}

	if s == "" {
		return ""
	}

	escChar := string([]byte{27}) // The actual ESC character (\x1b)

	// 1. If it already starts with the true ESC character, it's perfect.
	if strings.HasPrefix(s, escChar) {
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 1: Already correct (starts with actual ESC). Output: %q\n", s)
		}
		return s
	}

	// 2. If it's a Go string literal like `\033[` (where \033 is a single ESC char in Go source)
	goOctalLiteralEsc := "\033" // This is the single ESC char in Go source code
	if strings.HasPrefix(s, goOctalLiteralEsc) {
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 2: Go literal form (like \"\\033[\"). Output: %q\n", s)
		}
		return s
	}

	// 3. CRITICAL FIX: Check for NULL character followed by "33["
	//    Input 's' is like "\x00" + "33[" + "0;97m"
	//    We want escChar + "[" + "0;97m"
	if len(s) > 0 && s[0] == '\x00' && strings.HasPrefix(s[1:], "33[") {
		// s[1:] is "33[0;97m"
		// s[1+len("33["):] is "0;97m" (this is the remainder of the code *after* "33[")
		// The '[' is already part of s[1:], so we take from s[1+len("33"):] to get the part after "33" which is "[0;97m"
		// Or, more directly, if s[1+2] is '[', then s[1+2:] is what we want.
		// s[1+len("33[")-1] is the '[' character itself.
		// The remainder of the code, including the '[', is s[1+len("33")-1:]
		// Correct: we need what comes *after* the "\x0033", which is the starting '[' and the code.
		// So, if s[1:3] == "33", then s[3] should be '['. The part we need is s[3:].
		// More robustly: s_after_null = s[1:]. If s_after_null starts with "33[", then take s_after_null[2:]
		// Which means original s[1+2:] or s[3:].
		// Let's test: if s = "\x0033[0m", s[3:] = "[0m"
		// Corrected should be: escChar (being \x1b) + "[0m"
		corrected := escChar + s[3:]
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 3 (FIXED): Corrected NULL followed by '33'. Input: %q, Corrected part: %q, Output: %q\n", s, s[3:], corrected)
		}
		return corrected
	}

	// 4. Check for the literal string "\\033[" (e.g., from YAML `"\033[...]"`)
	yamlLiteralOctalEsc := `\033[`
	if strings.HasPrefix(s, yamlLiteralOctalEsc) {
		// s is like literal backslash, 0, 3, 3, [
		// We want to replace this whole prefix with escChar + "["
		// The part after `\033` is what we need, which starts with `[`
		// s[len(yamlLiteralOctalEsc)-1:] should give us the `[` and onwards.
		corrected := escChar + s[len(yamlLiteralOctalEsc)-1:]
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 4: Corrected YAML literal '\\033['. Output: %q\n", corrected)
		}
		return corrected
	}

	// 5. Check for the literal string "\\x1b[" (e.g., from YAML `"\x1b[...]"`)
	yamlLiteralHexEsc := `\x1b[`
	if strings.HasPrefix(s, yamlLiteralHexEsc) {
		corrected := escChar + s[len(yamlLiteralHexEsc)-1:]
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 5: Corrected YAML literal '\\x1b['. Output: %q\n", corrected)
		}
		return corrected
	}

	// 6. Check for the plain literal string "33[" (if no null prefix or other literal prefixes)
	literalDigitsEsc := "33["
	if strings.HasPrefix(s, literalDigitsEsc) {
		// s is like "33[0m"
		// We want escChar + "[0m"
		// s[len(literalDigitsEsc)-1:] is "[0m"
		corrected := escChar + s[len(literalDigitsEsc)-1:]
		if os.Getenv("FO_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] Condition 6: Corrected plain literal '33['. Output: %q\n", corrected)
		}
		return corrected
	}

	if os.Getenv("FO_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[DEBUG ensureEscapePrefix] No known prefix transformation applied. Returning as-is: %q\n", s)
	}
	return s
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

	cfg.Colors.Process = "\033[0;97m"
	cfg.Colors.Success = "\033[0;97m"
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
	cfg.Elements["Print_Header_Highlight"] = ElementStyleDef{TextCase: "none", TextStyle: []string{"bold"}, ColorFG: "White", ColorBG: "BlueBg"}
	cfg.Elements["Print_Success_Style"] = ElementStyleDef{ColorFG: "Success"}

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
	case "process":
		codeToProcess = c.Colors.Process
	case "success":
		codeToProcess = c.Colors.Success
	case "warning":
		codeToProcess = c.Colors.Warning
	case "error":
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
		if strings.Contains(name, "[") && (strings.HasPrefix(name, "\033") || strings.HasPrefix(name, "\\033") || strings.HasPrefix(name, "\\x1b") || strings.HasPrefix(name, "33") || (len(name) > 0 && name[0] == '\x00' && strings.Contains(name, "33["))) {
			codeToProcess = name
		}
	}

	escChar := string([]byte{27})
	if codeToProcess == "" {
		switch lowerName {
		case "process", "success", "white":
			codeToProcess = escChar + "[0;97m"
		case "warning":
			codeToProcess = escChar + "[0;33m"
		case "error":
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
	return ensureEscapePrefix(codeToProcess)
}

func (c *Config) ResetColor() string {
	if c.IsMonochrome {
		return ""
	}
	resetCode := c.Colors.Reset
	if resetCode == "" {
		resetCode = string([]byte{27}) + "[0m"
	}
	return ensureEscapePrefix(resetCode)
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
	if original.Patterns.Intent != nil {
		copied.Patterns.Intent = make(map[string][]string)
		for k, v := range original.Patterns.Intent {
			s := make([]string, len(v))
			copy(s, v)
			copied.Patterns.Intent[k] = s
		}
	}
	if original.Patterns.Output != nil {
		copied.Patterns.Output = make(map[string][]string)
		for k, v := range original.Patterns.Output {
			s := make([]string, len(v))
			copy(s, v)
			copied.Patterns.Output[k] = s
		}
	}
	if original.Tools != nil {
		copied.Tools = make(map[string]*ToolConfig)
		for k, vOriginalTool := range original.Tools {
			if vOriginalTool == nil {
				copied.Tools[k] = nil
				continue
			}
			toolCfgCopy := *vOriginalTool
			if vOriginalTool.OutputPatterns != nil {
				toolCfgCopy.OutputPatterns = make(map[string][]string)
				for pk, pv := range vOriginalTool.OutputPatterns {
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
		White   string `yaml:"white,omitempty"`
		BlueFg  string `yaml:"blue_fg,omitempty"`
		BlueBg  string `yaml:"blue_bg,omitempty"`
		Bold    string `yaml:"bold,omitempty"`
		Italic  string `yaml:"italic,omitempty"`
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
