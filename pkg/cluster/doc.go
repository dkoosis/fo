// Package cluster groups go test failures that share a root cause —
// same topmost user-code stack frame OR same normalized assertion
// text — so renderers can collapse N failures from one bug into one
// expandable group.
//
// Two failures land in the same cluster iff at least one of these is
// true (default ModeOr):
//
//   - They share the same topmost user-code stack frame
//     (file:line, excluding stdlib, testing, runtime, and testify
//     assert/require helpers).
//   - They share the same normalized assertion-text anchor
//     (see Normalize for the substitution pipeline).
//
// Cluster is a pure deterministic function: shuffling the input slice
// yields byte-identical output. The package has no dependency on
// pkg/report — the report→cluster adapter lives in fo-u15.3.2.
//
// See docs/superpowers/plans/fo-u15.3.1-cluster-heuristic.md for the
// full design rationale.
package cluster
