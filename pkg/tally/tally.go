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
	"github.com/dkoosis/fo/pkg/view"
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

// ErrNoHeader is returned when input lacks the tally header line.
var ErrNoHeader = errors.New("tally: missing '# fo:tally' header")

// ErrNoRows is returned when the header is present but no data rows
// followed.
var ErrNoRows = errors.New("tally: no data rows")

// ErrMalformedRow wraps row-level shape and parse failures. Wrapped via
// fmt.Errorf("...: %w", ErrMalformedRow) at call sites — sentinel keeps
// err113 happy and lets callers errors.Is on a single root.
var ErrMalformedRow = errors.New("tally: malformed row")

// Parse reads tally input from r and returns the parsed Tally.
// Malformed data lines (no count, non-numeric count) cause a parse
// error pinned to the line number; tolerant to leading whitespace and
// comment/blank lines.
func Parse(r io.Reader) (Tally, error) {
	var t Tally
	tool, err := hygiene.Scan(r, hygiene.Spec{
		Prefix:      HeaderPrefix,
		Name:        "tally",
		ErrNoHeader: ErrNoHeader,
		ErrNoRows:   ErrNoRows,
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
		return Row{}, fmt.Errorf("%w: expected '<count> <label>', got %q", ErrMalformedRow, line)
	}
	countTok := line[:idx]
	label := strings.TrimSpace(line[idx+1:])
	if label == "" {
		return Row{}, fmt.Errorf("%w: missing label after count %q", ErrMalformedRow, countTok)
	}
	v, err := strconv.ParseFloat(countTok, 64)
	if err != nil {
		return Row{}, fmt.Errorf("%w: non-numeric count %q", ErrMalformedRow, countTok)
	}
	return Row{Label: label, Value: v}, nil
}

// ToLeaderboard builds a view.Leaderboard from t. Rows are emitted in
// input order; Total is the sum of all values (used by the renderer to
// scale bars).
func (t Tally) ToLeaderboard() view.Leaderboard {
	rows := make([]view.LbRow, len(t.Rows))
	var total float64
	for i, r := range t.Rows {
		rows[i] = view.LbRow{Label: r.Label, Value: r.Value}
		total += r.Value
	}
	return view.Leaderboard{Rows: rows, Total: total}
}
