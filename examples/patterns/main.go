package main

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/design"
)

func main() {
	// Use the built-in Unicode vibrant theme
	cfg := design.UnicodeVibrantTheme()

	fmt.Println("=== Pattern Examples ===\n")

	// Example 1: Sparkline
	fmt.Println("1. Sparkline - Trend Visualization")
	sparkline := &design.Sparkline{
		Label:  "Build time trend",
		Values: []float64{2.3, 2.1, 2.4, 1.9, 1.8, 2.0, 1.7, 1.6},
		Unit:   "s",
	}
	fmt.Println(sparkline.Render(cfg))
	fmt.Println()

	sparkline2 := &design.Sparkline{
		Label:  "Test coverage",
		Values: []float64{82.0, 83.5, 85.0, 86.2, 87.1, 88.0},
		Unit:   "%",
	}
	fmt.Println(sparkline2.Render(cfg))
	fmt.Println()

	// Example 2: Leaderboard
	fmt.Println("2. Leaderboard - Slowest Tests")
	leaderboard := &design.Leaderboard{
		Label:      "Slowest Tests",
		MetricName: "Duration",
		Direction:  "highest",
		TotalCount: 247,
		ShowRank:   true,
		Items: []design.LeaderboardItem{
			{Name: "TestLargeDataProcessing", Metric: "5.2s", Value: 5.2, Rank: 1},
			{Name: "TestComplexQueryExecution", Metric: "3.8s", Value: 3.8, Rank: 2},
			{Name: "TestNetworkIntegration", Metric: "2.9s", Value: 2.9, Rank: 3},
			{Name: "TestConcurrentOperations", Metric: "2.1s", Value: 2.1, Rank: 4},
			{Name: "TestDatabaseMigrations", Metric: "1.8s", Value: 1.8, Rank: 5},
		},
	}
	fmt.Println(leaderboard.Render(cfg))
	fmt.Println()

	// Example 3: TestTable
	fmt.Println("3. TestTable - Test Results")
	testTable := &design.TestTable{
		Label: "Unit Test Results",
		Results: []design.TestTableItem{
			{Name: "pkg/api", Status: "pass", Duration: "2.1s", Count: 42},
			{Name: "pkg/database", Status: "pass", Duration: "1.8s", Count: 28},
			{Name: "pkg/auth", Status: "fail", Duration: "0.5s", Count: 15, Details: "authentication timeout"},
			{Name: "pkg/utils", Status: "pass", Duration: "0.3s", Count: 38},
			{Name: "pkg/models", Status: "skip", Duration: "0.0s", Count: 12, Details: "requires external service"},
		},
	}
	fmt.Println(testTable.Render(cfg))
	fmt.Println()

	// Example 4: Summary
	fmt.Println("4. Summary - Build Summary")
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
	fmt.Println()

	// Example 5: Comparison
	fmt.Println("5. Comparison - Performance Changes")
	comparison := &design.Comparison{
		Label: "Performance vs. Previous Build",
		Changes: []design.ComparisonItem{
			{Label: "Build time", Before: "5.2s", After: "4.1s", Change: -1.1, Unit: "s"},
			{Label: "Binary size", Before: "42MB", After: "38MB", Change: -4, Unit: "MB"},
			{Label: "Test coverage", Before: "85%", After: "88%", Change: 3, Unit: "%"},
			{Label: "Test duration", Before: "52.1s", After: "45.2s", Change: -6.9, Unit: "s"},
		},
	}
	fmt.Println(comparison.Render(cfg))
	fmt.Println()

	// Example 6: Inventory
	fmt.Println("6. Inventory - Build Artifacts")
	inventory := &design.Inventory{
		Label: "Generated Artifacts",
		Items: []design.InventoryItem{
			{Name: "myapp", Size: "38.2MB", Path: "./bin/myapp"},
			{Name: "myapp-darwin-amd64", Size: "39.1MB", Path: "./dist/myapp-darwin-amd64"},
			{Name: "myapp-darwin-arm64", Size: "37.8MB", Path: "./dist/myapp-darwin-arm64"},
			{Name: "myapp-linux-amd64", Size: "38.5MB", Path: "./dist/myapp-linux-amd64"},
			{Name: "myapp-windows-amd64.exe", Size: "39.8MB", Path: "./dist/myapp-windows-amd64.exe"},
		},
	}
	fmt.Println(inventory.Render(cfg))
}
