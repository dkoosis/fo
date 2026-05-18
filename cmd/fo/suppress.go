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
	rs := loadSuppressRuleset(r, path, stderr)
	if rs == nil {
		return
	}
	report.ApplyFilter(r, rs, time.Now())
}

// loadSuppressRuleset reads path and parses it into a Ruleset. Returns
// nil if the file is absent, empty, or fails to load/parse (with a
// Notice appended to r and a line to stderr). Suitable for streaming
// callers that want to load once and reuse the ruleset across snapshots
// (fo-2sk).
func loadSuppressRuleset(r *report.Report, path string, stderr io.Writer) *suppress.Ruleset {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		fmt.Fprintf(stderr, "fo: suppress: %v\n", err)
		if r != nil {
			r.Notices = append(r.Notices,
				fmt.Sprintf("suppress: load failed (%v) — .fo/ignore not applied", err))
		}
		return nil
	}
	defer f.Close()

	rules, err := suppress.Parse(f)
	if err != nil {
		// Partial parse: surface the errors but still apply any rules
		// that did parse — one stale line shouldn't silently disable
		// the rest of the file.
		fmt.Fprintf(stderr, "fo: suppress: %v\n", err)
		if r != nil {
			msg := fmt.Sprintf("suppress: parse errors (%v)", err)
			if len(rules) > 0 {
				msg += fmt.Sprintf(" — %d valid rule(s) applied", len(rules))
			} else {
				msg += " — .fo/ignore not applied"
			}
			r.Notices = append(r.Notices, msg)
		}
		if len(rules) == 0 {
			return nil
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return suppress.NewRuleset(rules)
}
