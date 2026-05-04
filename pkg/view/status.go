package view

import (
	"fmt"
	"io"
	"strings"
)

type StatusRow struct {
	State string
	Label string
	Value string
	Note  string
}

// RenderStatusLLM emits one row per line: aligned state token, label,
// optional value, optional note. Token-dense; no decoration.
func RenderStatusLLM(w io.Writer, tool string, rows []StatusRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "# %s\n", tool); err != nil {
			return err
		}
	}
	labelMax := 0
	for _, r := range rows {
		if l := len(r.Label); l > labelMax {
			labelMax = l
		}
	}
	for _, r := range rows {
		extra := strings.TrimSpace(r.Value + " " + r.Note)
		if _, err := fmt.Fprintf(w, "%-4s %-*s  %s\n", r.State, labelMax, r.Label, extra); err != nil {
			return err
		}
	}
	return nil
}

// RenderStatusHuman emits a banner + counts header followed by the LLM
// table body. Mono-only first cut; theme routing comes later.
func RenderStatusHuman(w io.Writer, tool string, rows []StatusRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "── %s ──\n", tool); err != nil {
			return err
		}
	}
	var ok, fail, warn, skip int
	for _, r := range rows {
		switch r.State {
		case "ok":
			ok++
		case "fail":
			fail++
		case "warn":
			warn++
		case "skip":
			skip++
		}
	}
	if _, err := fmt.Fprintf(w, "%d ok · %d fail · %d warn · %d skip\n\n", ok, fail, warn, skip); err != nil {
		return err
	}
	return RenderStatusLLM(w, "", rows)
}
