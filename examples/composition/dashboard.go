package main

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/design"
)

// This example demonstrates composing multiple visualization patterns
// together to create a comprehensive build dashboard.
//
// Run: go run dashboard.go

func main() {
	// Use the vibrant Unicode theme for rich visualization
	cfg := design.UnicodeVibrantTheme()

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Build Dashboard - Composition Example               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Pattern 1: Summary - Overall build metrics
	fmt.Println("## Build Summary")
	summary := &design.Summary{
		Label: "Build Status",
		Metrics: []design.SummaryItem{
			{Label: "Total Packages", Value: "247", Type: "info"},
			{Label: "Tests Passed", Value: "1,834", Type: "success"},
			{Label: "Coverage", Value: "88.3%", Type: "success"},
			{Label: "Build Time", Value: "4.2s", Type: "info"},
		},
	}
	fmt.Println(summary.Render(cfg))
	fmt.Println()

	// Pattern 2: Sparkline - Build time trend over last 10 runs
	fmt.Println("## Build Performance Trends")
	buildTimeTrend := &design.Sparkline{
		Label:  "Build time (last 10 runs)",
		Values: []float64{5.2, 4.8, 4.9, 4.5, 4.3, 4.7, 4.2, 4.1, 4.0, 4.2},
		Unit:   "s",
	}
	fmt.Println(buildTimeTrend.Render(cfg))

	coverageTrend := &design.Sparkline{
		Label:  "Test coverage",
		Values: []float64{85.0, 85.5, 86.2, 87.1, 87.8, 88.0, 88.1, 88.2, 88.3, 88.3},
		Unit:   "%",
	}
	fmt.Println(coverageTrend.Render(cfg))
	fmt.Println()

	// Pattern 3: Leaderboard - Slowest tests (optimization targets)
	fmt.Println("## Optimization Targets")
	slowTests := &design.Leaderboard{
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
	fmt.Println(slowTests.Render(cfg))
	fmt.Println()

	// Pattern 4: Comparison - Before/After metrics
	fmt.Println("## Sprint Progress")
	comparison := &design.Comparison{
		Label: "Metrics (start → end of sprint)",
		Changes: []design.ComparisonItem{
			{Label: "Build time", Before: "5.2s", After: "4.2s", Change: -1.0, Unit: "s"},
			{Label: "Binary size", Before: "42MB", After: "38MB", Change: -4.0, Unit: "MB"},
			{Label: "Test coverage", Before: "85%", After: "88.3%", Change: 3.3, Unit: "%"},
			{Label: "Test count", Before: "1,650", After: "1,834", Change: 184, Unit: " tests"},
		},
	}
	fmt.Println(comparison.Render(cfg))
	fmt.Println()

	// Pattern 5: Inventory - Build artifacts produced
	fmt.Println("## Build Artifacts")
	artifacts := &design.Inventory{
		Label: "Generated Binaries",
		Items: []design.InventoryItem{
			{Name: "myapp", Size: "2.3 MB", Path: "./bin/myapp"},
			{Name: "myctl", Size: "1.1 MB", Path: "./bin/myctl"},
			{Name: "mylib.so", Size: "450 KB", Path: "./lib/mylib.so"},
		},
	}
	fmt.Println(artifacts.Render(cfg))
	fmt.Println()

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("This dashboard combines 5 different patterns to provide")
	fmt.Println("a comprehensive view of the build process:")
	fmt.Println("  • Summary: High-level metrics")
	fmt.Println("  • Sparkline: Trends over time")
	fmt.Println("  • Leaderboard: Ranked optimization targets")
	fmt.Println("  • Comparison: Before/after deltas")
	fmt.Println("  • Inventory: Build artifacts")
	fmt.Println()
	fmt.Println("Each pattern is rendered independently and composed")
	fmt.Println("together to create a cohesive dashboard.")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
