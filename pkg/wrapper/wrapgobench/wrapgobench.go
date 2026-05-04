// Package wrapgobench converts raw `go test -bench` output into fo's
// metrics format. One metrics row is emitted per benchmark/metric pair
// (so "BenchmarkFoo  1234 ns/op  56 B/op  2 allocs/op" produces three
// rows). For benchstat tabular output (delta columns, geomean rows,
// confidence intervals), see the separate benchstat wrapper.
package wrapgobench

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var benchRe = regexp.MustCompile(`^(Benchmark[\w/.-]+)\s+\d+\s+(.+)$`)

// goMaxProcsSuffixRe strips the trailing "-<N>" GOMAXPROCS tag the
// runtime appends to bench names. Anchored so embedded hyphens survive.
var goMaxProcsSuffixRe = regexp.MustCompile(`-\d+$`)

func Convert(r io.Reader, w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# fo:metrics tool=gobench"); err != nil {
		return err
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		m := benchRe.FindStringSubmatch(line)
		if m == nil {
			continue
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
	}
	return sc.Err()
}

func unitKey(u string) string {
	return strings.NewReplacer("/", "_").Replace(u)
}
