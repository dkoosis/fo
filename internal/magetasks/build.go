package magetasks

import (
	"fmt"
	"strings"
	"time"
)

// BuildAll builds all binaries.
func BuildAll() error {
	version := getGitVersion()
	commit := getGitCommit()
	date := time.Now().UTC().Format(time.RFC3339)

	ldflags := fmt.Sprintf(
		"-s -w -X '%s/internal/version.Version=%s' -X '%s/internal/version.CommitHash=%s' -X '%s/internal/version.BuildDate=%s'",
		ModulePath, version, ModulePath, commit, ModulePath, date,
	)

	if err := Run("Build fo", "go", "build", "-ldflags", ldflags, "-o", BinPath, "./cmd"); err != nil {
		return err
	}

	SetSectionSummary("fo binary ready")
	return nil
}

// Clean removes build artifacts.
func Clean() error {
	if err := Run("Remove bin directory", "rm", "-rf", "./bin"); err != nil {
		PrintWarning("Failed to remove bin directory: " + err.Error())
	}
	_ = Run("Clean Go cache", "go", "clean", "-cache") // Ignore error for cleanup command

	SetSectionSummary("Build artifacts cleaned")
	return nil
}

func getGitVersion() string {
	out, err := RunCapture("Get git version", "git", "describe", "--tags", "--always", "--dirty", "--match=v*")
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(out)
}

func getGitCommit() string {
	out, err := RunCapture("Get git commit", "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}
