// Package design provides JSON serialization for structured output mode.
package design

import (
	"encoding/json"
	"fmt"
	"time"
)

// JSONOutput represents the structured JSON output format for AI/automation consumption.
type JSONOutput struct {
	Version     string                 `json:"version"`
	PatternType string                 `json:"pattern_type"`
	Metadata    JSONMetadata           `json:"metadata"`
	Data        map[string]interface{} `json:"data"`
}

// JSONMetadata contains metadata about the command execution and pattern.
type JSONMetadata struct {
	ExitCode       int       `json:"exit_code"`
	Duration       string    `json:"duration"`
	DurationMs     int64     `json:"duration_ms"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Command        string    `json:"command,omitempty"`
	Args           []string  `json:"args,omitempty"`
	Label          string    `json:"label,omitempty"`
	Classification string    `json:"classification,omitempty"`
}

// ToJSON converts a Pattern to JSON output format.
func ToJSON(p Pattern, metadata JSONMetadata) ([]byte, error) {
	output := JSONOutput{
		Version:     "1.0",
		PatternType: string(p.PatternType()),
		Metadata:    metadata,
		Data:        make(map[string]interface{}),
	}

	// Serialize pattern-specific data
	switch pattern := p.(type) {
	case *Sparkline:
		output.Data = map[string]interface{}{
			"label":  pattern.Label,
			"values": pattern.Values,
			"min":    pattern.Min,
			"max":    pattern.Max,
			"unit":   pattern.Unit,
		}
	case *Leaderboard:
		items := make([]map[string]interface{}, len(pattern.Items))
		for i, item := range pattern.Items {
			items[i] = map[string]interface{}{
				"name":    item.Name,
				"metric":  item.Metric,
				"value":   item.Value,
				"rank":    item.Rank,
				"context": item.Context,
			}
		}
		output.Data = map[string]interface{}{
			"label":       pattern.Label,
			"metric_name": pattern.MetricName,
			"direction":   pattern.Direction,
			"total_count": pattern.TotalCount,
			"show_rank":   pattern.ShowRank,
			"items":       items,
		}
	case *TestTable:
		results := make([]map[string]interface{}, len(pattern.Results))
		for i, result := range pattern.Results {
			results[i] = map[string]interface{}{
				"name":     result.Name,
				"status":   result.Status,
				"duration": result.Duration,
				"count":    result.Count,
				"details":  result.Details,
			}
		}
		output.Data = map[string]interface{}{
			"label":   pattern.Label,
			"density": string(pattern.Density),
			"results": results,
		}
	case *Summary:
		metrics := make([]map[string]interface{}, len(pattern.Metrics))
		for i, metric := range pattern.Metrics {
			metrics[i] = map[string]interface{}{
				"label": metric.Label,
				"value": metric.Value,
				"type":  metric.Type,
			}
		}
		output.Data = map[string]interface{}{
			"label":   pattern.Label,
			"metrics": metrics,
		}
	case *Comparison:
		changes := make([]map[string]interface{}, len(pattern.Changes))
		for i, change := range pattern.Changes {
			changes[i] = map[string]interface{}{
				"label":  change.Label,
				"before": change.Before,
				"after":  change.After,
				"change": change.Change,
				"unit":   change.Unit,
			}
		}
		output.Data = map[string]interface{}{
			"label":   pattern.Label,
			"changes": changes,
		}
	case *Inventory:
		items := make([]map[string]interface{}, len(pattern.Items))
		for i, item := range pattern.Items {
			items[i] = map[string]interface{}{
				"name": item.Name,
				"size": item.Size,
				"path": item.Path,
			}
		}
		output.Data = map[string]interface{}{
			"label": pattern.Label,
			"items": items,
		}
	default:
		return nil, fmt.Errorf("unsupported pattern type for JSON: %T", p)
	}

	return json.MarshalIndent(output, "", "  ")
}
