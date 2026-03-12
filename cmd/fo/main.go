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
//	terminal  — styled Unicode output (default when TTY)
//	llm       — terse plain text for AI consumption (default when piped)
//	json      — structured JSON for automation
package main

import (
	"bufio"
	"context"
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
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `fo — focused build output renderer

USAGE
  <input-command> | fo [FLAGS]
  <tool-output>   | fo wrap sarif --tool <name> [FLAGS]

INPUT FORMATS (auto-detected from stdin)
  SARIF 2.1.0     Static analysis results (golangci-lint, gosec, etc.)
  go test -json   Test execution stream (supports live + batch)
  report          Multi-tool delimited report (--- tool:X format:Y ---)

OUTPUT FORMATS (--format)
  auto            TTY → terminal, piped → llm (default)
  terminal        Styled Unicode with color and sparklines
  llm             Terse plain text, no ANSI — optimized for AI consumption
  json            Structured JSON for automation

FLAGS
  --format <mode>   Output format: auto | terminal | llm | json (default: auto)
  --theme <name>    Color theme: default | orca | mono (default: default)

SUBCOMMANDS
  fo wrap sarif    Convert line-based diagnostics to SARIF 2.1.0
    --tool <name>    Tool name for SARIF driver (required)
    --rule <id>      Default rule ID (default: finding)
    --level <level>  Severity: error | warning | note (default: warning)
    --version <str>  Tool version string

EXIT CODES
  0   Clean — no errors or test failures
  1   Failures — lint errors or test failures present
  2   Usage error — bad flags, unrecognized input, stdin problems

EXAMPLES
  golangci-lint run --output.sarif.path=stdout ./... | fo
  go test -json ./... | fo
  go test -json ./... | fo --format llm
  go vet ./... 2>&1 | fo wrap sarif --tool govet | fo
  gofmt -l ./... | fo wrap sarif --tool gofmt --rule needs-formatting

BEHAVIOR NOTES
  - Reads all input from stdin; does not accept file arguments
  - TTY auto-detection: terminal style when stdout is a TTY, LLM mode when piped
  - Live streaming mode activates for go test -json when stdout is a TTY
  - NO_COLOR env var forces mono theme
  - SARIF input supports multiple runs (multiple tools in one document)
  - Report format: sections delimited by "--- tool:<name> format:<fmt> ---"
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Check for subcommands before flag parsing
	if len(args) > 0 && args[0] == "wrap" {
		return runWrap(args[1:], stdin, stdout, stderr)
	}

	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	formatFlag := fs.String("format", "auto", "Output format: auto, terminal, llm, json")
	themeFlag := fs.String("theme", "default", "Theme: default, orca, mono")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Peek stdin to detect format without consuming
	br := bufio.NewReaderSize(stdin, 8*1024)
	peeked, _ := br.Peek(4096)
	if len(peeked) == 0 {
		fmt.Fprintf(stderr, "fo: no input on stdin\n")
		return 2
	}

	format := detect.Sniff(peeked)

	// Stream mode: go test -json + TTY stdout + auto format
	if format == detect.GoTestJSON && isTTYWriter(stdout) && *formatFlag == "auto" {
		return runStream(stdin, br, stdout)
	}

	// Batch mode
	patterns, code := runBatch(br, format, *formatFlag, *themeFlag, stdout, stderr)
	if code >= 0 {
		return code
	}

	output := selectRenderer(resolveFormat(*formatFlag, stdout), *themeFlag, stdout).Render(patterns)
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
	return stream.Run(ctx, br, stdout, width, height, nil)
}

// runBatch reads, detects, parses, and validates input in batch mode.
// Returns (patterns, -1) on success; (nil, exitCode) on error.
func runBatch(br *bufio.Reader, format detect.Format, formatFlag, themeFlag string, stdout, stderr io.Writer) ([]pattern.Pattern, int) {
	input, err := io.ReadAll(br)
	if err != nil {
		fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		return nil, 2
	}
	if len(input) == 0 {
		fmt.Fprintf(stderr, "fo: no input on stdin\n")
		return nil, 2
	}
	if format == detect.Unknown {
		format = detect.Sniff(input)
	}

	patterns, parseCode := parseInput(format, input, stderr)
	if parseCode >= 0 {
		return nil, parseCode
	}

	mode := resolveFormat(formatFlag, stdout)
	switch mode {
	case "terminal", "llm", "json":
		// valid
	default:
		fmt.Fprintf(stderr, "fo: unknown format %q (expected auto, terminal, llm, json)\n", formatFlag)
		return nil, 2
	}
	_ = themeFlag // consumed by caller via selectRenderer
	return patterns, -1
}

// parseInput parses raw bytes according to the detected format.
// Returns (patterns, -1) on success; (nil, exitCode) on error.
func parseInput(format detect.Format, input []byte, stderr io.Writer) ([]pattern.Pattern, int) {
	switch format {
	case detect.SARIF:
		doc, err := sarif.ReadBytes(input)
		if err != nil {
			fmt.Fprintf(stderr, "fo: parsing SARIF: %v\n", err)
			return nil, 2
		}
		return mapper.FromSARIF(doc), -1
	case detect.GoTestJSON:
		results, malformed, err := testjson.ParseBytes(input)
		if err != nil {
			fmt.Fprintf(stderr, "fo: parsing go test -json: %v\n", err)
			return nil, 2
		}
		if malformed > 0 {
			fmt.Fprintf(stderr, "fo: warning: %d malformed line(s) skipped\n", malformed)
		}
		return mapper.FromTestJSON(results), -1
	case detect.Report:
		sections, err := report.Parse(input)
		if err != nil {
			fmt.Fprintf(stderr, "fo: parsing report: %v\n", err)
			return nil, 2
		}
		return mapper.FromReport(sections), -1
	default:
		fmt.Fprintf(stderr, "fo: unrecognized input format (expected SARIF, go test -json, or report)\n")
		return nil, 2
	}
}

func selectRenderer(mode, themeName string, w io.Writer) render.Renderer {
	switch mode {
	case "json":
		return render.NewJSON()
	case "llm":
		return render.NewLLM()
	default:
		theme := render.ThemeByName(themeName)
		// Honor NO_COLOR
		if os.Getenv("NO_COLOR") != "" {
			theme = render.MonoTheme()
		}
		width := 80
		if f, ok := w.(*os.File); ok {
			if tw, _, err := term.GetSize(int(f.Fd())); err == nil && tw > 0 { //nolint:gosec // file descriptor fits in int on all supported platforms
				width = tw
			}
		}
		return render.NewTerminal(theme, width)
	}
}

func resolveFormat(format string, w io.Writer) string {
	if format != "auto" {
		return format
	}
	// Auto-detect: TTY = terminal, piped = llm
	if f, ok := w.(*os.File); ok {
		if term.IsTerminal(int(f.Fd())) { //nolint:gosec // file descriptor fits in int on all supported platforms
			return "terminal"
		}
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
				if r.Status == "fail" {
					return 1
				}
			}
		case *pattern.Error:
			return 1
		}
	}
	return 0
}

// --- fo wrap sarif subcommand ---

const wrapUsage = `fo wrap sarif — convert line diagnostics to SARIF 2.1.0

USAGE
  <tool> ... 2>&1 | fo wrap sarif --tool <name> [FLAGS]

Reads file:line:col: message lines from stdin, emits SARIF JSON to stdout.
Supports three line formats:
  file.go:15:3: message      (file, line, column, message)
  file.go:42: message        (file, line, message)
  file.go                    (file only — uses "needs formatting")

FLAGS
  --tool <name>     Tool name for SARIF driver.name (required)
  --rule <id>       Default rule ID (default: finding)
  --level <level>   Severity: error | warning | note (default: warning)
  --version <str>   Tool version string

EXAMPLES
  go vet ./... 2>&1 | fo wrap sarif --tool govet
  gofmt -l ./...    | fo wrap sarif --tool gofmt --rule needs-formatting
  staticcheck ./... | fo wrap sarif --tool staticcheck --level error
`

func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "fo wrap: subcommand required (sarif, jscpd, archlint)\n\n")
		fmt.Fprint(stderr, wrapUsage)
		return 2
	}
	switch args[0] {
	case "sarif":
		// fall through to sarif handling below
	case "jscpd":
		return runWrapJscpd(stdin, stdout, stderr)
	case "archlint":
		return runWrapArchlint(stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "fo wrap: unknown subcommand %q (expected sarif, jscpd, archlint)\n\n", args[0])
		fmt.Fprint(stderr, wrapUsage)
		return 2
	}

	fs := flag.NewFlagSet("fo wrap sarif", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, wrapUsage) }
	toolName := fs.String("tool", "", "Tool name for SARIF driver.name (required)")
	ruleID := fs.String("rule", "finding", "Default rule ID")
	level := fs.String("level", "warning", "Default severity: error|warning|note")
	version := fs.String("version", "", "Tool version string")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	if *toolName == "" {
		fmt.Fprintf(stderr, "fo wrap sarif: --tool is required\n")
		return 2
	}

	b := sarif.NewBuilder(*toolName, *version)
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		file, ln, col, msg := parseDiagLine(line)
		if file == "" {
			continue // silently drop unrecognized lines
		}
		b.AddResult(*ruleID, *level, msg, file, ln, col)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "fo wrap sarif: reading stdin: %v\n", err)
		return 2
	}

	if _, err := b.WriteTo(stdout); err != nil {
		fmt.Fprintf(stderr, "fo wrap sarif: writing output: %v\n", err)
		return 2
	}

	return 0
}

// parseDiagLine parses Go diagnostic formats:
//  1. file.go:line:col: message
//  2. file.go:line: message
//  3. path/to/file.go  (file-only, e.g., gofmt -l)
//
// Handles Windows drive-letter prefixes (e.g. C:\path\file.go:10:5: msg).
func parseDiagLine(line string) (file string, ln, col int, msg string) {
	rest := line
	var prefix string

	// Strip Windows drive letter (e.g. "C:") so the colon-split works.
	if len(rest) >= 3 && rest[1] == ':' && (rest[2] == '\\' || rest[2] == '/') {
		prefix = rest[:2]
		rest = rest[2:]
	}

	// Try file:line:col: message
	parts := strings.SplitN(rest, ":", 4)
	if len(parts) >= 4 {
		var l, c int
		if _, err := fmt.Sscanf(parts[1], "%d", &l); err == nil {
			if _, err := fmt.Sscanf(parts[2], "%d", &c); err == nil {
				return prefix + parts[0], l, c, strings.TrimSpace(parts[3])
			}
		}
	}

	// Try file:line: message
	if len(parts) >= 3 {
		var l int
		if _, err := fmt.Sscanf(parts[1], "%d", &l); err == nil {
			return prefix + parts[0], l, 0, strings.TrimSpace(strings.Join(parts[2:], ":"))
		}
	}

	// Try file-only (must end in .go or have path separators)
	trimmed := strings.TrimSpace(line)
	if strings.HasSuffix(trimmed, ".go") || strings.Contains(trimmed, "/") {
		if !strings.Contains(trimmed, " ") {
			return trimmed, 0, 0, "needs formatting"
		}
	}

	return "", 0, 0, ""
}
