package sarif

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/design"
)

func TestOrchestratorRun(t *testing.T) {
	// Skip if golangci-lint not available
	if _, err := os.Stat("/opt/homebrew/bin/golangci-lint"); os.IsNotExist(err) {
		t.Skip("golangci-lint not found")
	}

	config := DefaultRendererConfig()
	config.Tools["golangci-lint"] = GolangciLintConfig()

	foConfig := design.DefaultConfig()

	orch := NewOrchestrator(config, foConfig)
	orch.SetBuildDir("../../build")

	var buf bytes.Buffer
	orch.SetWriter(&buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tools := []ToolSpec{
		{
			Name:      "golangci-lint",
			Command:   "golangci-lint",
			Args:      []string{"run", "./..."},
			SARIFPath: "../../build/golangci.sarif",
		},
	}

	results, err := orch.Run(ctx, tools)
	if err != nil {
		t.Fatalf("orchestrator run failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	t.Logf("Output:\n%s", buf.String())
	t.Logf("Duration: %v", results[0].Duration)

	if results[0].Document != nil {
		stats := ComputeStats(results[0].Document)
		t.Logf("Issues: %d", stats.TotalIssues)
	}
}

func TestMultiSpinner(t *testing.T) {
	// Quick visual test - just verify it doesn't panic
	config := DefaultRendererConfig()
	foConfig := design.DefaultConfig()

	orch := NewOrchestrator(config, foConfig)

	var buf bytes.Buffer
	orch.SetWriter(&buf)

	tools := []ToolSpec{
		{Name: "tool-1", Command: "echo", Args: []string{"ok"}},
		{Name: "tool-2", Command: "echo", Args: []string{"ok"}},
	}

	ms := orch.newMultiSpinner(tools)

	// Quick start/stop cycle
	ms.Start()
	time.Sleep(200 * time.Millisecond)
	ms.SetStatus(0, "ok", 100*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	ms.SetStatus(1, "issues", 150*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	ms.Stop()

	// Just verify no panic
	t.Log("Multi-spinner test passed")
}
