package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/report"
)

// RunLogVersion is the on-disk version for the replay/trend run log,
// independent of the diff sidecar and findings snapshot.
const RunLogVersion = 1

// MaxRunLog bounds how many runs the log retains. Old enough to spot a
// regression that crept in across a dozen "looked fine" runs, small enough
// that the file stays trivially cheap to read and rewrite.
const MaxRunLog = 100

// RunLogPath returns the resolved run-log path. The log is an append-only
// (bounded) history of run summaries powering `fo replay` and `fo trend`.
func RunLogPath() string { return filepath.Join(Dir(), "run-log.json") }

// RunLogEntry is one run's summary: enough to chart a rule's trend or list
// recent runs, but not the full findings (those are the snapshot's job).
type RunLogEntry struct {
	At          time.Time      `json:"at"`
	Tool        string         `json:"tool,omitempty"`
	RuleCounts  map[string]int `json:"rule_counts,omitempty"`
	Errors      int            `json:"errors"`
	Warnings    int            `json:"warnings"`
	Notes       int            `json:"notes"`
	TestsFailed int            `json:"tests_failed"`
	TestsPassed int            `json:"tests_passed"`
}

// RunLog is the on-disk envelope. Entries run oldest-first so a trend
// sparkline reads left (past) to right (present).
type RunLog struct {
	Version int           `json:"version"`
	Entries []RunLogEntry `json:"entries"`
}

// RunLogEntryFromReport summarizes a Report for the log. GeneratedAt is
// used when set so replayed timestamps match the run, falling back to wall
// clock for synthetic reports.
func RunLogEntryFromReport(r *report.Report) RunLogEntry {
	e := RunLogEntry{At: r.GeneratedAt, Tool: r.Tool, RuleCounts: map[string]int{}}
	if e.At.IsZero() {
		e.At = time.Now().UTC()
	}
	for i := range r.Findings {
		f := &r.Findings[i]
		if f.RuleID != "" {
			e.RuleCounts[f.RuleID]++
		}
		switch f.Severity {
		case report.SeverityError:
			e.Errors++
		case report.SeverityWarning:
			e.Warnings++
		case report.SeverityNote:
			e.Notes++
		}
	}
	for i := range r.Tests {
		switch r.Tests[i].Outcome {
		case report.OutcomeFail, report.OutcomePanic, report.OutcomeBuildError:
			e.TestsFailed++
		case report.OutcomePass:
			e.TestsPassed++
		case report.OutcomeSkip:
			// skips are neither pass nor fail for trend purposes
		}
	}
	if len(e.RuleCounts) == 0 {
		e.RuleCounts = nil
	}
	return e
}

// LoadRunLog reads the run log. Missing file → (nil, nil). Version skew is
// treated like a missing file by callers (start fresh).
func LoadRunLog(path string) (*RunLog, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil //nolint:nilnil // missing log is not an error
		}
		return nil, fmt.Errorf("state: open %s: %w", path, err)
	}
	defer f.Close()
	b, err := boundread.All(f, sidecarMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var rl RunLog
	if err := json.Unmarshal(b, &rl); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if rl.Version != RunLogVersion {
		return nil, ErrVersionSkew
	}
	return &rl, nil
}

// AppendRunLog returns a new RunLog with entry pushed onto the end and the
// history trimmed to MaxRunLog (oldest dropped first).
func AppendRunLog(prev *RunLog, entry RunLogEntry) *RunLog {
	out := &RunLog{Version: RunLogVersion}
	if prev != nil {
		out.Entries = append(out.Entries, prev.Entries...)
	}
	out.Entries = append(out.Entries, entry)
	if len(out.Entries) > MaxRunLog {
		out.Entries = out.Entries[len(out.Entries)-MaxRunLog:]
	}
	return out
}

// SaveRunLog writes rl atomically, mirroring Save's durability contract.
func SaveRunLog(path string, rl *RunLog) error {
	return writeAtomic(path, ".run-log.*.tmp", rl)
}

// RuleSeries returns the per-run count of ruleID across the log, oldest
// first — the input to a trend sparkline. A run with no occurrences
// contributes 0 so gaps are visible rather than skipped.
func (rl *RunLog) RuleSeries(ruleID string) []int {
	if rl == nil {
		return nil
	}
	out := make([]int, len(rl.Entries))
	for i := range rl.Entries {
		out[i] = rl.Entries[i].RuleCounts[ruleID]
	}
	return out
}
