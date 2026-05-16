package cluster

// Input is the per-failure data the clusterer needs. Callers build a
// slice of Input from their own failure shape. Key is opaque to the
// clusterer — it is echoed back unchanged in Cluster.Members.
type Input struct {
	Key     string
	Package string
	Test    string
	Outcome string // "fail" | "panic" | "build_error"
	Output  string
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
	_ = cfg.withDefaults()
	out := []Cluster{}
	_ = inputs
	return out
}

// Extract returns the signals for one Input.
func Extract(in Input) Signals {
	return Signals{}
}
