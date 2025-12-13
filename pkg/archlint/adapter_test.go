package archlint

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestParseCleanResult(t *testing.T) {
	input := `{
		"Type": "models.Check",
		"Payload": {
			"ExecutionWarnings": [],
			"ArchHasWarnings": false,
			"ArchWarningsDeps": [],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"ModuleName": "github.com/example/project",
			"Qualities": [
				{"ID": "component_imports", "Used": true}
			]
		}
	}`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.Payload.ArchHasWarnings {
		t.Error("Expected ArchHasWarnings to be false")
	}

	if result.Payload.ModuleName != "github.com/example/project" {
		t.Errorf("Expected module name 'github.com/example/project', got '%s'", result.Payload.ModuleName)
	}

	stats := ComputeStats(result)
	if stats.TotalViolations != 0 {
		t.Errorf("Expected 0 violations, got %d", stats.TotalViolations)
	}
}

func TestParseWithViolations(t *testing.T) {
	input := `{
		"Type": "models.Check",
		"Payload": {
			"ExecutionWarnings": [],
			"ArchHasWarnings": true,
			"ArchWarningsDeps": [
				{
					"ComponentFrom": "domain",
					"ComponentTo": "infra",
					"FileFrom": "internal/domain/service.go",
					"FileTo": "internal/infra/db.go",
					"Reference": {"Line": 15, "Column": 2, "Name": "db.Connect"}
				},
				{
					"ComponentFrom": "domain",
					"ComponentTo": "infra",
					"FileFrom": "internal/domain/handler.go",
					"FileTo": "internal/infra/cache.go",
					"Reference": {"Line": 42, "Column": 5, "Name": "cache.Get"}
				}
			],
			"ArchWarningsNotMatched": ["scripts/helper.go"],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"ModuleName": "github.com/example/project",
			"Qualities": []
		}
	}`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	stats := ComputeStats(result)
	if stats.TotalViolations != 3 {
		t.Errorf("Expected 3 violations, got %d", stats.TotalViolations)
	}
	if stats.DepViolations != 2 {
		t.Errorf("Expected 2 dep violations, got %d", stats.DepViolations)
	}
	if stats.NotMatchedFiles != 1 {
		t.Errorf("Expected 1 unmatched file, got %d", stats.NotMatchedFiles)
	}
	if stats.ByComponent["domain"] != 2 {
		t.Errorf("Expected 2 violations from 'domain', got %d", stats.ByComponent["domain"])
	}
}

func TestRenderCleanResult(t *testing.T) {
	input := `{
		"Type": "models.Check",
		"Payload": {
			"ExecutionWarnings": [],
			"ArchHasWarnings": false,
			"ArchWarningsDeps": [],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"ModuleName": "github.com/example/project",
			"Qualities": []
		}
	}`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	output := adapter.Render(result)
	if !strings.Contains(output, "OK") {
		t.Error("Expected 'OK' in output for clean result")
	}
	if !strings.Contains(output, "Architecture Check") {
		t.Error("Expected 'Architecture Check' in output")
	}
}

func TestRenderWithViolations(t *testing.T) {
	input := `{
		"Type": "models.Check",
		"Payload": {
			"ExecutionWarnings": [],
			"ArchHasWarnings": true,
			"ArchWarningsDeps": [
				{
					"ComponentFrom": "domain",
					"ComponentTo": "infra",
					"FileFrom": "internal/domain/service.go",
					"FileTo": "internal/infra/db.go",
					"Reference": {"Line": 15, "Column": 2, "Name": "db.Connect"}
				}
			],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"ModuleName": "github.com/example/project",
			"Qualities": []
		}
	}`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseBytes([]byte(input))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	output := adapter.Render(result)
	if !strings.Contains(output, "FAIL") {
		t.Error("Expected 'FAIL' in output for violations")
	}
	if !strings.Contains(output, "domain") {
		t.Error("Expected component name 'domain' in output")
	}
	if !strings.Contains(output, "infra") {
		t.Error("Expected target component 'infra' in output")
	}
}

func TestIsArchLintJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid arch-lint JSON",
			input:    `{"Type": "models.Check", "Payload": {"ArchHasWarnings": false}}`,
			expected: true,
		},
		{
			name:     "valid with warnings",
			input:    `{"Type": "models.Check", "Payload": {"ArchWarningsDeps": []}}`,
			expected: true,
		},
		{
			name:     "SARIF format",
			input:    `{"$schema": "https://sarif.schema", "runs": []}`,
			expected: false,
		},
		{
			name:     "go test JSON",
			input:    `{"Action": "run", "Package": "foo"}`,
			expected: false,
		},
		{
			name:     "empty",
			input:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsArchLintJSON([]byte(tt.input))
			if result != tt.expected {
				t.Errorf("IsArchLintJSON(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestQuickRender(t *testing.T) {
	input := `{
		"Type": "models.Check",
		"Payload": {
			"ExecutionWarnings": [],
			"ArchHasWarnings": false,
			"ArchWarningsDeps": [],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"ModuleName": "test",
			"Qualities": []
		}
	}`

	output, err := QuickRender([]byte(input))
	if err != nil {
		t.Fatalf("QuickRender failed: %v", err)
	}
	if output == "" {
		t.Error("Expected non-empty output")
	}
}
