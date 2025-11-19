//go:build mage

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMagefileIntegration_Build(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Run mage build using -d and -w flags to specify the directory
	dir := getExamplesDir(t)
	cmd := exec.Command("mage", "-d", dir, "-w", dir, "build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mage build failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Go Build") {
		t.Errorf("expected output to contain 'Go Build', got: %s", outputStr)
	}
}

func TestMagefileIntegration_ConsoleReuse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := getExamplesDir(t)

	// Test that console can be reused for multiple commands
	// The Build target uses the same console instance
	cmd := exec.Command("mage", "-d", dir, "-w", dir, "build")
	_, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("first mage build failed: %v", err)
	}

	// Run again to verify console can be reused
	cmd2 := exec.Command("mage", "-d", dir, "-w", dir, "build")
	_, err2 := cmd2.CombinedOutput()

	if err2 != nil {
		t.Fatalf("second mage build failed: %v", err2)
	}
}

func TestMagefileIntegration_MultipleTargets(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := getExamplesDir(t)

	// Run multiple targets sequentially
	targets := []string{"build", "test"}

	for _, target := range targets {
		cmd := exec.Command("mage", "-d", dir, "-w", dir, target)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("mage %s failed: %v\nOutput: %s", target, err, output)
		}
	}
}

func TestMagefileIntegration_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := getExamplesDir(t)

	// Test non-existent target
	cmd := exec.Command("mage", "-d", dir, "-w", dir, "nonexistent")
	_, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("expected error for non-existent target")
	}
}

func getExamplesDir(t *testing.T) string {
	t.Helper()
	// Get the directory where this test file lives
	// When running from project root, we need to find examples/mage
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	// Check if we're in examples/mage by looking for both magefile.go and README.md
	// (the project root has magefile.go but the examples/mage also has a README.md)
	if _, err := os.Stat(filepath.Join(wd, "magefile.go")); err == nil {
		// Check if this is examples/mage by looking for its specific go.mod module name
		if data, err := os.ReadFile(filepath.Join(wd, "go.mod")); err == nil {
			if strings.Contains(string(data), "github.com/davidkoosis/fo/examples/mage") {
				return wd // We're in examples/mage
			}
		}
	}

	// Try examples/mage from current directory
	examplesDir := filepath.Join(wd, "examples", "mage")
	if _, err := os.Stat(filepath.Join(examplesDir, "magefile.go")); err == nil {
		return examplesDir
	}

	t.Fatalf("could not find examples/mage directory from %s", wd)
	return ""
}
