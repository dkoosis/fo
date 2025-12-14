package dashboard

import (
	"github.com/charmbracelet/lipgloss"
)

// DashboardTheme holds all visual styling for the dashboard TUI.
type DashboardTheme struct {
	// Colors
	Colors DashboardColors `yaml:"colors"`

	// Icons for status indicators
	Icons DashboardIcons `yaml:"icons"`

	// Title bar
	Title DashboardTitleStyle `yaml:"title"`

	// Spinner configuration
	Spinner DashboardSpinnerConfig `yaml:"spinner"`

	// Subsystems for test coverage grouping (optional, has defaults)
	Subsystems []SubsystemConfig `yaml:"subsystems,omitempty"`
}

// SubsystemConfig defines an architectural subsystem for test grouping.
type SubsystemConfig struct {
	Name     string   `yaml:"name"`     // Display name (e.g., "core", "domain")
	Patterns []string `yaml:"patterns"` // Path patterns to match (e.g., "/internal/core/")
}

// DashboardColors defines the color palette for the dashboard.
type DashboardColors struct {
	Primary   string `yaml:"primary"`   // Main accent (title, selected, borders)
	Success   string `yaml:"success"`   // Success state (checkmarks)
	Error     string `yaml:"error"`     // Error state (X marks)
	Warning   string `yaml:"warning"`   // Running/in-progress state
	Muted     string `yaml:"muted"`     // Secondary text, pending items
	Text      string `yaml:"text"`      // Normal text
	Border    string `yaml:"border"`    // Border color
	Highlight string `yaml:"highlight"` // Selected item background
}

// DashboardIcons defines the icons used in the dashboard.
type DashboardIcons struct {
	Pending string `yaml:"pending"` // Pending task icon
	Running string `yaml:"running"` // Running task icon (static, spinner animates)
	Success string `yaml:"success"` // Success icon
	Error   string `yaml:"error"`   // Error icon
	Group   string `yaml:"group"`   // Group header prefix
	Select  string `yaml:"select"`  // Selected item marker
}

// DashboardTitleStyle defines the title bar appearance.
type DashboardTitleStyle struct {
	Text       string `yaml:"text"`       // Title text
	Icon       string `yaml:"icon"`       // Title icon/emoji
	Background string `yaml:"background"` // Title background color (uses Primary if empty)
}

// DashboardSpinnerConfig defines spinner animation settings.
type DashboardSpinnerConfig struct {
	Frames   string `yaml:"frames"`   // Space-separated spinner frames
	Interval int    `yaml:"interval"` // Milliseconds between frames
}

// CompiledTheme holds pre-built lipgloss styles from a DashboardTheme.
type CompiledTheme struct {
	// Colors as lipgloss.Color
	colorPrimary   lipgloss.Color
	colorSuccess   lipgloss.Color
	colorError     lipgloss.Color
	colorWarning   lipgloss.Color
	colorMuted     lipgloss.Color
	colorText      lipgloss.Color
	colorBorder    lipgloss.Color
	colorHighlight lipgloss.Color

	// Pre-built styles
	TitleStyle        lipgloss.Style
	GroupHeaderStyle  lipgloss.Style
	TaskListStyle     lipgloss.Style
	SelectedStyle     lipgloss.Style
	UnselectedStyle   lipgloss.Style
	DetailBoxStyle    lipgloss.Style
	DetailHeaderStyle lipgloss.Style
	StatusBarStyle    lipgloss.Style
	SuccessIconStyle  lipgloss.Style
	ErrorIconStyle    lipgloss.Style
	RunningIconStyle  lipgloss.Style
	PendingIconStyle  lipgloss.Style
	DurationStyle     lipgloss.Style

	// Icons
	Icons DashboardIcons

	// Title
	TitleText string
	TitleIcon string

	// Spinner
	SpinnerFrames   []string
	SpinnerInterval int

	// Subsystems for test grouping
	Subsystems []SubsystemConfig
}

// DefaultDashboardTheme returns the default dashboard theme configuration.
func DefaultDashboardTheme() *DashboardTheme {
	return &DashboardTheme{
		Colors: DashboardColors{
			Primary:   "#7D56F4", // Purple
			Success:   "#04B575", // Green
			Error:     "#FF5F56", // Red
			Warning:   "#FFBD2E", // Yellow/Orange
			Muted:     "#626262", // Gray
			Text:      "#CCCCCC", // Light gray
			Border:    "#444444", // Dark gray
			Highlight: "#7D56F4", // Purple (same as primary)
		},
		Icons: DashboardIcons{
			Pending: "\u25cb", // ○
			Running: "\u2800", // placeholder, spinner frames override
			Success: "\u2713", // ✓
			Error:   "\u2717", // ✗
			Group:   "\u25b8", // ▸
			Select:  "\u25b6", // ▶
		},
		Title: DashboardTitleStyle{
			Text: "fo Dashboard",
			Icon: "\u26a1", // ⚡
		},
		Spinner: DashboardSpinnerConfig{
			Frames:   "\u280b \u2819 \u2838 \u2834 \u2826 \u2807", // ⠋ ⠙ ⠸ ⠴ ⠦ ⠇
			Interval: 300,
		},
		Subsystems: DefaultSubsystems(),
	}
}

