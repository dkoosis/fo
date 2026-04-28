package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/state"
)

// attachDiff loads prior state, classifies the current report, sets
// r.Diff, then appends and saves. Tolerant: any I/O or parse error
// leaves r.Diff unset and emits a single warning to stderr — the run
// itself must not fail because of sidecar trouble.
func attachDiff(r *report.Report, statePath string, noState bool, stderr io.Writer) {
	if noState || r == nil || statePath == "" {
		return
	}
	prev, err := state.Load(statePath)
	if err != nil {
		fmt.Fprintf(stderr, "fo: state: %v (starting fresh)\n", err)
		prev = nil
	}
	d := state.Classify(prev, r)
	env := state.EnvelopeOf(d)
	r.Diff = envelopeToDiffSummary(env)

	updated := state.Append(prev, state.RunFromReport(r))
	if err := state.Save(statePath, updated); err != nil {
		fmt.Fprintf(stderr, "fo: state: save: %v\n", err)
	}
}

func envelopeToDiffSummary(e state.Envelope) *report.DiffSummary {
	return &report.DiffSummary{
		Headline:        e.Headline,
		New:             convertItems(e.New),
		Resolved:        convertItems(e.Resolved),
		Regressed:       convertItems(e.Regressed),
		Flaky:           convertItems(e.Flaky),
		PersistentCount: e.PersistentCount,
	}
}

func convertItems(items []state.Item) []report.DiffItem {
	out := make([]report.DiffItem, len(items))
	for i, it := range items {
		out[i] = report.DiffItem{
			Fingerprint:   it.Fingerprint,
			RuleID:        it.RuleID,
			File:          it.File,
			Severity:      string(it.Severity),
			PriorSeverity: string(it.PriorSeverity),
			Class:         string(it.Class),
		}
	}
	return out
}

// writeDiffDetail emits a plain-text block listing new and regressed
// findings with file:line rule message, for LLM consumers who need to
// act on the delta rather than just count it.
func writeDiffDetail(w io.Writer, r *report.Report) {
	if r == nil || r.Diff == nil {
		return
	}
	newItems := r.Diff.New
	regressed := r.Diff.Regressed
	if len(newItems) == 0 && len(regressed) == 0 {
		return
	}

	// Build fingerprint → finding index for O(1) lookup.
	byFP := make(map[string]report.Finding, len(r.Findings))
	for _, f := range r.Findings {
		if f.Fingerprint != "" {
			byFP[f.Fingerprint] = f
		}
	}

	var sb strings.Builder
	if len(newItems) > 0 {
		fmt.Fprintf(&sb, "\nNEW (%d)\n", len(newItems))
		for _, item := range newItems {
			writeDiffLine(&sb, item, byFP)
		}
	}
	if len(regressed) > 0 {
		fmt.Fprintf(&sb, "\nREGRESSED (%d)\n", len(regressed))
		for _, item := range regressed {
			writeDiffLine(&sb, item, byFP)
		}
	}
	_, _ = io.WriteString(w, sb.String())
}

func writeDiffLine(sb *strings.Builder, item report.DiffItem, byFP map[string]report.Finding) {
	if f, ok := byFP[item.Fingerprint]; ok {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(sb, "  %s  %s  %s\n", loc, f.RuleID, f.Message)
	} else {
		fmt.Fprintf(sb, "  %s  %s\n", item.File, item.RuleID)
	}
}
