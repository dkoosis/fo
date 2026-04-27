package testjson

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/fingerprint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/score"
)

// ToReport projects parsed go test -json package results onto the canonical
// Report shape. Each panic, build error, failed test, and passing package
// becomes a TestResult; the Test field is "" for package-level results.
//
// Outcomes carry priority via Score: panics > build errors > test failures
// > passes. Sorting is stable on (Score desc, Package, Test).
func ToReport(results []TestPackageResult) *report.Report {
	r := &report.Report{
		Tool:        "go test",
		GeneratedAt: time.Now().UTC(),
	}

	for _, pkg := range results {
		switch {
		case pkg.Panicked:
			out := strings.Join(pkg.PanicOutput, "\n")
			r.Tests = append(r.Tests, report.TestResult{
				Package:     pkg.Name,
				Outcome:     report.OutcomePanic,
				Duration:    pkg.Duration,
				Output:      out,
				FixCommand:  fmt.Sprintf("go test %s -v", pkg.Name),
				Fingerprint: fingerprint.Fingerprint("PANIC", pkg.Name, out),
				Score:       score.Score(score.SeverityWeightError, 1, pkg.Name) * panicBoost,
			})
		case pkg.BuildError != "":
			r.Tests = append(r.Tests, report.TestResult{
				Package:     pkg.Name,
				Outcome:     report.OutcomeBuildError,
				Duration:    pkg.Duration,
				Output:      pkg.BuildError,
				FixCommand:  "go build " + pkg.Name,
				Fingerprint: fingerprint.Fingerprint("BUILD_ERROR", pkg.Name, pkg.BuildError),
				Score:       score.Score(score.SeverityWeightError, 1, pkg.Name) * buildErrorBoost,
			})
		case pkg.Failed > 0:
			for _, ft := range pkg.FailedTests {
				out := strings.Join(ft.Output, "\n")
				r.Tests = append(r.Tests, report.TestResult{
					Package:     pkg.Name,
					Test:        ft.Name,
					Outcome:     report.OutcomeFail,
					Output:      out,
					FixCommand:  testFixCommand(pkg.Name, ft.Name),
					Fingerprint: fingerprint.Fingerprint(ft.Name, pkg.Name, out),
					Score:       score.Score(score.SeverityWeightError, 1, pkg.Name),
				})
			}
		default:
			outcome := report.OutcomePass
			if pkg.Passed == 0 && pkg.Skipped > 0 {
				outcome = report.OutcomeSkip
			}
			r.Tests = append(r.Tests, report.TestResult{
				Package:  pkg.Name,
				Outcome:  outcome,
				Duration: pkg.Duration,
			})
		}
	}

	sort.SliceStable(r.Tests, func(i, j int) bool {
		if r.Tests[i].Score != r.Tests[j].Score {
			return r.Tests[i].Score > r.Tests[j].Score
		}
		if r.Tests[i].Package != r.Tests[j].Package {
			return r.Tests[i].Package < r.Tests[j].Package
		}
		return r.Tests[i].Test < r.Tests[j].Test
	})

	return r
}

// ToReportWithMeta stamps DataHash from raw input bytes the caller already
// has.
func ToReportWithMeta(results []TestPackageResult, rawInput []byte) *report.Report {
	r := ToReport(results)
	if len(rawInput) > 0 {
		sum := sha256.Sum256(rawInput)
		r.DataHash = hex.EncodeToString(sum[:])
	}
	return r
}

// Boost factors so panics outrank build errors outrank test failures even
// when raw severity weight + centrality coincide. Tunable here only.
const (
	panicBoost      = 100.0
	buildErrorBoost = 10.0
)

// testFixCommand builds a `go test -run` invocation for a single failed
// test. Subtests (TestFoo/case_1) become anchored regex segments
// (^TestFoo$/^case_1$) so only that exact subtest path matches.
func testFixCommand(pkg, testName string) string {
	parts := strings.Split(testName, "/")
	anchored := make([]string, len(parts))
	for i, p := range parts {
		anchored[i] = "^" + p + "$"
	}
	return fmt.Sprintf("go test -run %s %s -v", strings.Join(anchored, "/"), pkg)
}
