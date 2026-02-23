package jtbd

import (
	"strings"

	"github.com/dkoosis/fo/pkg/testjson"
)

// Assemble joins job definitions, source annotations, and test results into a Report.
func Assemble(jobs []Job, annotations []Annotation, results []testjson.TestPackageResult) *Report {
	// Build annotation index: "pkg#func" → jobIDs
	annoIndex := make(map[string][]string)
	for _, a := range annotations {
		key := a.Package + "#" + a.FuncName
		annoIndex[key] = a.JobIDs
	}

	// Build test result index: "pkg#func" → status (top-level only, no subtests)
	testIndex := make(map[string]string)
	for _, pkg := range results {
		for _, test := range pkg.AllTests {
			if strings.Contains(test.Name, "/") {
				continue
			}
			key := pkg.Name + "#" + test.Name
			testIndex[key] = strings.ToLower(test.Status)
		}
	}

	// For each job, collect test outcomes
	jobTests := make(map[string][]TestOutcome)
	for key, jobIDs := range annoIndex {
		parts := strings.SplitN(key, "#", 2)
		pkg, fn := parts[0], parts[1]

		status := "unknown"
		if s, ok := testIndex[key]; ok {
			status = s
		}

		outcome := TestOutcome{Package: pkg, FuncName: fn, Status: status}
		for _, jobID := range jobIDs {
			jobTests[jobID] = append(jobTests[jobID], outcome)
		}
	}

	// Build report
	report := &Report{Total: len(jobs)}
	for _, job := range jobs {
		tests := jobTests[job.ID]
		jr := JobResult{Job: job, Tests: tests, Total: len(tests)}

		if len(tests) == 0 {
			jr.Status = "wip"
			report.WIP++
		} else {
			allPass := true
			for _, t := range tests {
				switch t.Status {
				case "pass":
					jr.Passed++
				case "fail":
					jr.Failed++
					allPass = false
				}
			}
			if allPass {
				jr.Status = "running"
				report.Running++
			} else {
				jr.Status = "broken"
				report.Broken++
			}
		}

		report.Jobs = append(report.Jobs, jr)
	}

	return report
}
