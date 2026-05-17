package scene

import (
	"errors"
	"strings"
	"testing"
)

func TestIsHeader(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# fo:scene\n", true},
		{"# fo:scene title=\"x\" actors=A,B\n## 01 · t\n", true},
		{"\n\n  # fo:scene\n", true},
		{"# fo:status\n", false},
		{"## 01 · t\n", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHeader([]byte(c.in)); got != c.want {
			t.Errorf("IsHeader(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParse_happy(t *testing.T) {
	in := `# fo:scene title="loto demo" actors=FrugalLapwing,OddPlover

## 01 · whoami — every session has a handle

> a fresh session lands in the repo.
> first question: what am I called here?

@FrugalLapwing $ loto whoami
  handle: FrugalLapwing
  uuid:   818772ce

> that handle is what peers will see.

## 02 · lock + status

@OddPlover $ loto lock a.go --intent "rename Type"
  ✓ locked count=1
  (exit 0)

@OddPlover $ loto fail
  boom
  (exit 2)
`
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Title != "loto demo" {
		t.Errorf("Title = %q", s.Title)
	}
	if len(s.Actors) != 2 || s.Actors[0] != "FrugalLapwing" || s.Actors[1] != "OddPlover" {
		t.Errorf("Actors = %v", s.Actors)
	}
	if len(s.Acts) != 2 {
		t.Fatalf("Acts = %d", len(s.Acts))
	}
	a0 := s.Acts[0]
	if a0.Number != "01" || !strings.HasPrefix(a0.Title, "whoami") {
		t.Errorf("act0 hdr = %+v", a0)
	}
	if len(a0.Beats) != 4 {
		t.Fatalf("act0 beats = %d", len(a0.Beats))
	}
	if a0.Beats[0].Kind != BeatNarration || a0.Beats[0].Narration != "a fresh session lands in the repo." {
		t.Errorf("act0 beat0 = %+v", a0.Beats[0])
	}
	if a0.Beats[2].Kind != BeatCommand {
		t.Fatalf("act0 beat2 kind = %v", a0.Beats[2].Kind)
	}
	cmd := a0.Beats[2].Command
	if cmd.Actor != "FrugalLapwing" || cmd.Cmd != "loto whoami" {
		t.Errorf("cmd = %+v", cmd)
	}
	if len(cmd.Output) != 2 || cmd.Output[0] != "handle: FrugalLapwing" {
		t.Errorf("cmd.Output = %v", cmd.Output)
	}
	if cmd.Exit != 0 {
		t.Errorf("cmd.Exit = %d, want 0 (no trailer)", cmd.Exit)
	}

	a1 := s.Acts[1]
	if len(a1.Beats) != 2 {
		t.Fatalf("act1 beats = %d", len(a1.Beats))
	}
	c0 := a1.Beats[0].Command
	if c0.Cmd != `loto lock a.go --intent "rename Type"` {
		t.Errorf("c0.Cmd = %q", c0.Cmd)
	}
	if c0.Exit != 0 || len(c0.Output) != 1 {
		t.Errorf("c0 = %+v", c0)
	}
	c1 := a1.Beats[1].Command
	if c1.Exit != 2 || len(c1.Output) != 1 || c1.Output[0] != "boom" {
		t.Errorf("c1 = %+v", c1)
	}
}

func TestParse_emptyScene(t *testing.T) {
	s, err := Parse(strings.NewReader("# fo:scene\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Acts) != 0 || s.Title != "" || len(s.Actors) != 0 {
		t.Errorf("empty scene non-zero: %+v", s)
	}
}

func TestParse_commentsAfterHeader(t *testing.T) {
	in := "# fo:scene\n# a comment\n## 01 · t\n# another\n> hi\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Acts) != 1 || len(s.Acts[0].Beats) != 1 {
		t.Fatalf("got %+v", s)
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"no header", "## 01 · t\n", errNoHeader},
		{"empty input", "", errNoHeader},
		{"malformed act no dot", "# fo:scene\n## 01 nothing\n", errMalformedAct},
		{"malformed act empty title", "# fo:scene\n## 01 · \n", errMalformedAct},
		{"malformed actor no dollar", "# fo:scene\n## 01 · t\n@actor cmd\n", errMalformedActor},
		{"malformed actor empty cmd", "# fo:scene\n## 01 · t\n@actor $ \n", errMalformedActor},
		{"bad exit code", "# fo:scene\n## 01 · t\n@a $ c\n  out\n  (exit oops)\n", errMalformedExit},
		{"unknown attr", "# fo:scene foo=bar\n", errUnknownAttr},
		{"narration before act", "# fo:scene\n> nope\n", errMalformedAct},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(c.in))
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want Is %v", err, c.want)
			}
		})
	}
}

