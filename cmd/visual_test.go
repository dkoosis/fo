//go:build ignore

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dkoosis/fo/fo"
)

// visualTestSuite runs comprehensive visual tests for fo rendering
// and saves outputs to files for design review and iteration.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output-dir>\n", os.Args[0])
		os.Exit(1)
	}

	outputDir := os.Args[1]
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running visual test suite, saving outputs to: %s\n\n", outputDir)

	// Test scenarios
	scenarios := []struct {
		name     string
		run      func(*fo.Console, *bytes.Buffer) error
		filename string
	}{
		{"Section Headers", testSectionHeaders, "01_section_headers.txt"},
		{"Test Results - All Pass", testResultsAllPass, "02_test_results_all_pass.txt"},
		{"Test Results - With Failures", testResultsWithFailures, "03_test_results_with_failures.txt"},
		{"Test Results - Mixed Coverage", testResultsMixedCoverage, "04_test_results_mixed_coverage.txt"},
		{"Quality Checks - All Pass", testQualityChecksAllPass, "05_quality_checks_all_pass.txt"},
		{"Quality Checks - With Warnings", testQualityChecksWithWarnings, "06_quality_checks_with_warnings.txt"},
		{"Build Workflow", testBuildWorkflow, "07_build_workflow.txt"},
		{"Sections with Nested Content", testSectionsWithNestedContent, "08_sections_nested_content.txt"},
		{"Live Sections", testLiveSections, "09_live_sections.txt"},
		{"Error Scenarios", testErrorScenarios, "10_error_scenarios.txt"},
		{"Long Content", testLongContent, "11_long_content.txt"},
		{"Multiple Themes", testMultipleThemes, "12_multiple_themes.txt"},
	}

	for i, scenario := range scenarios {
		fmt.Printf("[%d/%d] Running: %s\n", i+1, len(scenarios), scenario.name)

		var buf bytes.Buffer
		console := fo.NewConsole(fo.ConsoleConfig{
			Out:        &buf,
			Monochrome: false, // Keep colors for visual review
		})

		if err := scenario.run(console, &buf); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
			continue
		}

		outputPath := filepath.Join(outputDir, scenario.filename)
		if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "  Error writing file: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Saved to: %s\n", scenario.filename)
	}

	fmt.Printf("\n✓ Visual test suite complete!\n")
	fmt.Printf("Review outputs in: %s\n", outputDir)
}

func testSectionHeaders(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Main Header")
	console.PrintSectionHeader("Section One")
	console.PrintSectionLine("Content line 1")
	console.PrintSectionLine("Content line 2")
	console.PrintSectionFooter()

	console.PrintSectionHeader("Section Two")
	console.PrintSectionLine("Different content")
	console.PrintSectionLine("More content")
	console.PrintSectionFooter()

	return nil
}

func testResultsAllPass(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Test Results - All Pass")

	renderer := fo.NewTestRenderer(console, buf)

	// Simulate test results
	renderer.RenderGroupHeader("cmd")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "cmd",
		Passed:   16,
		Failed:   0,
		Skipped:  0,
		Duration: 2 * time.Second,
		Coverage: 71.0,
	})
	renderer.RenderGroupFooter()

	renderer.RenderGroupHeader("fo")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "fo",
		Passed:   70,
		Failed:   0,
		Skipped:  0,
		Duration: 1 * time.Second,
		Coverage: 32.0,
	})
	renderer.RenderGroupFooter()

	renderer.RenderGroupHeader("pkg")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "adapter",
		Passed:   17,
		Failed:   0,
		Skipped:  0,
		Duration: 6 * time.Millisecond,
		Coverage: 95.0,
	})
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "design",
		Passed:   236,
		Failed:   0,
		Skipped:  0,
		Duration: 1 * time.Second,
		Coverage: 78.0,
	})
	renderer.RenderGroupFooter()

	renderer.RenderAll()
	return nil
}

