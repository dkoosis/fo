package main

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// defaultIgnoreDirs are directory basenames skipped by the recursive watcher.
// Match the search-scope policy in CLAUDE.md.
var defaultIgnoreDirs = []string{
	"vendor", "node_modules", ".git", ".fo", ".worktrees",
	"build", "dist", ".trash",
}

// shouldIgnoreDir reports whether a directory basename should be skipped
// during the recursive walk. Hidden dirs (starting with '.') other than
// '.' itself are also skipped.
func shouldIgnoreDir(name string) bool {
	if name == "" || name == "." {
		return false
	}
	if slices.Contains(defaultIgnoreDirs, name) {
		return true
	}
	return strings.HasPrefix(name, ".")
}

// loadGitignorePatterns reads .gitignore at root and returns trimmed,
// non-comment, non-empty patterns. Best-effort: a missing file yields nil.
func loadGitignorePatterns(root string) []string {
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var pats []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		pats = append(pats, line)
	}
	return pats
}

// matchOnePattern reports whether rel matches a single gitignore pattern.
func matchOnePattern(rel, base, p string) bool {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return false
	}
	if ok, _ := filepath.Match(p, base); ok {
		return true
	}
	if ok, _ := filepath.Match(p, rel); ok {
		return true
	}
	if strings.ContainsAny(p, "*?[") {
		return false
	}
	for part := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		if part == p {
			return true
		}
	}
	return false
}

// matchGitignore reports whether path (relative to gitignore root) matches
// any pattern. Supports basename and full-path glob match — not the full
// gitignore spec, but enough for typical build-output entries.
func matchGitignore(rel string, patterns []string) bool {
	if rel == "" {
		return false
	}
	base := filepath.Base(rel)
	for _, p := range patterns {
		if matchOnePattern(rel, base, p) {
			return true
		}
	}
	return false
}

// addRecursive walks root and adds every non-ignored directory to w.
func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip unreadable subtrees rather than abort
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && shouldIgnoreDir(d.Name()) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

// handleFsEvent processes a single fsnotify event: extends the watcher when
// new directories are created, then reports whether the event is relevant.
func handleFsEvent(w *fsnotify.Watcher, ev fsnotify.Event, root string, gitignorePats []string) bool {
	if !relevant(ev, root, gitignorePats) {
		return false
	}
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			_ = addRecursive(w, ev.Name)
		}
	}
	return true
}

// watchTree starts a recursive fsnotify watcher rooted at root and emits a
// signal on the returned channel for every relevant change. The channel is
// closed when ctx is canceled. Errors only fire on initial setup; runtime
// errors (e.g. removed dirs) are swallowed.
//
// Filtering: defaultIgnoreDirs by basename, hidden dirs by prefix, and
// .gitignore patterns at root. New directories created during the run are
// added to the watcher unless ignored.
func watchTree(ctx context.Context, root string) (<-chan struct{}, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addRecursive(w, root); err != nil {
		_ = w.Close()
		return nil, err
	}
	ignorePats := loadGitignorePatterns(root)

	out := make(chan struct{})
	go runWatcher(ctx, w, root, ignorePats, out)
	return out, nil
}

// runWatcher is the goroutine body for watchTree.
func runWatcher(ctx context.Context, w *fsnotify.Watcher, root string, ignorePats []string, out chan<- struct{}) {
	defer close(out)
	defer func() { _ = w.Close() }()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !handleFsEvent(w, ev, root, ignorePats) {
				continue
			}
			select {
			case out <- struct{}{}:
			case <-ctx.Done():
				return
			}
		case _, ok := <-w.Errors:
			if !ok {
				return
			}
		}
	}
}

// relevant reports whether an fsnotify event should trigger a rerun.
func relevant(ev fsnotify.Event, root string, gitignorePats []string) bool {
	if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	base := filepath.Base(ev.Name)
	if shouldIgnoreDir(base) {
		return false
	}
	if strings.HasSuffix(base, "~") || strings.HasPrefix(base, ".#") {
		return false
	}
	rel, err := filepath.Rel(root, ev.Name)
	if err == nil && matchGitignore(rel, gitignorePats) {
		return false
	}
	if rel == "" {
		return true
	}
	if slices.ContainsFunc(strings.Split(filepath.ToSlash(rel), "/"), shouldIgnoreDir) {
		return false
	}
	return true
}

// debounce coalesces bursts on in into a single emission on the returned
// channel after d of quiet. The output channel closes when in closes or
// ctx is canceled.
func debounce(ctx context.Context, in <-chan struct{}, d time.Duration) <-chan struct{} {
	out := make(chan struct{})
	go runDebounce(ctx, in, d, out)
	return out
}

// runDebounce is the goroutine body for debounce.
func runDebounce(ctx context.Context, in <-chan struct{}, d time.Duration, out chan<- struct{}) {
	defer close(out)
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-in:
			if !ok {
				flushPending(ctx, timer, out)
				return
			}
			timer, timerC = resetTimer(timer, timerC, d)
		case <-timerC:
			timer = nil
			timerC = nil
			select {
			case out <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// resetTimer creates or restarts the debounce timer.
func resetTimer(timer *time.Timer, timerC <-chan time.Time, d time.Duration) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(d)
		return timer, timer.C
	}
	if !timer.Stop() {
		<-timerC
	}
	timer.Reset(d)
	return timer, timerC
}

// flushPending emits a final signal if a timer was armed when the input closed.
func flushPending(ctx context.Context, timer *time.Timer, out chan<- struct{}) {
	if timer == nil {
		return
	}
	select {
	case out <- struct{}{}:
	case <-ctx.Done():
	}
}
