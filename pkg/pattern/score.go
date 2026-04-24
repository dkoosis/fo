package pattern

import "strings"

// Severity weights used by Score. Tunable as constants — no runtime flag.
const (
	SeverityWeightError   = 3
	SeverityWeightWarning = 2
	SeverityWeightNote    = 1
)

// File centrality factors used by Score. Tests get the lowest weight so that
// production code defects sort above test-file defects of equal severity.
const (
	CentralityRoot     = 1.0  // cmd/* or pkg/* roots
	CentralityInternal = 0.5  // internal/* paths
	CentralityTest     = 0.25 // any *_test.go file
	CentralityDefault  = 1.0  // anything else (treat as root-equivalent)
)

// SeverityWeight maps a SARIF level string to its score weight.
// Unknown levels (including empty and "none") get the note weight.
func SeverityWeight(level string) int {
	switch level {
	case "error":
		return SeverityWeightError
	case "warning":
		return SeverityWeightWarning
	case "note":
		return SeverityWeightNote
	default:
		return SeverityWeightNote
	}
}

// FileCentrality returns the centrality factor for path.
//
// Precedence (first match wins):
//  1. *_test.go        → CentralityTest    (test files, even under cmd/ or pkg/)
//  2. internal/        → CentralityInternal
//  3. cmd/ or pkg/     → CentralityRoot
//  4. anything else    → CentralityDefault
//
// Test-file precedence is intentional: a defect inside a *_test.go file
// under pkg/ is still test code and should sort below production defects
// of equal severity.
func FileCentrality(path string) float64 {
	p := strings.ReplaceAll(path, "\\", "/")
	if strings.HasSuffix(p, "_test.go") {
		return CentralityTest
	}
	// Match "internal/" as a path segment, not a substring of a filename.
	if p == "internal" || strings.HasPrefix(p, "internal/") || strings.Contains(p, "/internal/") {
		return CentralityInternal
	}
	if strings.HasPrefix(p, "cmd/") || strings.Contains(p, "/cmd/") ||
		strings.HasPrefix(p, "pkg/") || strings.Contains(p, "/pkg/") {
		return CentralityRoot
	}
	return CentralityDefault
}

// Score is the deterministic priority score for a finding:
//
//	score = severityWeight × occurrenceCount × fileCentrality(path)
//
// Higher scores sort first. Inputs are expected to be non-negative; callers
// that pass an occurrenceCount of 0 will get a score of 0.
func Score(severityWeight, occurrenceCount int, path string) float64 {
	return float64(severityWeight) * float64(occurrenceCount) * FileCentrality(path)
}