func testResultsWithFailures(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Test Results - With Failures")

	renderer := fo.NewTestRenderer(console, buf)

	renderer.RenderGroupHeader("cmd")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:        "cmd",
		Passed:      14,
		Failed:      2,
		Skipped:     0,
		Duration:    1 * time.Second,
		Coverage:    71.0,
		FailedTests: []string{
			"TestRun_ManagesExecutionFlow_When_DifferentInputsProvided",
			"TestRun_ManagesExecutionFlow_When_ArgumentsProvided",
		},
	})
	renderer.RenderGroupFooter()

	renderer.RenderGroupHeader("pkg")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "adapter",
		Passed:   17,
		Failed:   0,
		Skipped:  0,
		Duration: 6 * time.Millisecond,
		Coverage: 95.0,
	})
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:        "design",
		Passed:      230,
		Failed:      6,
		Skipped:     0,
		Duration:    1 * time.Second,
		Coverage:    78.0,
		FailedTests: []string{
			"TestRenderPattern_Sparkline_When_EmptyValues",
			"TestRenderPattern_Leaderboard_When_NoItems",
		},
	})
	renderer.RenderGroupFooter()

	renderer.RenderAll()
	return nil
}

func testResultsMixedCoverage(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Test Results - Mixed Coverage")

	renderer := fo.NewTestRenderer(console, buf)

	renderer.RenderGroupHeader("internal")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "config",
		Passed:   23,
		Failed:   0,
		Skipped:  0,
		Duration: 1 * time.Second,
		Coverage: 52.0, // Warning level
	})
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "magetasks",
		Passed:   15,
		Failed:   0,
		Skipped:  0,
		Duration: 1 * time.Second,
		Coverage: 11.0, // Error level
	})
	renderer.RenderGroupFooter()

	renderer.RenderGroupHeader("pkg")
	renderer.RenderPackageLine(fo.TestPackageResult{
		Name:     "adapter",
		Passed:   17,
		Failed:   0,
		Skipped:  0,
		Duration: 6 * time.Millisecond,
		Coverage: 95.0, // Good level
	})
	renderer.RenderGroupFooter()

	renderer.RenderAll()
	return nil
}

