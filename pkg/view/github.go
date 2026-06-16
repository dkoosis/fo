package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
)

// RenderGitHub writes findings as GitHub Actions workflow commands, the
// `::error file=…,line=…,col=…::message` annotations GitHub renders inline
// on a pull request's changed lines. No custom Action is needed — any
// workflow step that pipes a tool through `fo --format=github` gets
// reviewed-scoped annotations for free.
//
// When the Report carries diff classification, only NEW and REGRESSED
// findings are emitted: persistent legacy findings would otherwise drown
// the diff's signal, the exact problem the GitHub view exists to solve. A
// Report without a diff (e.g. --no-state) emits every finding.
//
// Tests are intentionally omitted — annotations are line-anchored and a
// failing test has no single source line to point at.
func RenderGitHub(w io.Writer, r report.Report) error {
	scope := changedFingerprints(r.Diff)
	for i := range r.Findings {
		f := &r.Findings[i]
		if scope != nil {
			if _, ok := scope[f.Fingerprint]; !ok {
				continue
			}
		}
		if _, err := fmt.Fprintln(w, ghAnnotation(f)); err != nil {
			return err
		}
	}
	return nil
}

// changedFingerprints returns the set of fingerprints to surface (new +
// regressed), or nil when there is no diff to scope by — nil means "emit
// everything", distinct from an empty set ("diff present, nothing new").
func changedFingerprints(d *report.DiffSummary) map[string]struct{} {
	if d == nil {
		return nil
	}
	set := make(map[string]struct{}, len(d.New)+len(d.Regressed))
	for _, it := range d.New {
		set[it.Fingerprint] = struct{}{}
	}
	for _, it := range d.Regressed {
		set[it.Fingerprint] = struct{}{}
	}
	return set
}

func ghAnnotation(f *report.Finding) string {
	var b strings.Builder
	b.WriteString("::")
	b.WriteString(ghLevel(f.Severity))
	b.WriteString(" file=")
	b.WriteString(ghEscapeProp(f.File))
	if f.Line > 0 {
		fmt.Fprintf(&b, ",line=%d", f.Line)
	}
	if f.Col > 0 {
		fmt.Fprintf(&b, ",col=%d", f.Col)
	}
	if f.ID != "" || f.RuleID != "" {
		b.WriteString(",title=")
		b.WriteString(ghEscapeProp(ghTitle(f)))
	}
	b.WriteString("::")
	b.WriteString(ghEscapeData(f.Message))
	return b.String()
}

func ghTitle(f *report.Finding) string {
	switch {
	case f.ID != "" && f.RuleID != "":
		return f.ID + " " + f.RuleID
	case f.ID != "":
		return f.ID
	default:
		return f.RuleID
	}
}

func ghLevel(s report.Severity) string {
	switch s {
	case report.SeverityError:
		return "error"
	case report.SeverityWarning:
		return "warning"
	case report.SeverityNote:
		return "notice"
	default:
		return "notice"
	}
}

// ghEscapeData escapes a workflow-command message per GitHub's rules:
// %→%25, CR→%0D, LF→%0A.
func ghEscapeData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// ghEscapeProp escapes a property value: the data escapes plus ,→%2C and
// :→%3A so a path or title cannot break out of the property list.
func ghEscapeProp(s string) string {
	s = ghEscapeData(s)
	s = strings.ReplaceAll(s, ",", "%2C")
	s = strings.ReplaceAll(s, ":", "%3A")
	return s
}
