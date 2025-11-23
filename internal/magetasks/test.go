package magetasks

import (
	"fmt"
	"os"
	"os/exec"
)

// TestAll runs all tests.
func TestAll() error {
	PrintH2Header("Tests")

	fmt.Println("Running tests...")
	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError("Tests failed")
		return err
	}

	PrintSuccess("All tests passed")
	return nil
}

// TestCoverage runs tests with coverage.
func TestCoverage() error {
	PrintH2Header("Test Coverage")

	fmt.Println("Running tests with coverage...")
	cmd := exec.Command("go", "test", "-coverprofile=coverage.out", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError("Tests failed")
		return err
	}

	// Show coverage report
	cmd = exec.Command("go", "tool", "cover", "-func=coverage.out")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // Ignore error for coverage display

	PrintSuccess("Coverage report generated")
	return nil
}

// TestRace runs tests with race detector.
func TestRace() error {
	PrintH2Header("Race Detector")

	fmt.Println("Running tests with race detector...")
	cmd := exec.Command("go", "test", "-race", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError("Race detector found issues")
		return err
	}

	PrintSuccess("No race conditions detected")
	return nil
}
