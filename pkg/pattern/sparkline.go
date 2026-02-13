package pattern

// Sparkline represents a word-sized trend graphic using Unicode blocks.
type Sparkline struct {
	Label  string
	Values []float64
	Min    float64 // 0 = auto-detect
	Max    float64 // 0 = auto-detect
	Unit   string  // e.g., "ms", "%", "MB"
}

func (s *Sparkline) Type() PatternType { return PatternTypeSparkline }
