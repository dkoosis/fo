package view

import (
	"fmt"
	"io"
	"strconv"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/tally"
	"github.com/dkoosis/fo/pkg/theme"
)

// LeaderboardFromTally builds a Leaderboard from a parsed tally stream.
// Lives here, not in pkg/tally, so tally stays a pure parser that returns
// report-level data — view, the sole renderer-facing package, decides how
// to shape it into a ViewSpec (fo-n25.2). Rows are emitted in input order;
// Total is the sum of all values (used by the renderer to scale bars).
func LeaderboardFromTally(t tally.Tally) Leaderboard {
	rows := make([]LbRow, len(t.Rows))
	var total float64
	for i, r := range t.Rows {
		rows[i] = LbRow{Label: r.Label, Value: r.Value}
		total += r.Value
	}
	return Leaderboard{Rows: rows, Total: total}
}

// RenderLeaderboardLLM emits a terse plain-text ranking. Used when fo's
// output mode is llm — bar charts are useless to AI consumers; a sorted
// "label  count" listing is the densest faithful form.
func RenderLeaderboardLLM(w io.Writer, v Leaderboard) error {
	labelMax := 0
	for _, r := range v.Rows {
		if l := len(r.Label); l > labelMax {
			labelMax = l
		}
	}
	for _, r := range v.Rows {
		val := strconv.FormatFloat(r.Value, 'f', -1, 64)
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", labelMax, r.Label, val); err != nil {
			return err
		}
	}
	return nil
}

// leaderboardBarWidth picks the bar width given total terminal width.
// Reserves room for label, value, and column gaps; clamps to [8, 40].
func leaderboardBarWidth(width, labelMax, valueMax int) int {
	// rough budget: width - labelMax - valueMax - 2 gaps of 2
	bar := width - labelMax - valueMax - 4
	bar = max(bar, 8)
	bar = min(bar, 40)
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
