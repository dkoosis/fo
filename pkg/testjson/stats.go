package testjson

import "time"

// Stats holds aggregate statistics across all packages.
type Stats struct {
	TotalTests   int
	Passed       int
	Failed       int
	Skipped      int
	Packages     int
	FailedPkgs   int
	Duration     time.Duration
	BuildErrors  int
	Panics       int
}

// ComputeStats aggregates statistics from package results.
func ComputeStats(results []TestPackageResult) Stats {
	var s Stats
	s.Packages = len(results)
	for _, r := range results {
		s.Passed += r.Passed
		s.Failed += r.Failed
		s.Skipped += r.Skipped
		s.TotalTests += r.TotalTests()
		if r.Duration > s.Duration {
			s.Duration = r.Duration
		}
		if r.Status() == "fail" {
			s.FailedPkgs++
		}
		if r.BuildError != "" {
			s.BuildErrors++
		}
		if r.Panicked {
			s.Panics++
		}
	}
	return s
}
