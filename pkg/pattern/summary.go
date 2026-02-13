package pattern

// Summary represents high-level metrics and counts.
type Summary struct {
	Label   string
	Metrics []SummaryItem
}

// SummaryItem is a single metric in a summary.
type SummaryItem struct {
	Label string // e.g., "Errors", "Warnings", "Passed"
	Value string // formatted value
	Kind  string // "success", "error", "warning", "info" — affects coloring
}

func (s *Summary) Type() PatternType { return PatternTypeSummary }
