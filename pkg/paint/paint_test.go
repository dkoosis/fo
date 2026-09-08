package paint_test

import (
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dkoosis/fo/pkg/paint"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestMain(m *testing.M) {
	// Force the lipgloss default renderer to emit real ANSI (matching
	// pkg/view's own TestMain) — go test's non-TTY stdout otherwise makes
	// every Style.Render a no-op, which would let
	// TestColumnize_ANSIStyledCellsAlignByVisibleWidth pass trivially
	// whether or not columnWidths actually strips escapes.
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

const dashBar = "----"

func TestBar_ProportionalFill(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value, max float64
		width      int
		want       string
	}{
		{"half", 5, 10, 4, "##--"},
		{"empty", 0, 10, 4, dashBar},
		{"full", 10, 10, 4, "####"},
		{"over", 15, 10, 4, "####"},
		{"negative_value", -1, 10, 4, dashBar},
		{"zero_max", 5, 0, 4, dashBar},
		{"negative_max", 5, -10, 4, dashBar},
		{"width_zero", 5, 10, 0, ""},
		{"width_negative", 5, 10, -1, ""},
		{"nan_value", math.NaN(), 10, 4, dashBar},
		{"inf_max", 5, math.Inf(1), 4, dashBar},
		{"rounding_up", 7, 10, 4, "###-"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := paint.Bar(tc.value, tc.max, tc.width, "#", "-")
			if got != tc.want {
				t.Errorf("Bar(%v, %v, %d) = %q, want %q",
					tc.value, tc.max, tc.width, got, tc.want)
			}
		})
	}
}

func TestSparkline(t *testing.T) {
	t.Parallel()

	if got := paint.Sparkline(nil); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}
	if got := paint.Sparkline([]float64{0, 0, 0}); got != "   " {
		t.Errorf("all zero = %q, want spaces", got)
	}

	if got := paint.Sparkline([]float64{5, 5, 5}); got == "" {
		t.Error("constant non-zero produced empty sparkline")
	}

	got := paint.Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8})
	if utf8RuneCount(got) != 8 {
		t.Errorf("len(8 values) rune count = %d, want 8 (%q)", utf8RuneCount(got), got)
	}
}

func TestPad(t *testing.T) {
	t.Parallel()

	if got := paint.PadLeft("ab", 5); got != "   ab" {
		t.Errorf("PadLeft = %q, want %q", got, "   ab")
	}
	if got := paint.PadLeft("toolong", 3); got != "toolong" {
		t.Errorf("PadLeft (too long) = %q, want unchanged", got)
	}
}

func TestColumnize_AlignsToWidestCell(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"a", "long-cell", "1"},
		{"bbb", "x", "22"},
	}
	got := paint.Columnize(rows, 2)

	want := "a    long-cell  1\n" +
		"bbb  x          22"
	if got != want {
		t.Errorf("Columnize =\n%s\n\nwant:\n%s", got, want)
	}
}

func TestColumnize_RaggedRows(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		{"a", "b", "c"},
		{"d"},
	}
	got := paint.Columnize(rows, 1)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("rows = %d, want 2", len(lines))
	}
}

func TestColumnize_Empty(t *testing.T) {
	t.Parallel()

	if got := paint.Columnize(nil, 1); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
}

// TestColumnize_ANSIStyledCellsAlignByVisibleWidth is the fo-d84 review
// fix: columnWidths used to measure each cell with utf8.RuneCountInString
// on the raw string, counting a lipgloss style's ANSI escape bytes as
// columns. A column mixing a styled cell (like pkg/view/diffout.go's
// diffSide, which wraps a non-empty value in a theme style but leaves the
// empty-placeholder side bare) and an unstyled cell of the same visible
// width got padded to the inflated width, misaligning every other row.
func TestColumnize_ANSIStyledCellsAlignByVisibleWidth(t *testing.T) {
	t.Parallel()

	bold := lipgloss.NewStyle().Bold(true)
	rows := [][]string{
		{bold.Render("x"), "unchecked error", "store.go:42"},
		{"!", "shadowed variable", "query.go:117"},
		{".", "exported func lacks doc", "api.go:8"},
	}
	got := paint.Columnize(rows, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("rows = %d, want 3:\n%s", len(lines), got)
	}

	// The label column must start at the same visible position in every
	// row — strip ANSI first so the assertion doesn't depend on
	// lipgloss's color-profile-dependent escape bytes for the styled cell.
	labels := []string{"unchecked", "shadowed", "exported"}
	var want int
	for i, line := range lines {
		visible := stripANSI(line)
		idx := strings.Index(visible, labels[i])
		if idx < 0 {
			t.Fatalf("row %d: label %q not found in %q", i, labels[i], visible)
		}
		if i == 0 {
			want = idx
		} else if idx != want {
			t.Errorf("row %d: label starts at column %d, want %d (rows misaligned)\nfull output:\n%s", i, idx, want, got)
		}
	}
}

func utf8RuneCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
