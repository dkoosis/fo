package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

// diffLineKind classifies one line of a test's failure output when that
// output looks like a structural diff (go-cmp's -/+ report or cupaloy's
// unified diff). Worthless line-by-line, the classification lets the
// renderer color removals and additions so the actual mismatch pops
// instead of drowning in an undifferentiated wall of text.
type diffLineKind int

const (
	kindPlain   diffLineKind = iota // not part of a diff
	kindDel                         // a removal (got, when comparing want↔got)
	kindAdd                         // an addition (want)
	kindHunk                        // a hunk/file header (@@, ---, +++)
	kindContext                     // unchanged context inside a diff
)

// looksLikeDiff reports whether output contains a recognizable diff block —
// at least one removal and one addition among diff-prefixed lines. Requiring
// both directions keeps ordinary log output (which may incidentally start a
// line with '-') from being re-colored.
func looksLikeDiff(output string) bool {
	var del, add bool
	for line := range strings.SplitSeq(output, "\n") {
		switch classifyDiffLine(line) {
		case kindDel:
			del = true
		case kindAdd:
			add = true
		case kindPlain, kindHunk, kindContext:
		}
		if del && add {
			return true
		}
	}
	return false
}

func classifyDiffLine(line string) diffLineKind {
	switch {
	case strings.HasPrefix(line, "@@"),
		strings.HasPrefix(line, "---"),
		strings.HasPrefix(line, "+++"):
		return kindHunk
	case strings.HasPrefix(line, "-"):
		return kindDel
	case strings.HasPrefix(line, "+"):
		return kindAdd
	case strings.HasPrefix(line, "  "):
		return kindContext
	default:
		return kindPlain
	}
}

// RenderDiffOutput themes a test's failure output when it is a structural
// diff (go-cmp / cupaloy), coloring removals red, additions green, and
// hunk/context lines muted so the mismatch is legible at a glance. Output
// that is not a diff is returned unchanged. The mono theme yields the same
// text with no escapes, so LLM/piped consumers see clean plain output.
func RenderDiffOutput(output string, t theme.Theme) string {
	if !looksLikeDiff(output) {
		return output
	}
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		switch classifyDiffLine(line) {
		case kindDel:
			lines[i] = renderKeepTabs(t.Error, line)
		case kindAdd:
			lines[i] = renderKeepTabs(t.Pass, line)
		case kindHunk, kindContext:
			lines[i] = renderKeepTabs(t.Muted, line)
		case kindPlain:
			// leave as-is
		}
	}
	return strings.Join(lines, "\n")
}

// renderKeepTabs applies a style without lipgloss's default tab→spaces
// conversion. Tabs are structural indentation in a go-cmp/cupaloy diff;
// flattening them to spaces would misalign the very columns the structural
// view exists to make legible.
func renderKeepTabs(s lipgloss.Style, line string) string {
	return s.TabWidth(lipgloss.NoTabConversion).Render(line)
}

// RenderStructuralDiff formats a field-structural diff (go-cmp/cupaloy,
// decomposed by pkg/testjson) as an aligned path/removed/added table —
// the reader sees which fields differ and their old/new values, not which
// lines of text differ. Follows the same Tufte idiom as the rest of
// pkg/view: paint.Columnize for alignment, theme colors for signal, no
// box-drawing. The mono theme (LLM/piped) yields the same table with no
// escapes. Returns "" for a nil diff or one with no fields.
func RenderStructuralDiff(sd *report.StructuralDiff, t theme.Theme) string {
	if sd == nil || len(sd.Fields) == 0 {
		return ""
	}
	rows := make([][]string, 0, len(sd.Fields))
	for _, f := range sd.Fields {
		rows = append(rows, []string{
			t.Muted.Render(f.Path),
			diffSide(f.Removed, t.Error, "-", t),
			diffSide(f.Added, t.Pass, "+", t),
		})
	}
	return paint.Columnize(rows, 2)
}

// diffSide renders one side of a field's before/after value, prefixed
// with its diff marker, or the theme's Same icon when that side is empty
// (a field that was only added or only removed) — the same "nothing to
// show here" glyph Delta/bullet/leaderboard use, so Mono/llm output gets
// the ASCII "=" instead of a hardcoded non-ASCII placeholder.
func diffSide(value string, style lipgloss.Style, marker string, t theme.Theme) string {
	if value == "" {
		return t.Icons.Same
	}
	return style.Render(marker + " " + value)
}
