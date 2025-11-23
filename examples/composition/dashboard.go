package main

import (
	"fmt"
	"strings"

	"github.com/dkoosis/fo/pkg/design"
)

// This example demonstrates composing multiple patterns to create a complete build dashboard.
// Composition allows you to tell a complete story: overall status, trends, hotspots, and outputs.
func main() {
	cfg := design.UnicodeVibrantTheme()

	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Build Dashboard - Composition Example               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝\n")

	// 1. Summary - Overall Status (what happened?)
	summary := &design.Summary{
		Label: "Build Summary",
		Metrics: []design.SummaryItem{
			{Label: "Total Tests", Value: "247", Type: "info"},
			{Label: "Passed", Value: "232", Type: "success"},
			{Label: "Failed", Value: "3", Type: "error"},
			{Label: "Skipped", Value: "12", Type: "warning"},
			{Label: "Duration", Value: "45.2s", Type: "info"},
			{Label: "Coverage", Value: "88.0%", Type: "success"},
		},
	}
	fmt.Println(summary.Render(cfg))

	// 2. Sparklines - Trends (how are we doing over time?)
	buildTimeTrend := &design.Sparkline{
		Label:  "Build time trend (last 8 builds)",
		Values: []float64{52.1, 50.3, 48.7, 49.2, 47.8, 46.5, 45.9, 45.2},
		Unit:   "s",
	}
	fmt.Println(buildTimeTrend.Render(cfg))

	coverageTrend := &design.Sparkline{
		Label:  "Test coverage trend",
		Values: []float64{82.0, 83.5, 85.0, 86.2, 87.1, 88.0},
		Unit:   "%",
	}
	fmt.Println(coverageTrend.Render(cfg))
	fmt.Println()

	// 3. Comparison - Changes (what improved/regressed?)
	comparison := &design.Comparison{
		Label: "Performance vs. Previous Build",
		Changes: []design.ComparisonItem{
			{Label: "Build time", Before: "45.9s", After: "45.2s", Change: -0.7, Unit: "s"},
			{Label: "Binary size", Before: "42MB", After: "38MB", Change: -4, Unit: "MB"},
			{Label: "Test coverage", Before: "87.1%", After: "88.0%", Change: 0.9, Unit: "%"},
			{Label: "Test count", Before: "245", After: "247", Change: 2, Unit: " tests"},
		},
	}
	fmt.Println(comparison.Render(cfg))

	// 4. Leaderboard - Hotspots (what needs attention?)
	slowestTests := &design.Leaderboard{
		Label:      "Slowest Tests (Optimization Targets)",
		MetricName: "Duration",
		Direction:  "highest",
		TotalCount: 247,
		ShowRank:   true,
		Items: []design.LeaderboardItem{
			{Name: "TestLargeDataProcessing", Metric: "5.2s", Value: 5.2, Rank: 1, Context: "pkg/database"},
			{Name: "TestComplexQueryExecution", Metric: "3.8s", Value: 3.8, Rank: 2, Context: "pkg/query"},
			{Name: "TestNetworkIntegration", Metric: "2.9s", Value: 2.9, Rank: 3, Context: "pkg/api"},
			{Name: "TestConcurrentOperations", Metric: "2.1s", Value: 2.1, Rank: 4, Context: "pkg/worker"},
			{Name: "TestDatabaseMigrations", Metric: "1.8s", Value: 1.8, Rank: 5, Context: "pkg/migrations"},
		},
	}
	fmt.Println(slowestTests.Render(cfg))

	// 5. TestTable - Detailed Results (show me the data)
	testTable := &design.TestTable{
		Label: "Test Results by Package",
		Results: []design.TestTableItem{
			{Name: "pkg/api", Status: "pass", Duration: "8.2s", Count: 42},
			{Name: "pkg/database", Status: "pass", Duration: "12.8s", Count: 28},
			{Name: "pkg/auth", Status: "fail", Duration: "1.5s", Count: 15, Details: "3 failures: authentication timeout"},
			{Name: "pkg/utils", Status: "pass", Duration: "1.3s", Count: 38},
			{Name: "pkg/models", Status: "pass", Duration: "2.4s", Count: 35},
			{Name: "pkg/views", Status: "skip", Duration: "0.0s", Count: 12, Details: "requires external service"},
			{Name: "pkg/controllers", Status: "pass", Duration: "5.2s", Count: 28},
			{Name: "pkg/middleware", Status: "pass", Duration: "3.6s", Count: 22},
			{Name: "pkg/validators", Status: "pass", Duration: "2.7s", Count: 18},
			{Name: "pkg/services", Status: "pass", Duration: "7.5s", Count: 11},
		},
		Density: design.DensityBalanced, // Use 2-column layout for space efficiency
	}
	fmt.Println(testTable.Render(cfg))

	// 6. Inventory - Artifacts (what was produced?)
	inventory := &design.Inventory{
		Label: "Build Artifacts",
		Items: []design.InventoryItem{
			{Name: "myapp", Size: "38.2MB", Path: "./bin/myapp"},
			{Name: "myapp-darwin-amd64", Size: "39.1MB", Path: "./dist/myapp-darwin-amd64"},
			{Name: "myapp-darwin-arm64", Size: "37.8MB", Path: "./dist/myapp-darwin-arm64"},
			{Name: "myapp-linux-amd64", Size: "38.5MB", Path: "./dist/myapp-linux-amd64"},
			{Name: "myapp-windows-amd64.exe", Size: "39.8MB", Path: "./dist/myapp-windows-amd64.exe"},
		},
	}
	fmt.Println(inventory.Render(cfg))

	fmt.Println("\n" + strings.Repeat("─", 66))
	fmt.Println("Dashboard Composition Benefits:")
	fmt.Println("  • Complete story: status → trends → changes → hotspots → details → outputs")
	fmt.Println("  • Cognitive load aware: summary first, details last")
	fmt.Println("  • Actionable: leaderboard highlights what to optimize")
	fmt.Println("  • Space-efficient: balanced density for test table")
	fmt.Println("  • Tufte-inspired: high data-ink ratio, small multiples")
}
