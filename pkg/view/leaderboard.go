package view

import (
	"strconv"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/theme"
)

// leaderboardBarWidth picks the bar width given total terminal width.
// Reserves room for label, value, and column gaps; clamps to [8, 40].
func leaderboardBarWidth(width, labelMax, valueMax int) int {
	// rough budget: width - labelMax - valueMax - 2 gaps of 2
	bar := width - labelMax - valueMax - 4
	if bar < 8 {
		bar = 8
	}
	if bar > 40 {
		bar = 40
	}
	return bar
}

func renderLeaderboard(v Leaderboard, t theme.Theme, width int) string {
	if len(v.Rows) == 0 {
		return ""
	}
	// label/value column widths
	labelMax := 0
	valueMax := 0
	values := make([]string, len(v.Rows))
	for i, r := range v.Rows {
		if l := len(r.Label); l > labelMax {
			labelMax = l
		}
		values[i] = strconv.FormatFloat(r.Value, 'f', -1, 64)
		if l := len(values[i]); l > valueMax {
			valueMax = l
		}
	}
	bw := leaderboardBarWidth(width, labelMax, valueMax)

	rows := make([][]string, 0, len(v.Rows))
	for i, r := range v.Rows {
		bar := paint.Bar(r.Value, v.Total, bw, t.Icons.Bar, t.Icons.BarEmpty)
		rows = append(rows, []string{
			r.Label,
			t.Muted.Render(bar),
			t.Bold.Render(paint.PadLeft(values[i], valueMax)),
		})
	}
	return paint.Columnize(rows, 2)
}
