package fo

import (
	"encoding/json"
	"os/exec"
	"testing"
	"time"
)

func TestTaskResult_ToJSON(t *testing.T) {
	result := &TaskResult{
		Label:    "Test Command",
		Intent:   "testing",
		Status:   "success",
		ExitCode: 0,
		Duration: 1234 * time.Millisecond,
		Lines: []Line{
			{Content: "test output", Type: "detail", Timestamp: time.Now()},
			{Content: "error message", Type: "error", Timestamp: time.Now()},
		},
	}

	jsonOutput, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &parsed); err != nil {
		t.Fatalf("ToJSON() output is not valid JSON: %v", err)
	}

	// Verify required fields
	if parsed["version"] != "1.0" {
		t.Errorf("version = %v, want 1.0", parsed["version"])
	}
	if parsed["label"] != "Test Command" {
		t.Errorf("label = %v, want 'Test Command'", parsed["label"])
	}
	exitCode, ok := parsed["exit_code"].(float64)
	if !ok {
		t.Fatalf("exit_code is not a float64, got %T", parsed["exit_code"])
	}
	if exitCode != 0 {
		t.Errorf("exit_code = %v, want 0", exitCode)
	}
	if parsed["status"] != "success" {
		t.Errorf("status = %v, want 'success'", parsed["status"])
	}

	// Verify lines array
	lines, ok := parsed["lines"].([]interface{})
	if !ok {
		t.Fatal("lines is not an array")
	}
	if len(lines) != 2 {
		t.Errorf("lines length = %d, want 2", len(lines))
	}
}

func TestTaskResult_ToJSON_WithError(t *testing.T) {
	result := &TaskResult{
		Label:    "Failed Command",
		Status:   "error",
		ExitCode: 1,
		Duration: 500 * time.Millisecond,
		Err:      &exec.ExitError{},
	}

	jsonOutput, err := result.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonOutput, &parsed); err != nil {
		t.Fatalf("ToJSON() output is not valid JSON: %v", err)
	}

	if parsed["error"] == nil {
		t.Error("error field should be present when Err is set")
	}
}
