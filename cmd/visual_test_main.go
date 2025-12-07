//go:build ignore

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dkoosis/fo/fo"
)

// themeNames defines all themes to generate outputs for
var themeNames = []string{"unicode_vibrant", "orca", "ascii_minimal"}

// scenarios defines all visual test cases
var scenarios = []struct {
	name     string
	key      string // short key for CLI selection
	run      func(*fo.Console, *bytes.Buffer) error
	filename string
}{
	{"Section Headers", "headers", testSectionHeaders, "01_section_headers.txt"},
	{"Test Results - All Pass", "pass", testResultsAllPass, "02_test_results_all_pass.txt"},
	{"Test Results - With Failures", "fail", testResultsWithFailures, "03_test_results_with_failures.txt"},
	{"Test Results - Mixed Coverage", "coverage", testResultsMixedCoverage, "04_test_results_mixed_coverage.txt"},
	{"Quality Checks - All Pass", "qa-pass", testQualityChecksAllPass, "05_quality_checks_all_pass.txt"},
	{"Quality Checks - With Warnings", "qa-warn", testQualityChecksWithWarnings, "06_quality_checks_with_warnings.txt"},
	{"Build Workflow", "build", testBuildWorkflow, "07_build_workflow.txt"},
	{"Sections with Nested Content", "nested", testSectionsWithNestedContent, "08_sections_nested_content.txt"},
	{"Live Sections", "live", testLiveSections, "09_live_sections.txt"},
	{"Error Scenarios", "errors", testErrorScenarios, "10_error_scenarios.txt"},
	{"Long Content", "long", testLongContent, "11_long_content.txt"},
}

// stripANSI removes ANSI escape codes from a string
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		listScenarios()
	case "show":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s show <scenario-key> [--theme <name>]\n", os.Args[0])
			os.Exit(1)
		}
		themeName := "unicode_vibrant" // default
		scenarioKey := os.Args[2]
		// Check for --theme flag
		for i := 3; i < len(os.Args)-1; i++ {
			if os.Args[i] == "--theme" {
				themeName = os.Args[i+1]
				break
			}
		}
		showScenario(scenarioKey, themeName)
	case "save":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: %s save <output-dir>\n", os.Args[0])
			os.Exit(1)
		}
		saveAll(os.Args[2])
	default:
		// Legacy mode: treat first arg as output dir
		saveAll(os.Args[1])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s <command> [args]

Commands:
  list              List all available scenarios and themes
  show <key>        Run a single scenario and print to stdout (with colors)
                    Options: --theme <name>  (default: unicode_vibrant)
  save <dir>        Save all scenarios for all themes to subdirectories

Themes: unicode_vibrant, orca, ascii_minimal

Examples:
  %s list
  %s show fail
  %s show headers --theme orca
  %s save visual_test_outputs
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func listScenarios() {
	fmt.Println("Available scenarios:")
	for _, s := range scenarios {
		fmt.Printf("  %-12s  %s\n", s.key, s.name)
	}
}

func showScenario(key, themeName string) {
	for _, s := range scenarios {
		if s.key == key || strings.EqualFold(s.name, key) {
			// Run directly to stdout with colors
			console := fo.NewConsole(fo.ConsoleConfig{
				ThemeName: themeName,
			})
			var buf bytes.Buffer
			if err := s.run(console, &buf); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Unknown scenario: %s\n", key)
	fmt.Fprintf(os.Stderr, "Run '%s list' to see available scenarios\n", os.Args[0])
	os.Exit(1)
}

func saveAll(outputDir string) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running visual test suite, saving outputs to: %s\n\n", outputDir)

	totalScenarios := len(themeNames) * len(scenarios)
	current := 0

	for _, themeName := range themeNames {
		themeDir := filepath.Join(outputDir, themeName)
		if err := os.MkdirAll(themeDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating theme directory %s: %v\n", themeName, err)
			continue
		}

		fmt.Printf("\n── Theme: %s ──\n", themeName)

		for _, scenario := range scenarios {
			current++
			fmt.Printf("[%d/%d] %s\n", current, totalScenarios, scenario.name)

			var buf bytes.Buffer
			console := fo.NewConsole(fo.ConsoleConfig{
				Out:        &buf,
				ThemeName:  themeName,
				Monochrome: false,
			})

			if err := scenario.run(console, &buf); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
				continue
			}

			// Save raw ANSI version (view with: cat file.ansi)
			ansiPath := filepath.Join(themeDir, strings.TrimSuffix(scenario.filename, ".txt")+".ansi")
			if err := os.WriteFile(ansiPath, buf.Bytes(), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "  Error writing ansi file: %v\n", err)
				continue
			}

			// Save stripped version for reading/diffing
			txtPath := filepath.Join(themeDir, scenario.filename)
			if err := os.WriteFile(txtPath, []byte(stripANSI(buf.String())), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "  Error writing txt file: %v\n", err)
				continue
			}

			fmt.Printf("  ✓ Saved: %s (.ansi + .txt)\n", strings.TrimSuffix(scenario.filename, ".txt"))
		}
	}

	fmt.Printf("\n✓ Visual test suite complete!\n")
	fmt.Printf("View themes:  cat %s/<theme>/*.ansi\n", outputDir)
	fmt.Printf("Compare:      diff %s/unicode_vibrant/ %s/orca/\n", outputDir, outputDir)
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
