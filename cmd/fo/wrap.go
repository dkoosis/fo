package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	"github.com/dkoosis/fo/pkg/wrapper/wraparchlinttext"
	"github.com/dkoosis/fo/pkg/wrapper/wrapcover"
	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	"github.com/dkoosis/fo/pkg/wrapper/wrapgobench"
	"github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
	"github.com/dkoosis/fo/pkg/wrapper/wrapleaderboard"
)

// wrapNames is the canonical list of `fo wrap` subcommands.
var wrapNames = []string{"archlint", "archlint-text", "cover", "diag", "gobench", "jscpd", "leaderboard"}

var wrapDescriptions = map[string]string{
	"archlint":      "Convert go-arch-lint JSON to SARIF",
	"archlint-text": "Convert go-arch-lint plain-text output to SARIF",
	"cover":         "Convert `go tool cover -func` output to fo:metrics",
	"diag":          "Convert line diagnostics (file:line:col: msg) to SARIF",
	"gobench":       "Convert raw `go test -bench` output to fo:metrics",
	"jscpd":         "Convert jscpd JSON duplication report to SARIF",
	"leaderboard":   "Convert '<count> <label>' tally to fo's tally format",
}

// plainConvert is a wrapper whose only behavior is "parse no flags, then
// run Convert(stdin, stdout)". The flagless wrappers all share this shape;
// keeping them in a table collapses runWrap to a single dispatch.
type plainConvert struct {
	flagSet string
	convert func(io.Reader, io.Writer) error
}

var plainWrappers = map[string]plainConvert{
	subArchlint:     {"fo wrap archlint", wraparchlint.Convert},
	subJSCPD:        {"fo wrap jscpd", wrapjscpd.Convert},
	"archlint-text": {"fo wrap archlint-text", wraparchlinttext.Convert},
	"cover":         {"fo wrap cover", wrapcover.Convert},
	"gobench":       {"fo wrap gobench", wrapgobench.Convert},
}

func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "fo wrap: wrapper name required\n\nAvailable wrappers: %s\n",
			strings.Join(wrapNames, ", "))
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		return runWrapHelp(stderr)
	}
	if args[0] == subList {
		return runWrapList(args[1:], stdout, stderr)
	}

	name := args[0]
	if pw, ok := plainWrappers[name]; ok {
		return runPlainWrap(pw, args[1:], stdin, stdout, stderr)
	}
	switch name {
	case subDiag:
		return runWrapDiag(args[1:], stdin, stdout, stderr)
	case subLeaderboard:
		return runWrapLeaderboard(args[1:], stdin, stdout, stderr)
	}

	fmt.Fprintf(stderr, "fo wrap: unknown wrapper %q\n\nAvailable wrappers: %s\n",
		name, strings.Join(wrapNames, ", "))
	return 2
}

// runPlainWrap parses (and rejects) flags for a flagless wrapper, then runs
// its Convert. Mirrors the per-wrapper arms it replaced exactly: flag.ErrHelp
// → 0, other parse error → 2, Convert error → "<flagSet>: <err>" on stderr + 2.
func runPlainWrap(pw plainConvert, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet(pw.flagSet, flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if err := pw.convert(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", pw.flagSet, err)
		return 2
	}
	return 0
}

func runWrapDiag(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo wrap diag", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts wrapdiag.DiagOpts
	fs.StringVar(&opts.Tool, "tool", "", "Tool name for SARIF driver.name (required)")
	fs.StringVar(&opts.Rule, "rule", "finding", "Default rule ID")
	fs.StringVar(&opts.Level, "level", "warning", "Default severity: error|warning|note")
	fs.StringVar(&opts.Version, "version", "", "Tool version string")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if err := wrapdiag.Convert(stdin, stdout, opts); err != nil {
		fmt.Fprintf(stderr, "fo wrap diag: %v\n", err)
		return 2
	}
	return 0
}

func runWrapLeaderboard(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo wrap leaderboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts wrapleaderboard.Opts
	fs.StringVar(&opts.Tool, "tool", "", "Tool name (recorded in tally header)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if err := wrapleaderboard.Convert(stdin, stdout, opts); err != nil {
		fmt.Fprintf(stderr, "fo wrap leaderboard: %v\n", err)
		return 2
	}
	return 0
}

func runWrapList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo wrap list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "Emit JSON array of {name, description}")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *asJSON {
		type entry struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		out := make([]entry, 0, len(wrapNames))
		for _, name := range wrapNames {
			out = append(out, entry{Name: name, Description: wrapDescriptions[name]})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "fo wrap list: %v\n", err)
			return 2
		}
		return 0
	}
	for _, name := range wrapNames {
		fmt.Fprintf(stdout, "%-12s %s\n", name, wrapDescriptions[name])
	}
	return 0
}

func runWrapHelp(stderr io.Writer) int {
	fmt.Fprintf(stderr, "fo wrap: convert tool output to SARIF or tally\n\n")
	for _, name := range wrapNames {
		fmt.Fprintf(stderr, "  %-12s %s\n", name, wrapDescriptions[name])
	}
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "  diag flags:")
	fmt.Fprintln(stderr, "    --tool <name>     Tool name for SARIF driver.name (required)")
	fmt.Fprintln(stderr, "    --rule <id>       Default rule ID (default: finding)")
	fmt.Fprintln(stderr, "    --level <sev>     Default severity: error|warning|note (default: warning)")
	fmt.Fprintln(stderr, "    --version <ver>   Tool version string")
	return 0
}