func testQualityChecksAllPass(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Quality Checks - All Pass")

	sections := []fo.Section{
		{
			Name:        "Go Format",
			Description: "Check code formatting",
			Run: func() error {
				time.Sleep(10 * time.Millisecond) // Simulate work
				return nil
			},
		},
		{
			Name:        "Go Vet",
			Description: "Run go vet",
			Run: func() error {
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		},
		{
			Name:        "Staticcheck",
			Description: "Run staticcheck",
			Run: func() error {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		},
		{
			Name:        "Golangci-lint",
			Description: "Run golangci-lint",
			Run: func() error {
				time.Sleep(200 * time.Millisecond)
				return nil
			},
		},
	}

	_, err := console.RunSections(sections...)
	return err
}

func testQualityChecksWithWarnings(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Quality Checks - With Warnings")

	sections := []fo.Section{
		{
			Name:        "Go Format",
			Description: "Check code formatting",
			Run: func() error {
				return nil
			},
		},
		{
			Name:        "Go Vet",
			Description: "Run go vet",
			Run: func() error {
				return fo.NewSectionWarning(errors.New("unused variable 'x' in main.go:42"))
			},
		},
		{
			Name:        "Staticcheck",
			Description: "Run staticcheck",
			Run: func() error {
				return nil
			},
		},
		{
			Name:        "Golangci-lint",
			Description: "Run golangci-lint",
			Run: func() error {
				// Return multiple warnings as a single error
				return fo.NewSectionWarning(fmt.Errorf("line is 120 characters (max 100); function complexity is 15 (max 10)"))
			},
		},
	}

	_, err := console.RunSections(sections...)
	return err
}

func testBuildWorkflow(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Build Workflow")

	// Build section
	buildSection := fo.Section{
		Name:        "Build",
		Description: "Build the fo binary",
		Run: func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		},
	}
	console.RunSection(buildSection)

	// Tests & Quality section
	testSection := fo.Section{
		Name:        "Tests & Quality",
		Description: "Run tests and quality checks",
		Run: func() error {
			// Simulate test output
			renderer := fo.NewTestRenderer(console, buf)
			renderer.RenderGroupHeader("pkg")
			renderer.RenderPackageLine(fo.TestPackageResult{
				Name:     "design",
				Passed:   236,
				Failed:   0,
				Skipped:  0,
				Duration: 1 * time.Second,
				Coverage: 78.0,
			})
			renderer.RenderGroupFooter()
			renderer.RenderAll()
			return nil
		},
	}
	console.RunSection(testSection)

	return nil
}

func testSectionsWithNestedContent(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Sections with Nested Content")

	console.PrintSectionHeader("Main Section")
	console.PrintSectionLine("Main content line 1")
	console.PrintSectionLine("Main content line 2")

	console.PrintSectionLine("")
	console.PrintSectionLine("  Sub-section:")
	console.PrintSectionLine("    - Item 1")
	console.PrintSectionLine("    - Item 2")
	console.PrintSectionLine("    - Item 3")

	console.PrintSectionLine("")
	console.PrintSectionLine("  Another sub-section:")
	console.PrintSectionLine("    • Point A")
	console.PrintSectionLine("    • Point B")

	console.PrintSectionFooter()
	return nil
}

func testLiveSections(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Live Sections")

	ls := fo.NewLiveSection("Live Updates", func(ls *fo.LiveSection) error {
		ls.AddRow("step1", "Initializing...")
		time.Sleep(20 * time.Millisecond)
		ls.UpdateRow("step1", "Initialized")

		ls.AddRow("step2", "Processing data...")
		time.Sleep(30 * time.Millisecond)
		ls.UpdateRow("step2", "Processed 100 items")

		ls.AddRow("step3", "Finalizing...")
		time.Sleep(20 * time.Millisecond)
		ls.UpdateRow("step3", "Complete")

		return nil
	})

	console.RunLiveSection(ls)
	return nil
}

func testErrorScenarios(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Error Scenarios")

	// Section with error
	errorSection := fo.Section{
		Name:        "Failing Task",
		Description: "This task will fail",
		Run: func() error {
			return fmt.Errorf("task failed: connection timeout")
		},
	}
	console.RunSection(errorSection)

	// Section with warnings
	warningSection := fo.Section{
		Name:        "Task with Warnings",
		Description: "This task has warnings",
		Run: func() error {
			return fo.NewSectionWarning(fmt.Errorf("deprecated API used; performance concern detected"))
		},
	}
	console.RunSection(warningSection)

	return nil
}

func testLongContent(console *fo.Console, buf *bytes.Buffer) error {
	console.PrintH1Header("Long Content")

	console.PrintSectionHeader("Section with Long Lines")
	console.PrintSectionLine("This is a very long line that should wrap properly when rendered in the terminal to ensure that the box borders remain aligned and the content is readable.")
	console.PrintSectionLine("")
	console.PrintSectionLine("Another long line with different content that also needs to wrap correctly: Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")
	console.PrintSectionLine("")
	console.PrintSectionLine("Short line")
	console.PrintSectionLine("Medium length line with some content")
	console.PrintSectionFooter()
	return nil
}

func testMultipleThemes(console *fo.Console, buf *bytes.Buffer) error {
	// Note: This would need to be extended to test different themes
	// For now, we'll just show the current theme
	console.PrintH1Header("Theme Testing")
	console.PrintSectionHeader("Current Theme")
	console.PrintSectionLine("This test shows the current theme configuration")
	console.PrintSectionLine("To test multiple themes, run this test with different theme configs")
	console.PrintSectionFooter()
	return nil
}

