// fo renders build tool output as information-dense terminal visualizations.
//
// Usage:
//
//	golangci-lint run --output.sarif.path=stdout ./... | fo
//	go test -json ./... | fo
//	go vet ./... 2>&1 | fo wrap sarif --tool govet
//
// Accepts two input formats on stdin:
//   - SARIF 2.1.0 (static analysis results)
//   - go test -json (test execution results)
//
// Output modes (auto-detected):
//
//	human  — styled Unicode output (default when TTY)
//	llm    — terse plain text for AI consumption (default when piped)
//	json   — structured JSON for automation
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/term"

	"github.com/dkoosis/fo/internal/detect"
	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/mapper"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/stream"
	"github.com/dkoosis/fo/pkg/testjson"
	"github.com/dkoosis/fo/pkg/wrapper"
	_ "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `fo — focused build output renderer

USAGE
  <input-command> | fo [FLAGS]
  <tool-output>   | fo wrap <name> [FLAGS]

INPUT FORMATS (auto-detected from stdin)
  SARIF 2.1.0     Static analysis results (golangci-lint, gosec, etc.)
  go test -json   Test execution stream (supports live + batch)

OUTPUT FORMATS (--format)
  auto            TTY → human, piped → llm (default)
  human           Styled Unicode with color and sparklines
  llm             Terse plain text, no ANSI — optimized for AI consumption
  json            Structured JSON for automation

FLAGS
  --format <mode>   Output format: auto | human | llm | json (default: auto)
  --theme <name>    Color theme: default | orca | mono (default: default)

SUBCOMMANDS
  fo wrap <name>     Convert tool output to SARIF or go-test-json
  fo wrap --help     Show available wrappers and their flags

EXIT CODES
  0   Clean — no errors or test failures
  1   Failures — lint errors or test failures present
  2   Usage error — bad flags, unrecognized input, stdin problems

EXAMPLES
  golangci-lint run --output.sarif.path=stdout ./... | fo
  go test -json ./... | fo
  go test -json ./... | fo --format llm
  go vet ./... 2>&1 | fo wrap diag --tool govet | fo
  gofmt -l ./... | fo wrap diag --tool gofmt --rule needs-formatting
  jscpd --reporters json . | fo wrap jscpd | fo

BEHAVIOR NOTES
  - Reads all input from stdin; does not accept file arguments
  - TTY auto-detection: human style when stdout is a TTY, LLM mode when piped
  - Live streaming mode activates for go test -json when stdout is a TTY
  - NO_COLOR env var forces mono theme
  - SARIF input supports multiple runs (multiple tools in one document)
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Check for subcommands before flag parsing
	if len(args) > 0 {
		switch args[0] {
		case "wrap":
			return runWrap(args[1:], stdin, stdout, stderr)
		case "help":
			fmt.Fprint(stderr, usage)
			return 0
		}
	}

	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	formatFlag := fs.String("format", "auto", "Output format: auto, human, llm, json")
	themeFlag := fs.String("theme", "default", "Theme: default, orca, mono")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// Peek stdin to detect format without consuming.
	// Peek returns err on short reads (io.EOF, ErrBufferFull) — those are fine
	// as long as we got bytes. A real read error with zero bytes is distinct.
	br := bufio.NewReaderSize(stdin, 8*1024)
	peeked, peekErr := br.Peek(4096)
	if len(peeked) == 0 {
		if peekErr != nil && peekErr != io.EOF {
			fmt.Fprintf(stderr, "fo: reading stdin: %v\n", peekErr)
		} else {
			fmt.Fprintf(stderr, "fo: no input on stdin\n")
		}
		return 2
	}

	format := detect.Sniff(peeked)

	// Stream mode: go test -json + TTY stdout + auto format
	if format == detect.GoTestJSON && isTTYWriter(stdout) && *formatFlag == "auto" {
		return runStream(stdin, br, stdout)
	}

	// Batch mode: read all input, parse, render.
	input, err := io.ReadAll(br)
	if err != nil {
		fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		return 2
	}
	if format == detect.Unknown {
		format = detect.Sniff(input)
	}

	patterns, err := parseInput(format, input, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}

	mode := resolveFormat(*formatFlag, stdout)
	switch mode {
	case "human", "llm", "json":
		// valid
	default:
		fmt.Fprintf(stderr, "fo: unknown format %q (expected auto, human, llm, json)\n", *formatFlag)
		return 2
	}

	output := selectRenderer(mode, *themeFlag).Render(patterns)
	fmt.Fprint(stdout, output)
	return exitCode(patterns)
}

