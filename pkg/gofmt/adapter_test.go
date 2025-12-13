package gofmt

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
)

func TestParseEmpty(t *testing.T) {
	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString("")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(result.Files))
	}
}

func TestParseSingleFile(t *testing.T) {
	input := "internal/foo/bar.go\n"

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(result.Files))
	}
	if result.Files[0] != "internal/foo/bar.go" {
		t.Errorf("Expected 'internal/foo/bar.go', got '%s'", result.Files[0])
	}
}

func TestParseMultipleFiles(t *testing.T) {
	input := `internal/foo/bar.go
internal/baz/qux.go
cmd/main.go
`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(result.Files))
	}
}

func TestParseWithBlankLines(t *testing.T) {
	input := `
internal/foo/bar.go

internal/baz/qux.go

`

	adapter := NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Files) != 2 {
		t.Errorf("Expected 2 files (blank lines skipped), got %d", len(result.Files))
	}
}

func TestRenderClean(t *testing.T) {
	adapter := NewAdapter(design.DefaultConfig())
	result := &Result{Files: nil}

	output := adapter.Render(result)
	if !strings.Contains(output, "OK") {
		t.Error("Expected 'OK' in output for clean result")
	}
	if !strings.Contains(output, "All formatted") {
		t.Error("Expected 'All formatted' in output")
	}
}

func TestRenderWithFiles(t *testing.T) {
	adapter := NewAdapter(design.DefaultConfig())
	result := &Result{
		Files: []string{
			"internal/foo/bar.go",
			"internal/foo/baz.go",
			"cmd/main.go",
		},
	}

	output := adapter.Render(result)
	if !strings.Contains(output, "FAIL") {
		t.Error("Expected 'FAIL' in output when files need formatting")
	}
	if !strings.Contains(output, "3") {
		t.Error("Expected file count '3' in output")
	}
	if !strings.Contains(output, "bar.go") {
		t.Error("Expected filename in output")
	}
}

func TestIsGofmtOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "single go file",
			input:    "foo.go\n",
			expected: true,
		},
		{
			name:     "multiple go files",
			input:    "foo.go\nbar.go\ninternal/baz.go\n",
			expected: true,
		},
		{
			name:     "empty (all formatted)",
			input:    "",
			expected: false, // empty means all formatted, but we can't detect format
		},
		{
			name:     "non-go file",
			input:    "foo.txt\n",
			expected: false,
		},
		{
			name:     "mixed files",
			input:    "foo.go\nbar.txt\n",
			expected: false,
		},
		{
			name:     "JSON output",
			input:    `{"Action": "run"}`,
			expected: false,
		},
		{
			name:     "SARIF output",
			input:    `{"$schema": "sarif"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGofmtOutput([]byte(tt.input))
			if result != tt.expected {
				t.Errorf("IsGofmtOutput(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestQuickRender(t *testing.T) {
	output, err := QuickRender("foo.go\nbar.go\n")
	if err != nil {
		t.Fatalf("QuickRender failed: %v", err)
	}
	if output == "" {
		t.Error("Expected non-empty output")
	}
	if !strings.Contains(output, "FAIL") {
		t.Error("Expected FAIL status")
	}
}

func TestQuickRenderEmpty(t *testing.T) {
	output, err := QuickRender("")
	if err != nil {
		t.Fatalf("QuickRender failed: %v", err)
	}
	if !strings.Contains(output, "OK") {
		t.Error("Expected OK status for empty input")
	}
}

func TestMapToPatterns(t *testing.T) {
	// Clean result
	result := &Result{Files: nil}
	patterns := MapToPatterns(result)
	if len(patterns) != 1 {
		t.Errorf("Expected 1 pattern (summary only) for clean result, got %d", len(patterns))
	}

	// Result with files
	result = &Result{Files: []string{"foo.go", "bar.go"}}
	patterns = MapToPatterns(result)
	if len(patterns) != 2 {
		t.Errorf("Expected 2 patterns (summary + file list), got %d", len(patterns))
	}
}
