package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/scene"
)

// RenderSceneLLM emits a scene in the canonical `# fo:scene` text form.
// Output is plain text with no ANSI; structure is preserved so the result
// round-trips through scene.Parse.
func RenderSceneLLM(w io.Writer, s scene.Scene) error {
	if err := writeSceneHeader(w, s); err != nil {
		return err
	}
	for i, act := range s.Acts {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "## %s · %s\n", act.Number, act.Title); err != nil {
			return err
		}
		if err := writeBeats(w, act.Beats); err != nil {
			return err
		}
		_ = i
	}
	return nil
}

func writeSceneHeader(w io.Writer, s scene.Scene) error {
	var b strings.Builder
	b.WriteString(scene.HeaderPrefix)
	if s.Title != "" {
		// title may contain spaces; quote it. Embedded quotes are escaped
		// with backslash to match the parser's quote-aware tokenizer.
		b.WriteString(` title="`)
		b.WriteString(escapeQuoted(s.Title))
		b.WriteByte('"')
	}
	if len(s.Actors) > 0 {
		b.WriteString(" actors=")
		b.WriteString(strings.Join(s.Actors, ","))
	}
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err
}

func writeBeats(w io.Writer, beats []scene.Beat) error {
	for _, beat := range beats {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		switch beat.Kind {
		case scene.BeatNarration:
			if err := writeNarration(w, beat.Narration); err != nil {
				return err
			}
		case scene.BeatCommand:
			if err := writeCommand(w, beat.Command); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeNarration(w io.Writer, text string) error {
	if text == "" {
		_, err := fmt.Fprintln(w, ">")
		return err
	}
	_, err := fmt.Fprintf(w, "> %s\n", text)
	return err
}

func writeCommand(w io.Writer, c scene.Command) error {
	if _, err := fmt.Fprintf(w, "@%s $ %s\n", c.Actor, c.Cmd); err != nil {
		return err
	}
	for _, line := range c.Output {
		if _, err := fmt.Fprintf(w, "  %s\n", line); err != nil {
			return err
		}
	}
	if c.Exit != 0 {
		if _, err := fmt.Fprintf(w, "  (exit %d)\n", c.Exit); err != nil {
			return err
		}
	}
	return nil
}

func escapeQuoted(s string) string {
	if !strings.ContainsAny(s, `"\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}
