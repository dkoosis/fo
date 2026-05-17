package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuppress_AddCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".fo", "ignore")
	t.Setenv("FO_IGNORE", path)

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"add", "SA1019", "--until=2026-12-31", "--reason=upstream"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := "SA1019 until=2026-12-31 reason=upstream\n"
	if string(got) != want {
		t.Fatalf("file=%q want=%q", got, want)
	}
	if !strings.Contains(out.String(), "SA1019") {
		t.Fatalf("stdout should echo added rule: %q", out.String())
	}
}

func TestSuppress_AddAppendsAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	if err := os.WriteFile(path, []byte("# header\nG115 glob=internal/legacy/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FO_IGNORE", path)

	var out, errOut bytes.Buffer
	if code := runSuppress([]string{"add", "SA1019"}, &out, &errOut); code != 0 {
		t.Fatalf("first add exit=%d stderr=%q", code, errOut.String())
	}
	// Adding the same rule again should be idempotent.
	out.Reset()
	errOut.Reset()
	if code := runSuppress([]string{"add", "SA1019"}, &out, &errOut); code != 0 {
		t.Fatalf("second add exit=%d stderr=%q", code, errOut.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(got), "SA1019") != 1 {
		t.Fatalf("SA1019 should appear once, got: %q", got)
	}
	if !strings.Contains(string(got), "G115 glob=internal/legacy/**") {
		t.Fatalf("existing entries should be preserved: %q", got)
	}
}

func TestSuppress_List(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	if err := os.WriteFile(path, []byte("SA1019\nG115 glob=cmd/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FO_IGNORE", path)

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "SA1019") || !strings.Contains(s, "G115") {
		t.Fatalf("list output missing rules: %q", s)
	}
}

func TestSuppress_ListEmptyWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FO_IGNORE", filepath.Join(dir, "nope"))

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"list"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("absent file should exit 0, got %d stderr=%q", code, errOut.String())
	}
}

func TestSuppress_Remove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	if err := os.WriteFile(path, []byte("SA1019 until=2026-12-31\nG115 glob=cmd/**\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FO_IGNORE", path)

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"remove", "SA1019"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "SA1019") {
		t.Fatalf("SA1019 should be removed: %q", got)
	}
	if !strings.Contains(string(got), "G115") {
		t.Fatalf("G115 should remain: %q", got)
	}
}

func TestSuppress_RemoveMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ignore")
	if err := os.WriteFile(path, []byte("G115\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FO_IGNORE", path)

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"remove", "SA1019"}, &out, &errOut)
	if code == 0 {
		t.Fatalf("removing missing rule should exit non-zero")
	}
}

func TestSuppress_UnknownSubcommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runSuppress([]string{"frobnicate"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("unknown subcommand should exit 2, got %d", code)
	}
}

func TestSuppress_AddRejectsInvalidDate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FO_IGNORE", filepath.Join(dir, "ignore"))

	var out, errOut bytes.Buffer
	code := runSuppress([]string{"add", "SA1019", "--until=tomorrow"}, &out, &errOut)
	if code == 0 {
		t.Fatalf("invalid date should exit non-zero")
	}
}
