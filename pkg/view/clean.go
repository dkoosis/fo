package view

import "github.com/dkoosis/fo/pkg/theme"

func renderClean(v Clean, t theme.Theme) string {
	msg := v.Message
	if msg == "" {
		msg = "no findings"
	}
	return t.Pass.Render(t.Icons.Pass + " " + msg)
}
