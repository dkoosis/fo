package magetasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/sarif"
)

// LintAll runs all linters.
func LintAll() error {
	var errs []error

	// Go format
	if err := LintFormat(); err != nil {
		errs = append(errs, err)
	}

	// Go vet
	if err := LintVet(); err != nil {
		errs = append(errs, err)
	}

	// Staticcheck (optional)
	if err := LintStaticcheck(); err != nil {
		if !IsCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	// Golangci-lint (optional)
	if err := LintGolangci(); err != nil {
		if !IsCommandNotFound(err) {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	SetSectionSummary("All linters passed")
	return nil
}

// LintFormat checks code formatting.
func LintFormat() error {
	return Run("Go Format", "go", "fmt", "./...")
}

// LintVet runs go vet.
func LintVet() error {
	return Run("Go Vet", "go", "vet", "./...")
}

// LintStaticcheck runs staticcheck.
func LintStaticcheck() error {
	if err := Run("Staticcheck", "staticcheck", "./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Staticcheck not found (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
			return err
		}
		return fmt.Errorf("staticcheck failed: %w", err)
	}
	return nil
}

// LintGolangci runs golangci-lint (outputs SARIF to build/golangci.sarif).
func LintGolangci() error {
	// Ensure build directory exists for SARIF output
	if err := os.MkdirAll("build", 0755); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	if err := Run("Golangci-lint", "golangci-lint", "run", "./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}

// LintGolangciFix runs golangci-lint with auto-fixes.
func LintGolangciFix() error {
	if err := Run("Golangci-lint Fix", "golangci-lint", "run", "--fix", "./..."); err != nil {
		if IsCommandNotFound(err) {
			PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
			return err
		}
		return fmt.Errorf("golangci-lint failed: %w", err)
	}
	return nil
}

// LintSARIF runs linters with SARIF output and renders results using fo patterns.
// This is the new SARIF-first linting approach with parallel execution and spinners.
func LintSARIF() error {
	// Ensure build directory exists
	if err := os.MkdirAll("build", 0755); err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}

	// Configure renderer
	config := sarif.DefaultRendererConfig()
	config.Tools["golangci-lint"] = sarif.GolangciLintConfig()

	foConfig := design.DefaultConfig()

	// Create orchestrator
	orch := sarif.NewOrchestrator(config, foConfig)
	orch.SetBuildDir("build")

	// Define tools to run
	tools := []sarif.ToolSpec{
		{
			Name:      "golangci-lint",
			Command:   "golangci-lint",
			Args:      []string{"run", "./..."},
			SARIFPath: "build/golangci.sarif",
		},
	}

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	results, err := orch.Run(ctx, tools)
	if err != nil {
		return fmt.Errorf("lint orchestrator: %w", err)
	}

	// Check for errors
	var hasErrors bool
	for _, r := range results {
		if r.Error != nil {
			hasErrors = true
			continue
		}
		if r.Document != nil {
			stats := sarif.ComputeStats(r.Document)
			if stats.ByLevel["error"] > 0 {
				hasErrors = true
			}
		}
	}

	if hasErrors {
		return fmt.Errorf("linting found issues")
	}

	return nil
}
