// Package wraparchlinttext converts go-arch-lint plain-text output into
// SARIF. The JSON output of go-arch-lint is already handled by
// wraparchlint; this wrapper exists for setups that pipe the text form
// (e.g. when the `--json` flag is unavailable or undesired in CI).
package wraparchlinttext

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/sarif"
)

const ruleID = "arch-lint/forbidden-import"

var headerRe = regexp.MustCompile(`^\[(Warning|Error)\] Component "([^"]+)" shouldn't import component "([^"]+)"`)

type pending struct {
	level string
	msg   string
	file  string
}

func parseHeader(line string) *pending {
	m := headerRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	level := "warning"
	if m[1] == "Error" {
		level = "error"
	}
	return &pending{level: level, msg: fmt.Sprintf("%s shouldn't import %s", m[2], m[3])}
}

func extractFile(line string) string {
	if !strings.HasPrefix(line, "  ") {
		return ""
	}
	trimmed := strings.TrimSpace(line)
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return ""
	}
	return trimmed[:idx]
}

// handleLine advances the parser state by one input line. A header line
// flushes any prior pending diagnostic and starts a new one; an indented
// continuation line records the file path on the in-flight pending.
func handleLine(line string, p *pending, b *sarif.Builder) *pending {
	if next := parseHeader(line); next != nil {
		emitPending(p, b)
		return next
	}
	if p != nil {
		if f := extractFile(line); f != "" {
			p.file = f
		}
	}
	return p
}

func emitPending(p *pending, b *sarif.Builder) {
	if p == nil {
		return
	}
	b.AddResult(ruleID, p.level, p.msg, p.file, 0, 0)
}

func Convert(r io.Reader, w io.Writer) error {
	br := bufio.NewReaderSize(r, 64*1024)

	b := sarif.NewBuilder("go-arch-lint", "")

	var p *pending
	var dropped int
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else {
			p = handleLine(string(raw), p, b)
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return fmt.Errorf("archlinttext: read: %w", err)
	}
	emitPending(p, b)
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "archlinttext: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)
	}
	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("archlinttext: encode: %w", err)
	}
	return nil
}
