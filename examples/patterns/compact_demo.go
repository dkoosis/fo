package main

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/design"
)

// This example demonstrates compact rendering modes for space-efficient output.
// Compact modes follow Tufte's data-ink ratio principle: maximize information per line.
func main() {
	cfg := design.UnicodeVibrantTheme()

	// Create a larger test result set to demonstrate density differences
	results := []design.TestTableItem{
		{Name: "pkg/api", Status: "pass", Duration: "2.1s"},
		{Name: "pkg/database", Status: "pass", Duration: "1.8s"},
		{Name: "pkg/auth", Status: "fail", Duration: "0.5s"},
		{Name: "pkg/utils", Status: "pass", Duration: "0.3s"},
		{Name: "pkg/models", Status: "pass", Duration: "0.4s"},
		{Name: "pkg/views", Status: "skip", Duration: "0.0s"},
		{Name: "pkg/controllers", Status: "pass", Duration: "1.2s"},
		{Name: "pkg/middleware", Status: "pass", Duration: "0.6s"},
		{Name: "pkg/validators", Status: "pass", Duration: "0.7s"},
		{Name: "pkg/formatters", Status: "pass", Duration: "0.5s"},
		{Name: "pkg/parsers", Status: "pass", Duration: "0.8s"},
		{Name: "pkg/serializers", Status: "pass", Duration: "0.9s"},
	}

	fmt.Println("=== Density Modes Comparison ===\n")

	// Detailed mode (one per line)
	fmt.Println("1. DETAILED MODE (one item per line)")
	detailed := &design.TestTable{
		Label:   "Test Results - Detailed",
		Results: results,
		Density: design.DensityDetailed,
	}
	fmt.Println(detailed.Render(cfg))

	// Balanced mode (2 columns)
	fmt.Println("\n2. BALANCED MODE (2 columns)")
	balanced := &design.TestTable{
		Label:   "Test Results - Balanced",
		Results: results,
		Density: design.DensityBalanced,
	}
	fmt.Println(balanced.Render(cfg))

	// Compact mode (3 columns)
	fmt.Println("\n3. COMPACT MODE (3 columns)")
	compact := &design.TestTable{
		Label:   "Test Results - Compact",
		Results: results,
		Density: design.DensityCompact,
	}
	fmt.Println(compact.Render(cfg))

	fmt.Println("\n=== Line Count Comparison ===")
	fmt.Printf("Detailed: ~%d lines\n", len(results))
	fmt.Printf("Balanced: ~%d lines (saves %d%%)\n", (len(results)+1)/2, 50)
	fmt.Printf("Compact:  ~%d lines (saves %d%%)\n", (len(results)+2)/3, 66)
	fmt.Println("\nCompact modes maximize terminal real estate while maintaining readability.")
	fmt.Println("Ideal for CI logs, multi-pattern dashboards, and large test suites.")
}
