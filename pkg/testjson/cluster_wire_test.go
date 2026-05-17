package testjson

import (
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

const (
	pkgX        = "example.com/pkg/x"
	xTestOutput = "    x_test.go:5: oops"
)

// assertSingleClusterOfTwo runs ToReport and verifies the resulting
// report contains exactly one cluster with two members, both stamped
// with its ClusterID. Shared helper for tests that expect two failing
// tests to cluster together.
func assertSingleClusterOfTwo(t *testing.T, results []TestPackageResult) {
	t.Helper()
	r := ToReport(results)
	if len(r.Clusters) != 1 {
		t.Fatalf("len(Clusters) = %d, want 1; got %#v", len(r.Clusters), r.Clusters)
	}
	got := r.Clusters[0]
	if got.ID == "" {
		t.Fatal("cluster ID empty")
	}
	if len(got.Members) != 2 {
		t.Fatalf("len(Members) = %d, want 2", len(got.Members))
	}
	var stamped int
	for i := range r.Tests {
		if r.Tests[i].ClusterID == got.ID {
			stamped++
		}
	}
	if stamped != 2 {
		t.Fatalf("expected 2 tests stamped with cluster ID %q, got %d", got.ID, stamped)
	}
}

// TestAttachClusters_SharedFrameGroupsFailures verifies that two failing
// tests sharing the same user-code frame land in the same cluster and
// both TestResults receive the matching ClusterID.
func TestAttachClusters_SharedFrameGroupsFailures(t *testing.T) {
	t.Parallel()
	assertSingleClusterOfTwo(t, []TestPackageResult{{
		Name:   pkgX,
		Failed: 2,
		FailedTests: []FailedTest{
			{Name: "TestA", Output: []string{xTestOutput}},
			{Name: "TestB", Output: []string{xTestOutput}},
		},
	}})
}

// TestAttachClusters_SingletonsHaveNoClusterID verifies that a failure
// not sharing a signal with any other failure does not get a ClusterID
// and does not appear in Report.Clusters.
func TestAttachClusters_SingletonsHaveNoClusterID(t *testing.T) {
	t.Parallel()
	results := []TestPackageResult{{
		Name:   pkgX,
		Failed: 2,
		FailedTests: []FailedTest{
			{Name: "TestA", Output: []string{"    a_test.go:5: alpha distinct"}},
			{Name: "TestB", Output: []string{"    b_test.go:9: bravo separate"}},
		},
	}}

	r := ToReport(results)

	if len(r.Clusters) != 0 {
		t.Fatalf("len(Clusters) = %d, want 0 (all singletons)", len(r.Clusters))
	}
	for _, tr := range r.Tests {
		if tr.ClusterID != "" {
			t.Errorf("test %s/%s got ClusterID %q, want empty",
				tr.Package, tr.Test, tr.ClusterID)
		}
	}
}

// TestAttachClusters_PassingTestsHaveNoClusterID verifies that passing
// (and skipped) tests are never fed to the clusterer and therefore
// never carry a ClusterID, even when failing siblings cluster up.
func TestAttachClusters_PassingTestsHaveNoClusterID(t *testing.T) {
	t.Parallel()
	results := []TestPackageResult{
		{
			Name:   pkgX,
			Failed: 2,
			FailedTests: []FailedTest{
				{Name: "TestA", Output: []string{xTestOutput}},
				{Name: "TestB", Output: []string{xTestOutput}},
			},
		},
		{
			Name:   "example.com/pkg/y",
			Passed: 1,
		},
	}

	r := ToReport(results)

	if len(r.Clusters) != 1 {
		t.Fatalf("len(Clusters) = %d, want 1", len(r.Clusters))
	}
	for _, tr := range r.Tests {
		if tr.Outcome == report.OutcomePass && tr.ClusterID != "" {
			t.Errorf("passing test %s got ClusterID %q, want empty",
				tr.Package, tr.ClusterID)
		}
	}
}

// TestAttachClusters_RetriesWithSameFingerprintBothCount verifies that two
// failing test results with identical fingerprints (e.g. a retried failure
// with the same name, package, and output) both survive the clusterer
// rather than collapsing to a singleton and being dropped. Regression for
// fo-juf: attachClusters previously used Fingerprint as cluster.Input.Key,
// and the clusterer dedupes by Key (last-write-wins).
func TestAttachClusters_RetriesWithSameFingerprintBothCount(t *testing.T) {
	t.Parallel()
	assertSingleClusterOfTwo(t, []TestPackageResult{{
		Name:   pkgX,
		Failed: 2,
		FailedTests: []FailedTest{
			{Name: "TestFlaky", Output: []string{"    x_test.go:5: boom"}},
			{Name: "TestFlaky", Output: []string{"    x_test.go:5: boom"}},
		},
	}})
}

// TestAttachClusters_CapsInputs verifies attachClusters does not feed
// the clusterer more than maxClusterInputs failures, bounding alloc for
// pathological runs with thousands of failures (fo-yax).
func TestAttachClusters_CapsInputs(t *testing.T) {
	t.Parallel()
	n := maxClusterInputs + 100
	failed := make([]FailedTest, n)
	for i := range failed {
		failed[i] = FailedTest{Name: "TestN", Output: []string{xTestOutput}}
	}
	results := []TestPackageResult{{Name: pkgX, Failed: n, FailedTests: failed}}
	r := ToReport(results)
	stamped := 0
	for i := range r.Tests {
		if r.Tests[i].ClusterID != "" {
			stamped++
		}
	}
	if stamped > maxClusterInputs {
		t.Fatalf("stamped=%d, want ≤ maxClusterInputs=%d", stamped, maxClusterInputs)
	}
}

// TestAttachClusters_EmptyTestsNoOp verifies attachClusters does not
// allocate Clusters for a Report with no tests.
func TestAttachClusters_EmptyTestsNoOp(t *testing.T) {
	t.Parallel()
	r := ToReport(nil)
	if r.Clusters != nil {
		t.Errorf("Clusters = %#v, want nil", r.Clusters)
	}
}
