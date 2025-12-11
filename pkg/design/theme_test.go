package design

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultTheme(t *testing.T) {
	theme := DefaultTheme()

	if theme.Name != "default" {
		t.Errorf("expected theme name 'default', got %q", theme.Name)
	}

	// Verify colors are set (not empty)
	if theme.Colors.Primary == "" {
		t.Error("expected Primary color to be set")
	}
	if theme.Colors.Success == "" {
		t.Error("expected Success color to be set")
	}
	if theme.Colors.Error == "" {
		t.Error("expected Error color to be set")
	}

	// Verify icons are set
	if theme.Icons.Success == "" {
		t.Error("expected Success icon to be set")
	}
	if theme.Icons.Error == "" {
		t.Error("expected Error icon to be set")
	}
}

func TestMonochromeTheme(t *testing.T) {
	theme := MonochromeTheme()

	if theme.Name != "monochrome" {
		t.Errorf("expected theme name 'monochrome', got %q", theme.Name)
	}

	// Verify colors are empty (monochrome)
	if theme.Colors.Primary != "" {
		t.Errorf("expected Primary color to be empty, got %q", theme.Colors.Primary)
	}
	if theme.Colors.Success != "" {
		t.Errorf("expected Success color to be empty, got %q", theme.Colors.Success)
	}

	// Verify ASCII icons are used
	if theme.Icons.Success != "[OK]" {
		t.Errorf("expected ASCII Success icon '[OK]', got %q", theme.Icons.Success)
	}
	if theme.Icons.Error != "[FAIL]" {
		t.Errorf("expected ASCII Error icon '[FAIL]', got %q", theme.Icons.Error)
	}
}

func TestNewTheme_BuildsStyles(_ *testing.T) {
	colors := ThemeColors{
		Primary: lipgloss.Color("39"),
		Success: lipgloss.Color("120"),
		Warning: lipgloss.Color("214"),
		Error:   lipgloss.Color("196"),
		Text:    lipgloss.Color("252"),
		Muted:   lipgloss.Color("242"),
		Subtle:  lipgloss.Color("238"),
	}
	icons := ThemeIcons{
		Success: "OK",
		Error:   "FAIL",
	}

	theme := NewTheme("test", colors, icons)

	// Verify styles were built
	// We can't easily inspect lipgloss.Style internals, but we can verify
	// they render without panicking
	_ = theme.Styles.Header.Render("test")
	_ = theme.Styles.StatusSuccess.Render("test")
	_ = theme.Styles.StatusError.Render("test")
	_ = theme.Styles.Box.Render("test")
}

func TestOrcaTheme(t *testing.T) {
	theme := OrcaTheme2()

	if theme.Name != "orca" {
		t.Errorf("expected theme name 'orca', got %q", theme.Name)
	}

	// Orca uses pale blue as primary
	if theme.Colors.Primary != lipgloss.Color("111") {
		t.Errorf("expected Primary color '111', got %q", theme.Colors.Primary)
	}
}
