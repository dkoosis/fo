package jtbd

// Job is a user-facing job definition.
type Job struct {
	ID        string
	Layer     string
	Domain    string // extracted from ID prefix (e.g., "KG" from "KG-P1")
	Statement string
}

// Annotation maps a test function to the JTBDs it serves.
type Annotation struct {
	Package  string   // full import path
	FuncName string   // e.g., "TestSave"
	JobIDs   []string // e.g., ["KG-P1", "CORE-2"]
}

// TestOutcome is the result of a single test function.
type TestOutcome struct {
	Package  string
	FuncName string
	Status   string // "pass", "fail", "skip"
}

// JobResult is one JTBD with its aggregate status.
type JobResult struct {
	Job    Job
	Status string // "running", "broken", "wip"
	Tests  []TestOutcome
	Passed int
	Failed int
	Total  int
}

// Report is the assembled JTBD coverage report.
type Report struct {
	Jobs    []JobResult
	Running int
	Broken  int
	WIP     int
	Total   int
}
