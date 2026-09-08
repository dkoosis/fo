package view

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

const goCmpOutput = `  MyStruct{
- 	Field: 1,
+ 	Field: 2,
  	Other: "x",
  }`

const cupaloyOutput = `--- Previous
+++ Current
@@ -1,3 +1,3 @@
-old line
+new line
 context`

func TestLooksLikeDiff(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"go-cmp", goCmpOutput, true},
		{"cupaloy", cupaloyOutput, true},
		{"plain log", "FAIL TestX\n  expected behavior\n  got panic", false},
		{"only removals", "- a\n- b", false}, // needs both directions
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeDiff(c.in); got != c.want {
				t.Errorf("looksLikeDiff(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestRenderDiffOutput_NonDiffUnchanged(t *testing.T) {
	in := "assertion failed\n  wanted truthy"
	if got := RenderDiffOutput(in, theme.Mono()); got != in {
		t.Errorf("non-diff output should pass through: got %q", got)
	}
}

func TestRenderDiffOutput_PreservesLineCountAndContent(t *testing.T) {
	// Styling may add ANSI (depending on the active color profile) but must
	// never drop, reorder, or merge lines, and every line's text must
	// survive — an LLM/piped reader strips the escapes and sees the diff
	// intact. Asserting on visible content (not bytes) keeps the test
	// independent of lipgloss's global color profile.
	for _, in := range []string{goCmpOutput, cupaloyOutput} {
		got := RenderDiffOutput(in, theme.Color())
		if gotN, wantN := strings.Count(got, "\n"), strings.Count(in, "\n"); gotN != wantN {
			t.Errorf("line count changed: got %d want %d", gotN, wantN)
		}
		if stripANSI(got) != in {
			t.Errorf("visible text changed after styling:\n--- visible\n%s\n--- want\n%s", stripANSI(got), in)
		}
	}
}

// structuralFixture is what pkg/testjson's detector would attach for
// goCmpOutput above: one changed field plus one added-only field, to
// exercise both the paired and the one-sided render path.
var structuralFixture = &report.StructuralDiff{
	Kind: "go-cmp",
	Fields: []report.DiffField{
		{Path: "MyStruct.Field", Removed: "1", Added: "2"},
		{Path: "MyStruct.Extra", Added: `"new"`},
	},
}

func TestRenderStructuralDiff_Nil(t *testing.T) {
	if got := RenderStructuralDiff(nil, theme.Mono()); got != "" {
		t.Errorf("RenderStructuralDiff(nil, ...) = %q, want empty", got)
	}
	empty := &report.StructuralDiff{Kind: "go-cmp"}
	if got := RenderStructuralDiff(empty, theme.Mono()); got != "" {
		t.Errorf("RenderStructuralDiff(no fields) = %q, want empty", got)
	}
}

// TestRenderStructuralDiff_FieldStructural is the core fo-d84 assertion:
// the reader sees field paths and values, not raw diff-marked lines. Runs
// both themes — Color (human/TTY) and Mono (llm/piped) — since both
// renderers are meant to benefit. Asserts on visible content (stripped of
// escapes), matching how the rest of this package tests theming: Mono
// still carries bold/dim structural styling (theme.go), it just drops
// color — same convention RenderDiffOutput already relies on.
func TestRenderStructuralDiff_FieldStructural(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   theme.Theme
	}{
		{"human", theme.Color()},
		{"llm", theme.Mono()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stripANSI(RenderStructuralDiff(structuralFixture, tc.th))
			for _, want := range []string{"MyStruct.Field", "- 1", "+ 2", "MyStruct.Extra", "+ \"new\""} {
				if !strings.Contains(got, want) {
					t.Errorf("rendered output missing %q:\n%s", want, got)
				}
			}
			// Line-oriented text ("Field: 1," etc.) must not survive —
			// the point of this render is that it replaces line-reading
			// with field-reading.
			if strings.Contains(got, "Field: 1,") {
				t.Errorf("rendered output still looks line-oriented:\n%s", got)
			}
		})
	}
}

// TestRenderStructuralDiff_OneSidedPlaceholder covers an added-only field:
// the empty side must render as the theme's Same icon — the same "nothing
// here" glyph Delta/bullet/leaderboard use — not a blank cell that would
// misalign with paired rows.
func TestRenderStructuralDiff_OneSidedPlaceholder(t *testing.T) {
	got := RenderStructuralDiff(structuralFixture, theme.Mono())
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	placeholder := theme.Mono().Icons.Same
	if !strings.Contains(lines[1], placeholder) {
		t.Errorf("added-only row missing placeholder %q: %q", placeholder, lines[1])
	}
}

// TestRenderStructuralDiff_MonoUsesASCIIPlaceholder is the fo-d84 review
// fix: the empty side must come from the active theme's Icons.Same, not a
// hardcoded glyph — Mono/llm output gets the ASCII "=", not "·".
func TestRenderStructuralDiff_MonoUsesASCIIPlaceholder(t *testing.T) {
	got := RenderStructuralDiff(structuralFixture, theme.Mono())
	if strings.Contains(got, "·") {
		t.Errorf("Mono render contains non-ASCII placeholder %q:\n%s", "·", got)
	}
	if !strings.Contains(got, "=") {
		t.Errorf("Mono render missing ASCII placeholder %q:\n%s", "=", got)
	}
}
