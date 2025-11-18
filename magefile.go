//go:build mage

package main

import (
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