// DefaultSubsystems returns the default architectural subsystem configuration.
// Projects can override this in .fo.yaml to match their package structure.
// Note: Patterns should NOT have trailing slashes since Go package paths don't.
func DefaultSubsystems() []SubsystemConfig {
	return []SubsystemConfig{
		{Name: "core", Patterns: []string{"/internal/core", "/core"}},
		{Name: "kg", Patterns: []string{"/internal/kg", "/kg"}},
		{Name: "domain", Patterns: []string{"/internal/domain", "/internal/tools", "/domain", "/tools"}},
		{Name: "adapter", Patterns: []string{"/internal/mcp", "/mcp"}},
		{Name: "worker", Patterns: []string{"/internal/proc", "/proc"}},
		{Name: "kits", Patterns: []string{"/internal/kits", "/internal/codekit", "/internal/testkit", "/kits", "/codekit", "/testkit"}},
		{Name: "util", Patterns: []string{"/internal/util", "/util", "/pkg"}},
	}
}

// Compile builds lipgloss styles from the theme configuration.
func (t *DashboardTheme) Compile() *CompiledTheme {
	ct := &CompiledTheme{}

	// Parse colors
	ct.colorPrimary = lipgloss.Color(t.Colors.Primary)
	ct.colorSuccess = lipgloss.Color(t.Colors.Success)
	ct.colorError = lipgloss.Color(t.Colors.Error)
	ct.colorWarning = lipgloss.Color(t.Colors.Warning)
	ct.colorMuted = lipgloss.Color(t.Colors.Muted)
	ct.colorText = lipgloss.Color(t.Colors.Text)
	ct.colorBorder = lipgloss.Color(t.Colors.Border)
	ct.colorHighlight = lipgloss.Color(t.Colors.Highlight)

	// Build styles
	ct.TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(ct.colorPrimary).
		Padding(0, 1)

	ct.GroupHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ct.colorPrimary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(ct.colorBorder).
		MarginTop(1).
		PaddingBottom(0)

	ct.TaskListStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ct.colorBorder).
		Padding(1, 2)

	ct.SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(ct.colorHighlight).
		Padding(0, 1)

	ct.UnselectedStyle = lipgloss.NewStyle().
		Foreground(ct.colorText).
		Padding(0, 1)

	ct.DetailBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ct.colorPrimary).
		Padding(1, 2)

	ct.DetailHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(ct.colorHighlight).
		Padding(0, 1)

	ct.StatusBarStyle = lipgloss.NewStyle().
		Foreground(ct.colorMuted).
		MarginTop(1)

	ct.SuccessIconStyle = lipgloss.NewStyle().Foreground(ct.colorSuccess).Bold(true)
	ct.ErrorIconStyle = lipgloss.NewStyle().Foreground(ct.colorError).Bold(true)
	ct.RunningIconStyle = lipgloss.NewStyle().Foreground(ct.colorWarning).Bold(true)
	ct.PendingIconStyle = lipgloss.NewStyle().Foreground(ct.colorMuted)
	ct.DurationStyle = lipgloss.NewStyle().Foreground(ct.colorMuted).Italic(true)

	// Copy icons
	ct.Icons = t.Icons

	// Title
	ct.TitleText = t.Title.Text
	ct.TitleIcon = t.Title.Icon

	// Parse spinner frames
	ct.SpinnerFrames = parseSpinnerFrames(t.Spinner.Frames)
	ct.SpinnerInterval = t.Spinner.Interval
	if ct.SpinnerInterval <= 0 {
		ct.SpinnerInterval = 300
	}

	// Copy subsystems (use defaults if empty)
	if len(t.Subsystems) > 0 {
		ct.Subsystems = t.Subsystems
	} else {
		ct.Subsystems = DefaultSubsystems()
	}

	return ct
}

// parseSpinnerFrames splits space-separated spinner characters.
func parseSpinnerFrames(s string) []string {
	if s == "" {
		return []string{"\u280b", "\u2819", "\u2838", "\u2834", "\u2826", "\u2807"}
	}
	var frames []string
	for _, r := range s {
		if r != ' ' {
			frames = append(frames, string(r))
		}
	}
	if len(frames) == 0 {
		return []string{"\u280b", "\u2819", "\u2838", "\u2834", "\u2826", "\u2807"}
	}
	return frames
}

// MutedColor returns the muted color for external use.
func (ct *CompiledTheme) MutedColor() lipgloss.Color {
	return ct.colorMuted
}
