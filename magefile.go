//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/dkoosis/fo/fo"
	"github.com/dkoosis/fo/internal/magetasks"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var (
	console = fo.DefaultConsole()
)

// Default target - run fo dashboard
var Default = Dashboard

func init() {
	if err := magetasks.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: %v\n", err)
		os.Exit(1)
	}
}

// Dashboard runs the fo dashboard TUI with parallel tasks (matches orca style)
func Dashboard() error {
	console.PrintH1Header("fo Dashboard")
	// Build fo first
	if err := sh.RunV("go", "build", "-o", "/tmp/fo-dashboard", "./cmd/fo"); err != nil {
		return fmt.Errorf("failed to build fo: %w", err)
	}
	// Run dashboard with TTY attached
	cmd := exec.Command("/tmp/fo-dashboard", "--dashboard",
		// Build
		"--task", "Build/compile:go build ./...",
		// Test
		"--task", "Test/unit:go test -json -cover ./...",
		// Lint
		"--task", "Lint/vet:go vet ./...",
		"--task", "Lint/gofmt:gofmt -l .",
		"--task", "Lint/filesize:filesize -dir=. -top=5",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Mage runs the standard build + test workflow
func Mage() error {
	buildID := getBuildID()
	if buildID != "" {
		magetasks.Console().PrintH1Header("fo Build Suite - " + buildID)
	}
	return magetasks.RunAll()
}

// getBuildID returns a meaningful build identifier (git commit hash).
func getBuildID() string {
	commit, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(commit)
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
	magetasks.Console().PrintH1Header("fo Quality Assurance")
	return magetasks.QualityCheck()
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

// Sarif runs linters with SARIF output and visual rendering
func (Lint) Sarif() error {
	return magetasks.LintSARIF()
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

// Visual runs the visual test suite for rendering validation
func (Test) Visual() error {
	console.PrintH1Header("Visual Test Suite")
	if err := os.MkdirAll("visual_test_outputs", 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	return sh.RunV("go", "run", "cmd/visual_test_main.go", "visual_test_outputs")
}

// Dashboard tests the dashboard TUI with parallel tasks (matches orca style)
func (Test) Dashboard() error {
	console.PrintH1Header("Dashboard TUI Test")
	// Build fo first
	if err := sh.RunV("go", "build", "-o", "/tmp/fo-dashboard", "./cmd/fo"); err != nil {
		return fmt.Errorf("failed to build fo: %w", err)
	}
	// Run dashboard with TTY attached (sh.RunV captures stdout, breaking TTY detection)
	cmd := exec.Command("/tmp/fo-dashboard", "--dashboard",
		// Build
		"--task", "Build/compile:go build ./...",
		// Test
		"--task", "Test/unit:go test -json -cover ./...",
		// Lint
		"--task", "Lint/vet:go vet ./...",
		"--task", "Lint/gofmt:gofmt -l .",
		"--task", "Lint/filesize:filesize -dir=. -top=5",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
