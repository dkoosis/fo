//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/dkoosis/fo/internal/magetasks"
	"github.com/dkoosis/fo/fo"
	"github.com/magefile/mage/mg"
)

var (
	console = fo.DefaultConsole()
)

// Default target - build the binary
var Default = Build

func init() {
	if err := magetasks.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: %v\n", err)
		os.Exit(1)
	}
}

// Build builds the fo binary
func Build() error {
	return magetasks.BuildAll()
}

// Clean removes build artifacts
func Clean() error {
	return magetasks.Clean()
}

// QA runs all quality assurance checks (uses fo library for better output)
func QA() error {
	magetasks.PrintH1Header("fo Quality Assurance")

	fmt.Println("🔍 Running QA checks...")

	// Format check
	if _, err := console.Run("Go Format", "go", "fmt", "./..."); err != nil {
		return fmt.Errorf("format check failed: %w", err)
	}

	// Go vet
	if _, err := console.Run("Go Vet", "go", "vet", "./..."); err != nil {
		return fmt.Errorf("vet failed: %w", err)
	}

	// Staticcheck
	if _, err := console.Run("Staticcheck", "staticcheck", "./..."); err != nil {
		if magetasks.IsCommandNotFound(err) {
			magetasks.PrintWarning("Staticcheck not found (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
		} else {
			return fmt.Errorf("staticcheck failed: %w", err)
		}
	}

	// Golangci-lint with extensive checks
	if _, err := console.Run("Golangci-lint", "golangci-lint", "run",
		"--enable-all",
		"--disable=exhaustivestruct,exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,gomnd,mnd,depguard,tagalign",
		"--timeout=5m",
		"./..."); err != nil {
		if magetasks.IsCommandNotFound(err) {
			magetasks.PrintWarning("Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
		} else {
			return fmt.Errorf("golangci-lint failed: %w", err)
		}
	}

	// Security scan
	if _, err := console.Run("Gosec Security Scan", "gosec", "-quiet", "./..."); err != nil {
		if magetasks.IsCommandNotFound(err) {
			magetasks.PrintWarning("Gosec not found (install: go install github.com/securego/gosec/v2/cmd/gosec@latest)")
		} else {
			return fmt.Errorf("gosec failed: %w", err)
		}
	}

	// Build
	if _, err := console.Run("Go Build", "go", "build", "./..."); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	magetasks.PrintSuccess("QA complete!")
	return nil
}

// Lint namespace for linting commands
type Lint mg.Namespace

// All runs all linters
func (Lint) All() error {
	return magetasks.LintAll()
}

// Format checks code formatting
func (Lint) Format() error {
	return magetasks.LintFormat()
}

// Vet runs go vet
func (Lint) Vet() error {
	return magetasks.LintVet()
}

// Staticcheck runs staticcheck
func (Lint) Staticcheck() error {
	return magetasks.LintStaticcheck()
}

// Golangci runs golangci-lint
func (Lint) Golangci() error {
	return magetasks.LintGolangci()
}

// Fix runs golangci-lint with auto-fixes
func (Lint) Fix() error {
	return magetasks.LintGolangciFix()
}

// Test namespace for testing commands
type Test mg.Namespace

// All runs all tests
func (Test) All() error {
	return magetasks.TestAll()
}

// Coverage runs tests with coverage
func (Test) Coverage() error {
	return magetasks.TestCoverage()
}

// Race runs tests with race detector
func (Test) Race() error {
	return magetasks.TestRace()
}

// Quality namespace for quality check commands
type Quality mg.Namespace

// Check runs the quality validation suite
func (Quality) Check() error {
	return magetasks.QualityCheck()
}

// Report generates the quality report
func (Quality) Report() error {
	return magetasks.QualityReport()
}
