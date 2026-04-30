package view

import (
	"strings"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

func renderHeadline(v Headline, t theme.Theme) string {
	var b strings.Builder
	b.WriteString(t.Heading.Render(v.Title))
	if v.Detail != "" {
		b.WriteString("\n")
		b.WriteString(t.Muted.Render(v.Detail))
	}
	for _, line := range v.Body {
		if line == "" {
			continue
		}
		b.WriteString("\n")
		b.WriteString(t.Muted.Render(line))
	}
	return b.String()
}

func renderAlert(v Alert, t theme.Theme) string {
	style := t.Bold
	switch v.Severity {
	case report.SeverityError:
		style = t.Error.Bold(true)
	case report.SeverityWarning:
		style = t.Warning.Bold(true)
	case report.SeverityNote:
		style = t.Note
	}
	prefix := style.Render(v.Prefix)
	value := t.Bold.Render(v.Value)
	out := prefix + " " + value
	if v.Detail != "" {
		out += " " + t.Muted.Render(v.Detail)
	}
	return out
}
