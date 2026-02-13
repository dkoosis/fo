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
)

// Pattern is the interface all visualization patterns implement.
// Patterns hold data; renderers decide how to present it.
type Pattern interface {
	Type() PatternType
}
