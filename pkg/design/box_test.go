package design

import (
	"strings"
	"testing"
)

func TestBox_Basic(t *testing.T) {
	theme := DefaultTheme()
	box := NewBox(theme).
		Width(40).
		Title("TEST").
		AddLine("Hello, World!")

	result := box.String()

	// Should contain the title
	if !strings.Contains(result, "TEST") {
		t.Error("expected box to contain title 'TEST'")
	}

	// Should contain the content
	if !strings.Contains(result, "Hello, World!") {
		t.Error("expected box to contain content")
	}

	// Should have border characters (rounded by default)
	if !strings.Contains(result, "╭") && !strings.Contains(result, "┌") {
		t.Error("expected box to have top-left corner")
	}
}

func TestBox_Disabled(t *testing.T) {
	theme := DefaultTheme()
	box := NewBox(theme).
		Width(40).
		Title("TEST").
		AddLine("Content").
		Disable()

	result := box.String()

	// Should contain content but no borders
	if !strings.Contains(result, "TEST") {
		t.Error("expected disabled box to contain title")
	}
	if !strings.Contains(result, "Content") {
		t.Error("expected disabled box to contain content")
	}

	// Should NOT have border characters
	if strings.Contains(result, "╭") || strings.Contains(result, "╰") {
		t.Error("expected disabled box to NOT have borders")
	}
}

func TestBox_MultipleLines(t *testing.T) {
	theme := DefaultTheme()
	box := NewBox(theme).
		Width(40).
		AddLines("Line 1", "Line 2", "Line 3")

	result := box.String()

	if !strings.Contains(result, "Line 1") {
		t.Error("expected box to contain Line 1")
	}
	if !strings.Contains(result, "Line 2") {
		t.Error("expected box to contain Line 2")
	}
	if !strings.Contains(result, "Line 3") {
		t.Error("expected box to contain Line 3")
	}
}

func TestBox_WithFooter(t *testing.T) {
	theme := DefaultTheme()
	box := NewBox(theme).
		Width(40).
		Title("HEADER").
		AddLine("Content").
		Footer("Status: OK")

	result := box.String()

	if !strings.Contains(result, "HEADER") {
		t.Error("expected box to contain header")
	}
	if !strings.Contains(result, "Content") {
		t.Error("expected box to contain content")
	}
	if !strings.Contains(result, "Status: OK") {
		t.Error("expected box to contain footer")
	}
}

func TestRenderBox_Convenience(t *testing.T) {
	theme := DefaultTheme()
	result := RenderBox(theme, "Title", "Line 1", "Line 2")

	if !strings.Contains(result, "Title") {
		t.Error("expected RenderBox output to contain title")
	}
	if !strings.Contains(result, "Line 1") {
		t.Error("expected RenderBox output to contain Line 1")
	}
}

func TestRenderInlineStatus(t *testing.T) {
	theme := DefaultTheme()

	tests := []struct {
		status   string
		message  string
		duration string
		wantIcon string
	}{
		{"success", "Complete", "100ms", theme.Icons.Success},
		{"warning", "Warnings", "", theme.Icons.Warning},
		{"error", "Failed", "2s", theme.Icons.Error},
		{"info", "Done", "", theme.Icons.Info},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := RenderInlineStatus(theme, tt.status, tt.message, tt.duration)

			if !strings.Contains(result, tt.wantIcon) {
				t.Errorf("expected status %q to use icon %q", tt.status, tt.wantIcon)
			}
			if !strings.Contains(result, tt.message) {
				t.Errorf("expected result to contain message %q", tt.message)
			}
			if tt.duration != "" && !strings.Contains(result, tt.duration) {
				t.Errorf("expected result to contain duration %q", tt.duration)
			}
		})
	}
}

func TestBox_NilTheme(t *testing.T) {
	// Box with nil theme should render plain content
	box := &Box{
		theme:   nil,
		width:   40,
		title:   "TEST",
		content: []string{"Content"},
	}

	result := box.String()

	// Should still produce output (plain mode)
	if !strings.Contains(result, "TEST") {
		t.Error("expected nil-theme box to contain title")
	}
	if !strings.Contains(result, "Content") {
		t.Error("expected nil-theme box to contain content")
	}
}