// isTTYWriter reports whether w is a terminal.
func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptor fits in int on all supported platforms
}

// termSize returns the terminal dimensions for w, defaulting to 80x24.
func termSize(w io.Writer) (width, height int) {
	width, height = 80, 24
	if f, ok := w.(*os.File); ok {
		if tw, th, err := term.GetSize(int(f.Fd())); err == nil { //nolint:gosec // file descriptor fits in int on all supported platforms
			if tw > 0 {
				width = tw
			}
			if th > 0 {
				height = th
			}
		}
	}
	return width, height
}

// runStream handles the live streaming path (go test -json + TTY).
func runStream(stdin io.Reader, br *bufio.Reader, stdout io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Close the underlying reader on cancel to unblock Stream's scanner goroutine.
	// bufio.Reader doesn't implement io.Closer, so Stream can't close it itself.
	if c, ok := stdin.(io.Closer); ok {
		stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
		defer stopClose()
	}
	width, height := termSize(stdout)
	return stream.Run(ctx, br, stdout, width, height)
}

// parseInput parses raw bytes according to the detected format.
// Malformed-line warnings are written to stderr; parse failures return an error.
func parseInput(format detect.Format, input []byte, stderr io.Writer) ([]pattern.Pattern, error) {
	switch format {
	case detect.SARIF:
		doc, err := sarif.ReadBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing SARIF: %w", err)
		}
		return mapper.FromSARIF(doc), nil
	case detect.GoTestJSON:
		results, malformed, err := testjson.ParseBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing go test -json: %w", err)
		}
		if malformed > 0 {
			fmt.Fprintf(stderr, "fo: warning: %d malformed line(s) skipped\n", malformed)
		}
		return mapper.FromTestJSON(results), nil
	case detect.Report:
		sections, err := report.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("parsing report: %w", err)
		}
		return mapper.FromReport(sections), nil
	default:
		return nil, fmt.Errorf("unrecognized input format (expected SARIF, go test -json, or report)")
	}
}

func selectRenderer(mode, themeName string) render.Renderer {
	switch mode {
	case "json":
		return render.NewJSON()
	case "llm":
		return render.NewLLM()
	default:
		theme := render.ThemeByName(themeName)
		if os.Getenv("NO_COLOR") != "" {
			theme = render.MonoTheme()
		}
		return render.NewHuman(theme)
	}
}

func resolveFormat(format string, w io.Writer) string {
	if format != "auto" {
		return format
	}
	if isTTYWriter(w) {
		return "human"
	}
	return "llm"
}

// exitCode returns 0 for clean, 1 for failures present.
// Failures propagate through TestTable fail items (real failures) or Error
// patterns (parse failures). Summary is display-only, not a decision input.
func exitCode(patterns []pattern.Pattern) int {
	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.TestTable:
			for _, r := range v.Results {
				if r.Status == pattern.StatusFail {
					return 1
				}
			}
		case *pattern.Error:
			return 1
		}
	}
	return 0
}

func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "fo wrap: wrapper name required\n\nAvailable wrappers: %s\n",
			strings.Join(wrapper.Names(), ", "))
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintf(stderr, "fo wrap: convert tool output to SARIF or go-test-json\n\n")
		for _, name := range wrapper.Names() {
			fmt.Fprintf(stderr, "  %-12s %s\n", name, wrapper.Description(name))
			w := wrapper.Get(name)
			fs := flag.NewFlagSet(name, flag.ContinueOnError)
			w.RegisterFlags(fs)
			// Single iteration: print flags if any exist.
			fs.VisitAll(func(f *flag.Flag) {
				fmt.Fprintf(stderr, "    --%-10s %s", f.Name, f.Usage)
				if f.DefValue != "" && f.DefValue != "false" {
					fmt.Fprintf(stderr, " (default: %s)", f.DefValue)
				}
				fmt.Fprintln(stderr)
			})
			fmt.Fprintln(stderr)
		}
		return 0
	}
	w := wrapper.Get(args[0])
	if w == nil {
		fmt.Fprintf(stderr, "fo wrap: unknown wrapper %q\n\nAvailable wrappers: %s\n",
			args[0], strings.Join(wrapper.Names(), ", "))
		return 2
	}

	// Framework owns flag lifecycle — wrapper just registers its flags.
	fs := flag.NewFlagSet("fo wrap "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	w.RegisterFlags(fs)
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	if err := w.Convert(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "fo wrap %s: %v\n", args[0], err)
		return 2
	}
	return 0
}
