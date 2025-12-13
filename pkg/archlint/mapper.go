package archlint

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/dkoosis/fo/pkg/design"
)

// Mapper converts go-arch-lint results to fo design patterns.
type Mapper struct{}

// NewMapper creates a new mapper.
func NewMapper() *Mapper {
	return &Mapper{}
}

// MapToPatterns converts go-arch-lint JSON result to fo patterns.
func (m *Mapper) MapToPatterns(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Always add summary
	patterns = append(patterns, m.mapToSummary(r))

	// Add leaderboard if there are violations
	if lb := m.mapToLeaderboard(r); lb != nil {
		patterns = append(patterns, lb)
	}

	// Add violation details
	patterns = append(patterns, m.mapToViolationList(r)...)

	return patterns
}

// mapToSummary creates a Summary pattern showing pass/fail status.
func (m *Mapper) mapToSummary(r *Result) *design.Summary {
	stats := ComputeStats(r)

	if stats.TotalViolations == 0 {
		return &design.Summary{
			Label: "Architecture Check",
			Metrics: []design.SummaryItem{
				{Label: "Status", Value: "OK", Type: "success"},
				{Label: "Module", Value: filepath.Base(r.Payload.ModuleName), Type: "info"},
			},
		}
	}

	var metrics []design.SummaryItem
	metrics = append(metrics, design.SummaryItem{
		Label: "Status",
		Value: "FAIL",
		Type:  "error",
	})

	if stats.DepViolations > 0 {
		metrics = append(metrics, design.SummaryItem{
			Label: "Import Violations",
			Value: fmt.Sprintf("%d", stats.DepViolations),
			Type:  "error",
		})
	}

	if stats.NotMatchedFiles > 0 {
		metrics = append(metrics, design.SummaryItem{
			Label: "Unmatched Files",
			Value: fmt.Sprintf("%d", stats.NotMatchedFiles),
			Type:  "warning",
		})
	}

	if stats.DeepScanWarnings > 0 {
		metrics = append(metrics, design.SummaryItem{
			Label: "Deep Scan Warnings",
			Value: fmt.Sprintf("%d", stats.DeepScanWarnings),
			Type:  "warning",
		})
	}

	if stats.OmittedCount > 0 {
		metrics = append(metrics, design.SummaryItem{
			Label: "Omitted",
			Value: fmt.Sprintf("%d", stats.OmittedCount),
			Type:  "muted",
		})
	}

	return &design.Summary{
		Label:   fmt.Sprintf("Architecture Check: %d violations", stats.TotalViolations),
		Metrics: metrics,
	}
}

// mapToLeaderboard creates a Leaderboard showing components with most violations.
func (m *Mapper) mapToLeaderboard(r *Result) *design.Leaderboard {
	stats := ComputeStats(r)

	if len(stats.ByComponent) == 0 {
		return nil
	}

	// Sort components by violation count
	type compCount struct {
		name  string
		count int
	}
	var components []compCount
	for name, count := range stats.ByComponent {
		components = append(components, compCount{name, count})
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].count > components[j].count
	})

	// Limit to top 5
	limit := 5
	if len(components) < limit {
		limit = len(components)
	}

	items := make([]design.LeaderboardItem, 0, limit)
	for i := 0; i < limit; i++ {
		c := components[i]
		items = append(items, design.LeaderboardItem{
			Name:   c.name,
			Metric: fmt.Sprintf("%d violations", c.count),
			Value:  float64(c.count),
			Rank:   i + 1,
		})
	}

	return &design.Leaderboard{
		Label:      "Components with Most Violations",
		MetricName: "Violations",
		Items:      items,
		Direction:  "highest",
		TotalCount: len(stats.ByComponent),
		ShowRank:   true,
	}
}

// mapToViolationList creates TestTable patterns for violation details.
func (m *Mapper) mapToViolationList(r *Result) []design.Pattern {
	var patterns []design.Pattern

	// Group dependency violations by source component
	if len(r.Payload.ArchWarningsDeps) > 0 {
		grouped := make(map[string][]DepWarning)
		for _, w := range r.Payload.ArchWarningsDeps {
			grouped[w.ComponentFrom] = append(grouped[w.ComponentFrom], w)
		}

		// Sort component names for consistent output
		var compNames []string
		for name := range grouped {
			compNames = append(compNames, name)
		}
		sort.Strings(compNames)

		for _, comp := range compNames {
			warnings := grouped[comp]
			items := make([]design.TestTableItem, 0, len(warnings))
			for _, w := range warnings {
				loc := fmt.Sprintf("%s:%d", filepath.Base(w.FileFrom), w.Reference.Line)
				details := fmt.Sprintf("imports %s (%s)", w.ComponentTo, w.Reference.Name)
				items = append(items, design.TestTableItem{
					Name:    loc,
					Status:  "fail",
					Details: details,
				})
			}

			patterns = append(patterns, &design.TestTable{
				Label:   fmt.Sprintf("%s (%d violations)", comp, len(warnings)),
				Results: items,
				Density: design.DensityDetailed,
			})
		}
	}

	// Unmatched files as a separate table
	if len(r.Payload.ArchWarningsNotMatched) > 0 {
		items := make([]design.TestTableItem, 0, len(r.Payload.ArchWarningsNotMatched))
		for _, f := range r.Payload.ArchWarningsNotMatched {
			items = append(items, design.TestTableItem{
				Name:    filepath.Base(f),
				Status:  "skip",
				Details: "not matched to any component",
			})
		}

		patterns = append(patterns, &design.TestTable{
			Label:   fmt.Sprintf("Unmatched Files (%d)", len(items)),
			Results: items,
			Density: design.DensityCompact,
		})
	}

	return patterns
}
