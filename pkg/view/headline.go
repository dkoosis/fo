package view

import (
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

func renderHeadline(v Headline, t theme.Theme) string {
	out := t.Heading.Render(v.Title)
	if v.Detail != "" {
		out += "\n" + t.Muted.Render(v.Detail)
	}
	for _, line := range v.Body {
		if line == "" {
			continue
		}
		out += "\n" + t.Muted.Render(line)
	}
	return out
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
