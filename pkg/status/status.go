// Package status parses fo's status input format — labeled rows with
// PASS/FAIL/WARN/SKIP state, used for contract tables, doctor checks,
// module gates, and any "list of named conditions" output that today
// gets handed to printf|awk.
//
// Format:
//
//	# fo:status [tool=<name>]
//	<state>  <label>  [value]  [note...]
//
// State is one of: ok | fail | warn | skip (case-insensitive). Lines
// beginning with # after the header are comments. Blank lines and
// leading whitespace on data rows are tolerated.
package status

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/hygiene"
)

const HeaderPrefix = "# fo:status"

type State string

const (
	StateOK   State = "ok"
	StateFail State = "fail"
	StateWarn State = "warn"
	StateSkip State = "skip"
)

type Row struct {
	State State  `json:"state"`
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
}

type Status struct {
	Tool string `json:"tool,omitempty"`
	Rows []Row  `json:"rows"`
}

func IsHeader(data []byte) bool {
	return hygiene.HasHeader(data, HeaderPrefix)
}

var (
	ErrNoHeader     = errors.New("status: missing '# fo:status' header")
	ErrNoRows       = errors.New("status: no data rows")
	ErrMalformedRow = errors.New("status: malformed row")
	ErrBadState     = errors.New("status: bad state token")
)

func Parse(r io.Reader) (Status, error) {
	var s Status
	tool, err := hygiene.Scan(r, hygiene.Spec{
		Prefix:      HeaderPrefix,
		Name:        "status",
		ErrNoHeader: ErrNoHeader,
		ErrNoRows:   ErrNoRows,
		OnRow: func(_ int, line string) error {
			row, perr := parseRow(line)
			if perr != nil {
				return perr
			}
			s.Rows = append(s.Rows, row)
			return nil
		},
	})
	if err != nil {
		return Status{}, err
	}
	s.Tool = tool
	return s, nil
}

func parseRow(line string) (Row, error) {
	idx := strings.IndexAny(line, " \t")
	if idx <= 0 {
		return Row{}, fmt.Errorf("%w: expected '<state> <label> ...', got %q", ErrMalformedRow, line)
	}
	st, err := parseState(line[:idx])
	if err != nil {
		return Row{}, err
	}
	rest := strings.TrimLeft(line[idx:], " \t")
	if rest == "" {
		return Row{}, fmt.Errorf("%w: missing label, got %q", ErrMalformedRow, line)
	}
	row := Row{State: st}
	if strings.ContainsRune(rest, '\t') {
		parts := strings.SplitN(rest, "\t", 3)
		row.Label = strings.TrimSpace(parts[0])
		if len(parts) >= 2 {
			row.Value = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			row.Note = strings.TrimSpace(parts[2])
		}
	} else {
		row.Label = strings.TrimSpace(rest)
	}
	if row.Label == "" {
		return Row{}, fmt.Errorf("%w: missing label, got %q", ErrMalformedRow, line)
	}
	return row, nil
}

func parseState(tok string) (State, error) {
	switch strings.ToLower(tok) {
	case "ok", "pass":
		return StateOK, nil
	case "fail", "error":
		return StateFail, nil
	case "warn", "warning":
		return StateWarn, nil
	case "skip":
		return StateSkip, nil
	}
	return "", fmt.Errorf("%w: %q", ErrBadState, tok)
}

// ViewRow is the renderer-facing shape; mirrors view.StatusRow so
// pkg/view doesn't need to import pkg/status.
type ViewRow struct {
	State string
	Label string
	Value string
	Note  string
}

// ToViewRows converts to the renderer's row shape.
func (s Status) ToViewRows() []ViewRow {
	out := make([]ViewRow, len(s.Rows))
	for i, r := range s.Rows {
		out[i] = ViewRow{State: string(r.State), Label: r.Label, Value: r.Value, Note: r.Note}
	}
	return out
}
