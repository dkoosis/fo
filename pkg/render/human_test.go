package render_test

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

func TestHumanRender_Leaderboard(t *testing.T) {
	t.Parallel()
	r := render.NewHuman(render.MonoTheme())

	tests := []struct {
		name   string
		board  *pattern.Leaderboard
		wants  []string
		empty  bool
	}{
		{
			name: "basic leaderboard with ranking",
			board: &pattern.Leaderboard{
				Label:    "Top Files",
				ShowRank: true,
				Items: []pattern.LeaderboardItem{
					{Name: "foo.go", Metric: "15", Rank: 1},
					{Name: "bar.go", Metric: "8", Rank: 2},
				},
			},
			wants: []string{"Top Files", "foo.go", "15", "bar.go", "8", "1.", "2."},
		},
		{
			name: "leaderboard with total count shows top N of M",
			board: &pattern.Leaderboard{
				Label:      "Issues",
				TotalCount: 50,
				Items: []pattern.LeaderboardItem{
					{Name: "a.go", Metric: "10"},
				},
			},
			wants: []string{"top 1 of 50"},
		},
		{
			name:  "empty leaderboard returns empty string",
			board: &pattern.Leaderboard{Label: "Empty", Items: nil},
			empty: true,
		},
		{
			name: "long names get truncated",
			board: &pattern.Leaderboard{
				Label: "Truncation",
				Items: []pattern.LeaderboardItem{
					{Name: strings.Repeat("x", 80), Metric: "1"},
				},
			},
			wants: []string{"..."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render([]pattern.Pattern{tc.board})
			if tc.empty {
				if out != "" {
					t.Fatalf("expected empty output, got:\n%s", out)
				}
				return
			}
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected output to contain %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestHumanRender_TestTable(t *testing.T) {
	t.Parallel()
	r := render.NewHuman(render.MonoTheme())

	tests := []struct {
		name  string
		table *pattern.TestTable
		wants []string
		empty bool
	}{
		{
			name: "pass/fail/skip items render with icons",
			table: &pattern.TestTable{
				Label: "test results",
				Results: []pattern.TestTableItem{
					{Name: "TestGood", Status: pattern.StatusPass, Duration: "0.01s"},
					{Name: "TestBad", Status: pattern.StatusFail, Duration: "1.20s"},
					{Name: "TestMeh", Status: pattern.StatusSkip},
				},
			},
			wants: []string{
				"test results",
				"+", "TestGood", "0.01s",
				"x", "TestBad", "1.20s",
				"!", "TestMeh",
			},
		},
		{
			name: "item with count shows test count",
			table: &pattern.TestTable{
				Label: "packages",
				Results: []pattern.TestTableItem{
					{Name: "pkg/foo", Status: pattern.StatusPass, Count: 42},
				},
			},
			wants: []string{"42 tests"},
		},
		{
			name: "item with details shows detail lines",
			table: &pattern.TestTable{
				Label: "failures",
				Results: []pattern.TestTableItem{
					{Name: "TestCrash", Status: pattern.StatusFail, Details: "panic: nil pointer\ngoroutine 1"},
				},
			},
			wants: []string{"panic: nil pointer", "goroutine 1"},
		},
		{
			name:  "empty results returns empty string",
			table: &pattern.TestTable{Label: "empty", Results: nil},
			empty: true,
		},
		{
			name: "long test names get truncated",
			table: &pattern.TestTable{
				Label: "Truncation",
				Results: []pattern.TestTableItem{
					{Name: strings.Repeat("T", 80), Status: pattern.StatusPass},
				},
			},
			wants: []string{"..."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := r.Render([]pattern.Pattern{tc.table})
			if tc.empty {
				if out != "" {
					t.Fatalf("expected empty output, got:\n%s", out)
				}
				return
			}
			for _, want := range tc.wants {
				if !strings.Contains(out, want) {
					t.Fatalf("expected output to contain %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestHumanRender_Error(t *testing.T) {
	t.Parallel()
	r := render.NewHuman(render.MonoTheme())

	out := r.Render([]pattern.Pattern{
		&pattern.Error{Source: "golangci-lint", Message: "config not found"},
	})

	for _, want := range []string{"golangci-lint", "config not found"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestHumanRender_MultiplePatternTypes(t *testing.T) {
	t.Parallel()
	r := render.NewHuman(render.MonoTheme())

	out := r.Render([]pattern.Pattern{
		&pattern.Summary{
			Label: "SARIF Report",
			Kind:  pattern.SummaryKindSARIF,
			Metrics: []pattern.SummaryItem{
				{Label: "issues", Value: "3", Kind: pattern.KindError},
			},
		},
		&pattern.Leaderboard{
			Label: "Top Files",
			Items: []pattern.LeaderboardItem{
				{Name: "main.go", Metric: "5"},
			},
		},
		&pattern.TestTable{
			Label: "diagnostics",
			Results: []pattern.TestTableItem{
				{Name: "SA1000:10:5", Status: pattern.StatusFail, Details: "bad call"},
			},
		},
	})

	for _, want := range []string{"SARIF Report", "Top Files", "main.go", "diagnostics", "bad call"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
