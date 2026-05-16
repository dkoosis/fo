// Package wrapgobench converts raw `go test -bench` output into fo's
// metrics format. One metrics row is emitted per benchmark/metric pair
// (so "BenchmarkFoo  1234 ns/op  56 B/op  2 allocs/op" produces three
// rows). For benchstat tabular output (delta columns, geomean rows,
// confidence intervals), see the separate benchstat wrapper.
package wrapgobench

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
)

var benchRe = regexp.MustCompile(`^(Benchmark[\w/.-]+)\s+\d+\s+(.+)$`)

// goMaxProcsSuffixRe strips the trailing "-<N>" GOMAXPROCS tag the
// runtime appends to bench names. Anchored so embedded hyphens survive.
var goMaxProcsSuffixRe = regexp.MustCompile(`-\d+$`)

func Convert(r io.Reader, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# fo:metrics tool=gobench"); err != nil {
		return err
	}
	br := bufio.NewReaderSize(r, 64*1024)
	var dropped int
	for {
		raw, oversize, err := lineread.Read(br)
		if oversize {
			dropped++
		} else {
			if werr := emitBenchRow(w, string(raw)); werr != nil {
				return werr
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return fmt.Errorf("wrap gobench: read: %w", err)
	}
	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "wrap gobench: dropped %d line(s) exceeding %d bytes\n", dropped, lineread.MaxLineLen)
	}
	return nil
}

func emitBenchRow(w io.Writer, line string) error {
	m := benchRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	name := goMaxProcsSuffixRe.ReplaceAllString(m[1], "")
	fields := strings.Fields(m[2])
	for i := 0; i+1 < len(fields); i += 2 {
		vTok := fields[i]
		unit := fields[i+1]
		if _, err := strconv.ParseFloat(vTok, 64); err != nil {
			continue
		}
		key := fmt.Sprintf("%s/%s", name, unitKey(unit))
		if _, err := fmt.Fprintf(w, "%s %s %s\n", key, vTok, unit); err != nil {
			return err
		}
	}
	return nil
}

func unitKey(u string) string {
	return strings.NewReplacer("/", "_").Replace(u)
}
