package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWrapListText(t *testing.T) {
	stdout, _, err := executeCommand("wrap", "list")
	if err != nil {
		t.Fatalf("wrap list: %v", err)
	}
	for _, name := range wrapNames {
		if !strings.Contains(stdout, name) {
			t.Errorf("output missing wrapper %q: %s", name, stdout)
		}
	}
}

func TestWrapListJSON(t *testing.T) {
	stdout, _, err := executeCommand("wrap", "list", "--json")
	if err != nil {
		t.Fatalf("wrap list --json: %v", err)
	}
	var got []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(got) != len(wrapNames) {
		t.Fatalf("got %d entries, want %d", len(got), len(wrapNames))
	}
	for i, name := range wrapNames {
		if got[i].Name != name {
			t.Errorf("entry %d: name %q, want %q", i, got[i].Name, name)
		}
		if got[i].Description == "" {
			t.Errorf("entry %d: empty description", i)
		}
	}
}
