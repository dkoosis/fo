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
					"ComponentA": {"Name": "store"},
					"ComponentB": {"Name": "eval"},
					"FileA": "internal/store/store.go",
					"FileB": "internal/eval/eval.go"
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
	if result.Violations[0].From != "store" || result.Violations[0].To != "eval" {
		t.Errorf("violation = %s → %s", result.Violations[0].From, result.Violations[0].To)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
