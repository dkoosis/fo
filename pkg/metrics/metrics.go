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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte(HeaderPrefix))
}

var (
	ErrNoHeader     = errors.New("metrics: missing '# fo:metrics' header")
	ErrNoRows       = errors.New("metrics: no data rows")
	ErrMalformedRow = errors.New("metrics: malformed row")
)

func Parse(r io.Reader) (Metrics, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	var m Metrics
	headerSeen := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !headerSeen {
			if !strings.HasPrefix(line, HeaderPrefix) {
				return Metrics{}, ErrNoHeader
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, HeaderPrefix))
			m.Tool = parseAttr(rest, "tool")
			headerSeen = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		row, err := parseRow(line)
		if err != nil {
			return Metrics{}, fmt.Errorf("metrics: line %d: %w", lineNo, err)
		}
		m.Rows = append(m.Rows, row)
	}
	if err := sc.Err(); err != nil {
		return Metrics{}, fmt.Errorf("metrics: read: %w", err)
	}
	if !headerSeen {
		return Metrics{}, ErrNoHeader
	}
	if len(m.Rows) == 0 {
		return Metrics{}, ErrNoRows
	}
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

func parseAttr(tail, key string) string {
	for tok := range strings.FieldsSeq(tail) {
		if eq := strings.IndexByte(tok, '='); eq > 0 && tok[:eq] == key {
			return tok[eq+1:]
		}
	}
	return ""
}
