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
	keyMax := 0
	for _, r := range rows {
		if l := len(r.Key); l > keyMax {
			keyMax = l
		}
	}
	for _, r := range rows {
		v := strconv.FormatFloat(r.Value, 'f', -1, 64)
		delta := ""
		switch {
		case r.New:
			delta = "  (new)"
		case r.Delta != 0:
			sign := "+"
			if r.Delta < 0 {
				sign = ""
			}
			delta = fmt.Sprintf("  (%s%s)", sign, strconv.FormatFloat(r.Delta, 'f', -1, 64))
		}
		unit := ""
		if r.Unit != "" {
			unit = " " + r.Unit
		}
		if _, err := fmt.Fprintf(w, "%-*s  %s%s%s\n", keyMax, r.Key, v, unit, delta); err != nil {
			return err
		}
	}
	return nil
}
