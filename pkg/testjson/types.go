// Package testjson parses go test -json NDJSON streams.
package testjson

import "time"

// Status represents the outcome of a test package.
type Status string

// Status values returned by TestPackageResult.Status.
const (
	StatusPass Status = "pass"
	StatusFail Status = "fail"
	StatusSkip Status = "skip"
)

// TestEvent represents a single event from go test -json output.
type TestEvent struct {
	Time       time.Time `json:"Time"`
	Action     string    `json:"Action"` // start, run, pass, fail, skip, output, build-output, build-fail, bench, pause, cont
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
