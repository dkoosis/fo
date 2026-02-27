package jscpd

import "testing"

func TestParse_NoClones(t *testing.T) {
	input := []byte(`{"duplicates": [], "statistics": {}}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Clones) != 0 {
		t.Errorf("got %d clones, want 0", len(result.Clones))
	}
}

func TestParse_WithClones(t *testing.T) {
	input := []byte(`{
		"duplicates": [
			{
				"format": "go",
				"lines": 20,
				"firstFile": {"name": "a.go", "start": 10, "end": 30,
					"startLoc": {"line": 10, "column": 1},
					"endLoc": {"line": 30, "column": 5}},
				"secondFile": {"name": "b.go", "start": 5, "end": 25,
					"startLoc": {"line": 5, "column": 1},
					"endLoc": {"line": 25, "column": 5}}
			}
		]
	}`)
	result, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Clones) != 1 {
		t.Fatalf("got %d clones, want 1", len(result.Clones))
	}
	c := result.Clones[0]
	if c.FileA != "a.go" || c.FileB != "b.go" {
		t.Errorf("files = %s, %s", c.FileA, c.FileB)
	}
	if c.Lines != 20 {
		t.Errorf("lines = %d, want 20", c.Lines)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
