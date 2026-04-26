package testjson

import "strings"

// FuncStatus represents the outcome of a single test function.
type FuncStatus int

const (
	FuncPass FuncStatus = iota
	FuncFail
	FuncSkip
)

// FuncKey identifies a test function within a package.
type FuncKey struct {
	Package string // e.g., "github.com/foo/bar"
	Func    string // e.g., "TestBaz"
}

// FuncResult holds the outcome of one test function in one package.
type FuncResult struct {
	Key    FuncKey
	Status FuncStatus
}

// FuncResults processes a TestEvent stream and returns per-function outcomes.
// Only top-level test functions are recorded (subtests filtered by "/" in Test).
// Last action wins when a test emits multiple pass/fail/skip events.
func FuncResults(events []TestEvent) map[FuncKey]FuncResult {
	results := make(map[FuncKey]FuncResult)
	for _, e := range events {
		if e.Test == "" {
			continue // package-level event
		}
		if strings.Contains(e.Test, "/") {
			continue // subtest
		}
		var status FuncStatus
		switch e.Action {
		case "pass":
			status = FuncPass
		case "fail":
			status = FuncFail
		case "skip":
			status = FuncSkip
		default:
			continue
		}
		key := FuncKey{Package: e.Package, Func: e.Test}
		results[key] = FuncResult{Key: key, Status: status}
	}
	return results
}
