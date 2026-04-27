package testjson_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/testjson"
)

func TestToReport_AllOutcomes(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{
			Name:     "github.com/example/clean",
			Passed:   5,
			Duration: 100 * time.Millisecond,
		},
		{
			Name:        "github.com/example/failed",
			Passed:      3,
			Failed:      1,
			Duration:    50 * time.Millisecond,
			FailedTests: []testjson.FailedTest{{Name: "TestFoo", Output: []string{"want bar, got baz"}}},
		},
		{
			Name:       "github.com/example/buildbroken",
			BuildError: "syntax error: unexpected }",
		},
		{
			Name:        "github.com/example/panicker",
			Panicked:    true,
			PanicOutput: []string{"runtime error: index out of range"},
		},
	}

	r := testjson.ToReport(results)

	if r.Tool != "go test" {
		t.Errorf("Tool = %q, want go test", r.Tool)
	}
	if len(r.Tests) != 4 {
		t.Fatalf("Tests = %d, want 4", len(r.Tests))
	}

	byOutcome := map[report.TestOutcome]report.TestResult{}
	for _, tr := range r.Tests {
		byOutcome[tr.Outcome] = tr
	}

	for _, want := range []report.TestOutcome{
		report.OutcomePass, report.OutcomeFail, report.OutcomeBuildError, report.OutcomePanic,
	} {
		if _, ok := byOutcome[want]; !ok {
			t.Errorf("missing outcome %q", want)
		}
	}

	panicScore := byOutcome[report.OutcomePanic].Score
	buildScore := byOutcome[report.OutcomeBuildError].Score
	failScore := byOutcome[report.OutcomeFail].Score
	passScore := byOutcome[report.OutcomePass].Score
	if !(panicScore > buildScore && buildScore > failScore && failScore >= passScore) {
		t.Errorf("score ordering wrong: panic=%v build=%v fail=%v pass=%v",
			panicScore, buildScore, failScore, passScore)
	}
}

func TestToReport_SortedByScoreDesc(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{Name: "pkg/c", Passed: 1},
		{Name: "pkg/a", Panicked: true, PanicOutput: []string{"boom"}},
		{Name: "pkg/b", BuildError: "broken"},
	}

	r := testjson.ToReport(results)

	if r.Tests[0].Outcome != report.OutcomePanic {
		t.Errorf("Tests[0].Outcome = %q, want panic", r.Tests[0].Outcome)
	}
	if r.Tests[1].Outcome != report.OutcomeBuildError {
		t.Errorf("Tests[1].Outcome = %q, want build_error", r.Tests[1].Outcome)
	}
	if r.Tests[2].Outcome != report.OutcomePass {
		t.Errorf("Tests[2].Outcome = %q, want pass", r.Tests[2].Outcome)
	}
}

func TestToReport_FailedTestFixCommandAnchored(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{{
		Name:   "pkg/x",
		Failed: 1,
		FailedTests: []testjson.FailedTest{{
			Name:   "TestFoo/case_one",
			Output: []string{"diff"},
		}},
	}}

	r := testjson.ToReport(results)
	if len(r.Tests) != 1 {
		t.Fatalf("Tests = %d, want 1", len(r.Tests))
	}
	got := r.Tests[0].FixCommand
	if !strings.Contains(got, "^TestFoo$/^case_one$") {
		t.Errorf("FixCommand = %q, want anchored subtest regex", got)
	}
}

func TestToReport_DeterministicFingerprint(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{{
		Name:        "pkg/x",
		Failed:      1,
		FailedTests: []testjson.FailedTest{{Name: "TestFoo", Output: []string{"oops"}}},
	}}

	a := testjson.ToReport(results)
	b := testjson.ToReport(results)
	if a.Tests[0].Fingerprint != b.Tests[0].Fingerprint {
		t.Errorf("fingerprint diverged: %s vs %s",
			a.Tests[0].Fingerprint, b.Tests[0].Fingerprint)
	}
}
