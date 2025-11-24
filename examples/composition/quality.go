package main

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/design"
)

// This example demonstrates composing patterns to create a quality metrics dashboard
// that compares coverage changes, highlights slow tests, and shows trends.
//
// Run: go run quality.go

func main() {
	// Use the vibrant Unicode theme for rich visualization
	cfg := design.UnicodeVibrantTheme()

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Quality Metrics Dashboard - Composition Example      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Pattern 1: Comparison - Coverage delta over time
	fmt.Println("## Coverage Changes")
	coverageChange := &design.Comparison{
		Label: "Coverage Delta (Last Sprint)",
		Changes: []design.ComparisonItem{
			{Label: "pkg/api", Before: "92%", After: "94%", Change: 2.0, Unit: "%"},
			{Label: "pkg/db", Before: "78%", After: "85%", Change: 7.0, Unit: "%"},
			{Label: "pkg/auth", Before: "88%", After: "88%", Change: 0.0, Unit: "%"},
			{Label: "pkg/utils", Before: "95%", After: "96%", Change: 1.0, Unit: "%"},
		},
	}
	fmt.Println(coverageChange.Render(cfg))
	fmt.Println()

	// Pattern 2: Leaderboard - Slowest tests (optimization targets)
	fmt.Println("## Performance Hotspots")
	slowestTests := &design.Leaderboard{
		Label:      "Slowest Tests",
		MetricName: "Duration",
		Direction:  "highest",
		TotalCount: 1834,
		ShowRank:   true,
		Items: []design.LeaderboardItem{
			{Name: "TestLargeDataProcessing", Metric: "5.2s", Value: 5.2, Rank: 1},
			{Name: "TestComplexQueryExecution", Metric: "3.8s", Value: 3.8, Rank: 2},
			{Name: "TestNetworkIntegration", Metric: "2.9s", Value: 2.9, Rank: 3},
			{Name: "TestDatabaseMigration", Metric: "2.1s", Value: 2.1, Rank: 4},
			{Name: "TestCacheWarmup", Metric: "1.8s", Value: 1.8, Rank: 5},
		},
	}
	fmt.Println(slowestTests.Render(cfg))
	fmt.Println()

	// Pattern 3: Sparkline - Coverage trend over last 10 runs
	fmt.Println("## Coverage Trend")
	coverageTrend := &design.Sparkline{
		Label:  "Coverage progression",
		Values: []float64{85.0, 85.5, 86.2, 87.1, 87.8, 88.0, 88.1, 88.2, 88.3, 88.3},
		Unit:   "%",
	}
	fmt.Println(coverageTrend.Render(cfg))
	fmt.Println()

	// Pattern 4: Leaderboard - Largest binaries (size analysis)
	fmt.Println("## Binary Size Analysis")
	largestBinaries := &design.Leaderboard{
		Label:      "Largest Binaries",
		MetricName: "Size",
		Direction:  "highest",
		ShowRank:   true,
		Items: []design.LeaderboardItem{
			{Name: "myapp", Metric: "45MB", Value: 45, Rank: 1},
			{Name: "myctl", Metric: "12MB", Value: 12, Rank: 2},
			{Name: "mylib.so", Metric: "8.5MB", Value: 8.5, Rank: 3},
		},
	}
	fmt.Println(largestBinaries.Render(cfg))
	fmt.Println()

	// Pattern 5: Summary - Overall quality metrics
	fmt.Println("## Quality Summary")
	qualitySummary := &design.Summary{
		Label: "Overall Quality Metrics",
		Metrics: []design.SummaryItem{
			{Label: "Test Coverage", Value: "88.3%", Type: "success"},
			{Label: "Total Tests", Value: "1,834", Type: "info"},
			{Label: "Passing Tests", Value: "1,820", Type: "success"},
			{Label: "Failing Tests", Value: "14", Type: "error"},
			{Label: "Avg Test Duration", Value: "0.8s", Type: "info"},
		},
	}
	fmt.Println(qualitySummary.Render(cfg))
	fmt.Println()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("This quality dashboard combines 5 patterns to provide")
	fmt.Println("a comprehensive view of code quality:")
	fmt.Println("  • Comparison: Coverage changes by package")
	fmt.Println("  • Leaderboard: Slowest tests (optimization targets)")
	fmt.Println("  • Sparkline: Coverage trend over time")
	fmt.Println("  • Leaderboard: Largest binaries (size analysis)")
	fmt.Println("  • Summary: Overall quality metrics")
	fmt.Println()
	fmt.Println("Each pattern focuses on a specific aspect of quality,")
	fmt.Println("allowing developers to quickly identify areas for improvement.")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

