package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFullLog_WritesCompleteInput(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())

	data := []byte("the complete unfiltered original\nline two\n")
	path, err := SaveFullLog(data)
	if err != nil {
		t.Fatalf("SaveFullLog: %v", err)
	}
	if path != FullLogPath() {
		t.Fatalf("path = %q, want %q", path, FullLogPath())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("log content = %q, want %q", got, data)
	}
}

func TestSaveFullLog_OverwritesPriorLog(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())

	if _, err := SaveFullLog([]byte("first run")); err != nil {
		t.Fatalf("SaveFullLog (first): %v", err)
	}
	path, err := SaveFullLog([]byte("second run"))
	if err != nil {
		t.Fatalf("SaveFullLog (second): %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second run" {
		t.Fatalf("log content = %q, want single most-recent run only", got)
	}
}

func TestSaveFullLog_MkdirFailureReturnsErr(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FO_STATE_DIR", filepath.Join(blocker, "sub"))

	if _, err := SaveFullLog([]byte("data")); err == nil {
		t.Fatalf("expected error when state dir cannot be created")
	}
}
