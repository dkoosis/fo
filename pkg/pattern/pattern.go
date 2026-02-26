// Package pattern defines the semantic data types for fo's output visualization.
// Patterns are pure data — renderers decide presentation.
package pattern

// PatternType identifies the kind of visualization pattern.
type PatternType string

const (
	PatternTypeSummary     PatternType = "summary"
	PatternTypeLeaderboard PatternType = "leaderboard"
	PatternTypeTestTable   PatternType = "test-table"
	PatternTypeSparkline   PatternType = "sparkline"
	PatternTypeComparison  PatternType = "comparison"
	PatternTypeError       PatternType = "error"
)

// Error represents a processing failure (e.g., unparseable section in a report).
// Distinct from test failures or lint diagnostics — this is fo's own error.
type Error struct {
	Source  string // tool name or component that failed
	Message string // what went wrong
}

func (e *Error) Type() PatternType { return PatternTypeError }

// Pattern is the interface all visualization patterns implement.
// Patterns hold data; renderers decide how to present it.
type Pattern interface {
	Type() PatternType
}
