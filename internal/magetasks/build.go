package magetasks

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// BuildAll builds all binaries
func BuildAll() error {
	PrintH2Header("Build")

	version := getGitVersion()
	commit := getGitCommit()
	date := time.Now().UTC().Format(time.RFC3339)

	ldflags := fmt.Sprintf("-s -w -X '%s/internal/version.Version=%s' -X '%s/internal/version.CommitHash=%s' -X '%s/internal/version.BuildDate=%s'",
		ModulePath, version, ModulePath, commit, ModulePath, date)

	fmt.Println("Building fo...")
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-o", BinPath, "./cmd")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		PrintError("Build failed")
		return err
	}

	PrintSuccess(fmt.Sprintf("Built: %s", BinPath))
	return nil
}

// Clean removes build artifacts
func Clean() error {
	PrintH2Header("Clean")

	os.RemoveAll("./bin")
	cmd := exec.Command("go", "clean", "-cache")
	cmd.Run()

	PrintSuccess("Cleaned build artifacts")
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
