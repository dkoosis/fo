// Package testjson parses `go test -json` NDJSON streams into the
// renderer-facing report.Report.
//
// Two parse modes:
//   - ParseBytes — buffered, for completed runs.
//   - Stream — incremental, so fo can render under a live test run
//     without waiting for EOF.
//
// Aggregation lives in funcresults.go (per-test) and stats.go (per-package);
// toreport.go lowers the aggregated results into a report.Report whose
// Tests slice the view layer consumes. Build failures and panics are
// preserved as first-class outcomes, not collapsed into "fail".
package testjson

import (
	"time"

	"github.com/dkoosis/fo/pkg/report"
)

// Status represents the outcome of a test package.
type Status string

// Status values returned by TestPackageResult.Status.
const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// Action is a TestEvent.Action value from `go test -json`.
type Action string

// TestEvent.Action values from `go test -json`. pass/fail/skip share the
// same string values as the Status constants above but are a distinct
// type — an Action describes one event, a Status summarizes a package.
const (
	ActionStart       Action = "start"
	ActionRun         Action = "run"
	ActionPass        Action = "pass"
	ActionFail        Action = "fail"
	ActionSkip        Action = "skip"
	ActionOutput      Action = "output"
	ActionBuildOutput Action = "build-output"
	ActionBuildFail   Action = "build-fail"
	ActionBench       Action = "bench"
	ActionPause       Action = "pause"
	ActionCont        Action = "cont"
)

// TestEvent represents a single event from go test -json output.
type TestEvent struct {
	Time       time.Time `json:"Time"`
	Action     Action    `json:"Action"`
	Package    string    `json:"Package"`
	Test       string    `json:"Test"`
	Elapsed    float64   `json:"Elapsed"`
	Output     string    `json:"Output"`
	ImportPath string    `json:"ImportPath"` // set on build-output / build-fail events
}

// TestPackageResult represents aggregated results for one package.
type TestPackageResult struct {
	Name        string
	Passed      int
	Failed      int
	Skipped     int
	Duration    time.Duration
	Coverage    float64
	FailedTests []FailedTest
	BuildError  string // non-empty if package failed to build
	Panicked    bool
	PanicOutput []string
}

// FailedTest captures a test failure with its output.
type FailedTest struct {
	Name   string
	Output []string

	// StructuralDiff, when set, is the field-level decomposition detected
	// for Output — computed once by the aggregator at the test's terminal
	// fail event (see handleFail/results in parser.go) and carried through
	// every later results() snapshot untouched, so a streaming run's
	// repeated ToReport calls over the accumulated result set don't
	// redetect it on every tick. nil means "not yet detected" (or, for a
	// FailedTest built directly rather than through the aggregator,
	// "caller didn't run detection") — ToReport computes it on demand in
	// that case.
	StructuralDiff *report.StructuralDiff
}

// TotalTests returns the total number of tests in this package.
func (r *TestPackageResult) TotalTests() int {
	return r.Passed + r.Failed + r.Skipped
}

// Status returns StatusPass, StatusFail, or StatusSkip for the package.
func (r *TestPackageResult) Status() Status {
	if r.BuildError != "" || r.Panicked || r.Failed > 0 {
		return StatusFail
	}
	if r.Passed == 0 && r.Skipped > 0 {
		return StatusSkip
	}
	return StatusPass
}
