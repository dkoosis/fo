// Package paint provides Tufte-Swiss visual primitives: bars, sparklines,
// alignment helpers. No box-drawing, no chrome — hierarchy comes from
// whitespace and glyph weight, not borders.
//
// Functions are pure: glyphs and widths are passed in by the caller
// (typically from a Theme). Paint does no I/O and holds no state.
package paint

import (
	"math"
	"strings"
	"unicode/utf8"
)

// Bar returns a `width`-cell horizontal bar filled in proportion to
// value/max, using `filled` and `empty` as the cell glyphs.
//
// Edge cases:
//   - width <= 0 returns ""
//   - max <= 0 returns all-empty (cannot scale)
//   - value <= 0 returns all-empty
//   - value >= max returns all-filled
//   - NaN or Inf in value or max clamps to all-empty
func Bar(value, max float64, width int, filled, empty string) string {
	if width <= 0 {
		return ""
	}
	if math.IsNaN(value) || math.IsNaN(max) || math.IsInf(value, 0) || math.IsInf(max, 0) {
		return strings.Repeat(empty, width)
	}
	if max <= 0 || value <= 0 {
		return strings.Repeat(empty, width)
	}
	if value >= max {
		return strings.Repeat(filled, width)
	}
	cells := int(math.Round(value / max * float64(width)))
	if cells < 0 {
		cells = 0
	}
	if cells > width {
		cells = width
	}
	return strings.Repeat(filled, cells) + strings.Repeat(empty, width-cells)
}

// sparkBlocks is the canonical 8-level Unicode block ramp.
// Index 0 is reserved for true zero; index 1..8 covers the value range.
var sparkBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline returns a single-line block-graph of `values`, one cell per
// value, scaled to the slice's min/max. For an empty slice returns "".
// All-equal values render as a flat mid-level bar.
func Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	minV, maxV := values[0], values[0]
	for _, v := range values[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	span := maxV - minV
	var b strings.Builder
	b.Grow(len(values) * 3)
	for _, v := range values {
		if v == 0 {
			b.WriteRune(sparkBlocks[0])
			continue
		}
		var idx int
		if span == 0 {
			idx = 4
		} else {
			idx = int(math.Round((v-minV)/span*7)) + 1
		}
		if idx < 1 {
			idx = 1
		}
		if idx > 8 {
			idx = 8
		}
		b.WriteRune(sparkBlocks[idx])
	}
	return b.String()
}

// PadLeft right-aligns s within a column of `width` runes, padding with
// ASCII spaces. If s is wider than width, returns s unchanged.
func PadLeft(s string, width int) string {
	w := utf8.RuneCountInString(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// PadRight left-aligns s within a column of `width` runes, padding with
// ASCII spaces. If s is wider than width, returns s unchanged.
func PadRight(s string, width int) string {
	w := utf8.RuneCountInString(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// Columnize aligns rows on whitespace columns. Each column is padded
// to the widest cell in that column; cells are joined with `gap` spaces.
// Trailing whitespace is trimmed from each row.
//
// All rows must have the same length; shorter rows are padded with
// empty cells so column count == max(len(row)).
func Columnize(rows [][]string, gap int) string {
	if len(rows) == 0 {
		return ""
	}
	if gap < 0 {
		gap = 0
	}
	cols := 0
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	widths := make([]int, cols)
	for _, r := range rows {
		for i, c := range r {
			if w := utf8.RuneCountInString(c); w > widths[i] {
				widths[i] = w
			}
		}
	}
	sep := strings.Repeat(" ", gap)
	var out strings.Builder
	for ri, r := range rows {
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			if i == cols-1 {
				out.WriteString(cell)
			} else {
				out.WriteString(PadRight(cell, widths[i]))
				out.WriteString(sep)
			}
		}
		if ri < len(rows)-1 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}
