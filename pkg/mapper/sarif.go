// Package mapper converts parsed input formats to visualization patterns.
package mapper

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
)

// FromSARIF converts a SARIF document into visualization patterns.
// Returns: Summary + Leaderboard (if >1 file) + TestTable per file group.
func FromSARIF(doc *sarif.Document) []pattern.Pattern {
	stats := sarif.ComputeStats(doc)
	var patterns []pattern.Pattern

	// 1. Summary pattern — always first
	patterns = append(patterns, sarifSummary(stats))

	// 2. Leaderboard — top files by issue count (skip if <=1 file)
	if lb := sarifLeaderboard(doc, stats); lb != nil {
		patterns = append(patterns, lb)
	}

	// 3. Issue list grouped by file
	groups := sarif.GroupByFile(doc)
	for _, g := range groups {
		if t := sarifFileTable(g); t != nil {
			patterns = append(patterns, t)
		}
	}

	return patterns
}

func sarifSummary(stats sarif.Stats) *pattern.Summary {
	if stats.TotalIssues == 0 {
		return &pattern.Summary{
			Label: "Analysis Results",
			Kind:  pattern.SummaryKindSARIF,
			Metrics: []pattern.SummaryItem{
				{Label: "Issues", Value: "0", Kind: "success"},
			},
		}
	}

	var metrics []pattern.SummaryItem
	if n := stats.ByLevel["error"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Errors", Value: fmt.Sprintf("%d", n), Kind: "error",
		})
	}
	if n := stats.ByLevel["warning"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Warnings", Value: fmt.Sprintf("%d", n), Kind: "warning",
		})
	}
	if n := stats.ByLevel["note"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Notes", Value: fmt.Sprintf("%d", n), Kind: "info",
		})
	}

	return &pattern.Summary{
		Label:   fmt.Sprintf("Analysis: %d issues", stats.TotalIssues),
		Kind:    pattern.SummaryKindSARIF,
		Metrics: metrics,
	}
}

func sarifLeaderboard(doc *sarif.Document, stats sarif.Stats) *pattern.Leaderboard {
	if len(stats.ByFile) <= 1 {
		return nil
	}

	topFiles := sarif.TopFiles(doc, 10)
	if len(topFiles) == 0 {
		return nil
	}

	items := make([]pattern.LeaderboardItem, len(topFiles))
	for i, f := range topFiles {
		displayName := filepath.Base(f.File)
		if dir := filepath.Dir(f.File); dir != "." {
			displayName = filepath.Join(filepath.Base(dir), displayName)
		}
		items[i] = pattern.LeaderboardItem{
			Name:    displayName,
			Metric:  fmt.Sprintf("%d issues", f.IssueCount),
			Value:   float64(f.IssueCount),
			Rank:    i + 1,
			Context: f.File,
		}
	}

	return &pattern.Leaderboard{
		Label:      "Files with Most Issues",
		MetricName: "Issues",
		Items:      items,
		Direction:  "highest",
		TotalCount: len(stats.ByFile),
		ShowRank:   true,
	}
}

func sarifFileTable(g sarif.GroupedResults) *pattern.TestTable {
	if len(g.Results) == 0 {
		return nil
	}

	// Sort: errors first, then warnings, then notes; within level by line number
	sorted := make([]sarif.Result, len(g.Results))
	copy(sorted, g.Results)
	sort.Slice(sorted, func(i, j int) bool {
		li, lj := levelPriority(sorted[i].Level), levelPriority(sorted[j].Level)
		if li != lj {
			return li < lj
		}
		return sorted[i].Line() < sorted[j].Line()
	})

	items := make([]pattern.TestTableItem, len(sorted))
	for i, r := range sorted {
		loc := ""
		if r.Line() > 0 {
			loc = fmt.Sprintf(":%d:%d", r.Line(), r.Col())
		}
		items[i] = pattern.TestTableItem{
			Name:    r.RuleID + loc,
			Status:  mapLevel(r.Level),
			Details: r.Message.Text,
		}
	}

	return &pattern.TestTable{
		Label:   g.Key,
		Results: items,
	}
}

func mapLevel(level string) string {
	switch level {
	case "error":
		return "fail"
	case "warning":
		return "skip"
	default:
		return "pass"
	}
}

func levelPriority(level string) int {
	switch level {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
