package magetasks

import (
	"fmt"
)

// QualityCheck runs all quality checks.
func QualityCheck() error {
	PrintH2Header("Quality Checks")

	// Run linters
	if err := LintAll(); err != nil {
		fmt.Println("Warning: Linting issues found")
	}

	// Run tests
	if err := TestAll(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// Build
	if err := BuildAll(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	PrintSuccess("Quality checks complete")
	return nil
}

// QualityReport generates a quality report.
func QualityReport() error {
	PrintH2Header("Quality Report")

	// For now, just run quality check
	// In the future, this could generate detailed metrics
	return QualityCheck()
}
