package testjson

// Test-only conveniences over production code: production never calls these,
// so deadcode must not see them in non-test files.

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
