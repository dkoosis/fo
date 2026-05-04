// Package wrapcover converts `go tool cover -func` output into fo's
// metrics format. Each per-function row becomes one metrics row keyed by
// "<path:line>:<func>"; the trailing "total:" row becomes "total".
package wrapcover

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func Convert(r io.Reader, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# fo:metrics tool=cover"); err != nil {
		return err
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pctTok := fields[len(fields)-1]
		if !strings.HasSuffix(pctTok, "%") {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(pctTok, "%"), 64)
		if err != nil {
			continue
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
		if _, err := fmt.Fprintf(w, "%s %s %%\n", key, strconv.FormatFloat(v, 'f', -1, 64)); err != nil {
			return err
		}
	}
	return sc.Err()
}
