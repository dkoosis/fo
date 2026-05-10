// Package wraparchlinttext converts go-arch-lint plain-text output into
// SARIF. The JSON output of go-arch-lint is already handled by
// wraparchlint; this wrapper exists for setups that pipe the text form
// (e.g. when the `--json` flag is unavailable or undesired in CI).
package wraparchlinttext

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

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

func Convert(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	b := sarif.NewBuilder("go-arch-lint", "")

	var p *pending
	flush := func() {
		if p == nil {
			return
		}
		b.AddResult(ruleID, p.level, p.msg, p.file, 0, 0)
		p = nil
	}

	for sc.Scan() {
		line := sc.Text()
		if next := parseHeader(line); next != nil {
			flush()
			p = next
			continue
		}
		if p == nil {
			continue
		}
		if f := extractFile(line); f != "" {
			p.file = f
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return fmt.Errorf("archlinttext: read: %w", err)
	}
	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("archlinttext: encode: %w", err)
	}
	return nil
}
