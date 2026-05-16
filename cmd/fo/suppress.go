package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/suppress"
)

// suppressPath returns the path to .fo/ignore for the current run. The
// FO_IGNORE env var overrides the default (cwd/.fo/ignore) for tests
// and non-standard layouts.
func suppressPath() string {
	if p := os.Getenv("FO_IGNORE"); p != "" {
		return p
	}
	return filepath.Join(".fo", "ignore")
}

// applySuppress loads .fo/ignore (if present) and runs report.ApplyFilter
// on r. Absent file is a no-op. Any other load/parse failure is recorded
// as a Notice; the run continues.
func applySuppress(r *report.Report, path string, stderr io.Writer) {
	if r == nil {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		fmt.Fprintf(stderr, "fo: suppress: %v\n", err)
		r.Notices = append(r.Notices,
			fmt.Sprintf("suppress: load failed (%v) — .fo/ignore not applied", err))
		return
	}
	defer f.Close()

	rules, err := suppress.Parse(f)
	if err != nil {
		fmt.Fprintf(stderr, "fo: suppress: %v\n", err)
		r.Notices = append(r.Notices,
			fmt.Sprintf("suppress: parse failed (%v) — .fo/ignore not applied", err))
		return
	}
	if len(rules) == 0 {
		return
	}
	report.ApplyFilter(r, suppress.NewRuleset(rules), time.Now())
}
