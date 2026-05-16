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
	"os"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
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
	br := bufio.NewReaderSize(r, 64*1024)

	var m Metrics
	headerSeen := false
	lineNo := 0
	var dropped int
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else if perr := absorbMetricsLine(raw, &m, &headerSeen, &lineNo); perr != nil {
			return Metrics{}, perr
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return Metrics{}, fmt.Errorf("metrics: read: %w", err)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "metrics: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)
	}
	if !headerSeen {
		return Metrics{}, ErrNoHeader
	}
	if len(m.Rows) == 0 {
		return Metrics{}, ErrNoRows
	}
	return m, nil
}

func absorbMetricsLine(raw []byte, m *Metrics, headerSeen *bool, lineNo *int) error {
	*lineNo++
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return nil
	}
	if !*headerSeen {
		if !strings.HasPrefix(line, HeaderPrefix) {
			return ErrNoHeader
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, HeaderPrefix))
		m.Tool = parseAttr(rest, "tool")
		*headerSeen = true
		return nil
	}
	if strings.HasPrefix(line, "#") {
		return nil
	}
	row, err := parseRow(line)
	if err != nil {
		return fmt.Errorf("metrics: line %d: %w", *lineNo, err)
	}
	m.Rows = append(m.Rows, row)
	return nil
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
