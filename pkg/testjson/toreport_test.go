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

// TestToReport_AttachesStructuralDiff is the fo-d84 wiring test: a failed
// test whose Output carries a go-cmp-shaped diff gets StructuralDiff
// populated on its TestResult; one with ordinary assertion text does not
// (no regression for the common case).
func TestToReport_AttachesStructuralDiff(t *testing.T) {
	t.Parallel()

	goCmpOutput := []string{
		"  pkg.MyStruct{",
		"- \tField: 1,",
		"+ \tField: 2,",
		"  }",
	}
	results := []testjson.TestPackageResult{{
		Name:   "pkg/x",
		Failed: 2,
		FailedTests: []testjson.FailedTest{
			{Name: "TestStructural", Output: goCmpOutput},
			{Name: "TestPlain", Output: []string{"want bar, got baz"}},
		},
	}}

	r := testjson.ToReport(results)
	byTest := map[string]report.TestResult{}
	for _, tr := range r.Tests {
		byTest[tr.Test] = tr
	}

	structural := byTest["TestStructural"]
	if structural.StructuralDiff == nil {
		t.Fatal("TestStructural: StructuralDiff = nil, want populated")
	}
	if structural.StructuralDiff.Kind != "go-cmp" {
		t.Errorf("Kind = %q, want go-cmp", structural.StructuralDiff.Kind)
	}
	if got := structural.Output; got != strings.Join(goCmpOutput, "\n") {
		t.Errorf("Output changed: got %q", got)
	}

	plain := byTest["TestPlain"]
	if plain.StructuralDiff != nil {
		t.Errorf("TestPlain: StructuralDiff = %#v, want nil", plain.StructuralDiff)
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

// fo-n25.6: an injected generatedAt is stamped verbatim instead of the
// wall clock, so output is deterministic under test.
func TestToReport_InjectedGeneratedAt(t *testing.T) {
	t.Parallel()

	want := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	r := testjson.ToReport(nil, want)
	if !r.GeneratedAt.Equal(want) {
		t.Errorf("GeneratedAt = %v, want %v", r.GeneratedAt, want)
	}
}

// fo-n25.6: omitting generatedAt falls back to the wall clock, matching
// prior (non-injectable) behavior.
func TestToReport_OmittedGeneratedAtUsesClock(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	r := testjson.ToReport(nil)
	after := time.Now().UTC()
	if r.GeneratedAt.Before(before) || r.GeneratedAt.After(after) {
		t.Errorf("GeneratedAt = %v, want between %v and %v", r.GeneratedAt, before, after)
	}
}
