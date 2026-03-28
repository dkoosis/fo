package mapper_test

import (
	"testing"
	"time"

	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/mapper"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
)

func TestFromSARIF_ReturnsNoPatterns_When_DocumentHasNoIssues(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{Version: "2.1.0", Runs: []sarif.Run{{Results: nil}}}

	got := mapper.FromSARIF(doc)
	if got != nil {
		t.Fatalf("FromSARIF() = %v, want nil for zero-issue document", got)
	}
}

func TestFromSARIF_MapsSummaryLeaderboardAndFileTables_When_DocumentHasMultipleFiles(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{{
			Results: []sarif.Result{
				{
					RuleID:  "E1",
					Level:   "error",
					Message: sarif.Message{Text: "fatal"},
					Locations: []sarif.Location{{PhysicalLocation: sarif.PhysicalLocation{
						ArtifactLocation: sarif.ArtifactLocation{URI: "src/a/file1.go"},
						Region:           sarif.Region{StartLine: 9, StartColumn: 3},
					}}},
				},
				{
					RuleID:  "W1",
					Level:   "warning",
					Message: sarif.Message{Text: "warn"},
					Locations: []sarif.Location{{PhysicalLocation: sarif.PhysicalLocation{
						ArtifactLocation: sarif.ArtifactLocation{URI: "src/a/file1.go"},
						Region:           sarif.Region{StartLine: 12, StartColumn: 7},
					}}},
				},
				{
					RuleID:  "N1",
					Level:   "note",
					Message: sarif.Message{Text: "note"},
					Locations: []sarif.Location{{PhysicalLocation: sarif.PhysicalLocation{
						ArtifactLocation: sarif.ArtifactLocation{URI: "src/b/file2.go"},
						Region:           sarif.Region{StartLine: 1, StartColumn: 1},
					}}},
				},
			},
		}},
	}

	got := mapper.FromSARIF(doc)
	if len(got) != 4 {
		t.Fatalf("len(FromSARIF()) = %d, want 4 patterns (summary + leaderboard + 2 tables)", len(got))
	}

	summary, ok := got[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] type = %T, want *pattern.Summary", got[0])
	}
	if summary.Kind != pattern.SummaryKindSARIF {
		t.Fatalf("summary.Kind = %q, want %q", summary.Kind, pattern.SummaryKindSARIF)
	}

	leaderboard, ok := got[1].(*pattern.Leaderboard)
	if !ok {
		t.Fatalf("patterns[1] type = %T, want *pattern.Leaderboard", got[1])
	}
	if len(leaderboard.Items) == 0 || leaderboard.Items[0].Context != "src/a/file1.go" {
		t.Fatalf("leaderboard top file = %q, want src/a/file1.go", leaderboard.Items[0].Context)
	}

	firstFileTable, ok := got[2].(*pattern.TestTable)
	if !ok {
		t.Fatalf("patterns[2] type = %T, want *pattern.TestTable", got[2])
	}
	if firstFileTable.Results[0].Name != "E1:9:3" || firstFileTable.Results[0].Status != pattern.StatusFail {
		t.Fatalf("first row = %#v, want error row sorted first and mapped to fail", firstFileTable.Results[0])
	}
	if firstFileTable.Results[1].Name != "W1:12:7" || firstFileTable.Results[1].Status != pattern.StatusSkip {
		t.Fatalf("second row = %#v, want warning row mapped to skip", firstFileTable.Results[1])
	}
}

func TestFromTestJSON_OrdersCriticalTablesBeforePasses_When_MixedPackageResults(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{Name: "github.com/acme/fo/pkg/pass", Passed: 2, Duration: 50 * time.Millisecond},
		{Name: "github.com/acme/fo/pkg/fail", Failed: 1, FailedTests: []testjson.FailedTest{{Name: "TestBad", Output: []string{"line1", "line2"}}}},
		{Name: "github.com/acme/fo/pkg/build", BuildError: "undefined: nope"},
		{Name: "github.com/acme/fo/pkg/panic", Panicked: true, PanicOutput: []string{"panic: boom", "trace"}},
	}

	got := mapper.FromTestJSON(results)
	if len(got) != 5 {
		t.Fatalf("len(FromTestJSON()) = %d, want 5 patterns", len(got))
	}

	summary, ok := got[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] type = %T, want *pattern.Summary", got[0])
	}
	if summary.Kind != pattern.SummaryKindTest {
		t.Fatalf("summary.Kind = %q, want %q", summary.Kind, pattern.SummaryKindTest)
	}

	panicTable := mustTable(t, got[1])
	buildTable := mustTable(t, got[2])
	failTable := mustTable(t, got[3])
	passTable := mustTable(t, got[4])

	if panicTable.Label[:5] != "PANIC" {
		t.Fatalf("patterns[1] label = %q, want PANIC table first", panicTable.Label)
	}
	if buildTable.Label[:10] != "BUILD FAIL" {
		t.Fatalf("patterns[2] label = %q, want BUILD FAIL table second", buildTable.Label)
	}
	if failTable.Results[0].Name != "TestBad" || failTable.Results[0].Status != pattern.StatusFail {
		t.Fatalf("failed test row = %#v, want failed test details", failTable.Results[0])
	}
	if passTable.Label != "Passing Packages (1)" {
		t.Fatalf("pass table label = %q, want Passing Packages (1)", passTable.Label)
	}
	if passTable.Results[0].Count != 2 || passTable.Results[0].Status != pattern.StatusPass {
		t.Fatalf("pass row = %#v, want count/status invariants", passTable.Results[0])
	}
}

func TestFromReport_EmitsSectionErrorPattern_When_SectionFormatIsUnknown(t *testing.T) {
	t.Parallel()

	sections := []report.Section{{
		Tool:    "mystery",
		Format:  "bogus",
		Content: []byte("{}"),
	}}

	got := mapper.FromReport(sections)
	if len(got) != 2 {
		t.Fatalf("len(FromReport()) = %d, want 2 (summary + error)", len(got))
	}

	summary, ok := got[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] type = %T, want *pattern.Summary", got[0])
	}
	if summary.Kind != pattern.SummaryKindReport {
		t.Fatalf("summary.Kind = %q, want %q", summary.Kind, pattern.SummaryKindReport)
	}
	if len(summary.Metrics) != 1 || summary.Metrics[0].Kind != pattern.KindError {
		t.Fatalf("summary metrics = %#v, want one error metric", summary.Metrics)
	}

	errPattern, ok := got[1].(*pattern.Error)
	if !ok {
		t.Fatalf("patterns[1] type = %T, want *pattern.Error", got[1])
	}
	if errPattern.Source != "mystery" {
		t.Fatalf("error source = %q, want mystery", errPattern.Source)
	}
}

func mustTable(t *testing.T, p pattern.Pattern) *pattern.TestTable {
	t.Helper()
	table, ok := p.(*pattern.TestTable)
	if !ok {
		t.Fatalf("pattern type = %T, want *pattern.TestTable", p)
	}
	return table
}
