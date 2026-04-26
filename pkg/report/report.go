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
	RuleID      string
	File        string
	Line        int
	Col         int
	Severity    Severity
	Message     string
	FixCommand  string
	Fingerprint string
	Score       float64
}

// TestResult is a single test or package outcome from go test -json.
// Test == "" means a package-level result (build error, panic, or whole-pkg
// pass/fail rollup).
type TestResult struct {
	Package     string
	Test        string
	Outcome     TestOutcome
	Duration    time.Duration
	Output      string
	FixCommand  string
	Fingerprint string
	Score       float64
}

// Report is the canonical shape from parser to pickView to renderer.
// One Report per analysis run. Substrate parsers produce it via ToReport;
// the renderer consumes it via pickView.
//
// Findings and Tests are flat — the renderer groups them when needed.
// Prior carries a previous-run reference for the Delta view (fo-40z.2);
// nil when no prior state exists.
type Report struct {
	Tool        string
	GeneratedAt time.Time
	DataHash    string

	Findings []Finding
	Tests    []TestResult

	Prior *Report
}
