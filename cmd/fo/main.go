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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/dkoosis/fo/internal/detect"
	"github.com/dkoosis/fo/internal/report"
	"github.com/dkoosis/fo/pkg/jtbd"
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
  fo jtbd            JTBD coverage report from go test -json + // Serves: annotations

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
		case "jtbd":
			return runJTBD(args[1:], stdin, stdout, stderr)
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
	meta := newRunMeta(input)
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

	output := selectRenderer(mode, *themeFlag, meta).Render(patterns)
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

func selectRenderer(mode, themeName string, meta render.RunMeta) render.Renderer {
	switch mode {
	case "json":
		return render.NewJSON().WithMeta(meta)
	case "llm":
		return render.NewLLM().WithMeta(meta)
	default:
		theme := render.ThemeByName(themeName)
		if os.Getenv("NO_COLOR") != "" {
			theme = render.MonoTheme()
		}
		return render.NewHuman(theme)
	}
}

// newRunMeta builds an envelope with a stable hash of the input bytes
// (first 12 hex chars of sha256) and an RFC3339 timestamp.
func newRunMeta(input []byte) render.RunMeta {
	sum := sha256.Sum256(input)
	return render.RunMeta{
		DataHash:    hex.EncodeToString(sum[:])[:12],
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
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
// Failures propagate through TestTable fail items (real failures), Error
// patterns (parse failures), or JTBDCoverage with failing jobs.
// Summary is display-only, not a decision input.
func exitCode(patterns []pattern.Pattern) int {
	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.TestTable:
			for _, r := range v.Results {
				if r.Status == pattern.StatusFail {
					return 1
				}
			}
		case *pattern.JTBDCoverage:
			for _, e := range v.Entries {
				if e.Fail > 0 {
					return 1
				}
			}
		case *pattern.Error:
			return 1
		}
	}
	return 0
}

func runJTBD(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo jtbd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sourceFlag := fs.String("source", "", "Source root (default: discover from cwd via go.mod)")
	formatFlag := fs.String("format", "auto", "Output format: auto, human, llm, json")
	themeFlag := fs.String("theme", "default", "Theme: default, orca, mono")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	// Discover source root.
	source := *sourceFlag
	if source == "" {
		var err error
		source, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "fo jtbd: %v\n", err)
			return 2
		}
	}

	// Batch: read all stdin.
	input, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo jtbd: reading stdin: %v\n", err)
		return 2
	}
	if len(input) == 0 {
		fmt.Fprintf(stderr, "fo jtbd: no input on stdin\nUsage: go test -json ./... | fo jtbd\n")
		return 2
	}

	// Parse go test -json events for per-function results.
	events, malformed, err := parseEvents(input)
	if err != nil {
		fmt.Fprintf(stderr, "fo jtbd: parsing go test -json: %v\n", err)
		return 2
	}
	if malformed > 0 {
		fmt.Fprintf(stderr, "fo jtbd: warning: %d malformed line(s) skipped\n", malformed)
	}
	results := testjson.FuncResults(events)

	// Scan annotations from source.
	annotations, err := jtbd.Scan(source)
	if err != nil {
		fmt.Fprintf(stderr, "fo jtbd: scanning annotations: %v\n", err)
		return 2
	}

	// Load optional manifest.
	jobs := jtbd.LoadManifest(source)

	// Assemble coverage.
	entries := jtbd.Assemble(annotations, results, jobs, stderr)

	// Compute summary counts.
	totalJobs := len(entries)
	coveredJobs := 0
	hasFailing := false
	for _, e := range entries {
		if e.TestCount > 0 {
			coveredJobs++
		}
		if e.Fail > 0 {
			hasFailing = true
		}
	}

	// Map to pattern.
	p := mapper.MapJTBDCoverage(entries, totalJobs, coveredJobs)

	// Render.
	mode := resolveFormat(*formatFlag, stdout)
	output := selectRenderer(mode, *themeFlag, newRunMeta(input)).Render([]pattern.Pattern{p})
	fmt.Fprint(stdout, output)

	// Exit codes per D6: 0=pass/uncovered, 1=any failing, 2=fo error.
	if hasFailing {
		return 1
	}
	return 0
}

// parseEvents decodes go test -json NDJSON into raw TestEvents for FuncResults.
func parseEvents(data []byte) ([]testjson.TestEvent, int, error) {
	var events []testjson.TestEvent
	var malformed int
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var e testjson.TestEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			malformed++
			continue
		}
		events = append(events, e)
	}
	return events, malformed, nil
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
