package scene_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/scene"
)

const castInput = `# fo:scene title="demo"

## 01 · first

> opening narration.

@a $ run
  ok

## 02 · second

@b $ go
  done
`

// idRender returns the canonical text of the partial scene — enough to
// assert progressive reveal without depending on the view layer.
func idRender(s scene.Scene) string {
	var b strings.Builder
	for _, act := range s.Acts {
		for _, beat := range act.Beats {
			switch beat.Kind {
			case scene.BeatNarration:
				b.WriteString("N:" + beat.Narration + "\n")
			case scene.BeatCommand:
				b.WriteString("C:" + beat.Command.Cmd + "\n")
			}
		}
	}
	return b.String()
}

func TestCast_oneFramePerBeat(t *testing.T) {
	s, err := scene.Parse(strings.NewReader(castInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	frames := scene.Cast(s, idRender, nil)

	// 3 beats total: narration + command in act 1, command in act 2.
	if len(frames) != 3 {
		t.Fatalf("want 3 frames, got %d", len(frames))
	}
	// Progressive reveal: each frame contains all prior content plus one
	// more line.
	for i, f := range frames {
		lines := strings.Count(strings.TrimRight(f.Content, "\n"), "\n") + 1
		if lines != i+1 {
			t.Errorf("frame %d: want %d content lines, got %d:\n%s", i, i+1, lines, f.Content)
		}
		if f.Delay != 0 {
			t.Errorf("frame %d: nil delay should give Delay 0, got %v", i, f.Delay)
		}
	}
	// Last frame holds the whole scene.
	want := "N:opening narration.\nC:run\nC:go\n"
	if frames[2].Content != want {
		t.Errorf("final frame\nwant: %q\ngot:  %q", want, frames[2].Content)
	}
}

func TestCast_delayApplied(t *testing.T) {
	s, err := scene.Parse(strings.NewReader(castInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	delay := func(b scene.Beat) time.Duration {
		if b.Kind == scene.BeatCommand {
			return 500 * time.Millisecond
		}
		return 100 * time.Millisecond
	}
	frames := scene.Cast(s, idRender, delay)
	want := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond}
	for i, f := range frames {
		if f.Delay != want[i] {
			t.Errorf("frame %d delay: want %v, got %v", i, want[i], f.Delay)
		}
	}
}

func TestCast_empty(t *testing.T) {
	if got := scene.Cast(scene.Scene{}, idRender, nil); got != nil {
		t.Errorf("empty scene: want nil frames, got %v", got)
	}
}

// TestCast_doesNotMutate guards scenePrefix's slice truncation from
// corrupting the source scene.
func TestCast_doesNotMutate(t *testing.T) {
	s, err := scene.Parse(strings.NewReader(castInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	before := len(s.Acts[0].Beats)
	_ = scene.Cast(s, idRender, nil)
	if after := len(s.Acts[0].Beats); after != before {
		t.Errorf("source mutated: act 0 beats %d → %d", before, after)
	}
}
