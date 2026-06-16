package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/state"
)

// attachDiff loads prior state, classifies the current report, sets
// r.Diff, then appends and saves. Save failure is reported back to the
// caller (so --state-strict can escalate) and recorded on r.Notices so
// every renderer — including JSON consumers and LLMs — sees that the
// next run's NEW/REGRESSED classification will be stale.
func attachDiff(r *report.Report, statePath string, policy statePolicy, stderr io.Writer) error {
	if policy == stateOff || r == nil || statePath == "" {
		return nil
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
		// Durability degraded (rename succeeded, parent-dir fsync
		// failed): the new state IS on disk, but on NFS/virtualized FS
		// a subsequent crash could revert the rename and re-surface
		// resolved findings as new. Surface as a Notice so LLM/JSON
		// consumers see it; do not fail the run, and do not treat as a
		// strict-save failure (fo-1x0).
		if errors.Is(err, state.ErrDurabilityDegraded) {
			fmt.Fprintf(stderr, "fo: state: warning: %v\n", err)
			r.Notices = append(r.Notices,
				fmt.Sprintf("state: durability degraded (%v) — sidecar may revert under crash; next run's diff may be stale", err))
			return nil
		}
		fmt.Fprintf(stderr, "fo: state: save: %v\n", err)
		r.Notices = append(r.Notices,
			fmt.Sprintf("state: save failed (%v) — next run's diff classification may be stale", err))
		return err
	}
	return nil
}

// assignAndPersistIDs assigns short handles (F-/T-) to the report and,
// unless state is off, pins them across runs: it loads the prior findings
// snapshot for handle stability, assigns, then writes the new snapshot for
// `fo explain` to resolve against. Snapshot I/O is best-effort — a failure
// degrades cross-run stability and explain, never the run itself, so it is
// surfaced on stderr only.
func assignAndPersistIDs(r *report.Report, policy statePolicy, stderr io.Writer) {
	if policy == stateOff || r == nil {
		report.AssignShortIDs(r)
		return
	}
	path := state.SnapshotPath()
	prev, err := state.LoadSnapshot(path)
	if err != nil {
		fmt.Fprintf(stderr, "fo: state: snapshot: %v (ids not pinned)\n", err)
		prev = nil
	}
	report.AssignShortIDsStable(r, prev.PriorIDs())
	if err := state.SaveSnapshot(path, state.SnapshotFromReport(r)); err != nil &&
		!errors.Is(err, state.ErrDurabilityDegraded) {
		fmt.Fprintf(stderr, "fo: state: snapshot save: %v (explain unavailable next run)\n", err)
	}
}

// recordRun appends a one-line summary of this run to the bounded run log
// that powers `fo replay` and `fo trend`. Like the snapshot, it is
// best-effort: a write failure costs history, never the run, so it is
// surfaced on stderr only and skipped entirely when state is off.
func recordRun(r *report.Report, policy statePolicy, stderr io.Writer) {
	if policy == stateOff || r == nil {
		return
	}
	path := state.RunLogPath()
	prev, err := state.LoadRunLog(path)
	if err != nil {
		fmt.Fprintf(stderr, "fo: state: run-log: %v (starting fresh)\n", err)
		prev = nil
	}
	updated := state.AppendRunLog(prev, state.RunLogEntryFromReport(r))
	if err := state.SaveRunLog(path, updated); err != nil &&
		!errors.Is(err, state.ErrDurabilityDegraded) {
		fmt.Fprintf(stderr, "fo: state: run-log save: %v (trend/replay may miss this run)\n", err)
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
		NewFailures:     convertItems(e.NewFailures),
		FixedFailures:   convertItems(e.FixedFailures),
		FlakyTests:      convertItems(e.FlakyTests),
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
	if r == nil {
		return
	}
	if r.Diff == nil {
		writeNotices(w, r)
		return
	}
	newItems := r.Diff.New
	regressed := r.Diff.Regressed
	if len(newItems) == 0 && len(regressed) == 0 {
		writeNotices(w, r)
		return
	}

	// Build fingerprint → finding index for O(1) lookup.
	byFP := make(map[string]report.Finding, len(r.Findings))
	for i := range r.Findings {
		if r.Findings[i].Fingerprint != "" {
			byFP[r.Findings[i].Fingerprint] = r.Findings[i]
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
	writeNotices(w, r)
}

// writeNotices emits a NOTICES block for LLM consumers so operational
// degradation (e.g. failed state.Save) is visible alongside findings,
// not just on stderr where Claude-as-consumer never sees it.
func writeNotices(w io.Writer, r *report.Report) {
	if r == nil || len(r.Notices) == 0 {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "\nNOTICES (%d)\n", len(r.Notices))
	for _, n := range r.Notices {
		fmt.Fprintf(&sb, "  %s\n", n)
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
