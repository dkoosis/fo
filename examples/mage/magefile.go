//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidkoosis/fo/mageconsole"
)

var console = mageconsole.DefaultConsole()

func init() {
	// Change to project root (two levels up from examples/mage)
	if cwd, err := os.Getwd(); err == nil {
		root := filepath.Join(cwd, "..", "..")
		os.Chdir(root)
	}
}

// Build builds the module using the mage console output.
func Build() error {
	_, err := console.Run("Go Build", "go", "build", "./...")
	return err
}

// Test runs the test suite using the mage console output.
func Test() error {
	_, err := console.Run("Go Test", "go", "test", "./...")
	return err
}

// QA runs all quality assurance checks.
func QA() error {
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
		fmt.Println("⚠️  Staticcheck failed (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
	}

	// Golangci-lint with extensive checks
	if _, err := console.Run("Golangci-lint", "golangci-lint", "run",
		"--enable-all",
		"--disable=exhaustivestruct,exhaustruct,varnamelen,ireturn,wrapcheck,nlreturn,gochecknoglobals,gomnd,mnd,depguard,tagalign",
		"--timeout=5m",
		"./..."); err != nil {
		fmt.Println("⚠️  Golangci-lint failed (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
	}

	// Security scan
	if _, err := console.Run("Gosec Security Scan", "gosec", "-quiet", "./..."); err != nil {
		fmt.Println("⚠️  Gosec failed (install: go install github.com/securego/gosec/v2/cmd/gosec@latest)")
	}

	// Build
	if _, err := console.Run("Go Build", "go", "build", "./..."); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("✅ QA complete!")
	return nil
}
