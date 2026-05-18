package cluster

import "sort"

// Input is the per-failure data the clusterer needs. Callers build a
// slice of Input from their own failure shape. Key is opaque to the
// clusterer — it is echoed back unchanged in Cluster.Members.
type Input struct {
	Key     string `json:"key"`
	Package string `json:"package,omitempty"`
	Test    string `json:"test,omitempty"`
	Outcome string `json:"outcome,omitempty"` // "fail" | "panic" | "build_error"
	Output  string `json:"output"`
}

// ClusterID is a short, stable, run-local identifier of the form
// "F-<6 hex>". Derived from sha256(Signature).
type ClusterID string

// Cluster is one group of failures that share a root-cause signal.
type Cluster struct {
	ID            ClusterID `json:"id"`
	Signature     string    `json:"signature"`
	SignatureKind string    `json:"signature_kind"` // "frame" | "norm" | "singleton"
	TopUserFrame  string    `json:"top_user_frame,omitempty"`
	NormSig       string    `json:"norm_sig,omitempty"`
	Members       []string  `json:"members"`
}

// Signals are the two extraction outputs for a single Input.
type Signals struct {
	TopUserFrame string
	NormSig      string
	AnchorLine   string
}

// Mode selects the clustering rule.
type Mode int

const (
	// ModeOr clusters when frames match OR norms match. Default.
	ModeOr Mode = iota
	// ModeAnd clusters when frames match AND norms match.
	ModeAnd
	// ModeFrameOnly clusters by frame match only.
	ModeFrameOnly
	// ModeNormOnly clusters by norm match only.
	ModeNormOnly
)

// Config exposes tunables. Zero-value is the shipping default.
//
// TrimAbsPaths is inverted (KeepAbsPaths) so the zero value yields the
// default behavior: module-root-relative or basename paths in frame
// strings. Set KeepAbsPaths=true for CI debug output that wants raw
// absolute paths.
type Config struct {
	Mode         Mode
	IDPrefix     string
	IDHexLen     int
	KeepAbsPaths bool
	MaxAnchorLen int
}

// Default values applied to zero-value Config fields.
const (
	DefaultIDPrefix     = "F-"
	DefaultIDHexLen     = 6
	DefaultMaxAnchorLen = 512
)

func (c Config) withDefaults() Config {
	if c.IDPrefix == "" {
		c.IDPrefix = DefaultIDPrefix
	}
	if c.IDHexLen == 0 {
		c.IDHexLen = DefaultIDHexLen
	}
	if c.MaxAnchorLen == 0 {
		c.MaxAnchorLen = DefaultMaxAnchorLen
	}
	return c
}

// Run is the entry point. Deterministic: shuffling inputs yields
// identical output. Empty inputs → empty (non-nil) result. Duplicate
// Keys are deduped with last-write-wins.
//
// Named Run instead of Cluster because Go forbids a function and type
// sharing a name in the same package; the type name Cluster is the one
// callers handle most often, so the function gives way.
func Run(inputs []Input) []Cluster {
	return RunWith(inputs, Config{})
}

