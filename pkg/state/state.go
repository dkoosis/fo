// Package state persists a sidecar record of prior fo runs and classifies
// the current run's findings against that history. The on-disk file is
// .fo/last-run.json by default, keyed by the stable fingerprint defined
// in pkg/fingerprint. History slots (≥2 prior runs) live in the schema
// from day one so that flake detection — which needs a resolved-then-new
// transition across two boundaries — does not force a later schema
// rewrite.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dkoosis/fo/pkg/report"
)

// SchemaVersion is the on-disk envelope version. Bumped only on a
// breaking shape change; readers reject unknown versions.
const SchemaVersion = 1

// MaxHistory bounds the number of prior runs kept in the file. Two is
// the minimum needed for flake detection (resolved at t-1, new at t,
// requires t-2 to confirm the prior occurrence). Three gives one slot
// of slack so a single transient run between two real states does not
// blow away the signal.
const MaxHistory = 3

// DefaultPath is the relative sidecar location used when --state-file
// is not supplied. Kept for compatibility with callers that record the
// resolved path; new code should prefer Path() so FO_STATE_DIR is honored.
const DefaultPath = ".fo/last-run.json"

// Dir returns the directory holding fo's sidecar files. Honors
// FO_STATE_DIR so test runs (and users with non-standard layouts) can
// redirect every sidecar — last-run.json, metrics-history.json, and
// any future addition — through one knob.
func Dir() string {
	if d := os.Getenv("FO_STATE_DIR"); d != "" {
		return d
	}
	return ".fo"
}

// Path returns the resolved last-run sidecar path.
func Path() string { return filepath.Join(Dir(), "last-run.json") }

// MetricsHistoryPath returns the resolved metrics-history sidecar path.
func MetricsHistoryPath() string { return filepath.Join(Dir(), "metrics-history.json") }

// File is the top-level on-disk envelope. Versioned so a future
// breaking change can refuse to read old files cleanly rather than
// silently corrupting state.
type File struct {
	Version int   `json:"version"`
	Runs    []Run `json:"runs"`
}

// Run is one persisted prior run. Findings is a fingerprint-keyed map
// from fingerprint to severity — the only fields the diff classifier
// needs. Storing the full Finding would bloat the sidecar without
// adding signal the classifier consumes.
type Run struct {
	GeneratedAt time.Time           `json:"generated_at"`
	DataHash    string              `json:"data_hash,omitempty"`
	Findings    map[string]Severity `json:"findings"`
}

// Severity is the persisted severity, copied from report.Severity. We
// keep our own alias so the sidecar shape does not move when the
// substrate adds severity levels.
type Severity string

const (
	SevError   Severity = "error"
	SevWarning Severity = "warning"
	SevNote    Severity = "note"
)

// ErrVersionSkew is returned by Load when the file's version does not
// match SchemaVersion. Callers treat this like a missing file and start
// fresh — there is no migration code by design.
var ErrVersionSkew = errors.New("state: schema version skew")

// Load reads the sidecar at path. Missing file returns (nil, nil) so
// the first run produces no diff. Malformed JSON or version skew
// returns a non-nil error so the caller can decide whether to overwrite
// or refuse; the CLI treats both as "start fresh".
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil //nolint:nilnil // missing file is not an error; callers treat (nil, nil) as "no prior state"
		}
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if f.Version != SchemaVersion {
		return nil, ErrVersionSkew
	}
	return &f, nil
}

// ErrDurabilityDegraded is returned (wrapped) by Save when the atomic
// rename succeeded but the parent-directory fsync did not. The new
// state IS on disk, but a crash before the next directory flush could
// revert the rename and resurrect previously-resolved findings as new.
// Callers detect via errors.Is to distinguish from a true save failure
// (fo-1x0).
var ErrDurabilityDegraded = errors.New("state: durability degraded (parent dir not fsynced)")

// Save writes a File atomically: write to a tmp file in the same
// directory, fsync, then rename. The same-directory tmp keeps the
// rename on the same filesystem (POSIX atomic). fsync before rename
// guards against a power loss between write and rename leaving an
// empty/partial file in place.
//
// On parent-directory fsync failure (e.g. NFS, virtualized FS), Save
// returns an error wrapping ErrDurabilityDegraded — the data IS on
// disk, but durability is reduced. Callers should treat this as a
// warning rather than a hard failure (fo-1x0).
func Save(path string, f *File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("state: mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".last-run.*.tmp")
	if err != nil {
		return fmt.Errorf("state: tempfile: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("state: encode: %w", err)
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

// syncDir opens dir and calls Sync so the parent directory's metadata
// (including the rename above) is flushed to disk. Indirected through a
// package var so tests can assert it's invoked with the right path
// without requiring real fault injection.
var syncDir = func(dir string) error {
	d, err := os.Open(dir) // dir is the parent of a caller-provided sidecar path
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

// Reset removes the sidecar file. Missing file is not an error —
// `fo state reset` is idempotent.
func Reset(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("state: reset %s: %w", path, err)
	}
	return nil
}

// Append produces a new File with current pushed onto the front of
// Runs and the slice trimmed to MaxHistory. The newest run is at index
// 0; the oldest at index len-1. This ordering is load-bearing for the
// flake classifier, which inspects Runs[0]..Runs[N] in temporal order.
func Append(prev *File, current Run) *File {
	out := &File{Version: SchemaVersion}
	out.Runs = append(out.Runs, current)
	if prev != nil {
		out.Runs = append(out.Runs, prev.Runs...)
	}
	if len(out.Runs) > MaxHistory {
		out.Runs = out.Runs[:MaxHistory]
	}
	return out
}

// RunFromReport projects a report.Report onto the persisted Run shape.
// Only RuleID severity per fingerprint is kept; everything else lives
// in the report itself, regenerated each invocation.
func RunFromReport(r *report.Report) Run {
	findings := make(map[string]Severity, len(r.Findings))
	for _, f := range r.Findings {
		if f.Fingerprint == "" {
			continue
		}
		findings[f.Fingerprint] = severityFromReport(f.Severity)
	}
	ts := r.GeneratedAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	return Run{
		GeneratedAt: ts,
		DataHash:    r.DataHash,
		Findings:    findings,
	}
}

func severityFromReport(s report.Severity) Severity {
	switch s {
	case report.SeverityError:
		return SevError
	case report.SeverityWarning:
		return SevWarning
	case report.SeverityNote:
		return SevNote
	default:
		return SevNote
	}
}

// severityRank orders severities so we can detect "increased" without
// hard-coding pair tables. Higher == worse.
func severityRank(s Severity) int {
	switch s {
	case SevError:
		return 3
	case SevWarning:
		return 2
	case SevNote:
		return 1
	default:
		return 0
	}
}
