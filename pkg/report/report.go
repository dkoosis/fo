// Package report defines the canonical Report shape that flows from
// parser to pickView to renderer in fo's v2 pipeline. One Report per
// analysis run; no overloaded status, no dual-dispatch.
package report

import "time"

// Severity is the level of a static-analysis finding. The set is closed:
// SARIF "none" maps to SeverityNote.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

// TestOutcome is the result of a single test or package execution.
// Panic and BuildError are first-class outcomes, not flavors of fail —
// they need distinct treatment in the Headline view.
type TestOutcome string

const (
	OutcomePass       TestOutcome = "pass"
	OutcomeFail       TestOutcome = "fail"
	OutcomeSkip       TestOutcome = "skip"
	OutcomePanic      TestOutcome = "panic"
	OutcomeBuildError TestOutcome = "build_error"
)

// Finding is a single static-analysis result. A Report's Findings slice is
// the canonical input to the Bullet, Grouped, Leaderboard, and Headline
// views.
type Finding struct {
	RuleID      string   `json:"rule_id,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Col         int      `json:"col,omitempty"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	FixCommand  string   `json:"fix_command,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Score       float64  `json:"score,omitempty"`
}

// TestResult is a single test or package outcome from go test -json.
// Test == "" means a package-level result (build error, panic, or whole-pkg
// pass/fail rollup).
type TestResult struct {
	Package     string        `json:"package"`
	Test        string        `json:"test,omitempty"`
	Outcome     TestOutcome   `json:"outcome"`
	Duration    time.Duration `json:"duration_ns,omitempty"`
	Output      string        `json:"output,omitempty"`
	FixCommand  string        `json:"fix_command,omitempty"`
	Fingerprint string        `json:"fingerprint,omitempty"`
	Score       float64       `json:"score,omitempty"`
}

// Report is the canonical shape from parser to pickView to renderer.
// One Report per analysis run. Substrate parsers produce it via ToReport;
// the renderer consumes it via pickView.
//
// Findings and Tests are flat — the renderer groups them when needed.
// Diff, when non-nil, drives both the JSON contract and the Delta view
// wrapper in pickView.
type Report struct {
	Tool        string       `json:"tool,omitempty"`
	GeneratedAt time.Time    `json:"generated_at"`
	DataHash    string       `json:"data_hash,omitempty"`
	Findings    []Finding    `json:"findings,omitempty"`
	Tests       []TestResult `json:"tests,omitempty"`
	Diff        *DiffSummary `json:"diff,omitempty"`
}

// DiffItem mirrors the shape of state.Item without importing pkg/state
// (state already depends on report; this preserves the one-way edge).
type DiffItem struct {
	Fingerprint   string `json:"fingerprint"`
	RuleID        string `json:"rule_id,omitempty"`
	File          string `json:"file,omitempty"`
	Severity      string `json:"severity"`
	PriorSeverity string `json:"prior_severity,omitempty"`
	Class         string `json:"class"`
}

// DiffSummary mirrors state.Envelope. Owned by pkg/report so the JSON
// contract for Report sits in one place; the CLI converts state.Envelope
// → DiffSummary at the wire-in seam.
type DiffSummary struct {
	Headline        string     `json:"headline"`
	New             []DiffItem `json:"new"`
	Resolved        []DiffItem `json:"resolved"`
	Regressed       []DiffItem `json:"regressed"`
	Flaky           []DiffItem `json:"flaky"`
	PersistentCount int        `json:"persistent_count"`
}
