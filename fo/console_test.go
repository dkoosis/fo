package fo

import (
	"bytes"
	"testing"
)

func TestNewConsole_DefaultConfig(t *testing.T) {
	c := NewConsole(ConsoleConfig{})
	if c == nil {
		t.Fatal("NewConsole returned nil")
		return
	}
	if c.cfg.ShowOutputMode != "on-fail" {
		t.Errorf("expected ShowOutputMode 'on-fail', got '%s'", c.cfg.ShowOutputMode)
	}
	if c.cfg.MaxBufferSize != 10*1024*1024 {
		t.Errorf("expected MaxBufferSize 10MB, got %d", c.cfg.MaxBufferSize)
	}
	if c.cfg.MaxLineLength != 1*1024*1024 {
		t.Errorf("expected MaxLineLength 1MB, got %d", c.cfg.MaxLineLength)
	}
	if c.cfg.Out == nil {
		t.Error("expected Out to be set to default")
	}
	if c.cfg.Err == nil {
		t.Error("expected Err to be set to default")
	}
}

func TestNewConsole_CustomWriters(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out: &out,
		Err: &errOut,
	})
	if c.cfg.Out != &out {
		t.Error("expected custom Out writer to be preserved")
	}
	if c.cfg.Err != &errOut {
		t.Error("expected custom Err writer to be preserved")
	}
}

func TestConsole_RunSimple_Success(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:        &out,
		Err:        &errOut,
		Monochrome: true, // Disable colors for easier testing
	})

	err := c.RunSimple("echo", "hello")
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
}

func TestConsole_RunSimple_CommandNotFound(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:        &out,
		Err:        &errOut,
		Monochrome: true,
	})

	err := c.RunSimple("nonexistent_command_12345")
	if err == nil {
		t.Error("expected error for nonexistent command")
	}
}

func TestConsole_RunSimple_NonZeroExit(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:        &out,
		Err:        &errOut,
		Monochrome: true,
	})

	err := c.RunSimple("sh", "-c", "exit 1")
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
	// RunSimple returns ErrNonZeroExit when result has non-zero exit code
	// but the underlying cmd error takes precedence
	// Check that we get an error (either exec.ExitError or ErrNonZeroExit)
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
}

func TestConsole_Run_Success(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:        &out,
		Err:        &errOut,
		Monochrome: true,
	})

	result, err := c.Run("test", "echo", "hello")
	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
		return
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Label != "test" {
		t.Errorf("expected label 'test', got '%s'", result.Label)
	}
}

func TestConsole_Run_FailedCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:        &out,
		Err:        &errOut,
		Monochrome: true,
	})

	result, err := c.Run("test", "sh", "-c", "exit 42")
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
	if result == nil {
		t.Fatal("expected result even on failure")
		return
	}
	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestConsole_Run_StreamMode(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:              &out,
		Err:              &errOut,
		Monochrome:       true,
		LiveStreamOutput: true,
	})

	result, err := c.Run("stream-test", "echo", "streaming")
	if err != nil {
		t.Errorf("expected success in stream mode, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestConsole_Run_CaptureMode(t *testing.T) {
	var out, errOut bytes.Buffer
	c := NewConsole(ConsoleConfig{
		Out:              &out,
		Err:              &errOut,
		Monochrome:       true,
		LiveStreamOutput: false,
		ShowOutputMode:   "always",
	})

	result, err := c.Run("capture-test", "echo", "captured")
	if err != nil {
		t.Errorf("expected success in capture mode, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if len(result.Lines) == 0 {
		t.Error("expected captured lines in result")
	}
}

func TestDefaultConsole(t *testing.T) {
	c := DefaultConsole()
	if c == nil {
		t.Fatal("DefaultConsole returned nil")
	}
}

func TestLine_Type(t *testing.T) {
	line := Line{
		Content: "test content",
		Type:    "error",
	}
	if line.Content != "test content" {
		t.Error("Line content mismatch")
	}
	if line.Type != "error" {
		t.Error("Line type mismatch")
	}
}

func TestTaskResult_Fields(t *testing.T) {
	result := TaskResult{
		Label:    "test",
		Intent:   "testing",
		Status:   "success",
		ExitCode: 0,
		Lines:    []Line{{Content: "line1"}},
	}
	if result.Label != "test" {
		t.Error("Label mismatch")
	}
	if result.ExitCode != 0 {
		t.Error("ExitCode mismatch")
	}
	if len(result.Lines) != 1 {
		t.Error("Lines count mismatch")
	}
}
