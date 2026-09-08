package testjson

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dkoosis/fo/pkg/cluster"
	"github.com/dkoosis/fo/pkg/fingerprint"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/score"
)

// maxClusterInputs caps how many failures attachClusters feeds into
// cluster.RunWith. cluster.RunWith allocates ~7 maps/slices sized by
// the input count; this bound keeps a pathological run (thousands of
// failures) from spiking memory (fo-yax). Above the cap, the tail is
// dropped — clusters still surface for the most common failures.
const maxClusterInputs = 5000

// ToReport projects parsed go test -json package results onto the canonical
// Report shape. Each panic, build error, failed test, and passing package
// becomes a TestResult; the Test field is "" for package-level results.
//
// Outcomes carry priority via Score: panics > build errors > test failures
// > passes. Sorting is stable on (Score desc, Package, Test).
//
// generatedAt, when given and non-zero, stamps Report.GeneratedAt instead
// of the wall clock — mirrors pkg/state/runlog.go's inject-or-fallback
// convention so output is deterministic under test.
func ToReport(results []TestPackageResult, generatedAt ...time.Time) *report.Report {
	var seed time.Time
	if len(generatedAt) > 0 {
		seed = generatedAt[0]
	}
	r := &report.Report{
		Tool:        "go test",
		GeneratedAt: report.GeneratedAtOrNow(seed),
	}

	for i := range results {
		pkg := &results[i]
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
				r.Tests = append(r.Tests, failedTestResult(pkg.Name, ft))
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

	attachClusters(r)
	return r
}

// failedTestResult builds the TestResult for one failed test. It reuses
// the aggregator's cached structural-diff detection when present (see
// handleFail in parser.go, which computes it once at the test's terminal
// fail event rather than leaving ToReport to redetect it on every
// streaming tick). StructuralDiffChecked is what makes that cache trusted
// even when the cached result is nil — a plain nil StructuralDiff can't
// tell "already tried, no match" from "never tried", and treating both the
// same way (redetect) defeated the memoization for the common case, an
// ordinary failure with no structural shape. A FailedTest built directly
// rather than through the aggregator (e.g. in tests) arrives with
// StructuralDiffChecked false, so this detects it on demand — unchanged
// from before that cache existed.
func failedTestResult(pkgName string, ft FailedTest) report.TestResult {
	out := strings.Join(ft.Output, "\n")
	sd := ft.StructuralDiff
	if !ft.StructuralDiffChecked {
		sd = detectStructuralDiff(out)
	}
	return report.TestResult{
		Package:        pkgName,
		Test:           ft.Name,
		Outcome:        report.OutcomeFail,
		Output:         out,
		FixCommand:     testFixCommand(pkgName, ft.Name),
		Fingerprint:    fingerprint.Fingerprint(ft.Name, pkgName, out),
		Score:          score.Score(score.SeverityWeightError, 1, pkgName),
		StructuralDiff: sd,
	}
}

// attachClusters runs the failure clusterer over failing tests in r and
// stamps Test.ClusterID + Report.Clusters. Singletons (clusters with one
// member) carry no ClusterID and are omitted from Report.Clusters — only
// groups with shared root cause survive.
func attachClusters(r *report.Report) {
	if len(r.Tests) == 0 {
		return
	}
	inputs, keyToTestIdx := buildClusterInputs(r.Tests)
	if len(inputs) == 0 {
		return
	}
	groups := cluster.Run(inputs)
	if len(groups) == 0 {
		return
	}
	if out := materializeClusters(r.Tests, keyToTestIdx, groups); len(out) > 0 {
		r.Clusters = out
	}
}

// buildClusterInputs collects failing tests into cluster.Input rows.
// Key must be unique per test result — cluster.Run dedupes by Key
// (last-write-wins), so retries that share a Fingerprint would collapse
// to one input and get dropped as a singleton (fo-juf). Cap inputs at
// maxClusterInputs to bound the 7-map/slice alloc inside cluster.RunWith
// for pathological runs (fo-yax).
func buildClusterInputs(tests []report.TestResult) ([]cluster.Input, map[string]int) {
	inCap := min(len(tests), maxClusterInputs)
	inputs := make([]cluster.Input, 0, inCap)
	keyToTestIdx := make(map[string]int, inCap)
	for i := range tests {
		t := &tests[i]
		if !isFailureOutcome(t.Outcome) {
			continue
		}
		if len(inputs) >= maxClusterInputs {
			break
		}
		key := strconv.Itoa(i)
		keyToTestIdx[key] = i
		inputs = append(inputs, cluster.Input{
			Key:     key,
			Package: t.Package,
			Test:    t.Test,
			Outcome: string(t.Outcome),
			Output:  t.Output,
		})
	}
	return inputs, keyToTestIdx
}

func materializeClusters(tests []report.TestResult, keyToTestIdx map[string]int, groups []cluster.Cluster) []report.Cluster {
	out := make([]report.Cluster, 0, len(groups))
	for _, g := range groups {
		if len(g.Members) < 2 {
			continue
		}
		id := string(g.ID)
		for _, m := range g.Members {
			if idx, ok := keyToTestIdx[m]; ok {
				tests[idx].ClusterID = id
			}
		}
		out = append(out, report.Cluster{
			ID:            id,
			Signature:     g.Signature,
			SignatureKind: g.SignatureKind,
			TopUserFrame:  g.TopUserFrame,
			NormSig:       g.NormSig,
			Members:       append([]string(nil), g.Members...),
		})
	}
	return out
}

func isFailureOutcome(o report.TestOutcome) bool {
	return o == report.OutcomeFail || o == report.OutcomePanic || o == report.OutcomeBuildError
}

// ToReportWithMeta stamps DataHash from raw input bytes the caller already
// has.
func ToReportWithMeta(results []TestPackageResult, rawInput []byte, generatedAt ...time.Time) *report.Report {
	r := ToReport(results, generatedAt...)
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
