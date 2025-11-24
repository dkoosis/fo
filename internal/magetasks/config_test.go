package magetasks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitialize(t *testing.T) {
	// Save original working directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test Initialize
	err = Initialize()
	if err != nil {
		t.Errorf("Initialize() returned error: %v", err)
	}

	// Verify bin directory was created
	binDir := filepath.Join(tmpDir, "bin")
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		t.Errorf("Initialize() should create bin directory, but it doesn't exist")
	}

	// Verify ProjectRoot is set (use filepath.EvalSymlinks to handle symlinks)
	expectedRoot, _ := filepath.EvalSymlinks(tmpDir)
	actualRoot, _ := filepath.EvalSymlinks(ProjectRoot)
	if actualRoot != expectedRoot {
		t.Errorf("ProjectRoot = %s, want %s", actualRoot, expectedRoot)
	}
}

func TestModulePath(t *testing.T) {
	if ModulePath == "" {
		t.Error("ModulePath should not be empty")
	}
	if ModulePath != "github.com/dkoosis/fo" {
		t.Errorf("ModulePath = %s, want github.com/dkoosis/fo", ModulePath)
	}
}

func TestBinPath(t *testing.T) {
	if BinPath == "" {
		t.Error("BinPath should not be empty")
	}
	if BinPath != "./bin/fo" {
		t.Errorf("BinPath = %s, want ./bin/fo", BinPath)
	}
}
