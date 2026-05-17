// Package wrapleaderboard converts plain `count label` tally input
// into fo's tally format. Unlike the SARIF-emitting wrappers
// (wrapdiag, wrapjscpd, wraparchlint), this wrapper emits a tally
// stream — the SARIF parser computes Score internally and discards
// caller-supplied counts, so a count→leaderboard pipeline cannot
// route through SARIF.
//
// Stdin format (one line per row):
//
//	<count> <label>
//
// Tolerates leading whitespace (so `sort | uniq -c` output flows
// through unchanged), comment lines beginning with `#`, and blank
// lines. Stdout is the tally format defined by pkg/tally.
package wrapleaderboard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/tally"
)

// Opts carries wrapleaderboard flags as plain values, mirroring the
// pattern of other wrap subcommands.
type Opts struct {
	Tool string
}

// ErrNoRows is returned when stdin yields no parseable rows.
var ErrNoRows = errors.New("wrap leaderboard: no rows on stdin")

// ErrMalformedRow wraps row-level shape/parse failures.
var ErrMalformedRow = errors.New("wrap leaderboard: malformed row")

// Convert reads tally input from r and writes the canonical tally
// format (with header) to w. Returns an error if no rows parse — a
// silent empty leaderboard would mislead callers piping zero-result
// pipelines.
func Convert(r io.Reader, w io.Writer, opts Opts) error {
	bw := bufio.NewWriter(w)
	defer func() { _ = bw.Flush() }()

	if err := writeHeader(bw, opts.Tool); err != nil {
		return err
	}

	br := bufio.NewReaderSize(r, 64*1024)
	rows := 0
	var dropped int
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else {
			wrote, perr := writeRow(bw, string(raw))
			if perr != nil {
				return perr
			}
			if wrote {
				rows++
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return fmt.Errorf("wrap leaderboard: read: %w", err)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "wrap leaderboard: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)
	}
	if rows == 0 {
		return ErrNoRows
	}
	return nil
}

// writeRow parses a single input line and emits one tally row. Returns
// (false, nil) when the line is blank or a comment.
func writeRow(w io.Writer, raw string) (bool, error) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return false, nil
	}
	countTok, label, err := splitCountLabel(line)
	if err != nil {
		return false, err
	}
	if _, err := fmt.Fprintf(w, "%s %s\n", countTok, label); err != nil {
		return false, err
	}
	return true, nil
}

func writeHeader(w io.Writer, tool string) error {
	header := tally.HeaderPrefix
	if tool != "" {
		header += " tool=" + tool
	}
	_, err := fmt.Fprintln(w, header)
	return err
}

func splitCountLabel(line string) (count, label string, err error) {
	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return "", "", fmt.Errorf("%w: expected '<count> <label>', got %q", ErrMalformedRow, line)
	}
	count = line[:idx]
	label = strings.TrimSpace(line[idx+1:])
	if label == "" {
		return "", "", fmt.Errorf("%w: missing label after count %q", ErrMalformedRow, count)
	}
	if _, perr := strconv.ParseFloat(count, 64); perr != nil {
		return "", "", fmt.Errorf("%w: non-numeric count %q", ErrMalformedRow, count)
	}
	return count, label, nil
}
