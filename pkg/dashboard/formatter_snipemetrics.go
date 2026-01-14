package dashboard

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SnipeMetricsFormatter handles snipe performance/quality metrics JSON output.
type SnipeMetricsFormatter struct{}

func (f *SnipeMetricsFormatter) Matches(command string) bool {
	return strings.Contains(command, "snipe-metrics") ||
		strings.Contains(command, "BASELINE.json") ||
		strings.Contains(command, "snipe baseline")
}

// SnipeMetrics matches the structure in snipe's BASELINE.json.
type SnipeMetrics struct {
	Timestamp string `json:"timestamp"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	Codebase  struct {
		GoFiles   int `json:"go_files"`
		Symbols   int `json:"symbols"`
		Refs      int `json:"refs"`
		CallEdges int `json:"call_edges"`
		DBSizeKB  int `json:"db_size_kb"`
	} `json:"codebase"`
	Index struct {
		TotalMs   int64 `json:"total_ms"`
		LoadMs    int64 `json:"load_ms"`
		ExtractMs int64 `json:"extract_ms"`
		PersistMs int64 `json:"persist_ms"`
		PeakMemMB int   `json:"peak_mem_mb"`
	} `json:"index"`
	Query struct {
		DefByNameMs float64 `json:"def_by_name_ms"`
		DefByPosMs  float64 `json:"def_by_pos_ms"`
		RefsByIDMs  float64 `json:"refs_by_id_ms"`
	} `json:"query"`
	Search struct {
		SimplePatternMs float64 `json:"simple_pattern_ms"`
		RegexPatternMs  float64 `json:"regex_pattern_ms"`
	} `json:"search"`
	Quality struct {
		SymbolsWithDoc   int     `json:"symbols_with_doc"`
		SymbolsWithSig   int     `json:"symbols_with_sig"`
		DocCoveragePct   float64 `json:"doc_coverage_pct"`
		RefsPerSymbol    float64 `json:"refs_per_symbol"`
		CallgraphCovPct  float64 `json:"callgraph_coverage_pct"`
	} `json:"quality"`
}

func (f *SnipeMetricsFormatter) Format(lines []string, width int) string {
	var b strings.Builder

	// Try to parse as SnipeMetrics
	var m SnipeMetrics
	joined := strings.Join(lines, "\n")
	if err := json.Unmarshal([]byte(joined), &m); err != nil {
		return (&PlainFormatter{}).Format(lines, width)
	}

	// Verify it's snipe metrics (has expected fields)
	if m.Codebase.Symbols == 0 && m.Index.TotalMs == 0 {
		return (&PlainFormatter{}).Format(lines, width)
	}

	s := Styles()

	// Header
	b.WriteString(s.Header.Render("◉ Snipe Metrics"))
	b.WriteString("  ")
	b.WriteString(s.File.Render(fmt.Sprintf("@ %s (%s)", m.GitCommit, m.Timestamp[:10])))
	b.WriteString("\n\n")

	// Performance section
	b.WriteString(s.Header.Render("Performance"))
	b.WriteString("\n")
	b.WriteString(formatMetricRow("  Index time", fmt.Sprintf("%dms", m.Index.TotalMs), s, metricsThreshold(float64(m.Index.TotalMs), 5000, 10000)))
	b.WriteString(formatMetricRow("    Load", fmt.Sprintf("%dms", m.Index.LoadMs), s, ""))
	b.WriteString(formatMetricRow("    Extract", fmt.Sprintf("%dms", m.Index.ExtractMs), s, ""))
	b.WriteString(formatMetricRow("    Persist", fmt.Sprintf("%dms", m.Index.PersistMs), s, ""))
	b.WriteString(formatMetricRow("  Def query", fmt.Sprintf("%.2fms", m.Query.DefByNameMs), s, metricsThreshold(m.Query.DefByNameMs, 50, 100)))
	b.WriteString(formatMetricRow("  Pos query", fmt.Sprintf("%.2fms", m.Query.DefByPosMs), s, metricsThreshold(m.Query.DefByPosMs, 100, 200)))
	b.WriteString(formatMetricRow("  Refs query", fmt.Sprintf("%.2fms", m.Query.RefsByIDMs), s, metricsThreshold(m.Query.RefsByIDMs, 50, 100)))
	b.WriteString(formatMetricRow("  Search", fmt.Sprintf("%.2fms", m.Search.SimplePatternMs), s, ""))
	b.WriteString(formatMetricRow("  Peak memory", fmt.Sprintf("%dMB", m.Index.PeakMemMB), s, metricsThreshold(float64(m.Index.PeakMemMB), 500, 1000)))
	b.WriteString("\n")

	// Codebase section
	b.WriteString(s.Header.Render("Codebase"))
	b.WriteString("\n")
	b.WriteString(formatMetricRow("  Go files", fmt.Sprintf("%d", m.Codebase.GoFiles), s, ""))
	b.WriteString(formatMetricRow("  Symbols", fmt.Sprintf("%d", m.Codebase.Symbols), s, ""))
	b.WriteString(formatMetricRow("  References", fmt.Sprintf("%d", m.Codebase.Refs), s, ""))
	b.WriteString(formatMetricRow("  Call edges", fmt.Sprintf("%d", m.Codebase.CallEdges), s, ""))
	b.WriteString(formatMetricRow("  DB size", fmt.Sprintf("%dKB", m.Codebase.DBSizeKB), s, ""))
	b.WriteString("\n")

	// Quality section
	b.WriteString(s.Header.Render("Quality"))
	b.WriteString("\n")
	b.WriteString(formatMetricRow("  Doc coverage", fmt.Sprintf("%.1f%%", m.Quality.DocCoveragePct), s, metricsThresholdHigherBetter(m.Quality.DocCoveragePct, 50, 30)))
	b.WriteString(formatMetricRow("  Refs/symbol", fmt.Sprintf("%.1f", m.Quality.RefsPerSymbol), s, ""))
	b.WriteString(formatMetricRow("  Call coverage", fmt.Sprintf("%.1f%%", m.Quality.CallgraphCovPct), s, ""))
	b.WriteString("\n")

	return b.String()
}

// formatMetricRow formats a single metric row with optional status indicator.
func formatMetricRow(label, value string, s *FormatterStyles, status string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-16s", label))

	switch status {
	case "good":
		b.WriteString(s.Success.Render(fmt.Sprintf("%12s", value)))
	case "warn":
		b.WriteString(s.Warn.Render(fmt.Sprintf("%12s", value)))
	case "bad":
		b.WriteString(s.Error.Render(fmt.Sprintf("%12s", value)))
	default:
		b.WriteString(s.File.Render(fmt.Sprintf("%12s", value)))
	}
	b.WriteString("\n")
	return b.String()
}

// metricsThreshold returns status based on value (lower is better).
func metricsThreshold(value, warnThreshold, badThreshold float64) string {
	if value > badThreshold {
		return "bad"
	}
	if value > warnThreshold {
		return "warn"
	}
	return "good"
}

// metricsThresholdHigherBetter returns status based on value (higher is better).
func metricsThresholdHigherBetter(value, goodThreshold, warnThreshold float64) string {
	if value >= goodThreshold {
		return "good"
	}
	if value >= warnThreshold {
		return "warn"
	}
	return "bad"
}

// PrefersBatch implements BatchFormatter - metrics JSON should not be streamed.
func (f *SnipeMetricsFormatter) PrefersBatch() bool {
	return true
}

// GetStatus implements StatusIndicator for content-aware menu icons.
func (f *SnipeMetricsFormatter) GetStatus(lines []string) IndicatorStatus {
	var m SnipeMetrics
	joined := strings.Join(lines, "\n")
	if err := json.Unmarshal([]byte(joined), &m); err != nil {
		return IndicatorDefault
	}

	// Check for performance issues
	if m.Query.DefByNameMs > 100 || m.Query.DefByPosMs > 200 {
		return IndicatorWarning
	}
	if m.Index.TotalMs > 10000 {
		return IndicatorWarning
	}

	return IndicatorSuccess
}
