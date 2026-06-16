package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/report"
)

// SnapshotVersion is the on-disk version for the findings snapshot. It is
// independent of SchemaVersion (the diff sidecar) so the two evolve apart.
const SnapshotVersion = 1

// SnapshotPath returns the resolved findings-snapshot path. The snapshot
// holds the last run's findings and failing tests with their assigned
// short IDs, powering `fo explain <id>` and cross-run ID stability. It is
// distinct from last-run.json, whose Run shape is deliberately lossy for
// diff classification.
func SnapshotPath() string { return filepath.Join(Dir(), "findings.json") }

// Snapshot is the persisted record of one run's addressable items. Unlike
// Run (the diff sidecar), it keeps full Finding/TestResult detail so a
// later `fo explain` can render a complete answer from the handle alone.
type Snapshot struct {
	Version  int                 `json:"version"`
	Tool     string              `json:"tool,omitempty"`
	Findings []report.Finding    `json:"findings,omitempty"`
	Tests    []report.TestResult `json:"tests,omitempty"`
}

// SnapshotFromReport projects a Report onto the Snapshot shape, keeping
// only items that carry a short ID (findings always; failing tests). The
// Report must already have been through report.AssignShortIDs.
func SnapshotFromReport(r *report.Report) *Snapshot {
	s := &Snapshot{Version: SnapshotVersion, Tool: r.Tool}
	for i := range r.Findings {
		if r.Findings[i].ID != "" {
			s.Findings = append(s.Findings, r.Findings[i])
		}
	}
	for i := range r.Tests {
		if r.Tests[i].ID != "" {
			s.Tests = append(s.Tests, r.Tests[i])
		}
	}
	return s
}

// PriorIDs extracts the fingerprint→handle maps the assigner needs to pin
// handles across runs. A nil snapshot yields a nil Prior (ephemeral IDs).
func (s *Snapshot) PriorIDs() *report.Prior {
	if s == nil {
		return nil
	}
	p := &report.Prior{
		Findings: make(map[string]string, len(s.Findings)),
		Tests:    make(map[string]string, len(s.Tests)),
	}
	for i := range s.Findings {
		if fp := s.Findings[i].Fingerprint; fp != "" {
			p.Findings[fp] = s.Findings[i].ID
		}
	}
	for i := range s.Tests {
		if fp := s.Tests[i].Fingerprint; fp != "" {
			p.Tests[fp] = s.Tests[i].ID
		}
	}
	return p
}

// Lookup returns the finding or test matching id (case-sensitive), and
// whether one was found. Findings and tests share the lookup but never
// the namespace (F-/T-), so at most one matches.
func (s *Snapshot) Lookup(id string) (*report.Finding, *report.TestResult, bool) {
	if s == nil || id == "" {
		return nil, nil, false
	}
	for i := range s.Findings {
		if s.Findings[i].ID == id {
			return &s.Findings[i], nil, true
		}
	}
	for i := range s.Tests {
		if s.Tests[i].ID == id {
			return nil, &s.Tests[i], true
		}
	}
	return nil, nil, false
}

// LoadSnapshot reads the findings snapshot at path. A missing file returns
// (nil, nil) — the first run has nothing to resolve. Version skew is
// treated like a missing file by callers (start fresh).
func LoadSnapshot(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil //nolint:nilnil // missing snapshot is not an error
		}
		return nil, fmt.Errorf("state: open %s: %w", path, err)
	}
	defer f.Close()
	b, err := boundread.All(f, sidecarMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", path, err)
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", path, err)
	}
	if s.Version != SnapshotVersion {
		return nil, ErrVersionSkew
	}
	return &s, nil
}

// SaveSnapshot writes s atomically (temp + fsync + rename), mirroring
// Save's durability contract. Like Save it may wrap ErrDurabilityDegraded
// when the parent-directory fsync fails but the data is on disk.
func SaveSnapshot(path string, s *Snapshot) error {
	return writeAtomic(path, ".findings.*.tmp", s)
}
