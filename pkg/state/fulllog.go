package state

import (
	"fmt"
	"os"
	"path/filepath"
)

// FullLogPath returns the resolved path for the tee'd full-output log.
// Single most-recent file, matching last-run.json's "one snapshot" shape
// rather than run-log.json's append history — the log exists to make the
// immediately preceding run's unfiltered output one command away, not to
// build a history.
func FullLogPath() string { return filepath.Join(Dir(), "full.log") }

// SaveFullLog durably writes data (the complete, unfiltered original
// input) to FullLogPath, overwriting any prior log. Returns the resolved
// path so the caller can point the reader at it.
func SaveFullLog(data []byte) (string, error) {
	path := FullLogPath()
	if err := writeAtomicBytes(path, ".full.*.tmp", data); err != nil {
		return "", err
	}
	return path, nil
}

// writeAtomicBytes mirrors writeAtomic's durable-write shape (tempfile,
// fsync, rename, parent-dir sync) for raw bytes instead of JSON.
func writeAtomicBytes(path, tmpPattern string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("state: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, tmpPattern)
	if err != nil {
		return fmt.Errorf("state: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("state: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("state: fsync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("state: close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("state: rename: %w", err)
	}
	if err := syncDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("%w: %w", ErrDurabilityDegraded, err)
	}
	return nil
}
