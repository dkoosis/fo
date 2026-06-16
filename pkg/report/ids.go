package report

import "sort"

// Prior carries the previous run's fingerprint→handle maps so a handle can
// keep pointing at the same defect across runs even as the surrounding set
// of findings changes. Both maps are fingerprint → full ID ("F-7a2").
type Prior struct {
	Findings map[string]string
	Tests    map[string]string
}

// AssignShortIDs gives each Finding and failing TestResult a short,
// human- and agent-addressable handle ("F-7a2", "T-3f1") derived from its
// Fingerprint — the shortest hex prefix unique within this run (minimum 3
// chars, grown until collision-free), so `fo explain F-7a2` resolves back
// to a single finding.
//
// Two rows sharing a Fingerprint (the same defect identity) share an ID by
// design. Rows with an empty Fingerprint are left without an ID. Findings
// and tests use independent namespaces (F- / T-).
func AssignShortIDs(r *Report) { AssignShortIDsStable(r, nil) }

// AssignShortIDsStable is AssignShortIDs with cross-run pinning: any
// fingerprint present in prior keeps its previously-assigned handle, and
// only genuinely new fingerprints mint fresh ones (collision-free against
// the reused handles). A nil prior reduces to the ephemeral assignment.
func AssignShortIDsStable(r *Report, prior *Prior) {
	if r == nil {
		return
	}
	var pf, pt map[string]string
	if prior != nil {
		pf, pt = prior.Findings, prior.Tests
	}
	assignFindingIDs(r.Findings, pf)
	assignTestIDs(r.Tests, pt)
}

func assignFindingIDs(findings []Finding, prior map[string]string) {
	fps := make([]string, 0, len(findings))
	for i := range findings {
		fps = append(fps, findings[i].Fingerprint)
	}
	ids := shortIDs(fps, "F-", prior)
	for i := range findings {
		findings[i].ID = ids[findings[i].Fingerprint]
	}
}

func assignTestIDs(tests []TestResult, prior map[string]string) {
	fps := make([]string, 0, len(tests))
	for i := range tests {
		fps = append(fps, tests[i].Fingerprint)
	}
	ids := shortIDs(fps, "T-", prior)
	for i := range tests {
		tests[i].ID = ids[tests[i].Fingerprint]
	}
}

// minPrefix is the floor for short-ID hex length. 3 hex chars = 4096
// distinct handles, ample for a typical run; the prefix grows only when a
// run is dense enough to collide.
const minPrefix = 3

// shortIDs maps each distinct non-empty fingerprint to "<prefix><hex>".
// Fingerprints found in prior reuse their stored hex; the rest mint the
// shortest hex prefix (>= minPrefix) that is unique among all current
// fingerprints and not already claimed by a reused handle. Empty
// fingerprints map to "" and are absent from the result.
func shortIDs(fingerprints []string, prefix string, prior map[string]string) map[string]string {
	distinct := make(map[string]struct{}, len(fingerprints))
	for _, fp := range fingerprints {
		if fp != "" {
			distinct[fp] = struct{}{}
		}
	}
	out := make(map[string]string, len(distinct))
	if len(distinct) == 0 {
		return out
	}
	used := make(map[string]struct{}, len(distinct)) // claimed hex (no prefix)
	reuseHandles(distinct, prefix, prior, out, used)
	mintHandles(distinct, prefix, out, used)
	return out
}

// reuseHandles pins prior handles onto surviving fingerprints. A stored ID
// is honored only when it is a genuine prefix of the current fingerprint —
// a defensive guard against a corrupted or fingerprint-changed sidecar.
func reuseHandles(distinct map[string]struct{}, prefix string, prior, out map[string]string, used map[string]struct{}) {
	for fp := range distinct {
		id, ok := prior[fp]
		if !ok || len(id) <= len(prefix) || id[:len(prefix)] != prefix {
			continue
		}
		hex := id[len(prefix):]
		if !hasPrefix(fp, hex) {
			continue
		}
		out[fp] = id
		used[hex] = struct{}{}
	}
}

// mintHandles assigns fresh handles to the not-yet-resolved fingerprints in
// sorted order (deterministic regardless of map iteration), each the
// shortest unique, unclaimed hex prefix.
func mintHandles(distinct map[string]struct{}, prefix string, out map[string]string, used map[string]struct{}) {
	fresh := make([]string, 0, len(distinct)-len(out))
	for fp := range distinct {
		if _, done := out[fp]; !done {
			fresh = append(fresh, fp)
		}
	}
	sort.Strings(fresh)
	for _, fp := range fresh {
		hex := mintHex(distinct, fp, used)
		out[fp] = prefix + hex
		used[hex] = struct{}{}
	}
}

// mintHex returns the shortest prefix of fp (>= minPrefix) that is unique
// among distinct and not already in used, falling back to the whole
// fingerprint if it cannot be distinguished any shorter.
func mintHex(distinct map[string]struct{}, fp string, used map[string]struct{}) string {
	for n := minPrefix; n < len(fp); n++ {
		hex := fp[:n]
		if _, claimed := used[hex]; !claimed && prefixUnique(distinct, fp, n) {
			return hex
		}
	}
	return fp
}

// prefixUnique reports whether fp's first n characters are shared by no
// other distinct fingerprint.
func prefixUnique(distinct map[string]struct{}, fp string, n int) bool {
	p := fp[:min(n, len(fp))]
	for other := range distinct {
		if other == fp {
			continue
		}
		if other[:min(n, len(other))] == p {
			return false
		}
	}
	return true
}

func hasPrefix(s, pre string) bool {
	return len(s) >= len(pre) && s[:len(pre)] == pre
}
