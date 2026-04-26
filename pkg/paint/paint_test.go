package paint_test

import (
	"math"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/paint"
)

func TestBar_ProportionalFill(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		value, max     float64
		width          int
		want           string
	}{
		{"half", 5, 10, 4, "##--"},
		{"empty", 0, 10, 4, "----"},
		{"full", 10, 10, 4, "####"},
		{"over", 15, 10, 4, "####"},
		{"negative_value", -1, 10, 4, "----"},
		{"zero_max", 5, 0, 4, "----"},
		{"negative_max", 5, -10, 4, "----"},
		{"width_zero", 5, 10, 0, ""},
		{"width_negative", 5, 10, -1, ""},
		{"nan_value", math.NaN(), 10, 4, "----"},
		{"inf_max", 5, math.Inf(1), 4, "----"},
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
	if got := paint.PadRight("ab", 5); got != "ab   " {
		t.Errorf("PadRight = %q, want %q", got, "ab   ")
	}
	if got := paint.PadLeft("toolong", 3); got != "toolong" {
		t.Errorf("PadLeft (too long) = %q, want unchanged", got)
	}

	if got := paint.PadRight("ⓐⓑ", 5); got != "ⓐⓑ   " {
		t.Errorf("PadRight unicode = %q, want %q", got, "ⓐⓑ   ")
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

func utf8RuneCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
