//go:build mage

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	binPath    = "./bin/fo"
	modulePath = "github.com/davidkoosis/fo"
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
