// Package mapper converts parsed input formats to visualization patterns.
package mapper

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/sarif"
)

// FromSARIF converts a SARIF document into visualization patterns.
// Returns: Summary + Leaderboard (if >1 file) + TestTable per file group.
// When there are 0 issues, returns nil — the caller (report mapper) already
// shows per-tool status in the top-level summary, so a detail block is noise.
func FromSARIF(doc *sarif.Document) []pattern.Pattern {
	return fromSARIF(doc, sarif.ComputeStats(doc))
}

func fromSARIF(doc *sarif.Document, stats sarif.Stats) []pattern.Pattern {
	if stats.TotalIssues == 0 {
		return nil
	}
	var patterns []pattern.Pattern

	// 1. Summary pattern — always first
	patterns = append(patterns, sarifSummary(stats))

	// 2. Leaderboard — top files by issue count (skip if <=1 file)
	if lb := sarifLeaderboard(doc, stats); lb != nil {
		patterns = append(patterns, lb)
	}

	// 3. Issue list grouped by file
	groups := sarif.GroupByFile(doc)

	// Pre-pass: count occurrences by (rule_id, normalized_message) across the
	// whole run so scoring reflects how widespread each defect is. Using the
	// existing Fingerprint (which also includes the file path) would undercount
	// identical defects spread across multiple files, so count on the
	// (rule_id, normalized_message) tuple directly.
	counts := sarifOccurrenceCounts(groups)

	for _, g := range groups {
		if t := sarifFileTable(g, counts); t != nil {
			patterns = append(patterns, t)
		}
	}

	return patterns
}

// sarifOccurrenceCounts counts (rule_id, normalized_message) occurrences
// across every result in the document.
func sarifOccurrenceCounts(groups []sarif.GroupedResults) map[string]int {
	counts := make(map[string]int)
	for _, g := range groups {
		for _, r := range g.Results {
			k := r.RuleID + "\x00" + pattern.NormalizeMessage(r.Message.Text)
			counts[k]++
		}
	}
	return counts
}

func sarifSummary(stats sarif.Stats) *pattern.Summary {
	var metrics []pattern.SummaryItem
	if n := stats.ByLevel["error"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Errors", Value: fmt.Sprintf("%d", n), Kind: pattern.KindError,
		})
	}
	if n := stats.ByLevel["warning"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Warnings", Value: fmt.Sprintf("%d", n), Kind: pattern.KindWarning,
		})
	}
	if n := stats.ByLevel["note"]; n > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Notes", Value: fmt.Sprintf("%d", n), Kind: pattern.KindInfo,
		})
	}

	// Catch issues with unexpected levels (e.g., "none", empty string)
	// so the metric breakdown accounts for every issue in TotalIssues.
	counted := stats.ByLevel["error"] + stats.ByLevel["warning"] + stats.ByLevel["note"]
	if other := stats.TotalIssues - counted; other > 0 {
		metrics = append(metrics, pattern.SummaryItem{
			Label: "Other", Value: fmt.Sprintf("%d", other), Kind: pattern.KindInfo,
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
			Name:   displayName,
			Metric: fmt.Sprintf("%d issues", f.IssueCount),
			Value:  float64(f.IssueCount),
			Rank:   i + 1,
		}
	}

	return &pattern.Leaderboard{
		Label:      "Files with Most Issues",
		MetricName: "Issues",
		Items:      items,
		TotalCount: len(stats.ByFile),
		ShowRank:   true,
	}
}

func sarifFileTable(g sarif.GroupedResults, counts map[string]int) *pattern.TestTable {
	if len(g.Results) == 0 {
		return nil
	}

	items := make([]pattern.TestTableItem, len(g.Results))
	for i, r := range g.Results {
		loc := ""
		if r.Line() > 0 {
			loc = fmt.Sprintf(":%d:%d", r.Line(), r.Col())
		}
		occ := counts[r.RuleID+"\x00"+pattern.NormalizeMessage(r.Message.Text)]
		if occ == 0 {
			occ = 1 // defensive: an item exists, so it occurs at least once
		}
		items[i] = pattern.TestTableItem{
			Name:        r.RuleID + loc,
			Status:      mapLevel(r.Level),
			Details:     r.Message.Text,
			FixCommand:  r.FixCommand(),
			Fingerprint: pattern.Fingerprint(r.RuleID, g.Key, r.Message.Text),
			Score:       pattern.Score(pattern.SeverityWeight(r.Level), occ, g.Key),
		}
	}

	// Sort by Score descending. Ties broken by status severity (fail > skip > pass)
	// then by line number so output is deterministic when scores collide.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score != items[j].Score {
			return items[i].Score > items[j].Score
		}
		si, sj := statusPriority(items[i].Status), statusPriority(items[j].Status)
		if si != sj {
			return si < sj
		}
		_, lineI, _ := parseRuleLoc(items[i].Name)
		_, lineJ, _ := parseRuleLoc(items[j].Name)
		return lineI < lineJ
	})

	return &pattern.TestTable{
		Label:   g.Key,
		Results: items,
	}
}

// parseRuleLoc extracts the :line:col suffix from a rule+location string
// ("ruleID:line:col" or "ruleID:line"). Returns zeros when absent.
func parseRuleLoc(name string) (rule string, line, col int) {
	// Minimal splitter — colons in rule IDs are not expected.
	parts := strings.Split(name, ":")
	rule = parts[0]
	if len(parts) >= 2 {
		line = atoi(parts[1])
	}
	if len(parts) >= 3 {
		col = atoi(parts[2])
	}
	return rule, line, col
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func mapLevel(level string) pattern.Status {
	switch level {
	case "error":
		return pattern.StatusFail
	case "warning":
		return pattern.StatusSkip
	default:
		return pattern.StatusPass
	}
}

func statusPriority(s pattern.Status) int {
	switch s {
	case pattern.StatusFail:
		return 0
	case pattern.StatusSkip:
		return 1
	default:
		return 2
	}
}

