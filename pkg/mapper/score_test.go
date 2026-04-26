package mapper_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/mapper"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/score"
)

// TestFromSARIF_ScoreUsesOccurrenceCountAcrossFiles verifies the mapper
// counts identical (rule_id, normalized_message) pairs across the whole
// document and folds the count into Score.
func TestFromSARIF_ScoreUsesOccurrenceCountAcrossFiles(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{{
			Results: []sarif.Result{
				// Same rule + message across 3 files → occurrence_count = 3.
				mkResult("DUP", "warning", "duplicate code block", "pkg/a/a.go", 10),
				mkResult("DUP", "warning", "duplicate code block", "pkg/b/b.go", 20),
				mkResult("DUP", "warning", "duplicate code block", "pkg/c/c.go", 30),
				// Singleton error in pkg/.
				mkResult("E1", "error", "fatal", "pkg/x/x.go", 1),
			},
		}},
	}

	patterns := mapper.FromSARIF(doc)

	// Find the table for pkg/a/a.go and check its Score reflects occurrence=3.
	wantDup := score.Score(score.SeverityWeightWarning, 3, "pkg/a/a.go") // 2 * 3 * 1.0 = 6
	wantE1 := score.Score(score.SeverityWeightError, 1, "pkg/x/x.go")    // 3 * 1 * 1.0 = 3

	if got := findScore(patterns, "pkg/a/a.go", "DUP:10:0"); got != wantDup {
		t.Errorf("DUP score = %v, want %v", got, wantDup)
	}
	if got := findScore(patterns, "pkg/x/x.go", "E1:1:0"); got != wantE1 {
		t.Errorf("E1 score = %v, want %v", got, wantE1)
	}
}

// TestFromSARIF_SortsItemsByScoreDescending verifies the mapper orders items
// within a file table by Score descending.
func TestFromSARIF_SortsItemsByScoreDescending(t *testing.T) {
	t.Parallel()

	doc := &sarif.Document{
		Version: "2.1.0",
		Runs: []sarif.Run{{
			Results: []sarif.Result{
				// All in the same file; vary severity so scores differ.
				mkResult("N1", "note", "low", "pkg/a/a.go", 5),
				mkResult("E1", "error", "high", "pkg/a/a.go", 99),
				mkResult("W1", "warning", "med", "pkg/a/a.go", 50),
			},
		}},
	}

	patterns := mapper.FromSARIF(doc)

	tbl := findTable(patterns, "pkg/a/a.go")
	if tbl == nil {
		t.Fatal("missing table for pkg/a/a.go")
	}
	if len(tbl.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(tbl.Results))
	}
	// Expect descending Score: error > warning > note.
	for i := 0; i+1 < len(tbl.Results); i++ {
		if tbl.Results[i].Score < tbl.Results[i+1].Score {
			t.Errorf("results not sorted by score desc: idx %d (%v) < idx %d (%v)",
				i, tbl.Results[i].Score, i+1, tbl.Results[i+1].Score)
		}
	}
}

// helpers

func mkResult(ruleID, level, msg, file string, line int) sarif.Result {
	return sarif.Result{
		RuleID:  ruleID,
		Level:   level,
		Message: sarif.Message{Text: msg},
		Locations: []sarif.Location{{PhysicalLocation: sarif.PhysicalLocation{
			ArtifactLocation: sarif.ArtifactLocation{URI: file},
			Region:           sarif.Region{StartLine: line, StartColumn: 0},
		}}},
	}
}

func findTable(patterns []pattern.Pattern, label string) *pattern.TestTable {
	for _, p := range patterns {
		if t, ok := p.(*pattern.TestTable); ok && t.Label == label {
			return t
		}
	}
	return nil
}

func findScore(patterns []pattern.Pattern, file, name string) float64 {
	t := findTable(patterns, file)
	if t == nil {
		return -1
	}
	for _, r := range t.Results {
		if r.Name == name {
			return r.Score
		}
	}
	return -1
}
