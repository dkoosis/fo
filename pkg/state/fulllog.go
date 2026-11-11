package state

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
)

func (s stateStore) fullLogPath() string { return filepath.Join(s.dir, "full.log") }

// SaveFullLog durably writes data (the complete, unfiltered original
// input) to FullLogPath, overwriting any prior log. Returns the resolved
// path so the caller can point the reader at it — even under
// ErrDurabilityDegraded, where the rename already landed the data on
// disk and only the parent-dir fsync failed.
func SaveFullLog(data []byte) (string, error) {
	s := newStateStore()
	path := s.fullLogPath()
	err := s.writeAtomicTo(path, ".full.*.tmp", func(w io.Writer) error {
		if _, werr := w.Write(data); werr != nil {
			return fmt.Errorf("write full log: %w", werr)
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrDurabilityDegraded) {
		return "", err
	}
	return path, err
}
