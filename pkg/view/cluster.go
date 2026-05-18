// Strategy: extend BulletItem with optional *ClusterRender (per Task 0 orient).
package view

import (
	"fmt"

	"github.com/dkoosis/fo/pkg/report"
)

// partitionTests splits tests into clustered (keyed by ClusterID, source order
// preserved per cluster) and singletons (tests with empty ClusterID).
func partitionTests(tests []report.TestResult) (map[string][]report.TestResult, []report.TestResult) {
	clustered := map[string][]report.TestResult{}
	singletons := make([]report.TestResult, 0, len(tests))
	for i := range tests {
		t := &tests[i]
		if t.ClusterID == "" {
			singletons = append(singletons, *t)
			continue
		}
		clustered[t.ClusterID] = append(clustered[t.ClusterID], *t)
	}
	return clustered, singletons
}

// sharedOutput reports whether every member's raw Output is byte-equal.
// On match: returns the shared Output and true.
// On any divergence (or empty input): returns "" and false.
// Conservative: no normalization. The clusterer's normalizer
// (pkg/cluster/normalize) strips numbers/paths/addrs and would otherwise
// collapse genuinely-divergent failures behind a single "shared:" line.
func sharedOutput(members []report.TestResult) (string, bool) {
	if len(members) == 0 {
		return "", false
	}
	first := members[0].Output
	for i := 1; i < len(members); i++ {
		if members[i].Output != first {
			return "", false
		}
	}
	return first, true
}

// clusterHeader formats the leading line of a cluster block.
// Human mode: "▸ <signature> · K tests · --expand=<id>" — ▸ is a static
// marker (fo is one-shot stdin→stdout; no clickable disclosure).
// LLM mode: "cluster <id> · <signature> · K tests" — no glyph, no flag hint.
func clusterHeader(c report.Cluster, k int, mode Mode) string {
	if mode == ModeLLM {
		return fmt.Sprintf("cluster %s · %s · %d tests", c.ID, c.Signature, k)
	}
	return fmt.Sprintf("▸ %s · %d tests · --expand=%s", c.Signature, k, c.ID)
}

// expandAll is the sentinel --expand value that opens every cluster.
const expandAll = "all"

// expandSet is the parsed form of --expand flag values.
// Empty set + all=false → all clusters collapsed (default).
type expandSet struct {
	ids map[string]struct{}
	all bool
}

func newExpandSet(values []string) expandSet {
	e := expandSet{ids: map[string]struct{}{}}
	for _, v := range values {
		if v == expandAll {
			e.all = true
			continue
		}
		e.ids[v] = struct{}{}
	}
	return e
}

func (e expandSet) wants(id string) bool {
	if e.all {
		return true
	}
	_, ok := e.ids[id]
	return ok
}

// NewExpandSet exposes expandSet construction to cmd/fo and external tests.
func NewExpandSet(values []string) expandSet { return newExpandSet(values) }
