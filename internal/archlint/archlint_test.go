package archlint

import "testing"

func TestParse_Clean(t *testing.T) {
	input := []byte(`{
		"Type": "models.Check",
		"Payload": {
			"ArchHasWarnings": false,
			"ArchWarningsDeps": [],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"OmittedCount": 0,
			"Qualities": [
				{"ID": "component_imports", "Used": true},
				{"ID": "deepscan", "Used": true}
			]
		}
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.HasWarnings {
		t.Error("expected no warnings")
	}
	if len(result.Violations) != 0 {
		t.Errorf("got %d violations, want 0", len(result.Violations))
	}
	if len(result.Checks) != 2 {
		t.Errorf("got %d checks, want 2", len(result.Checks))
	}
}

func TestParse_WithViolation(t *testing.T) {
	input := []byte(`{
		"Type": "models.Check",
		"Payload": {
			"ArchHasWarnings": true,
			"ArchWarningsDeps": [
				{
					"ComponentName": "agentSupervisor",
					"FileRelativePath": "/internal/agent/supervisor/supervisor.go",
					"FileAbsolutePath": "/home/user/project/internal/agent/supervisor/supervisor.go",
					"ResolvedImportName": "github.com/example/project/internal/agent/shell",
					"Reference": {
						"Valid": true,
						"File": "/home/user/project/internal/agent/supervisor/supervisor.go",
						"Line": 15,
						"Offset": 2
					}
				}
			],
			"ArchWarningsNotMatched": [],
			"ArchWarningsDeepScan": [],
			"Qualities": [{"ID": "component_imports", "Used": true}]
		}
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasWarnings {
		t.Error("expected warnings")
	}
	if len(result.Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(result.Violations))
	}
	v := result.Violations[0]
	if v.From != "agentSupervisor" {
		t.Errorf("From = %q, want %q", v.From, "agentSupervisor")
	}
	if v.To != "github.com/example/project/internal/agent/shell" {
		t.Errorf("To = %q, want full import path", v.To)
	}
	if v.FileFrom != "/internal/agent/supervisor/supervisor.go" {
		t.Errorf("FileFrom = %q", v.FileFrom)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
