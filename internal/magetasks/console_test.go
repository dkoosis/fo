package magetasks

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintH1Header(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintH1Header("Test Title")

	output := buf.String()

	if !strings.Contains(output, "TEST TITLE") {
		t.Errorf("PrintH1Header output should contain 'TEST TITLE', got: %s", output)
	}
}

func TestPrintH2Header(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintH2Header("Test Section")

	output := buf.String()

	if !strings.Contains(output, "TEST SECTION") {
		t.Errorf("PrintH2Header output should contain 'TEST SECTION', got: %s", output)
	}
}

func TestPrintSuccess(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintSuccess("Operation completed")

	output := buf.String()

	if !strings.Contains(output, "Operation completed") {
		t.Errorf("PrintSuccess output should contain message, got: %s", output)
	}
}

func TestPrintWarning(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintWarning("Warning message")

	output := buf.String()

	if !strings.Contains(output, "Warning message") {
		t.Errorf("PrintWarning output should contain message, got: %s", output)
	}
}

func TestPrintError(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintError("Error message")

	output := buf.String()

	if !strings.Contains(output, "Error message") {
		t.Errorf("PrintError output should contain message, got: %s", output)
	}
}

func TestPrintInfo(t *testing.T) {
	// Capture output using SetConsoleWriter
	var buf bytes.Buffer
	SetConsoleWriter(&buf)
	defer ResetConsole()

	PrintInfo("Info message")

	output := buf.String()

	if !strings.Contains(output, "Info message") {
		t.Errorf("PrintInfo output should contain message, got: %s", output)
	}
}
