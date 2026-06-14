// Package metrics parses fo's metrics input format — keyed numeric
// values used for hygiene rollups (coverage %, LOC counts, build time,
// benchmark deltas, dependency counts). Renders as a labeled value list
// with delta sparklines when sidecar history is present.
//
// Format:
//
//	# fo:metrics [tool=<name>]
//	<key>  <value>  [unit]
package metrics

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/hygiene"
)

const HeaderPrefix = "# fo:metrics"

type Row struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type Metrics struct {
	Tool string `json:"tool,omitempty"`
	Rows []Row  `json:"rows"`
}

func IsHeader(data []byte) bool {
	return hygiene.HasHeader(data, HeaderPrefix)
}

var (
	ErrNoHeader     = errors.New("metrics: missing '# fo:metrics' header")
	ErrNoRows       = errors.New("metrics: no data rows")
	ErrMalformedRow = errors.New("metrics: malformed row")
)

func Parse(r io.Reader) (Metrics, error) {
	var m Metrics
	tool, err := hygiene.Scan(r, hygiene.Spec{
		Prefix:      HeaderPrefix,
		Name:        "metrics",
		ErrNoHeader: ErrNoHeader,
		ErrNoRows:   ErrNoRows,
		OnRow: func(_ int, line string) error {
			row, perr := parseRow(line)
			if perr != nil {
				return perr
			}
			m.Rows = append(m.Rows, row)
			return nil
		},
	})
	if err != nil {
		return Metrics{}, err
	}
	m.Tool = tool
	return m, nil
}

func parseRow(line string) (Row, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Row{}, fmt.Errorf("%w: expected '<key> <value> [unit]', got %q", ErrMalformedRow, line)
	}
	v, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Row{}, fmt.Errorf("%w: non-numeric value %q", ErrMalformedRow, fields[1])
	}
	row := Row{Key: fields[0], Value: v}
	if len(fields) >= 3 {
		row.Unit = fields[2]
	}
	return row, nil
}
