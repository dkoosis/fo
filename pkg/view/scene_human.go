package view

import (
	"fmt"
	"hash/fnv"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/theme"
)

// sceneRuleWidth is the default horizontal rule width used for act
// headers when no terminal width is known. Matches the conventional
// 60-char column for Tufte-Swiss text layouts.
const sceneRuleWidth = 60

// actorPalette is a small fixed palette of lipgloss colors. Actor
// names hash into this slice for stable per-scene colorization.
// 256-color ANSI; chosen to be distinguishable on dark and light TTYs
// without clashing with theme severity/outcome colors (red/green).
var actorPalette = []lipgloss.Color{
	lipgloss.Color("39"),  // blue
	lipgloss.Color("141"), // violet
	lipgloss.Color("178"), // gold
	lipgloss.Color("44"),  // teal
	lipgloss.Color("204"), // pink
	lipgloss.Color("108"), // sage
}

// RenderSceneHuman renders a parsed Scene for a TTY: horizontal-rule
// act headers, dimmed narration, accent-colored actor prompts, and
// monospace command output. Exit zero is suppressed; non-zero is
// rendered in the error color.
func RenderSceneHuman(w io.Writer, s scene.Scene) error {
	// theme.Default respects NO_COLOR (downgrades to Mono); Color()
	// would hardcode ANSI regardless (fo-5r4).
	t := theme.Default(theme.OutputTTY)
	return renderScene(w, s, t)
}

func renderScene(w io.Writer, s scene.Scene, t theme.Theme) error {
	if s.Title != "" {
		if _, err := fmt.Fprintf(w, "%s\n\n", t.Heading.Render(s.Title)); err != nil {
			return err
		}
	}
	rule := strings.Repeat("─", sceneRuleWidth)
	for i, act := range s.Acts {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if err := renderAct(w, act, t, rule); err != nil {
			return err
		}
	}
	return nil
}

func renderAct(w io.Writer, act scene.Act, t theme.Theme, rule string) error {
	if _, err := fmt.Fprintf(w, "%s\n%s\n\n", t.Muted.Render(rule),
		t.Heading.Render(act.Number+" · "+act.Title)); err != nil {
		return err
	}
	for _, beat := range act.Beats {
		switch beat.Kind {
		case scene.BeatNarration:
			if err := renderNarration(w, beat.Narration, t); err != nil {
				return err
			}
		case scene.BeatCommand:
			if err := renderCommand(w, beat.Command, t); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderNarration(w io.Writer, text string, t theme.Theme) error {
	_, err := fmt.Fprintf(w, "  %s\n", t.Muted.Render(text))
	return err
}

func renderCommand(w io.Writer, cmd scene.Command, t theme.Theme) error {
	actor := actorStyle(cmd.Actor, t).Render(cmd.Actor)
	prompt := t.Bold.Render("❯")
	if _, err := fmt.Fprintf(w, "%s %s %s\n", actor, prompt, cmd.Cmd); err != nil {
		return err
	}
	for _, line := range cmd.Output {
		if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
			return err
		}
	}
	if cmd.Exit != 0 {
		exit := t.Fail.Render(fmt.Sprintf("(exit %d)", cmd.Exit))
		if _, err := fmt.Fprintf(w, "  %s\n", exit); err != nil {
			return err
		}
	}
	return nil
}

// actorStyle returns a stable foreground style for the actor by
// hashing the name into actorPalette. Under mono themes (NO_COLOR /
// non-TTY) the foreground is dropped so the actor appears in plain
// bold (fo-5r4).
func actorStyle(actor string, t theme.Theme) lipgloss.Style {
	if t.Name == "mono" {
		return lipgloss.NewStyle().Bold(true)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(actor))
	// Mod in uint32 space, then convert. Casting Sum32 to int and
	// negating breaks on 32-bit (math.MinInt32 stays negative after
	// negation and panics on slice index) (fo-5r4).
	idx := int(h.Sum32() % uint32(len(actorPalette))) //nolint:gosec // len of fixed palette is small positive int
	return lipgloss.NewStyle().Foreground(actorPalette[idx]).Bold(true)
}
