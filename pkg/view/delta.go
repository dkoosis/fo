package view

import (
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/theme"
)

// renderDelta paints the inner view first, then a single-line bucket
// strip below summarising change vs prior. The strip joins each bucket
// as "label arrow count" separated by two spaces.
func renderDelta(v Delta, t theme.Theme, width int) string {
	inner := ""
	if v.Inner != nil {
		inner = Render(v.Inner, t, width)
	}
	parts := make([]string, 0, len(v.Buckets))
	for _, b := range v.Buckets {
		var arrow string
		var style = t.Muted
		switch {
		case b.Direction > 0:
			arrow = t.Icons.Up
			style = t.Fail // up is bad in finding-counts
		case b.Direction < 0:
			arrow = t.Icons.Down
			style = t.Pass
		default:
			arrow = t.Icons.Same
		}
		seg := b.Label + " " + style.Render(arrow) + " " + strconv.Itoa(b.Count)
		parts = append(parts, seg)
	}
	strip := strings.Join(parts, "  ")
	if inner == "" {
		return strip
	}
	if strip == "" {
		return inner
	}
	return inner + "\n\n" + strip
}