func TestParse_headerAttrs(t *testing.T) {
	in := `# fo:scene title="hello world" actors=A,B,C
## 01 · t
> x
`
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Title != "hello world" {
		t.Errorf("Title = %q", s.Title)
	}
	if len(s.Actors) != 3 || s.Actors[2] != "C" {
		t.Errorf("Actors = %v", s.Actors)
	}
}

func TestParse_outputTerminatesOnTabIndent(t *testing.T) {
	// "  \tafter" is not a valid 2-space output line; it terminates the
	// command and is reparsed as narration.
	in := "# fo:scene\n## 01 · t\n@a $ cmd\n  out1\n  \t> after\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	beats := s.Acts[0].Beats
	if len(beats) != 2 {
		t.Fatalf("beats = %+v", beats)
	}
	if got := beats[0].Command.Output; len(got) != 1 || got[0] != "out1" {
		t.Errorf("Output = %v, want [out1]", got)
	}
	if beats[1].Kind != BeatNarration || beats[1].Narration != "after" {
		t.Errorf("narr beat = %+v", beats[1])
	}
}

func TestParse_outputTerminatesOnNonIndented(t *testing.T) {
	in := `# fo:scene
## 01 · t
@a $ cmd
  out1
  out2
> narration after
`
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	beats := s.Acts[0].Beats
	if len(beats) != 2 {
		t.Fatalf("beats = %d", len(beats))
	}
	if beats[0].Kind != BeatCommand || len(beats[0].Command.Output) != 2 {
		t.Errorf("cmd beat = %+v", beats[0])
	}
	if beats[1].Kind != BeatNarration || beats[1].Narration != "narration after" {
		t.Errorf("narr beat = %+v", beats[1])
	}
}

// fo-fl0.5 grammar edge cases.

// Consecutive > lines produce separate narration beats, preserving order.
func TestParse_consecutiveNarrationBeats(t *testing.T) {
	in := "# fo:scene\n## 01 · t\n> first\n> second\n> third\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	beats := s.Acts[0].Beats
	if len(beats) != 3 {
		t.Fatalf("beats = %d, want 3: %+v", len(beats), beats)
	}
	for i, want := range []string{"first", "second", "third"} {
		if beats[i].Kind != BeatNarration || beats[i].Narration != want {
			t.Errorf("beat[%d] = %+v, want narration %q", i, beats[i], want)
		}
	}
}

// Blank lines between narration beats are ignored (act-scope skips them).
func TestParse_blankLineBetweenNarrationBeats(t *testing.T) {
	in := "# fo:scene\n## 01 · t\n> first\n\n> second\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	beats := s.Acts[0].Beats
	if len(beats) != 2 || beats[0].Narration != "first" || beats[1].Narration != "second" {
		t.Errorf("beats = %+v", beats)
	}
}

// (exit 0) is the implicit default — omitted exit line still yields Exit==0.
func TestParse_defaultExitZero(t *testing.T) {
	in := "# fo:scene\n## 01 · t\n@a $ cmd\n  out\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := s.Acts[0].Beats[0].Command.Exit; got != 0 {
		t.Errorf("Exit = %d, want 0", got)
	}
}

// Non-zero exit is captured on Command.Exit.
func TestParse_nonZeroExit(t *testing.T) {
	in := "# fo:scene\n## 01 · t\n@a $ cmd\n  out\n  (exit 137)\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := s.Acts[0].Beats[0].Command.Exit; got != 137 {
		t.Errorf("Exit = %d, want 137", got)
	}
}

// An empty act header followed immediately by the next act parses
// cleanly — first act has zero beats, second has its content.
func TestParse_emptyActFollowedByNextAct(t *testing.T) {
	in := "# fo:scene\n## 01 · empty\n## 02 · second\n> hi\n"
	s, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(s.Acts) != 2 {
		t.Fatalf("acts = %d", len(s.Acts))
	}
	if len(s.Acts[0].Beats) != 0 {
		t.Errorf("act 0 beats = %+v, want empty", s.Acts[0].Beats)
	}
	if len(s.Acts[1].Beats) != 1 || s.Acts[1].Beats[0].Narration != "hi" {
		t.Errorf("act 1 beats = %+v", s.Acts[1].Beats)
	}
}

// Parser errors carry the offending line number in the message.
func TestParse_errorsCarryLineNumber(t *testing.T) {
	in := "# fo:scene\n## 01 · t\n@a oops no dollar\n"
	_, err := Parse(strings.NewReader(in))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Errorf("error missing line 3: %v", err)
	}
}
