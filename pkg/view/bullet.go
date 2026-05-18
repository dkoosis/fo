package view

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

// styler is a single-arg adapter over lipgloss.Style.Render (which is
// variadic). Using it lets glyphFor return a uniform 1-arg renderer.
type styler func(string) string

func wrap(s lipgloss.Style) styler { return func(x string) string { return s.Render(x) } }

// glyphFor returns the styled glyph for a row, picking severity over
// outcome when both are set. Falls back to the bullet glyph + identity
// style when neither is set.
func glyphFor(item BulletItem, t theme.Theme) (string, styler) {
	if item.Severity != "" {
		switch item.Severity {
		case report.SeverityError:
			return t.Icons.Fail, wrap(t.Error)
		case report.SeverityWarning:
			return t.Icons.Warn, wrap(t.Warning)
		case report.SeverityNote:
			return t.Icons.Note, wrap(t.Note)
		}
	}
	if item.Outcome != "" {
		switch item.Outcome {
		case report.OutcomePass:
			return t.Icons.Pass, wrap(t.Pass)
		case report.OutcomeFail:
			return t.Icons.Fail, wrap(t.Fail)
		case report.OutcomeSkip:
			return t.Icons.Note, wrap(t.Skip)
		case report.OutcomePanic:
			return t.Icons.Panic, wrap(t.Panic)
		case report.OutcomeBuildError:
			return t.Icons.BuildError, wrap(t.BuildError)
		}
	}
	return t.Icons.Bullet, func(s string) string { return s }
}

// bulletRows builds the [][]string columnize input plus a parallel
// slice of fix lines (one entry per row, "" when no fix).
func bulletRows(items []BulletItem, t theme.Theme) ([][]string, []string) {
	rows := make([][]string, 0, len(items))
	fixes := make([]string, 0, len(items))
	for _, it := range items {
		glyph, style := glyphFor(it, t)
		rows = append(rows, []string{style(glyph), it.Label, t.Muted.Render(it.Value)})
		if it.FixCommand != "" {
			fixes = append(fixes, "  "+t.Muted.Render("fix: "+it.FixCommand))
		} else {
			fixes = append(fixes, "")
		}
	}
	return rows, fixes
}

// interleave Columnize output with fix lines. Columnize produces one
// '\n'-joined string; we split, then weave in the fix lines that
// belong to each row.
func interleaveFixes(table string, fixes []string) string {
	if table == "" {
		return ""
	}
	lines := strings.Split(table, "\n")
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
		if i < len(fixes) && fixes[i] != "" {
			b.WriteString(fixes[i])
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderBullet(v Bullet, t theme.Theme) string {
	if len(v.Items) == 0 {
		return ""
	}
	// Clusters render as multi-line blocks outside the columnize grid;
	// singletons share one grid below. Order is preserved.
	var blocks []string
	var singletons []BulletItem
	flushSingletons := func() {
		if len(singletons) == 0 {
			return
		}
		rows, fixes := bulletRows(singletons, t)
		blocks = append(blocks, interleaveFixes(paint.Columnize(rows, 2), fixes))
		singletons = singletons[:0]
	}
	for _, it := range v.Items {
		if it.Cluster != nil {
			flushSingletons()
			blocks = append(blocks, renderClusterBlock(it.Cluster, t, it.Cluster.LLMMode))
			continue
		}
		singletons = append(singletons, it)
	}
	flushSingletons()
	return strings.Join(blocks, "\n")
}

// renderClusterBlock paints a ClusterRender as header + indented member rows.
// llmMode switches to Shape A (shared-output dedupe) / Shape B (per-member).
func renderClusterBlock(cr *ClusterRender, t theme.Theme, llmMode bool) string {
	var b strings.Builder
	if llmMode {
		b.WriteString(cr.Header)
		b.WriteByte('\n')
		if cr.UsesSharedRow {
			b.WriteString("  shared: ")
			b.WriteString(cr.SharedOutput)
			b.WriteByte('\n')
			names := make([]string, len(cr.Members))
			for i := range cr.Members {
				names[i] = cr.Members[i].Test
			}
			b.WriteString("  members: ")
			b.WriteString(strings.Join(names, ", "))
			return b.String()
		}
		for i := range cr.Members {
			m := &cr.Members[i]
			b.WriteString("  ")
			b.WriteString(m.Test)
			b.WriteString(": ")
			b.WriteString(m.Output)
			if i < len(cr.Members)-1 {
				b.WriteByte('\n')
			}
		}
		return b.String()
	}
	// Human mode: theme the header — signature+count via Heading,
	// the "--expand=…" hint via Muted.
	header := cr.Header
	if idx := strings.Index(header, " · --expand="); idx >= 0 {
		header = t.Heading.Render(header[:idx]) + t.Muted.Render(header[idx:])
	} else {
		header = t.Heading.Render(header)
	}
	b.WriteString(header)
	b.WriteByte('\n')
	rows, fixes := bulletRows(membersAsItems(cr.Members), t)
	body := interleaveFixes(paint.Columnize(rows, 2), fixes)
	// Indent member lines by 2 spaces for visual grouping.
	for line := range strings.SplitSeq(body, "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// membersAsItems wraps cluster members as BulletItems so they reuse the
// existing test-line formatter (glyph + label + value).
func membersAsItems(members []report.TestResult) []BulletItem {
	out := make([]BulletItem, 0, len(members))
	for i := range members {
		out = append(out, testItem(members[i]))
	}
	return out
}

func renderGrouped(v Grouped, t theme.Theme) string {
	var sections []string
	for _, sec := range v.Sections {
		if len(sec.Items) == 0 {
			continue
		}
		head := t.Heading.Render(sec.Label)
		rows, fixes := bulletRows(sec.Items, t)
		body := interleaveFixes(paint.Columnize(rows, 2), fixes)
		sections = append(sections, head+"\n"+body)
	}
	return strings.Join(sections, "\n\n")
}
