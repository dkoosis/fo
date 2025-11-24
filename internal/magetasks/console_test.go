package magetasks

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPrintH1Header(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintH1Header("Test Title")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Test Title") {
		t.Errorf("PrintH1Header output should contain 'Test Title', got: %s", output)
	}
	if !strings.Contains(output, "=") {
		t.Errorf("PrintH1Header output should contain '=' separator, got: %s", output)
	}
}

func TestPrintH2Header(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintH2Header("Test Section")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "=== Test Section ===") {
		t.Errorf("PrintH2Header output should contain '=== Test Section ===', got: %s", output)
	}
}

func TestPrintSuccess(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintSuccess("Operation completed")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Operation completed") {
		t.Errorf("PrintSuccess output should contain message, got: %s", output)
	}
}

func TestPrintWarning(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintWarning("Warning message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Warning message") {
		t.Errorf("PrintWarning output should contain message, got: %s", output)
	}
}

func TestPrintError(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintError("Error message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Error message") {
		t.Errorf("PrintError output should contain message, got: %s", output)
	}
}

func TestPrintInfo(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	PrintInfo("Info message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Info message") {
		t.Errorf("PrintInfo output should contain message, got: %s", output)
	}
}
