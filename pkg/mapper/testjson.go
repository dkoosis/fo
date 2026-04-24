package mapper

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/testjson"
)

// FromTestJSON converts test results into visualization patterns.
// Returns: Summary + TestTable per failed package + TestTable for passing packages.
func FromTestJSON(results []testjson.TestPackageResult) []pattern.Pattern {
	return fromTestJSON(results, testjson.ComputeStats(results))
}

func fromTestJSON(results []testjson.TestPackageResult, stats testjson.Stats) []pattern.Pattern {
	var patterns []pattern.Pattern

	// 1. Summary
	patterns = append(patterns, testSummary(stats))

	// Sort: panics first, then build errors, then failed, then passed
	sorted := make([]testjson.TestPackageResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		pi, pj := pkgPriority(sorted[i]), pkgPriority(sorted[j])
		if pi != pj {
			return pi < pj
		}
		return sorted[i].Name < sorted[j].Name
	})

	// 2. Emit patterns by priority: panics, build errors, test failures, then passes.
	// sorted is already ordered by pkgPriority, so a single pass suffices.
	var passItems []pattern.TestTableItem
	for _, r := range sorted {
		switch {
		case r.Panicked:
			patterns = append(patterns, panicTable(r))
		case r.BuildError != "":
			patterns = append(patterns, buildErrorTable(r))
		case r.Failed > 0:
			patterns = append(patterns, failedPkgTable(r))
		default:
			passItems = append(passItems, pattern.TestTableItem{
				Name:     shortPkgName(r.Name),
				Status:   pattern.StatusPass,
				Duration: formatDuration(r.Duration),
				Count:    r.TotalTests(),
			})
		}
	}
	if len(passItems) > 0 {
		patterns = append(patterns, &pattern.TestTable{
			Label:   fmt.Sprintf("Passing Packages (%d)", len(passItems)),
			Results: passItems,
		})
	}

	return patterns
}

func testSummary(s testjson.Stats) *pattern.Summary {
	var metrics []pattern.SummaryItem

	if s.Panics > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Panics", Value: fmt.Sprintf("%d", s.Panics), Kind: pattern.KindError,
		})
	}
	if s.BuildErrors > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Build Errors", Value: fmt.Sprintf("%d", s.BuildErrors), Kind: pattern.KindError,
		})
	}
	if s.Failed > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Failed", Value: fmt.Sprintf("%d/%d tests", s.Failed, s.TotalTests), Kind: pattern.KindError,
		})
	}
	if s.Passed > 0 {
		kind := pattern.KindSuccess
		if s.Failed > 0 {
			kind = pattern.KindInfo
		}
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Passed", Value: fmt.Sprintf("%d/%d tests", s.Passed, s.TotalTests), Kind: kind,
		})
	}
	if s.Skipped > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Skipped", Value: fmt.Sprintf("%d", s.Skipped), Kind: pattern.KindWarning,
		})
	}
	metrics = append(metrics, pattern.SummaryItem{
		Label: "Packages", Value: fmt.Sprintf("%d", s.Packages), Kind: pattern.KindInfo,
	})

	label := fmt.Sprintf("PASS (%s)", formatDuration(s.Duration))
	if s.Failed > 0 || s.BuildErrors > 0 || s.Panics > 0 {
		label = fmt.Sprintf("FAIL %d/%d tests, %d packages affected (%s)",
			s.Failed, s.TotalTests, s.FailedPkgs, formatDuration(s.Duration))
	}

	return &pattern.Summary{
		Label:   label,
		Kind:    pattern.SummaryKindTest,
		Metrics: metrics,
	}
}

func panicTable(r testjson.TestPackageResult) *pattern.TestTable {
	details := truncateLines(r.PanicOutput, 5)
	// Panic without a specific test name: run the whole package verbosely.
	fixCmd := fmt.Sprintf("go test %s -v", r.Name)
	items := []pattern.TestTableItem{{
		Name:        "PANIC",
		Status:      pattern.StatusFail,
		Details:     details,
		Fingerprint: pattern.Fingerprint("PANIC", r.Name, details),
		FixCommand:  fixCmd,
		Score:       pattern.Score(pattern.SeverityWeightError, 1, r.Name),
	}}
	return &pattern.TestTable{
		Label:   "PANIC " + shortPkgName(r.Name),
		Results: items,
	}
}

func buildErrorTable(r testjson.TestPackageResult) *pattern.TestTable {
	details := truncateString(r.BuildError, 300)
	items := []pattern.TestTableItem{{
		Name:        "BUILD ERROR",
		Status:      pattern.StatusFail,
		Details:     details,
		Fingerprint: pattern.Fingerprint("BUILD_ERROR", r.Name, details),
		FixCommand:  fmt.Sprintf("go build %s", r.Name),
		Score:       pattern.Score(pattern.SeverityWeightError, 1, r.Name),
	}}
	return &pattern.TestTable{
		Label:   "BUILD FAIL " + shortPkgName(r.Name),
		Results: items,
	}
}

func failedPkgTable(r testjson.TestPackageResult) *pattern.TestTable {
	items := make([]pattern.TestTableItem, 0, len(r.FailedTests))
	// Test failures are always error-severity with occurrence=1 per test;
	// centrality is drawn from the package import path.
	pathKey := r.Name
	for _, ft := range r.FailedTests {
		details := truncateLines(ft.Output, 3)
		items = append(items, pattern.TestTableItem{
			Name:        ft.Name,
			Status:      pattern.StatusFail,
			Details:     details,
			Fingerprint: pattern.Fingerprint(ft.Name, r.Name, details),
			FixCommand:  testFixCommand(r.Name, ft.Name),
			Score:       pattern.Score(pattern.SeverityWeightError, 1, pathKey),
		})
	}
	return &pattern.TestTable{
		Label:   fmt.Sprintf("FAIL %s (%d/%d failed)", shortPkgName(r.Name), r.Failed, r.TotalTests()),
		Results: items,
	}
}

// testFixCommand builds a `go test -run` command that reproduces a single
// failed test. Subtests (TestFoo/case_1) become anchored segments
// (^TestFoo$/^case_1$) so the regex matches only that exact subtest path.
func testFixCommand(pkg, testName string) string {
	parts := strings.Split(testName, "/")
	anchored := make([]string, len(parts))
	for i, p := range parts {
		anchored[i] = "^" + p + "$"
	}
	return fmt.Sprintf("go test -run %s %s -v", strings.Join(anchored, "/"), pkg)
}

func pkgPriority(r testjson.TestPackageResult) int {
	if r.Panicked {
		return 0
	}
	if r.BuildError != "" {
		return 1
	}
	if r.Failed > 0 {
		return 2
	}
	return 3
}

func shortPkgName(name string) string {
	// Strip common module prefix to show relative package path
	for _, prefix := range []string{"/internal/", "/cmd/", "/pkg/", "/examples/"} {
		if idx := strings.Index(name, prefix); idx != -1 {
			return name[idx+1:]
		}
	}
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return name
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return "0s"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func truncateLines(lines []string, max int) string {
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	result := strings.Join(lines[:max], "\n")
	return result + fmt.Sprintf("\n... (%d more lines)", len(lines)-max)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
