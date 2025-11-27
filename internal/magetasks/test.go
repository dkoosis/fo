package magetasks

import (
	"os"
	"strings"
)

// TestAll runs all tests with formatted output.
func TestAll() error {
	return TestReport()
}

// TestReport runs all test suites with formatted output.
func TestReport() error {
	// Check if race detector is supported
	raceSupported := hasRaceSupport()
	if !raceSupported {
		PrintInfo("CGO disabled; running tests without -race")
	}

	// Build test args
	args := []string{"-v"}
	if raceSupported {
		args = append(args, "-race")
	}
	args = append(args, "-cover")
	args = append(args, listGoTestPackages()...)

	// Use custom formatter for animated output
	return RunFormattedTests(args)
}

// TestCoverage runs tests with coverage.
func TestCoverage() error {
	args := []string{"-coverprofile=coverage.out", "-covermode=atomic"}
	args = append(args, listGoTestPackages()...)

	if err := RunFormattedTests(args); err != nil {
		return err
	}

	// Show coverage report
	summary, err := RunCapture("Get coverage summary", "go", "tool", "cover", "-func=coverage.out")
	if err != nil {
		return err
	}
	printCoverageSummary(summary)
	return nil
}

// TestRace runs tests with race detector.
func TestRace() error {
	args := []string{"-race", "-v"}
	args = append(args, listGoTestPackages()...)
	return RunFormattedTests(args)
}

func listGoTestPackages() []string {
	output, err := RunCapture("List packages", "go", "list", "./...")
	if err != nil {
		return []string{"./..."}
	}

	fields := strings.Fields(output)
	pkgs := append(make([]string, 0, len(fields)), fields...)

	if len(pkgs) == 0 {
		return []string{"./..."}
	}

	return pkgs
}

func hasRaceSupport() bool {
	if env := os.Getenv("CGO_ENABLED"); env != "" {
		env = strings.ToLower(strings.TrimSpace(env))
		return env != "0" && env != "false"
	}
	// Default to true for most systems
	return true
}

func printCoverageSummary(summary string) {
	lines := strings.Split(summary, "\n")
	if len(lines) > 0 {
		// Print the last line which contains total coverage
		lastLine := lines[len(lines)-1]
		if strings.Contains(lastLine, "total:") {
			Console().PrintText(lastLine)
		}
	}
}
