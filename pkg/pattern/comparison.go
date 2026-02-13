package pattern

// Comparison represents before/after metric comparisons.
type Comparison struct {
	Label   string
	Changes []ComparisonItem
}

// ComparisonItem is a single before/after delta.
type ComparisonItem struct {
	Label  string
	Before string
	After  string
	Change float64 // positive or negative
	Unit   string  // e.g., "%", "MB", "ms"
}

func (c *Comparison) Type() PatternType { return PatternTypeComparison }
