package view

import (
	"strings"
	"testing"

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
