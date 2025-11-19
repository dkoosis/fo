//go:build mage

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/davidkoosis/fo/mageconsole"
)

var (
	binPath    = "./bin/fo"
	modulePath = "github.com/davidkoosis/fo"
	console    = mageconsole.DefaultConsole()
)

// Build builds the fo binary.
func Build() error {
	if err := os.MkdirAll("./bin", 0o755); err != nil {
		return err
	}

	version := getGitVersion()
	commit := getGitCommit()
	date := time.Now().UTC().Format(time.RFC3339)

	ldflags := fmt.Sprintf("-s -w -X '%s/internal/version.Version=%s' -X '%s/internal/version.CommitHash=%s' -X '%s/internal/version.BuildDate=%s'",
		modulePath, version, modulePath, commit, modulePath, date)

	fmt.Println("Building fo...")
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", binPath, "./cmd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("Built: %s\n", binPath)
	return nil
}

// Clean removes build artifacts.
func Clean() error {
	os.RemoveAll("./bin")
	cmd := exec.Command("go", "clean", "-cache")
	cmd.Run()
	fmt.Println("Cleaned")
	return nil
}

func getGitVersion() string {
	out, err := exec.Command("git", "describe", "--tags", "--always", "--dirty", "--match=v*").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}

func getGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// isCommandNotFound checks if the error indicates the command was not found.
// This handles exec.ErrNotFound and platform-specific string fallbacks.
func isCommandNotFound(err error) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	// Fallback string matching for edge cases
	errStr := err.Error()
	if strings.Contains(errStr, "executable file not found") {
		return true
	}
	if strings.Contains(errStr, "no such file or directory") {
		return true
	}
	return false
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
		if isCommandNotFound(err) {
			fmt.Println("⚠️  Staticcheck not found (install: go install honnef.co/go/tools/cmd/staticcheck@latest)")
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
		if isCommandNotFound(err) {
			fmt.Println("⚠️  Golangci-lint not found (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)")
		} else {
			return fmt.Errorf("golangci-lint failed: %w", err)
		}
	}

	// Security scan
	if _, err := console.Run("Gosec Security Scan", "gosec", "-quiet", "./..."); err != nil {
		if isCommandNotFound(err) {
			fmt.Println("⚠️  Gosec not found (install: go install github.com/securego/gosec/v2/cmd/gosec@latest)")
		} else {
			return fmt.Errorf("gosec failed: %w", err)
		}
	}

	// Build
	if _, err := console.Run("Go Build", "go", "build", "./..."); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	fmt.Println("✅ QA complete!")
	return nil
}
