package sarif_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
)

func TestComputeStats_ComputesLevelRuleAndFileTotals_When_DocumentHasMixedResults(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{{
			Results: []sarif.Result{
				result("rule-a", "error", "a.go", 10, 2),
				result("rule-b", "warning", "a.go", 20, 4),
				result("rule-a", "note", "b.go", 1, 1),
				{RuleID: "rule-c", Level: "none"}, // no location should not contribute to file buckets.
			},
		}},
	}

	stats := sarif.ComputeStats(doc)

	if stats.TotalIssues != 4 {
		t.Fatalf("total issues invariant violated: got %d want %d", stats.TotalIssues, 4)
	}

	assertInt(t, stats.ByLevel["error"], 1, "error count")
	assertInt(t, stats.ByLevel["warning"], 1, "warning count")
	assertInt(t, stats.ByLevel["note"], 1, "note count")
	assertInt(t, stats.ByLevel["none"], 1, "none count")
	assertInt(t, stats.ByRule["rule-a"], 2, "rule-a count")
	assertInt(t, stats.ByRule["rule-b"], 1, "rule-b count")
	assertInt(t, stats.ByRule["rule-c"], 1, "rule-c count")
	assertInt(t, stats.ByFile["a.go"], 2, "a.go count")
	assertInt(t, stats.ByFile["b.go"], 1, "b.go count")

	levelsTotal := 0
	for _, count := range stats.ByLevel {
		levelsTotal += count
	}
	if levelsTotal != stats.TotalIssues {
		t.Fatalf("invariant violated: total by levels (%d) != total issues (%d)", levelsTotal, stats.TotalIssues)
	}
}

func TestTopFiles_ReturnsSortedLimitedCounts_When_DocumentIncludesMissingLocations(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{{
			Results: []sarif.Result{
				result("rule-a", "error", "alpha.go", 1, 1),
				result("rule-b", "warning", "beta.go", 1, 1),
				result("rule-c", "warning", "alpha.go", 2, 1),
				result("rule-d", "error", "alpha.go", 3, 1),
				result("rule-e", "note", "beta.go", 2, 1),
				{RuleID: "rule-f", Level: "error"}, // no location should be ignored.
			},
		}},
	}

	tests := []struct {
		name  string
		limit int
		want  []sarif.FileIssue
	}{
		{
			name:  "no limit returns all files sorted by issue count",
			limit: 0,
			want: []sarif.FileIssue{
				{File: "alpha.go", IssueCount: 3, ErrorCount: 2, WarnCount: 1},
				{File: "beta.go", IssueCount: 2, ErrorCount: 0, WarnCount: 1},
			},
		},
		{
			name:  "positive limit truncates leaderboard",
			limit: 1,
			want: []sarif.FileIssue{
				{File: "alpha.go", IssueCount: 3, ErrorCount: 2, WarnCount: 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := sarif.TopFiles(doc, tc.limit)

			assertFileIssuesEqual(t, got, tc.want)
			for i := 1; i < len(got); i++ {
				if got[i-1].IssueCount < got[i].IssueCount {
					t.Fatalf("invariant violated: top files are not sorted descending at %d", i)
				}
			}
		})
	}
}

func TestGroupByFile_GroupsByPrimaryLocationAndPreservesFirstSeenOrder_When_MixedFilesAndUnknown(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{
			{Results: []sarif.Result{
				result("r1", "error", "a.go", 1, 1),
				{RuleID: "r2", Level: "warning"}, // no location should go to unknown bucket.
				result("r3", "warning", "b.go", 2, 2),
			}},
			{Results: []sarif.Result{
				result("r4", "note", "a.go", 3, 3),
				{RuleID: "r5", Level: "error"},
			}},
		},
	}

	got := sarif.GroupByFile(doc)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(got))
	}

	assertString(t, got[0].Key, "a.go", "group 0 key")
	assertString(t, got[1].Key, "unknown", "group 1 key")
	assertString(t, got[2].Key, "b.go", "group 2 key")
	assertInt(t, len(got[0].Results), 2, "a.go group size")
	assertInt(t, len(got[1].Results), 2, "unknown group size")
	assertInt(t, len(got[2].Results), 1, "b.go group size")
}

func TestResultLineAndCol_ReturnsZero_When_ResultHasNoLocations(t *testing.T) {
	t.Parallel()

	var r sarif.Result
	assertInt(t, r.Line(), 0, "line without locations")
	assertInt(t, r.Col(), 0, "col without locations")
}

func TestResultLineAndCol_ReturnsPrimaryLocationCoordinates_When_MultipleLocations(t *testing.T) {
	t.Parallel()

	r := sarif.Result{
		Locations: []sarif.Location{
			location("first.go", 33, 7),
			location("second.go", 90, 1),
		},
	}

	assertInt(t, r.Line(), 33, "line from primary location")
	assertInt(t, r.Col(), 7, "col from primary location")
}

func result(ruleID, level, file string, line, col int) sarif.Result {
	r := sarif.Result{RuleID: ruleID, Level: level}
	if file != "" {
		r.Locations = []sarif.Location{location(file, line, col)}
	}
	return r
}

func location(file string, line, col int) sarif.Location {
	return sarif.Location{PhysicalLocation: sarif.PhysicalLocation{
		ArtifactLocation: sarif.ArtifactLocation{URI: file},
		Region:           sarif.Region{StartLine: line, StartColumn: col},
	}}
}

func assertInt(t *testing.T, got, want int, field string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %d want %d", field, got, want)
	}
}

func assertString(t *testing.T, got, want, field string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %q want %q", field, got, want)
	}
}

func assertFileIssuesEqual(t *testing.T, got, want []sarif.FileIssue) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("file issues length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("file issue[%d] mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}
