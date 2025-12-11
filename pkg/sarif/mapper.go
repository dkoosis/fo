package sarif

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dkoosis/fo/pkg/design"
)

// Mapper converts SARIF results to fo design patterns.
type Mapper struct {
	config ToolRenderConfig
}

// NewMapper creates a mapper with the given config.
func NewMapper(config ToolRenderConfig) *Mapper {
	return &Mapper{config: config}
}

// MapToPatterns converts a SARIF document to fo patterns based on config.
func (m *Mapper) MapToPatterns(doc *Document) []design.Pattern {
	var patterns []design.Pattern

	for _, pc := range m.config.Patterns {
		switch pc.Type {
		case "summary":
			if p := m.mapToSummary(doc, pc); p != nil {
				patterns = append(patterns, p)
			}
		case "leaderboard":
			if p := m.mapToLeaderboard(doc, pc); p != nil {
				patterns = append(patterns, p)
			}
		case "issue_list":
			patterns = append(patterns, m.mapToIssueList(doc, pc)...)
		}
	}

	return patterns
}

// mapToSummary creates a Summary pattern from SARIF stats.
func (m *Mapper) mapToSummary(doc *Document, pc PatternConfig) *design.Summary {
	stats := ComputeStats(doc)

	if stats.TotalIssues == 0 {
		return &design.Summary{
			Label: "Analysis Results",
			Metrics: []design.SummaryItem{
				{Label: "Issues", Value: "0", Type: "success"},
			},
		}
	}

	var metrics []design.SummaryItem

	switch pc.GroupBy {
	case "severity":
		// Group by severity level
		if n := stats.ByLevel["error"]; n > 0 {
			metrics = append(metrics, design.SummaryItem{
				Label: "Errors",
				Value: fmt.Sprintf("%d", n),
				Type:  "error",
			})
		}
		if n := stats.ByLevel["warning"]; n > 0 {
			metrics = append(metrics, design.SummaryItem{
				Label: "Warnings",
				Value: fmt.Sprintf("%d", n),
				Type:  "warning",
			})
		}
		if n := stats.ByLevel["note"]; n > 0 {
			metrics = append(metrics, design.SummaryItem{
				Label: "Notes",
				Value: fmt.Sprintf("%d", n),
				Type:  "info",
			})
		}

	case "rule_id":
		// Group by rule, sorted by count
		type ruleCount struct {
			rule  string
			count int
		}
		var rules []ruleCount
		for rule, count := range stats.ByRule {
			rules = append(rules, ruleCount{rule, count})
		}
		sort.Slice(rules, func(i, j int) bool {
			return rules[i].count > rules[j].count
		})

		for _, rc := range rules {
			metrics = append(metrics, design.SummaryItem{
				Label: rc.rule,
				Value: fmt.Sprintf("%d", rc.count),
				Type:  "info",
			})
		}
	}

	if len(metrics) == 0 {
		return nil
	}

	return &design.Summary{
		Label:   fmt.Sprintf("Analysis: %d issues", stats.TotalIssues),
		Metrics: metrics,
	}
}

// mapToLeaderboard creates a Leaderboard pattern from SARIF file stats.
func (m *Mapper) mapToLeaderboard(doc *Document, pc PatternConfig) *design.Leaderboard {
	limit := pc.Limit
	if limit == 0 {
		limit = 10
	}

	topFiles := TopFiles(doc, limit)
	if len(topFiles) == 0 {
		return nil
	}

	items := make([]design.LeaderboardItem, 0, len(topFiles))
	for i, f := range topFiles {
		// Shorten file path for display
		displayName := filepath.Base(f.File)
		if dir := filepath.Dir(f.File); dir != "." {
			displayName = filepath.Join(filepath.Base(dir), displayName)
		}

		items = append(items, design.LeaderboardItem{
			Name:    displayName,
			Metric:  fmt.Sprintf("%d issues", f.IssueCount),
			Value:   float64(f.IssueCount),
			Rank:    i + 1,
			Context: f.File, // Full path as context
		})
	}

	stats := ComputeStats(doc)

	return &design.Leaderboard{
		Label:      "Files with Most Issues",
		MetricName: "Issues",
		Items:      items,
		Direction:  "highest",
		TotalCount: len(stats.ByFile),
		ShowRank:   true,
	}
}

// mapToIssueList creates TestTable patterns for issue lists.
// Returns multiple patterns when grouping by file or rule.
func (m *Mapper) mapToIssueList(doc *Document, pc PatternConfig) []design.Pattern {
	var patterns []design.Pattern

	switch pc.GroupBy {
	case "file":
		groups := GroupByFile(doc)
		for _, g := range groups {
			if p := m.groupToTestTable(g, pc); p != nil {
				patterns = append(patterns, p)
			}
		}

	case "rule_id":
		groups := GroupByRule(doc)
		for _, g := range groups {
			if p := m.groupToTestTable(g, pc); p != nil {
				patterns = append(patterns, p)
			}
		}

	default:
		// No grouping - single list
		if len(doc.Runs) > 0 {
			allResults := GroupedResults{
				Key:     "All Issues",
				Results: doc.Runs[0].Results,
			}
			if p := m.groupToTestTable(allResults, pc); p != nil {
				patterns = append(patterns, p)
			}
		}
	}

	return patterns
}

// groupToTestTable converts a group of results to a TestTable pattern.
func (m *Mapper) groupToTestTable(g GroupedResults, pc PatternConfig) *design.TestTable {
	if len(g.Results) == 0 {
		return nil
	}

	limit := pc.Limit
	results := g.Results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	items := make([]design.TestTableItem, 0, len(results))
	for _, r := range results {
		status := m.mapLevel(r.Level)
		location := ""
		if len(r.Locations) > 0 {
			loc := r.Locations[0].PhysicalLocation
			location = fmt.Sprintf(":%d:%d", loc.Region.StartLine, loc.Region.StartColumn)
		}

		items = append(items, design.TestTableItem{
			Name:     r.RuleID + location,
			Status:   status,
			Duration: "", // Not applicable for lint issues
			Details:  r.Message.Text,
		})
	}

	density := design.DensityDetailed
	switch pc.Detail {
	case "compact":
		density = design.DensityCompact
	case "balanced":
		density = design.DensityBalanced
	}

	label := g.Key
	if len(g.Results) != len(results) {
		label = fmt.Sprintf("%s (%d of %d)", g.Key, len(results), len(g.Results))
	}

	return &design.TestTable{
		Label:   label,
		Results: items,
		Density: density,
	}
}

// mapLevel converts SARIF level to fo status.
func (m *Mapper) mapLevel(level string) string {
	if m.config.SeverityMapping != nil {
		if mapped, ok := m.config.SeverityMapping[level]; ok {
			// Convert fo semantic type to TestTable status
			switch mapped {
			case "error":
				return "fail"
			case "warning":
				return "skip" // Use skip for warnings (shows warning icon)
			case "info":
				return "pass" // Use pass for info (shows checkmark)
			}
		}
	}

	// Default mapping
	switch level {
	case "error":
		return "fail"
	case "warning":
		return "skip"
	default:
		return "pass"
	}
}
