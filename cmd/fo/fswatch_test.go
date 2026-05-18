package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldIgnoreDir(t *testing.T) {
	cases := map[string]bool{
		"":             false,
		".":            false,
		"src":          false,
		"pkg":          false,
		"vendor":       true,
		"node_modules": true,
		".git":         true,
		".fo":          true,
		".worktrees":   true,
		".idea":        true, // hidden by prefix
		"dist":         true,
	}
	for name, want := range cases {
		if got := shouldIgnoreDir(name); got != want {
			t.Errorf("shouldIgnoreDir(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMatchGitignore(t *testing.T) {
	pats := []string{"*.log", "build/", "/dist", "tmp"}
	cases := map[string]bool{
		"foo.log":     true,
		"a/b/foo.log": true,
		"build":       true,
		"build/x":     true,
		"dist":        true,
		"tmp":         true,
		"x/tmp/y":     true,
		"foo.go":      false,
		"src/main.go": false,
	}
	for rel, want := range cases {
		if got := matchGitignore(rel, pats); got != want {
			t.Errorf("matchGitignore(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestLoadGitignorePatterns(t *testing.T) {
	dir := t.TempDir()
	contents := "# comment\n\n*.log\nbuild/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadGitignorePatterns(dir)
	want := []string{"*.log", "build/"}
	if len(got) != len(want) {
		t.Fatalf("loadGitignorePatterns: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("pattern %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDebounce_CoalescesBurst(t *testing.T) {
	in := make(chan struct{}, 8)
	for range 5 {
		in <- struct{}{}
	}
	close(in)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := debounce(ctx, in, 30*time.Millisecond)

	count := 0
	for range out {
		count++
	}
	if count != 1 {
		t.Fatalf("debounce: 5 burst events should coalesce to 1 emission, got %d", count)
	}
}

func TestDebounce_SeparateBurstsEmitSeparately(t *testing.T) {
	in := make(chan struct{}, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out := debounce(ctx, in, 30*time.Millisecond)

	in <- struct{}{}
	in <- struct{}{}
	// First burst settles.
	select {
	case <-out:
	case <-time.After(time.Second):
		t.Fatal("first burst never emitted")
	}

	in <- struct{}{}
	select {
	case <-out:
	case <-time.After(time.Second):
		t.Fatal("second burst never emitted")
	}
	close(in)
}

func TestWatchTree_DetectsFileWrite(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	events, err := watchTree(ctx, dir)
	if err != nil {
		t.Fatalf("watchTree: %v", err)
	}

	// Give the watcher a moment to start.
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("watchTree: never observed file-write event")
	}
}

func TestWatchTree_IgnoresVendorDir(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "vendor")
	if err := os.Mkdir(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	events, err := watchTree(ctx, dir)
	if err != nil {
		t.Fatalf("watchTree: %v", err)
	}

	// Positive control: write a non-vendor file first and wait for the
	// event to confirm the watcher is wired up. Without this, the
	// negative assertion below could pass for the wrong reason (slow
	// fsnotify start-up rather than actual filtering). fo-u60.
	if err := os.WriteFile(filepath.Join(dir, "alive.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("watchTree: control event never arrived — watcher not alive")
	}

	// Drain any follow-up events from the control write.
	for draining := true; draining; {
		select {
		case <-events:
		case <-time.After(50 * time.Millisecond):
			draining = false
		}
	}

	if err := os.WriteFile(filepath.Join(vendor, "ignored.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("watchTree: should not emit for files under vendor/")
	case <-time.After(400 * time.Millisecond):
		// Expected: no event.
	}
}

func TestParseWatchArgs_FlagsBeforeSeparator(t *testing.T) {
	cmd, opts, err := parseWatchArgsWithOpts([]string{"-debounce=200ms", "-source=" + sourceStdin, "--", "go", testArg})
	if err != nil {
		t.Fatalf("parseWatchArgsWithOpts: %v", err)
	}
	if !equalSlice(cmd, []string{"go", testArg}) {
		t.Errorf("cmd: got %v", cmd)
	}
	if opts.debounce != 200*time.Millisecond {
		t.Errorf("debounce: got %v", opts.debounce)
	}
	if opts.source != sourceStdin {
		t.Errorf("source: got %q", opts.source)
	}
}

func TestParseWatchArgs_RejectsUnknownSource(t *testing.T) {
	if _, _, err := parseWatchArgsWithOpts([]string{"-source=bogus", "--", "true"}); err == nil {
		t.Fatal("want error for invalid source")
	}
}
