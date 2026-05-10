package view

import (
	"fmt"
	"io"
	"strconv"
)

type MetricRow struct {
	Key   string
	Value float64
	Unit  string
	Delta float64 // 0 if New, or genuinely unchanged
	New   bool    // true when no prior sample matched — render "(new)"
}

func RenderMetricsLLM(w io.Writer, tool string, rows []MetricRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "# %s\n", tool); err != nil {
			return err
		}
	}
	for _, r := range rows {
		v := strconv.FormatFloat(r.Value, 'f', -1, 64)
		if r.Unit != "" {
			if _, err := fmt.Fprintf(w, "%s %s %s\n", r.Key, v, r.Unit); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "%s %s\n", r.Key, v); err != nil {
			return err
		}
	}
	return nil
}

func RenderMetricsHuman(w io.Writer, tool string, rows []MetricRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "── %s ──\n", tool); err != nil {
			return err
		}
	}
	keyMax := maxKeyLen(rows)
	for _, r := range rows {
		v := strconv.FormatFloat(r.Value, 'f', -1, 64)
		unit := formatUnit(r.Unit)
		delta := formatDelta(r)
		if _, err := fmt.Fprintf(w, "%-*s  %s%s%s\n", keyMax, r.Key, v, unit, delta); err != nil {
			return err
		}
	}
	return nil
}

func maxKeyLen(rows []MetricRow) int {
	keyMax := 0
	for _, r := range rows {
		if l := len(r.Key); l > keyMax {
			keyMax = l
		}
	}
	return keyMax
}

func formatUnit(unit string) string {
	if unit == "" {
		return ""
	}
	return " " + unit
}

func formatDelta(r MetricRow) string {
	switch {
	case r.New:
		return "  (new)"
	case r.Delta != 0:
		sign := "+"
		if r.Delta < 0 {
			sign = ""
		}
		return fmt.Sprintf("  (%s%s)", sign, strconv.FormatFloat(r.Delta, 'f', -1, 64))
	}
	return ""
}
