package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	f, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if f != nil {
		t.Fatalf("missing file should return nil File, got %+v", f)
	}
}

func TestLoad_Malformed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "last.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestLoad_VersionSkew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "last.json")
	if err := os.WriteFile(p, []byte(`{"version":99,"runs":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(p)
	if !errors.Is(err, ErrVersionSkew) {
		t.Fatalf("want ErrVersionSkew, got %v", err)
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "sub", "last.json")
	in := &File{
		Version: SchemaVersion,
		Runs: []Run{
			{
				GeneratedAt: time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC),
				DataHash:    "abc123",
				Findings:    map[string]Severity{"fp1": SevError, "fp2": SevWarning},
			},
		},
	}
	if err := Save(p, in); err != nil {
		t.Fatalf("save: %v", err)
	}
	out, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if out.Version != in.Version || len(out.Runs) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", out)
	}
	if out.Runs[0].Findings["fp1"] != SevError {
		t.Fatalf("severity mismatch: %v", out.Runs[0].Findings)
	}
}

func TestSave_NoTmpLeak(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "last.json")
	if err := Save(p, &File{Version: SchemaVersion}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "last.json" {
			t.Fatalf("unexpected leftover file: %s", e.Name())
		}
	}
}

func TestReset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "last.json")
	if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Reset(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected file gone")
	}
	// idempotent
	if err := Reset(p); err != nil {
		t.Fatalf("reset of missing file should not error: %v", err)
	}
}

func TestAppend_TrimsToMaxHistory(t *testing.T) {
	t.Parallel()
	prev := &File{Version: SchemaVersion}
	for i := 0; i < MaxHistory+5; i++ {
		prev = Append(prev, Run{Findings: map[string]Severity{}})
	}
	if len(prev.Runs) != MaxHistory {
		t.Fatalf("want %d runs, got %d", MaxHistory, len(prev.Runs))
	}
}

func TestAppend_NewestFirst(t *testing.T) {
	t.Parallel()
	t1 := Run{GeneratedAt: time.Unix(1, 0), Findings: map[string]Severity{"a": SevError}}
	t2 := Run{GeneratedAt: time.Unix(2, 0), Findings: map[string]Severity{"b": SevError}}
	f := Append(nil, t1)
	f = Append(f, t2)
	if f.Runs[0].GeneratedAt != t2.GeneratedAt {
		t.Fatalf("newest should be at index 0")
	}
}
