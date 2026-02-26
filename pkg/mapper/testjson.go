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
	stats := testjson.ComputeStats(results)
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

	// 2. Panic packages — highest priority
	for _, r := range sorted {
		if r.Panicked {
			patterns = append(patterns, panicTable(r))
		}
	}

	// 3. Build error packages
	for _, r := range sorted {
		if r.BuildError != "" && !r.Panicked {
			patterns = append(patterns, buildErrorTable(r))
		}
	}

	// 4. Failed packages with test details
	for _, r := range sorted {
		if r.Failed > 0 && r.BuildError == "" && !r.Panicked {
			patterns = append(patterns, failedPkgTable(r))
		}
	}

	// 5. Passing packages — collapsed into one table
	var passItems []pattern.TestTableItem
	for _, r := range sorted {
		if r.Status() == "pass" {
			passItems = append(passItems, pattern.TestTableItem{
				Name:     shortPkgName(r.Name),
				Status:   "pass",
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
			Label: "Panics", Value: fmt.Sprintf("%d", s.Panics), Kind: "error",
		})
	}
	if s.BuildErrors > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Build Errors", Value: fmt.Sprintf("%d", s.BuildErrors), Kind: "error",
		})
	}
	if s.Failed > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Failed", Value: fmt.Sprintf("%d/%d tests", s.Failed, s.TotalTests), Kind: "error",
		})
	}
	if s.Passed > 0 {
		kind := "success"
		if s.Failed > 0 {
			kind = "info"
		}
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Passed", Value: fmt.Sprintf("%d/%d tests", s.Passed, s.TotalTests), Kind: kind,
		})
	}
	if s.Skipped > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Skipped", Value: fmt.Sprintf("%d", s.Skipped), Kind: "warning",
		})
	}
	metrics = append(metrics, pattern.SummaryItem{
		Label: "Packages", Value: fmt.Sprintf("%d", s.Packages), Kind: "info",
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
	items := []pattern.TestTableItem{{
		Name:    "PANIC",
		Status:  "fail",
		Details: truncateLines(r.PanicOutput, 5),
	}}
	return &pattern.TestTable{
		Label:   "PANIC " + shortPkgName(r.Name),
		Results: items,
	}
}

func buildErrorTable(r testjson.TestPackageResult) *pattern.TestTable {
	items := []pattern.TestTableItem{{
		Name:    "BUILD ERROR",
		Status:  "fail",
		Details: truncateString(r.BuildError, 300),
	}}
	return &pattern.TestTable{
		Label:   "BUILD FAIL " + shortPkgName(r.Name),
		Results: items,
	}
}

func failedPkgTable(r testjson.TestPackageResult) *pattern.TestTable {
	items := make([]pattern.TestTableItem, 0, len(r.FailedTests))
	for _, ft := range r.FailedTests {
		items = append(items, pattern.TestTableItem{
			Name:    ft.Name,
			Status:  "fail",
			Details: truncateLines(ft.Output, 3),
		})
	}
	return &pattern.TestTable{
		Label:   fmt.Sprintf("FAIL %s (%d/%d failed)", shortPkgName(r.Name), r.Failed, r.TotalTests()),
		Results: items,
	}
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
