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
	"strconv"
	"strings"

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

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	rows := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		countTok, label, err := splitCountLabel(line)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(bw, "%s %s\n", countTok, label); err != nil {
			return err
		}
		rows++
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("wrap leaderboard: read: %w", err)
	}
	if rows == 0 {
		return ErrNoRows
	}
	return nil
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
