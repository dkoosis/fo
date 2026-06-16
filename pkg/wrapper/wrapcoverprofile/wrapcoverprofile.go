// Package wrapcoverprofile converts a Go coverage profile (the file
// produced by `go test -coverprofile=…`) into SARIF 2.1.0, emitting one
// note-level finding per uncovered block. Unlike `fo wrap cover`, which
// summarizes `go tool cover -func` percentages into fo:metrics, this
// wrapper surfaces the *locations* that lack coverage so they flow through
// the normal findings pipeline — and, via the multiplex protocol, sit
// inline beside test failures and lint findings instead of in a separate
// report the reader has to correlate by hand.
//
// Coverprofile line format (after the `mode:` header):
//
//	name.go:startLine.startCol,endLine.endCol numStmt count
//
// A count of 0 means the block's statements never executed.
package wrapcoverprofile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/sarif"
)

const ruleUncovered = "uncovered"

// Convert reads a coverprofile from r and writes SARIF to w. Covered
// blocks and the mode header are skipped; every uncovered block becomes a
// note finding anchored at its start line/column.
func Convert(r io.Reader, w io.Writer) error {
	b := sarif.NewBuilder("cover", "")
	br := bufio.NewReaderSize(r, 64*1024)
	var dropped int
	for {
		line, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else {
			addBlock(b, string(line))
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return fmt.Errorf("reading coverprofile: %w", err)
	}
	_ = dropped // oversize coverprofile lines are malformed; silently skipped
	_, err := b.WriteTo(w)
	return err
}

// addBlock parses one coverprofile line and, when it describes an
// uncovered block, appends a note finding. Malformed lines and the
// `mode:` header are ignored.
func addBlock(b *sarif.Builder, line string) {
	file, startLine, startCol, stmts, count, ok := parseBlock(line)
	if !ok || count != 0 {
		return
	}
	msg := fmt.Sprintf("%d statement(s) uncovered", stmts)
	b.AddResult(ruleUncovered, sarif.LevelNote, msg, file, startLine, startCol)
}

// parseBlock parses "file:sl.sc,el.ec n count". ok is false for the
// header, blank lines, or anything that does not match the shape.
func parseBlock(line string) (file string, startLine, startCol, stmts, count int, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "mode:") {
		return "", 0, 0, 0, 0, false
	}
	// Split off the trailing " <numStmt> <count>".
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return "", 0, 0, 0, 0, false
	}
	count, err := strconv.Atoi(fields[len(fields)-1])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	stmts, err = strconv.Atoi(fields[len(fields)-2])
	if err != nil {
		return "", 0, 0, 0, 0, false
	}
	// The location is everything before those two trailing numbers; a file
	// path may legally contain spaces, so rejoin rather than assume one field.
	loc := strings.TrimSpace(strings.TrimSuffix(line, fields[len(fields)-1]))
	loc = strings.TrimSpace(strings.TrimSuffix(loc, fields[len(fields)-2]))
	file, startLine, startCol, ok = parseLocation(loc)
	if !ok {
		return "", 0, 0, 0, 0, false
	}
	return file, startLine, startCol, stmts, count, true
}

// parseLocation parses "file:startLine.startCol,endLine.endCol" into the
// file and start position. The end position is unused — a finding points
// at where the uncovered block begins.
func parseLocation(loc string) (file string, startLine, startCol int, ok bool) {
	head, _, found := strings.Cut(loc, ",") // head = file:startLine.startCol
	if !found {
		return "", 0, 0, false
	}
	colon := strings.LastIndexByte(head, ':')
	if colon < 0 {
		return "", 0, 0, false
	}
	file = head[:colon]
	startLine, startCol, ok = parseLineDotCol(head[colon+1:])
	if file == "" || !ok {
		return "", 0, 0, false
	}
	return file, startLine, startCol, true
}

func parseLineDotCol(s string) (line, col int, ok bool) {
	lineStr, colStr, found := strings.Cut(s, ".")
	if !found {
		return 0, 0, false
	}
	line, err := strconv.Atoi(lineStr)
	if err != nil {
		return 0, 0, false
	}
	col, err = strconv.Atoi(colStr)
	if err != nil {
		return 0, 0, false
	}
	return line, col, true
}
