package main

import (
	"fmt"
	"io"

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
