// Package tally parses fo's tally input format — a count→label
// distribution that renders as a Leaderboard view. The format is the
// minimal shape needed to feed arbitrary tallies (e.g. `sort | uniq -c`
// output) into fo without going through SARIF (whose parser computes
// scores internally and would discard caller-supplied counts).
//
// Format:
//
//	# fo:tally [tool=<name>]
//	<count> <label>
//	<count> <label>
//	...
//
// One header line, then count/label rows. Leading whitespace is
// tolerated on data rows so `sort | uniq -c` output (which right-aligns
// counts) is accepted verbatim. Lines beginning with `#` after the
// header are comments and ignored. Blank lines are ignored.
package tally

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/hygiene"
)

// HeaderPrefix is the sentinel that marks tally input. Used by fo's
// stdin sniffer to route tally streams away from SARIF/test-json
// parsing.
const HeaderPrefix = "# fo:tally"

// Row is one count/label pair.
type Row struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// Tally is a parsed tally stream.
type Tally struct {
	Tool string `json:"tool,omitempty"`
	Rows []Row  `json:"rows"`
}

// IsHeader reports whether data begins with the tally header sentinel
// (after optional leading whitespace). Cheap; safe on partial peeked
// input.
func IsHeader(data []byte) bool {
	return hygiene.HasHeader(data, HeaderPrefix)
}

// errNoHeader is returned when input lacks the tally header line.
var errNoHeader = errors.New("tally: missing '# fo:tally' header")

// errNoRows is returned when the header is present but no data rows
// followed.
var errNoRows = errors.New("tally: no data rows")

// errMalformedRow wraps row-level shape and parse failures. Wrapped via
// fmt.Errorf("...: %w", errMalformedRow) at call sites — sentinel keeps
// err113 happy and lets callers errors.Is on a single root.
var errMalformedRow = errors.New("tally: malformed row")

// Parse reads tally input from r and returns the parsed Tally. Oversize
// dropped-line warnings are written to stderr (nil silences them).
// Malformed data lines (no count, non-numeric count) cause a parse
// error pinned to the line number; tolerant to leading whitespace and
// comment/blank lines.
func Parse(r io.Reader, stderr io.Writer) (Tally, error) {
	var t Tally
	tool, err := hygiene.Scan(r, hygiene.Spec{
		Prefix:      HeaderPrefix,
		Name:        "tally",
		ErrNoHeader: errNoHeader,
		ErrNoRows:   errNoRows,
		Stderr:      stderr,
		OnRow: func(_ int, line string) error {
			row, perr := parseRow(line)
			if perr != nil {
				return perr
			}
			t.Rows = append(t.Rows, row)
			return nil
		},
	})
	if err != nil {
		return Tally{}, err
	}
	t.Tool = tool
	return t, nil
}

// parseRow splits a data line into count + label. Count is the first
// whitespace-separated token; label is the trimmed remainder.
func parseRow(line string) (Row, error) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return Row{}, fmt.Errorf("%w: expected '<count> <label>', got %q", errMalformedRow, line)
	}
	countTok := line[:idx]
	label := strings.TrimSpace(line[idx+1:])
	if label == "" {
		return Row{}, fmt.Errorf("%w: missing label after count %q", errMalformedRow, countTok)
	}
	v, err := strconv.ParseFloat(countTok, 64)
	if err != nil {
		return Row{}, fmt.Errorf("%w: non-numeric count %q", errMalformedRow, countTok)
	}
	return Row{Label: label, Value: v}, nil
}
