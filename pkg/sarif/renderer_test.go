package sarif

import (
	"strings"
	"testing"
)

func TestQuickRender(t *testing.T) {
	// Use the test fixture with 121 issues
	output, err := QuickRender("testdata/golangci-lint-121-issues.sarif")
	if err != nil {
		t.Fatalf("failed to render test fixture: %v", err)
	}

	if output == "" {
		t.Error("expected non-empty output")
	}

	// Verify key elements are present
	if !strings.Contains(output, "121 issues") && !strings.Contains(output, "Analysis:") {
		t.Error("expected summary with issue count")
	}

	t.Logf("Rendered output:\n%s", output)
}

func TestComputeStats(t *testing.T) {
	doc := &Document{
		Version: "2.1.0",
		Runs: []Run{
			{
				Tool: Tool{Driver: Driver{Name: "test-tool"}},
				Results: []Result{
					{RuleID: "rule1", Level: "error", Message: Message{Text: "msg1"}},
					{RuleID: "rule1", Level: "error", Message: Message{Text: "msg2"}},
					{RuleID: "rule2", Level: "warning", Message: Message{Text: "msg3"}},
				},
			},
		},
	}

	stats := ComputeStats(doc)

	if stats.TotalIssues != 3 {
		t.Errorf("expected 3 total issues, got %d", stats.TotalIssues)
	}
	if stats.ByLevel["error"] != 2 {
		t.Errorf("expected 2 errors, got %d", stats.ByLevel["error"])
	}
	if stats.ByLevel["warning"] != 1 {
		t.Errorf("expected 1 warning, got %d", stats.ByLevel["warning"])
	}
	if stats.ByRule["rule1"] != 2 {
		t.Errorf("expected 2 rule1 issues, got %d", stats.ByRule["rule1"])
	}
}

func TestMapperSummary(t *testing.T) {
	doc := &Document{
		Version: "2.1.0",
		Runs: []Run{
			{
				Tool: Tool{Driver: Driver{Name: "golangci-lint"}},
				Results: []Result{
					{RuleID: "errcheck", Level: "error", Message: Message{Text: "unchecked error"}},
					{RuleID: "goconst", Level: "warning", Message: Message{Text: "magic string"}},
					{RuleID: "goconst", Level: "warning", Message: Message{Text: "magic string 2"}},
				},
			},
		},
	}

	config := GolangciLintConfig()
	mapper := NewMapper(config)
	patterns := mapper.MapToPatterns(doc)

	if len(patterns) == 0 {
		t.Fatal("expected at least one pattern")
	}

	t.Logf("Generated %d patterns", len(patterns))
}
