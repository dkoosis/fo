package render_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/render"
)

func TestThemeByName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantName string
	}{
		{"default", "default"},
		{"orca", "orca"},
		{"mono", "mono"},
		{"unknown", "default"},
		{"", "default"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			theme := render.ThemeByName(tc.input)
			if theme.Name != tc.wantName {
				t.Fatalf("ThemeByName(%q).Name = %q, want %q", tc.input, theme.Name, tc.wantName)
			}
		})
	}
}

func TestMonoTheme_NoColorIcons(t *testing.T) {
	t.Parallel()
	theme := render.MonoTheme()

	// Mono icons should be ASCII-safe
	if theme.Icons.Pass != "+" {
		t.Fatalf("MonoTheme pass icon = %q, want %q", theme.Icons.Pass, "+")
	}
	if theme.Icons.Fail != "x" {
		t.Fatalf("MonoTheme fail icon = %q, want %q", theme.Icons.Fail, "x")
	}
}
