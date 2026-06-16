package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/state"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
)

// runExplain handles `fo explain <id>` — it resolves a short handle
// (F-7a2 / T-3f1) emitted by a prior run back to the full finding or test,
// reading the findings snapshot written by that run. The handle is the
// addressable surface: an agent reading fo's output can ask fo to expand
// any line it cares about without re-running the underlying tool.
func runExplain(args []string, stdout, stderr io.Writer) int {
	id := ""
	for _, a := range args {
		if a == "-h" || a == flagHelp {
			fmt.Fprintln(stderr, "usage: fo explain <id>   (id is a handle like F-7a2 or T-3f1 from a prior run)")
			return 0
		}
		if !strings.HasPrefix(a, "-") && id == "" {
			id = a
		}
	}
	if id == "" {
		fmt.Fprintln(stderr, "fo explain: an id is required (e.g. fo explain F-7a2)")
		return 2
	}

	snap, err := state.LoadSnapshot(state.SnapshotPath())
	if err != nil {
		fmt.Fprintf(stderr, "fo explain: %v\n", err)
		return 2
	}
	if snap == nil {
		fmt.Fprintln(stderr, "fo explain: no prior run found — run fo once to produce handles, then explain one")
		return 2
	}

	f, tr, ok := snap.Lookup(id)
	if !ok {
		fmt.Fprintf(stderr, "fo explain: %s not found in the last run (handles expire when a new run overwrites them)\n", id)
		return 2
	}

	t := resolveTheme("auto", stdout)
	if f != nil {
		fmt.Fprint(stdout, explainFinding(f, t))
	} else {
		fmt.Fprint(stdout, explainTest(tr, t))
	}
	return 0
}

func explainFinding(f *report.Finding, t theme.Theme) string {
	var b strings.Builder
	rule := f.RuleID
	if rule == "" {
		rule = "(no rule id)"
	}
	loc := fmt.Sprintf("%s:%d", f.File, f.Line)
	if f.Col > 0 {
		loc += fmt.Sprintf(":%d", f.Col)
	}
	fmt.Fprintf(&b, "%s  %s  %s  %s\n",
		t.Heading.Render(f.ID), severityWord(f.Severity, t), rule, t.Muted.Render(loc))
	fmt.Fprintf(&b, "  %s\n", f.Message)
	if f.FixCommand != "" {
		fmt.Fprintf(&b, "  %s\n", t.Muted.Render("fix: "+f.FixCommand))
	}
	if url := ruleDocURL(rule); url != "" {
		fmt.Fprintf(&b, "  %s\n", t.Muted.Render("docs: "+url))
	}
	return b.String()
}

func explainTest(tr *report.TestResult, t theme.Theme) string {
	var b strings.Builder
	name := tr.Test
	if name == "" {
		name = "(package-level)"
	}
	fmt.Fprintf(&b, "%s  %s  %s  %s\n",
		t.Heading.Render(tr.ID), string(tr.Outcome), name, t.Muted.Render(tr.Package))
	if tr.FixCommand != "" {
		fmt.Fprintf(&b, "  %s\n", t.Muted.Render("fix: "+tr.FixCommand))
	}
	if out := strings.TrimRight(tr.Output, "\n"); out != "" {
		rendered := view.RenderDiffOutput(out, t)
		for line := range strings.SplitSeq(rendered, "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}
	return b.String()
}

func severityWord(s report.Severity, t theme.Theme) string {
	switch s {
	case report.SeverityError:
		return t.Error.Render("error")
	case report.SeverityWarning:
		return t.Warning.Render("warning")
	case report.SeverityNote:
		return t.Note.Render("note")
	default:
		return string(s)
	}
}

// ruleDocURL returns a canonical documentation URL for rule families fo
// recognizes, or "" when none is known. fo does not fetch — it points the
// reader at the authoritative page so an agent stops re-deriving the same
// rule docs (capability #8). staticcheck's families are stably addressable;
// other tools get no guessed URL rather than a wrong one.
func ruleDocURL(rule string) string {
	switch {
	case hasAnyPrefix(rule, "SA", "ST", "S1", "QF"):
		return "https://staticcheck.dev/docs/checks#" + rule
	default:
		return ""
	}
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
