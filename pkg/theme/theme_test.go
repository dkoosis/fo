package theme_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/theme"
)

func TestMono_NoColors(t *testing.T) {
	t.Parallel()

	m := theme.Mono()
	if m.Name != "mono" {
		t.Errorf("Name = %q, want mono", m.Name)
	}

	for _, c := range []struct {
		name string
		got  string
	}{
		{"Pass icon", m.Icons.Pass},
		{"Fail icon", m.Icons.Fail},
		{"Bar", m.Icons.Bar},
		{"Up", m.Icons.Up},
	} {
		for _, r := range c.got {
			if r > 0x7F {
				t.Errorf("%s = %q contains non-ASCII rune %q", c.name, c.got, r)
			}
		}
	}
}

func TestColor_OverlaysMono(t *testing.T) {
	t.Parallel()

	c := theme.Color()
	if c.Name != "color" {
		t.Errorf("Name = %q, want color", c.Name)
	}

	if c.Icons.Pass == theme.Mono().Icons.Pass {
		t.Errorf("expected color preset to upgrade Pass glyph (got %q)", c.Icons.Pass)
	}
}

func TestDefault_NoColorEnvForcesMono(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := theme.Default(true)
	if got.Name != "mono" {
		t.Errorf("Default(TTY=true) with NO_COLOR=1 = %q, want mono", got.Name)
	}
}

func TestDefault_NonTTYForcesMono(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got := theme.Default(false)
	if got.Name != "mono" {
		t.Errorf("Default(TTY=false) = %q, want mono", got.Name)
	}
}

func TestDefault_TTYWithColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got := theme.Default(true)
	if got.Name != "color" {
		t.Errorf("Default(TTY=true) = %q, want color", got.Name)
	}
}

func TestDefault_AllSeverityStylesPopulated(t *testing.T) {
	t.Parallel()

	for _, th := range []theme.Theme{theme.Mono(), theme.Color()} {
		t.Run(th.Name, func(t *testing.T) {
			t.Parallel()
			if th.Error.Render("x") == "" {
				t.Error("Error style produced empty render")
			}
			if th.Warning.Render("x") == "" {
				t.Error("Warning style produced empty render")
			}
			if th.Note.Render("x") == "" {
				t.Error("Note style produced empty render")
			}
		})
	}
}
