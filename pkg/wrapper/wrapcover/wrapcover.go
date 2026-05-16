// Package wrapcover converts `go tool cover -func` output into fo's
// metrics format. Each per-function row becomes one metrics row keyed by
// "<path:line>:<func>"; the trailing "total:" row becomes "total".
package wrapcover

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
)

func Convert(r io.Reader, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# fo:metrics tool=cover"); err != nil {
		return err
	}
	br := bufio.NewReaderSize(r, 64*1024)
	var dropped int
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else if werr := emitCoverRow(w, string(raw)); werr != nil {
			return werr
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return fmt.Errorf("wrap cover: read: %w", err)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "wrap cover: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)
	}
	return nil
}

func emitCoverRow(w io.Writer, line string) error {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil
	}
	pctTok := fields[len(fields)-1]
	if !strings.HasSuffix(pctTok, "%") {
		return nil
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(pctTok, "%"), 64)
	if err != nil {
		return nil
	}
	var key string
	switch fields[0] {
	case "total:":
		key = "total"
	default:
		loc := strings.TrimSuffix(fields[0], ":")
		fn := ""
		if len(fields) >= 3 {
			fn = fields[1]
		}
		key = loc + ":" + fn
	}
	_, err = fmt.Fprintf(w, "%s %s %%\n", key, strconv.FormatFloat(v, 'f', -1, 64))
	return err
}
