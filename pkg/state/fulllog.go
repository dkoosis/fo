package state

import (
	"errors"
	"io"
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
// path so the caller can point the reader at it — even under
// ErrDurabilityDegraded, where the rename already landed the data on
// disk and only the parent-dir fsync failed.
func SaveFullLog(data []byte) (string, error) {
	path := FullLogPath()
	err := writeAtomicTo(path, ".full.*.tmp", func(w io.Writer) error {
		_, werr := w.Write(data)
		return werr
	})
	if err != nil && !errors.Is(err, ErrDurabilityDegraded) {
		return "", err
	}
	return path, err
}
