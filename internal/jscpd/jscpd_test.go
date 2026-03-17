package jscpd

import "testing"

func TestParse_NoClones(t *testing.T) {
	input := []byte(`{"duplicates": [], "statistics": {}}`)
	clones, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(clones) != 0 {
		t.Errorf("got %d clones, want 0", len(clones))
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
	clones, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(clones) != 1 {
		t.Fatalf("got %d clones, want 1", len(clones))
	}
	c := clones[0]
	if c.Format != "go" {
		t.Errorf("format = %q, want %q", c.Format, "go")
	}
	if c.Lines != 20 {
		t.Errorf("lines = %d, want 20", c.Lines)
	}
	if c.FileA != "a.go" || c.StartA != 10 || c.EndA != 30 {
		t.Errorf("first = %s:%d-%d, want a.go:10-30", c.FileA, c.StartA, c.EndA)
	}
	if c.FileB != "b.go" || c.StartB != 5 || c.EndB != 25 {
		t.Errorf("second = %s:%d-%d, want b.go:5-25", c.FileB, c.StartB, c.EndB)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}
