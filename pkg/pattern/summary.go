package pattern

// SummaryKind identifies the source format for dispatch (avoids string-prefix matching).
type SummaryKind string

const (
	SummaryKindSARIF  SummaryKind = "sarif"
	SummaryKindTest   SummaryKind = "test"
	SummaryKindReport SummaryKind = "report"
)

// ItemKind controls styling/coloring of summary items.
type ItemKind string

const (
	KindSuccess ItemKind = "success"
	KindError   ItemKind = "error"
	KindWarning ItemKind = "warning"
	KindInfo    ItemKind = "info"
)

// Summary represents high-level metrics and counts.
type Summary struct {
	Label   string
	Kind    SummaryKind // dispatch key for renderers
	Metrics []SummaryItem
}

// SummaryItem is a single metric in a summary.
type SummaryItem struct {
	Label string   // e.g., "Errors", "Warnings", "Passed"
	Value string   // formatted value
	Kind  ItemKind // KindSuccess, KindError, KindWarning, KindInfo — affects coloring
}

func (s *Summary) Type() PatternType { return PatternTypeSummary }
