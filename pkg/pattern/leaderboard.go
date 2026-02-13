package pattern

// Leaderboard represents a ranked list of items by metric.
type Leaderboard struct {
	Label      string
	MetricName string // e.g., "Issues", "Duration"
	Items      []LeaderboardItem
	Direction  string // "highest" or "lowest"
	TotalCount int    // total before filtering to top N
	ShowRank   bool
}

// LeaderboardItem is a single ranked entry.
type LeaderboardItem struct {
	Name    string  // display name
	Metric  string  // formatted value (e.g., "2.3s", "12 warnings")
	Value   float64 // numeric value for sorting
	Rank    int
	Context string // optional extra context
}

func (l *Leaderboard) Type() PatternType { return PatternTypeLeaderboard }
