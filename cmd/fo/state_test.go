package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestStateReset_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "last-run.json")
	if err := os.WriteFile(p, []byte(`{"version":1,"runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"state", "reset", "--state-file", p}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("state file should be gone, err=%v", err)
	}
}

func TestStateReset_MissingFileIsOK(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "absent.json")

	var stdout, stderr bytes.Buffer
	code := run([]string{"state", "reset", "--state-file", p}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, stderr.String())
	}
}

func TestState_RequiresSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"state"}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("want exit 2, got %d", code)
	}
}
