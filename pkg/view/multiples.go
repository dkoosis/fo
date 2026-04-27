package view

import (
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

// counterStyle picks the theme style for a counter's severity tag.
func counterStyle(c Counter, t theme.Theme) styler {
	switch c.Severity {
	case report.SeverityError:
		return wrap(t.Error)
	case report.SeverityWarning:
		return wrap(t.Warning)
	case report.SeverityNote:
		return wrap(t.Note)
	}
	return wrap(t.Bold)
}

// renderCell builds one tile as a 2-row block: label on top, then a
// line of "spark  counter1 counter2 ...". Either spark or counters may
// be empty.
func renderCell(c MultipleCell, t theme.Theme) string {
	head := t.Bold.Render(c.Label)
	var body strings.Builder
	if len(c.Sparks) > 0 {
		body.WriteString(t.Muted.Render(paint.Sparkline(c.Sparks)))
	}
	for i, ctr := range c.Counters {
		if i > 0 || body.Len() > 0 {
			body.WriteString("  ")
		}
		val := counterStyle(ctr, t)(strconv.Itoa(ctr.Value))
		body.WriteString(val)
		if ctr.Label != "" {
			body.WriteString(" ")
			body.WriteString(t.Muted.Render(ctr.Label))
		}
	}
	if body.Len() == 0 {
		return head
	}
	return head + "\n" + body.String()
}

// renderSmallMultiples lays cells out in a grid, columns chosen to
// fill the available width. Each cell is two lines (label + body); we
// stack rows of cells with column alignment provided by Columnize.
func renderSmallMultiples(v SmallMultiples, t theme.Theme, width int) string {
	if len(v.Cells) == 0 {
		return ""
	}
	// pick columns: aim for ~24 cols per cell minimum
	cols := min(max(width/24, 1), len(v.Cells))

	rendered := make([]string, len(v.Cells))
	for i, c := range v.Cells {
		rendered[i] = renderCell(c, t)
	}

	// chunk into rows of `cols` cells; each cell is 1-2 lines
	var out strings.Builder
	for start := 0; start < len(rendered); start += cols {
		end := min(start+cols, len(rendered))
		out.WriteString(paint.Columnize(buildGridRows(rendered[start:end]), 3))
		if end < len(rendered) {
			out.WriteString("\n\n")
		}
	}
	return out.String()
}

// buildGridRows converts a slice of multi-line cell strings into a
// [][]string grid suitable for Columnize: gridRows[lineIdx][colIdx].
func buildGridRows(cells []string) [][]string {
	cellLines := make([][]string, len(cells))
	maxLines := 0
	for i, c := range cells {
		cellLines[i] = strings.Split(c, "\n")
		if len(cellLines[i]) > maxLines {
			maxLines = len(cellLines[i])
		}
	}
	gridRows := make([][]string, maxLines)
	for li := range maxLines {
		row := make([]string, len(cells))
		for ci := range cells {
			if li < len(cellLines[ci]) {
				row[ci] = cellLines[ci][li]
			}
		}
		gridRows[li] = row
	}
	return gridRows
}