// RunWith is the configurable form of Run.
func RunWith(inputs []Input, cfg Config) []Cluster {
	cfg = cfg.withDefaults()
	out := []Cluster{}
	if len(inputs) == 0 {
		return out
	}

	// Dedupe by Key, last-write-wins, then sort to remove map-order
	// dependence on the rest of the pipeline. Empty Key is invalid —
	// it would collapse unrelated failures together (fo-yax).
	byKey := make(map[string]Input, len(inputs))
	for _, in := range inputs {
		if in.Key == "" {
			continue
		}
		byKey[in.Key] = in
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	recs := make([]record, len(keys))
	for i, k := range keys {
		in := byKey[k]
		recs[i] = record{input: in, signals: extractWith(in, cfg)}
	}

	uf := newUnionFind(len(recs))
	switch cfg.Mode {
	case ModeOr:
		unionBy(uf, recs, func(s Signals) string { return s.TopUserFrame })
		unionBy(uf, recs, func(s Signals) string { return s.NormSig })
	case ModeAnd:
		// Cluster only when both signals match. Implemented by
		// keying on the (frame, norm) tuple; empty signals never
		// merge.
		unionBy(uf, recs, func(s Signals) string {
			if s.TopUserFrame == "" || s.NormSig == "" {
				return ""
			}
			return s.TopUserFrame + "\x00" + s.NormSig
		})
	case ModeFrameOnly:
		unionBy(uf, recs, func(s Signals) string { return s.TopUserFrame })
	case ModeNormOnly:
		unionBy(uf, recs, func(s Signals) string { return s.NormSig })
	}

	// Build groups: root → member indices.
	groups := make(map[int][]int)
	for i := range recs {
		r := uf.find(i)
		groups[r] = append(groups[r], i)
	}

	taken := make(map[ClusterID]struct{}, len(groups))
	clusters := make([]Cluster, 0, len(groups))
	for _, members := range groups {
		c := buildCluster(members, recs, cfg, taken)
		clusters = append(clusters, c)
	}

	// Stable final order: member count desc, ID asc.
	sort.SliceStable(clusters, func(i, j int) bool {
		if len(clusters[i].Members) != len(clusters[j].Members) {
			return len(clusters[i].Members) > len(clusters[j].Members)
		}
		return clusters[i].ID < clusters[j].ID
	})
	return clusters
}

func extractWith(in Input, cfg Config) Signals {
	anchor := extractAnchor(in.Output, cfg.MaxAnchorLen)
	return Signals{
		TopUserFrame: extractTopUserFrame(in.Output, pathMode(cfg.KeepAbsPaths)),
		NormSig:      Normalize(anchor),
		AnchorLine:   anchor,
	}
}

func unionBy(uf *unionFind, recs []record, key func(Signals) string) {
	groups := make(map[string]int)
	for i := range recs {
		k := key(recs[i].signals)
		if k == "" {
			continue
		}
		if first, ok := groups[k]; ok {
			uf.union(first, i)
		} else {
			groups[k] = i
		}
	}
}

func buildCluster(members []int, recs []record, cfg Config, taken map[ClusterID]struct{}) Cluster {
	// Pick signature: most common non-empty TopUserFrame; ties go to
	// lexicographically smallest. If no frames, fall back to most
	// common NormSig. If both empty for all members → singleton.
	frame := mostCommon(members, recs, func(s Signals) string { return s.TopUserFrame })
	norm := mostCommon(members, recs, func(s Signals) string { return s.NormSig })

	var signature, kind string
	switch {
	case frame != "":
		signature, kind = frame, "frame"
	case norm != "":
		signature, kind = norm, "norm"
	default:
		signature, kind = recs[members[0]].input.Key, "singleton"
	}

	id := disambiguate(makeClusterID(signature, cfg.IDPrefix, cfg.IDHexLen), taken)

	keys := make([]string, len(members))
	for i, m := range members {
		keys[i] = recs[m].input.Key
	}
	sort.Strings(keys)

	return Cluster{
		ID:            id,
		Signature:     signature,
		SignatureKind: kind,
		TopUserFrame:  frame,
		NormSig:       norm,
		Members:       keys,
	}
}

// mostCommon returns the most frequent non-empty value produced by
// pick across members. Ties go to the lexicographically smallest
// value so output is deterministic.
func mostCommon(members []int, recs []record, pick func(Signals) string) string {
	counts := make(map[string]int)
	for _, m := range members {
		v := pick(recs[m].signals)
		if v == "" {
			continue
		}
		counts[v]++
	}
	if len(counts) == 0 {
		return ""
	}
	type kv struct {
		k string
		n int
	}
	all := make([]kv, 0, len(counts))
	for k, n := range counts {
		all = append(all, kv{k, n})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].n != all[j].n {
			return all[i].n > all[j].n
		}
		return all[i].k < all[j].k
	})
	return all[0].k
}

type record struct {
	input   Input
	signals Signals
}

type unionFind struct {
	parent []int
	rank   []int
}

func newUnionFind(n int) *unionFind {
	p := make([]int, n)
	r := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &unionFind{parent: p, rank: r}
}

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	// Deterministic merge: smaller-index root wins ties. Keeps
	// output independent of insertion order beyond the sorted keys.
	if u.rank[ra] < u.rank[rb] {
		ra, rb = rb, ra
	} else if u.rank[ra] == u.rank[rb] && ra > rb {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	if u.rank[ra] == u.rank[rb] {
		u.rank[ra]++
	}
}
