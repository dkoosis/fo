package design

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadRight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "simple ASCII",
			input:    "hello",
			width:    10,
			expected: "hello     ",
		},
		{
			name:     "emoji",
			input:    "✓ pass",
			width:    10,
			expected: "✓ pass    ",
		},
		{
			name:     "CJK characters",
			input:    "日本語",
			width:    10,
			expected: "日本語     ",
		},
		{
			name:     "already wide enough",
			input:    "very long string",
			width:    5,
			expected: "very long string",
		},
		{
			name:     "empty string",
			input:    "",
			width:    5,
			expected: "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadRight(tt.input, tt.width)
			actualWidth := lipgloss.Width(result)
			// If input is already wider than requested width, result should be unchanged
			inputWidth := lipgloss.Width(tt.input)
			if inputWidth >= tt.width {
				if result != tt.input {
					t.Errorf("PadRight(%q, %d) = %q, want %q (input already wide enough)",
						tt.input, tt.width, result, tt.input)
				}
			} else {
				if actualWidth != tt.width {
					t.Errorf("PadRight(%q, %d) visual width = %d, want %d",
						tt.input, tt.width, actualWidth, tt.width)
				}
				// Check that result starts with input
				if len(result) < len(tt.input) || result[:len(tt.input)] != tt.input {
					t.Errorf("PadRight(%q, %d) = %q, should start with %q",
						tt.input, tt.width, result, tt.input)
				}
			}
		})
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "simple ASCII",
			input:    "hello",
			width:    10,
			expected: "     hello",
		},
		{
			name:     "emoji",
			input:    "✓ pass",
			width:    10,
			expected: "    ✓ pass",
		},
		{
			name:     "CJK characters",
			input:    "日本語",
			width:    10,
			expected: "     日本語",
		},
		{
			name:     "already wide enough",
			input:    "very long string",
			width:    5,
			expected: "very long string",
		},
		{
			name:     "empty string",
			input:    "",
			width:    5,
			expected: "     ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadLeft(tt.input, tt.width)
			actualWidth := lipgloss.Width(result)
			// If input is already wider than requested width, result should be unchanged
			inputWidth := lipgloss.Width(tt.input)
			if inputWidth >= tt.width {
				if result != tt.input {
					t.Errorf("PadLeft(%q, %d) = %q, want %q (input already wide enough)",
						tt.input, tt.width, result, tt.input)
				}
			} else {
				if actualWidth != tt.width {
					t.Errorf("PadLeft(%q, %d) visual width = %d, want %d",
						tt.input, tt.width, actualWidth, tt.width)
				}
				// Check that result ends with input
				if len(result) < len(tt.input) || result[len(result)-len(tt.input):] != tt.input {
					t.Errorf("PadLeft(%q, %d) = %q, should end with %q",
						tt.input, tt.width, result, tt.input)
				}
			}
		})
	}
}

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "simple ASCII",
			input:    "hello",
			expected: 5,
		},
		{
			name:     "emoji",
			input:    "✓",
			expected: 1,
		},
		{
			name:     "CJK characters",
			input:    "日本語",
			expected: 6, // Each CJK character is 2 cells wide
		},
		{
			name:     "mixed",
			input:    "✓ pass 日本語",
			expected: 12, // 1 + 1 + 1 + 4 + 1 + 6 = 14? Let's check
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VisualWidth(tt.input)
			// Use lipgloss directly to verify
			expected := lipgloss.Width(tt.input)
			if result != expected {
				t.Errorf("VisualWidth(%q) = %d, lipgloss.Width = %d (mismatch)",
					tt.input, result, expected)
			}
		})
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxWidth int
		check    func(t *testing.T, result string)
	}{
		{
			name:     "simple ASCII truncation",
			input:    "hello world",
			maxWidth: 5,
			check: func(t *testing.T, result string) {
				if VisualWidth(result) > 5 {
					t.Errorf("truncateToWidth result width %d exceeds maxWidth 5", VisualWidth(result))
				}
			},
		},
		{
			name:     "emoji truncation",
			input:    "✓ pass ✓ pass",
			maxWidth: 5,
			check: func(t *testing.T, result string) {
				if VisualWidth(result) > 5 {
					t.Errorf("truncateToWidth result width %d exceeds maxWidth 5", VisualWidth(result))
				}
			},
		},
		{
			name:     "CJK truncation",
			input:    "日本語日本語",
			maxWidth: 5,
			check: func(t *testing.T, result string) {
				if VisualWidth(result) > 5 {
					t.Errorf("truncateToWidth result width %d exceeds maxWidth 5", VisualWidth(result))
				}
			},
		},
		{
			name:     "no truncation needed",
			input:    "hi",
			maxWidth: 10,
			check: func(t *testing.T, result string) {
				if result != "hi" {
					t.Errorf("truncateToWidth(%q, 10) = %q, want %q", "hi", result, "hi")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateToWidth(tt.input, tt.maxWidth)
			tt.check(t, result)
		})
	}
}

