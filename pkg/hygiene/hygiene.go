// Package hygiene provides the shared parse scaffolding for fo's hygiene
// input formats (status, metrics, tally). Each format is a
// `# fo:<kind> [attr=val]` header followed by whitespace-delimited data
// rows; they differ only in row grammar. This package owns the common
// machinery — bounded line reading, header detection, attribute
// extraction, comment/blank skipping, and the no-header/no-rows guards —
// so each format package supplies only its row parser via Spec.OnRow.
package hygiene

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
)

// HasHeader reports whether data begins with prefix after optional
// leading whitespace. Cheap; safe on partial peeked input — used by
// fo's stdin sniffer to route hygiene streams.
func HasHeader(data []byte, prefix string) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte(prefix))
}

// ParseAttr pulls a `key=value` attribute out of a header tail. Returns
// "" when key is absent. Only `tool=` is recognized by callers today;
// unknown keys are ignored.
func ParseAttr(tail, key string) string {
	for tok := range strings.FieldsSeq(tail) {
		if eq := strings.IndexByte(tok, '='); eq > 0 && tok[:eq] == key {
			return tok[eq+1:]
		}
	}
	return ""
}

// Spec configures a hygiene parse over a single format.
type Spec struct {
	// Prefix is the header sentinel, e.g. "# fo:status".
	Prefix string
	// Name is the format name used in error messages, e.g. "status".
	Name string
	// ErrNoHeader is returned when input lacks the header line.
	ErrNoHeader error
	// ErrNoRows is returned when the header is present but no data rows follow.
	ErrNoRows error
	// OnRow is called for each data row (non-blank, non-comment, post-header)
	// with the 1-based source line number. A returned error is wrapped with
	// the format name and line number before propagating.
	OnRow func(lineNo int, line string) error
}

type scanState struct {
	tool       string
	headerSeen bool
	lineNo     int
	rows       int
}

// Scan runs the shared hygiene parse loop over r and returns the value
// of the header's `tool=` attribute. It enforces the header-present and
// at-least-one-row invariants via spec's sentinels. Oversize lines are
// dropped with a stderr warning, matching the per-format behavior.
func Scan(r io.Reader, spec Spec) (string, error) {
	br := bufio.NewReaderSize(r, 64*1024)

	var st scanState
	dropped := 0
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else if perr := absorb(raw, spec, &st); perr != nil {
			return "", perr
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return "", fmt.Errorf("%s: read: %w", spec.Name, err)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "%s: dropped %d line(s) exceeding %d bytes\n", spec.Name, dropped, lineread.MaxLineLen)
	}
	if !st.headerSeen {
		return "", spec.ErrNoHeader
	}
	if st.rows == 0 {
		return "", spec.ErrNoRows
	}
	return st.tool, nil
}

func absorb(raw []byte, spec Spec, st *scanState) error {
	st.lineNo++
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return nil
	}
	if !st.headerSeen {
		if !strings.HasPrefix(line, spec.Prefix) {
			return spec.ErrNoHeader
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, spec.Prefix))
		st.tool = ParseAttr(rest, "tool")
		st.headerSeen = true
		return nil
	}
	if strings.HasPrefix(line, "#") {
		return nil
	}
	if err := spec.OnRow(st.lineNo, line); err != nil {
		return fmt.Errorf("%s: line %d: %w", spec.Name, st.lineNo, err)
	}
	st.rows++
	return nil
}
