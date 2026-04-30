// fo renders build tool output as information-dense terminal visualizations.
//
// Usage:
//
//	golangci-lint run --output.sarif.path=stdout ./... | fo
//	go test -json ./... | fo
//	go vet ./... 2>&1 | fo wrap diag --tool govet
//
// Accepts two input formats on stdin:
//   - SARIF 2.1.0 (static analysis results)
//   - go test -json (test execution results)
//
// Output formats (--format):
//
//	auto   — TTY → human, piped → llm (default)
//	human  — Tufte-Swiss styled terminal output
//	llm    — token-dense plain text, no ANSI
//	json   — machine-parseable Report JSON
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"runtime/debug"
	"strings"

	"golang.org/x/term"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/state"
	"github.com/dkoosis/fo/pkg/testjson"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
	"github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	"github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

const (
	formatHuman = "human"
	formatLLM   = "llm"
	formatJSON  = "json"
)

// version is the build version. Override with -ldflags "-X main.version=v1.2.3".
// When unset and the binary was installed via `go install`, falls back to the
// module version reported by debug.ReadBuildInfo.
var version = "dev"

var (
	errUnrecognizedInput    = errors.New("unrecognized input (expected SARIF or go test -json)")
	errTruncatedTestJSON    = errors.New("no complete events recovered (truncated stream?)")
	errUnknownFormat        = errors.New("unknown format (expected auto, human, llm, json)")
	errUnknownSectionFormat = errors.New("unknown section format")
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// resolveVersion returns the build version. If main.version was set via
// ldflags, it wins; otherwise fall back to module info from debug.ReadBuildInfo
// (populated by `go install module@version`).
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

const usage = `fo — focused build output renderer

USAGE
  <input-command> | fo [FLAGS]
  <tool-output>   | fo wrap <name> [FLAGS]

INPUT FORMATS (auto-detected from stdin)
  SARIF 2.1.0     Static analysis results (golangci-lint, gosec, etc.)
  go test -json   Test execution stream

OUTPUT FORMATS (--format)
  auto            TTY → human, piped → llm (default)
  human           Tufte-Swiss styled terminal output
  llm             Token-dense plain text, no ANSI
  json            Machine-parseable Report JSON

FLAGS
  --format <mode>     auto | human | llm | json (default: auto)
  --theme <name>      color | mono (default: auto — color on TTY, mono otherwise)
  --state-file <path> Sidecar state file (default: .fo/last-run.json)
  --no-state          Skip diff classification and sidecar I/O
  --state-strict      Exit non-zero (2) if sidecar Save fails
  --stream            Stream go test -json incrementally (avoids 256 MiB
                      input cap; enabled automatically on TTY+auto)

SUBCOMMANDS
  fo wrap <name>     Convert tool output to SARIF
  fo wrap --help     Show available wrappers
  fo --version       Print build version and exit

EXIT CODES
  0   Clean — no errors or test failures
  1   Failures — lint errors or test failures present
  2   Usage error — bad flags, unrecognized input, stdin problems
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "wrap":
			return runWrap(args[1:], stdin, stdout, stderr)
		case "state":
			return runState(args[1:], stdout, stderr)
		case "help", "-h", "--help":
			fmt.Fprint(stderr, usage)
			return 0
		case "version", "-version", "--version":
			fmt.Fprintln(stdout, resolveVersion())
			return 0
		}
		// Reject unknown non-flag positional args (e.g. typos like
		// `fo nonsense`). Otherwise the flag parser stops at the arg,
		// stdin is empty, and the user gets a misleading "no input" error.
		if !strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(stderr, "fo: unknown subcommand %q\n\nRun 'fo --help' for usage.\n", args[0])
			return 2
		}
	}

	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	formatFlag := fs.String("format", "auto", "Output format: auto, human, llm, json")
	themeFlag := fs.String("theme", "auto", "Theme: auto, color, mono")
	stateFile := fs.String("state-file", state.DefaultPath, "Sidecar state file path")
	noState := fs.Bool("no-state", false, "Skip diff classification and sidecar I/O")
	stateStrict := fs.Bool("state-strict", false, "Exit non-zero if sidecar Save fails")
	streamFlag := fs.Bool("stream", false, "Stream go test -json incrementally (avoids 256 MiB cap)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

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

	mode, err := resolveFormat(*formatFlag, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}

	// Streaming dispatch: go test -json input only.
	//   - TTY + format=auto → incremental render (existing path).
	//   - --stream (any format) → incremental parse, single batch render.
	// Non-go-test input (SARIF, multiplex) ignores --stream and falls
	// through to the batch path.
	if sniffGoTestJSON(peeked) {
		ttyAuto := *formatFlag == "auto" && isTTYWriter(stdout)
		switch {
		case ttyAuto:
			return runStream(stdin, br, stdout, resolveTheme(*themeFlag, stdout), *stateFile, *noState, *stateStrict, stderr)
		case *streamFlag:
			return runStreamBatch(stdin, br, stdout, mode, *themeFlag, *stateFile, *noState, *stateStrict, stderr)
		}
	}

	input, err := boundread.All(br, 0)
	if err != nil {
		if errors.Is(err, boundread.ErrInputTooLarge) {
			fmt.Fprintf(stderr, "fo: %v (pass --stream for go test -json to bypass this cap)\n", err)
		} else {
			fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		}
		return 2
	}

	r, err := parseToReport(input, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}

	saveErr := attachDiff(r, *stateFile, *noState, stderr)

	if err := renderMode(mode, r, stdout, *themeFlag); err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	if saveErr != nil && *stateStrict {
		return 2
	}
	return exitCodeReport(r)
}

func renderMode(mode string, r *report.Report, stdout io.Writer, themeName string) error {
	if mode == formatJSON {
		return writeReportJSON(stdout, r)
	}
	t := resolveTheme(themeName, stdout)
	viewMode := view.ModeHuman
	if mode == formatLLM {
		t = theme.Mono()
		viewMode = view.ModeLLM
	}
	width := termSize(stdout)
	if err := view.RenderReportMode(stdout, *r, t, width, viewMode); err != nil {
		return err
	}
	if mode == formatLLM {
		writeDiffDetail(stdout, r)
	}
	return nil
}

// sniffGoTestJSON returns true when peeked stdin starts with a go test -json
// event line. Inlined so the v2 dispatch doesn't import internal/detect.
func sniffGoTestJSON(data []byte) bool {
	data = bytes.TrimLeft(data, " \t\n\r")
	if len(data) == 0 || data[0] != '{' {
		return false
	}
	first := data
	if i := bytes.IndexAny(data, "\n\r"); i >= 0 {
		first = data[:i]
	}
	var ev struct {
		Action string `json:"Action"`
	}
	if err := json.Unmarshal(first, &ev); err != nil {
		return false
	}
	switch ev.Action {
	case "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip":
		return true
	}
	return false
}

// sniffSARIF returns true when data is a SARIF 2.1.0 document. Tolerates
// trailing text (golangci-lint v2 appends a summary).
func sniffSARIF(data []byte) bool {
	var probe struct {
		Version string            `json:"version"`
		Runs    []json.RawMessage `json:"runs"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&probe); err != nil {
		return false
	}
	return probe.Version == "2.1.0" && probe.Runs != nil
}

// parseToReport sniffs the input format and parses it into a *report.Report.
// Multi-tool delimiter protocol takes precedence; SARIF next; go test -json
// is the fallback when SARIF probe fails.
func parseToReport(input []byte, stderr io.Writer) (*report.Report, error) {
	if report.HasDelimiter(input) {
		return parseMultiplex(input, stderr)
	}
	trimmed := bytes.TrimLeft(input, " \t\n\r")
	if len(trimmed) == 0 {
		return nil, unrecognizedInputErr(input)
	}
	if trimmed[0] != '{' {
		return parseTestJSONTolerant(input, stderr)
	}
	if sniffSARIF(input) {
		doc, err := sarif.ReadBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing SARIF: %w", err)
		}
		return sarif.ToReportWithMeta(doc, input), nil
	}
	if sniffGoTestJSON(input) {
		results, malformed, err := testjson.ParseBytes(input)
		if err != nil {
			return nil, fmt.Errorf("parsing go test -json: %w", err)
		}
		if malformed > 0 {
			fmt.Fprintf(stderr, "fo: warning: %d malformed line(s) skipped\n", malformed)
		}
		return testjson.ToReportWithMeta(results, input), nil
	}
	return parseTestJSONTolerant(input, stderr)
}

// lineDiagPattern matches a typical compiler/linter line diagnostic:
//
//	path/to/file.ext:LINE[:COL]: message
//
// Path component must contain at least one non-colon, non-space char and
// commonly includes / or .; line/col are decimal; the trailing message
// must be non-empty. Conservative on purpose so URLs and timestamps don't
// false-positive.
var lineDiagPattern = regexp.MustCompile(`^[^:\s]*[./][^:\s]*:\d+(:\d+)?:\s+\S`)

// looksLikeLineDiagnostics returns true when input contains at least
// minHits lines matching lineDiagPattern. Used to suggest 'fo wrap diag'
// when stdin is raw compiler output rather than SARIF or go test -json
// (fo-tl4).
func looksLikeLineDiagnostics(input []byte) bool {
	const minHits = 2
	hits := 0
	for len(input) > 0 {
		nl := bytes.IndexByte(input, '\n')
		var line []byte
		if nl < 0 {
			line = input
			input = nil
		} else {
			line = input[:nl]
			input = input[nl+1:]
		}
		if lineDiagPattern.Match(bytes.TrimRight(line, "\r")) {
			hits++
			if hits >= minHits {
				return true
			}
		}
	}
	return false
}

// unrecognizedInputErr returns errUnrecognizedInput, optionally wrapped
// with a hint to pipe through 'fo wrap diag' when the input looks like
// raw line-diagnostic output (fo-tl4).
func unrecognizedInputErr(input []byte) error {
	if looksLikeLineDiagnostics(input) {
		return fmt.Errorf(
			"%w\nhint: input looks like line diagnostics — try piping through: fo wrap diag --tool <name>",
			errUnrecognizedInput,
		)
	}
	return errUnrecognizedInput
}

// hasJSONShapedLine reports whether any non-empty line in input begins with
// '{' after leading whitespace — a cheap heuristic that the caller intended
// to pipe NDJSON.
func hasJSONShapedLine(input []byte) bool {
	for len(input) > 0 {
		nl := bytes.IndexByte(input, '\n')
		var line []byte
		if nl < 0 {
			line = input
			input = nil
		} else {
			line = input[:nl]
			input = input[nl+1:]
		}
		trimmed := bytes.TrimLeft(line, " \t\r")
		if len(trimmed) > 0 && trimmed[0] == '{' {
			return true
		}
	}
	return false
}

// parseTestJSONTolerant attempts to parse input as go test -json even when
// it doesn't start with '{' — wrapped commands sometimes prepend banners or
// progress lines before the JSON stream. Accept iff at least one valid event
// parsed; otherwise distinguish three failure modes:
//   - parser IO error → wrap and return so operators see the real cause
//   - input had JSON-shaped lines but none parsed (malformed > 0, no results)
//     → return a precise truncated-stream diagnostic instead of the generic
//     'unrecognized input' (fo-6w5)
//   - no signal at all (no results, no malformed) → errUnrecognizedInput
func parseTestJSONTolerant(input []byte, stderr io.Writer) (*report.Report, error) {
	results, malformed, err := testjson.ParseBytes(input)
	if err != nil {
		return nil, fmt.Errorf("parsing go test -json: %w", err)
	}
	if len(results) == 0 {
		// Distinguish JSON-shaped-but-broken input (truncated stream) from
		// pure-prose input (wrong tool). If any line begins with '{', the
		// caller meant to feed go test -json — surface a parse diagnostic
		// instead of collapsing to errUnrecognizedInput (fo-6w5).
		if malformed > 0 && hasJSONShapedLine(input) {
			return nil, fmt.Errorf("parsing go test -json: %d line(s) failed to parse: %w", malformed, errTruncatedTestJSON)
		}
		return nil, unrecognizedInputErr(input)
	}
	if malformed > 0 {
		fmt.Fprintf(stderr, "fo: warning: %d malformed line(s) skipped\n", malformed)
	}
	return testjson.ToReportWithMeta(results, input), nil
}

// parseMultiplex parses a multi-tool delimited stream and merges every
// section's findings/tests into one Report. Per-section parse failures
// surface as synthetic error-severity findings so silent crashes can't
// masquerade as a clean run.
func parseMultiplex(input []byte, stderr io.Writer) (*report.Report, error) {
	sections, prelude, err := report.ParseSections(input)
	if err != nil {
		var ufe *report.UnknownFormatError
		if errors.As(err, &ufe) {
			return nil, fmt.Errorf(
				"%w\nhint: for raw line-diagnostic text (e.g. 'go vet', 'gofmt'), pipe through 'fo wrap diag --tool <name>' to produce SARIF",
				err,
			)
		}
		return nil, fmt.Errorf("parsing report sections: %w", err)
	}
	if len(prelude) > 0 {
		fmt.Fprintf(stderr, "fo: warning: %d byte(s) before first --- tool: --- delimiter discarded\n", len(prelude))
	}
	merged := &report.Report{Tool: "multi"}
	for _, sec := range sections {
		if f, ok := sectionStatusFinding(sec); ok {
			merged.Findings = append(merged.Findings, f)
		}
		body := bytes.TrimSpace(sec.Content)
		if len(body) == 0 {
			continue
		}
		sub, perr := parseSection(sec, body, stderr)
		if perr != nil {
			merged.Findings = append(merged.Findings, report.Finding{
				RuleID:   "fo/section-parse-error",
				Severity: report.SeverityError,
				Message:  fmt.Sprintf("tool=%s format=%s: %v", sec.Tool, sec.Format, perr),
			})
			continue
		}
		merged.Findings = append(merged.Findings, sub.Findings...)
		merged.Tests = append(merged.Tests, sub.Tests...)
		if sub.GeneratedAt.After(merged.GeneratedAt) {
			merged.GeneratedAt = sub.GeneratedAt
		}
	}
	return merged, nil
}

// sectionStatusFinding returns a synthetic finding for non-ok section statuses.
// Returns (finding, true) when the status warrants a finding; (_, false) for
// ok/clean/empty (normal execution).
func sectionStatusFinding(sec report.Section) (report.Finding, bool) {
	switch sec.Status {
	case report.StatusTimeout:
		return report.Finding{
			RuleID:   "fo/section-timeout",
			Severity: report.SeverityError,
			Message:  fmt.Sprintf("tool=%s timed out before producing output", sec.Tool),
		}, true
	case report.StatusError:
		return report.Finding{
			RuleID:   "fo/section-error",
			Severity: report.SeverityError,
			Message:  fmt.Sprintf("tool=%s exited with an error", sec.Tool),
		}, true
	case report.StatusPartial:
		return report.Finding{
			RuleID:   "fo/section-partial",
			Severity: report.SeverityWarning,
			Message:  fmt.Sprintf("tool=%s produced partial output (may have been interrupted)", sec.Tool),
		}, true
	case report.StatusSkipped:
		return report.Finding{
			RuleID:   "fo/section-skipped",
			Severity: report.SeverityNote,
			Message:  fmt.Sprintf("tool=%s was skipped", sec.Tool),
		}, true
	default:
		return report.Finding{}, false
	}
}

func parseSection(sec report.Section, body []byte, stderr io.Writer) (*report.Report, error) {
	switch sec.Format {
	case "sarif":
		doc, err := sarif.ReadBytes(body)
		if err != nil {
			return nil, fmt.Errorf("parsing SARIF: %w", err)
		}
		return sarif.ToReportWithMeta(doc, body), nil
	case "testjson":
		results, malformed, err := testjson.ParseBytes(body)
		if err != nil {
			return nil, fmt.Errorf("parsing go test -json: %w", err)
		}
		if malformed > 0 {
			fmt.Fprintf(stderr, "fo: warning: tool=%s %d malformed line(s) skipped\n", sec.Tool, malformed)
		}
		return testjson.ToReportWithMeta(results, body), nil
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownSectionFormat, sec.Format)
	}
}

func writeReportJSON(w io.Writer, r *report.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func resolveFormat(format string, w io.Writer) (string, error) {
	switch format {
	case "auto":
		if isTTYWriter(w) {
			return formatHuman, nil
		}
		return formatLLM, nil
	case formatHuman, formatLLM, formatJSON:
		return format, nil
	default:
		return "", fmt.Errorf("%w: %q", errUnknownFormat, format)
	}
}

// resolveTheme picks the theme. NO_COLOR env or non-TTY stdout forces mono;
// explicit --theme overrides auto.
func resolveTheme(name string, w io.Writer) theme.Theme {
	if os.Getenv("NO_COLOR") != "" {
		return theme.Mono()
	}
	switch name {
	case "color":
		return theme.Color()
	case "mono":
		return theme.Mono()
	default:
		return theme.Default(isTTYWriter(w))
	}
}

func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptor fits in int
}

func termSize(w io.Writer) int {
	width := 80
	if f, ok := w.(*os.File); ok {
		if tw, _, err := term.GetSize(int(f.Fd())); err == nil { //nolint:gosec // G115: term.GetSize takes fd from validated *os.File
			if tw > 0 {
				width = tw
			}
		}
	}
	return width
}

// runStream pumps go test -json events into per-package Report snapshots and
// hands them to view.RenderStream. One channel send per finished package
// keeps PickView's total-driven thresholds meaningful. Cancellation (SIGINT)
// closes the underlying reader so blocked Reads unblock promptly — fo-op6.
func runStream(stdin io.Reader, br *bufio.Reader, stdout io.Writer, t theme.Theme, stateFile string, noState, stateStrict bool, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runStreamCtx(ctx, stdin, br, stdout, t, stateFile, noState, stateStrict, stderr)
}

// runStreamCtx is runStream's testable core: cancellation root injected.
// Streams events incrementally — never buffers the whole stdin — so large
// CI runs cannot OOM and Ctrl-C exits within the next event boundary.
func runStreamCtx(ctx context.Context, stdin io.Reader, br *bufio.Reader, stdout io.Writer, t theme.Theme, stateFile string, noState, stateStrict bool, stderr io.Writer) int {
	if c, ok := stdin.(io.Closer); ok {
		stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
		defer stopClose()
	}
	width := termSize(stdout)

	// br already wraps stdin and holds the sniffed prefix. Wrap it as a
	// ReadCloser whose Close propagates to stdin (if closable) so
	// testjson.Stream's cancel path unblocks an in-flight Read.
	rc := &bufioReadCloser{Reader: br, closer: closerOf(stdin)}

	snapshots := make(chan report.Report, 8)
	finalCh := make(chan *report.Report, 1)
	parseErrCh := make(chan error, 1)
	saveErrCh := make(chan error, 1)

	go func() {
		defer close(snapshots)
		agg := testjson.NewAggregator()
		_, err := testjson.Stream(ctx, rc, func(e testjson.TestEvent) {
			agg.ProcessEvent(e)
			// Emit a snapshot only at package-finish events. Per-test
			// events would flood RenderStream and PickView.
			if e.Test == "" && (e.Action == "pass" || e.Action == "fail" || e.Action == "skip") {
				sendCoalesceSnapshot(ctx, snapshots, *testjson.ToReport(agg.Results()))
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			parseErrCh <- err
		}
		// Final snapshot with diff attached. Same code path as batch.
		r := testjson.ToReport(agg.Results())
		saveErrCh <- attachDiff(r, stateFile, noState, stderr)
		finalCh <- r
		select {
		case snapshots <- *r:
		case <-ctx.Done():
		}
	}()

	renderErr := view.RenderStream(ctx, stdout, snapshots, t, width)
	final := <-finalCh
	select {
	case perr := <-parseErrCh:
		fmt.Fprintf(stderr, "fo: %v\n", perr)
		return 2
	default:
	}
	if renderErr != nil && !errors.Is(renderErr, context.Canceled) {
		fmt.Fprintf(stderr, "fo: %v\n", renderErr)
		return 2
	}
	if saveErr := <-saveErrCh; saveErr != nil && stateStrict {
		return 2
	}
	return exitCodeReport(final)
}

// sendCoalesceSnapshot delivers snap to ch without blocking the parser when
// ch is full. If a slow renderer (or slow stdout writer) leaves stale
// snapshots queued, the oldest one is dropped to make room for the latest.
// Safe with a single producer goroutine. Closes fo-4qh: under rapid
// fan-out (1k packages) with a slow renderer, the parser previously blocked
// on a buffered channel send and delayed both progress and ctx cancellation.
func sendCoalesceSnapshot(ctx context.Context, ch chan report.Report, snap report.Report) {
	select {
	case ch <- snap:
		return
	case <-ctx.Done():
		return
	default:
	}
	// Channel full — drop one stale snapshot (single-producer invariant
	// means no other sender races us) and retry. Worst case the renderer
	// drains in parallel and our send finds an empty slot; either way we
	// never block.
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- snap:
	case <-ctx.Done():
	}
}

// runStreamBatch parses go test -json incrementally (so memory never grows
// with input size) but renders a single batch report in the requested mode.
// Used when --stream is set with format=human|llm|json and stdout is not a
// TTY-driving incremental render. Closes fo-frl: piped CI callers can opt
// into streaming and bypass the 256 MiB boundread cap.
func runStreamBatch(stdin io.Reader, br *bufio.Reader, stdout io.Writer, mode, themeName, stateFile string, noState, stateStrict bool, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if c, ok := stdin.(io.Closer); ok {
		stopClose := context.AfterFunc(ctx, func() { _ = c.Close() })
		defer stopClose()
	}
	rc := &bufioReadCloser{Reader: br, closer: closerOf(stdin)}

	agg := testjson.NewAggregator()
	_, err := testjson.Stream(ctx, rc, func(e testjson.TestEvent) {
		agg.ProcessEvent(e)
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	r := testjson.ToReport(agg.Results())
	saveErr := attachDiff(r, stateFile, noState, stderr)
	if err := renderMode(mode, r, stdout, themeName); err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	if saveErr != nil && stateStrict {
		return 2
	}
	return exitCodeReport(r)
}

// bufioReadCloser pairs a *bufio.Reader (carrying the sniffed prefix) with
// the underlying stdin's Close so context-cancel can interrupt blocked
// Reads. closer may be nil for non-closable stdin (tests, pipes).
type bufioReadCloser struct {
	*bufio.Reader
	closer io.Closer
}

func (b *bufioReadCloser) Close() error {
	if b.closer != nil {
		return b.closer.Close()
	}
	return nil
}

func closerOf(r io.Reader) io.Closer {
	if c, ok := r.(io.Closer); ok {
		return c
	}
	return nil
}

// exitCodeReport: 1 if any error finding or non-pass/skip test outcome.
func exitCodeReport(r *report.Report) int {
	if r == nil {
		return 0
	}
	for _, f := range r.Findings {
		if f.Severity == report.SeverityError {
			return 1
		}
	}
	for _, t := range r.Tests {
		switch t.Outcome {
		case report.OutcomeFail, report.OutcomePanic, report.OutcomeBuildError:
			return 1
		case report.OutcomePass, report.OutcomeSkip:
			// not a failure
		}
	}
	return 0
}

// runState handles `fo state <subcommand>`. Currently only `reset`.
func runState(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo state", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateFile := fs.String("state-file", state.DefaultPath, "Sidecar state file path")
	if len(args) == 0 {
		fmt.Fprintln(stderr, "fo state: subcommand required (reset)")
		return 2
	}
	sub := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	switch sub {
	case "reset":
		if err := state.Reset(*stateFile); err != nil {
			fmt.Fprintf(stderr, "fo state reset: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "fo: state reset (%s)\n", *stateFile)
		return 0
	default:
		fmt.Fprintf(stderr, "fo state: unknown subcommand %q\n", sub)
		return 2
	}
}

// wrapNames is the canonical list of `fo wrap` subcommands.
var wrapNames = []string{"archlint", "diag", "jscpd"}

var wrapDescriptions = map[string]string{
	"archlint": "Convert go-arch-lint JSON to SARIF",
	"diag":     "Convert line diagnostics (file:line:col: msg) to SARIF",
	"jscpd":    "Convert jscpd JSON duplication report to SARIF",
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

	name := args[0]
	switch name {
	case "archlint":
		fs := flag.NewFlagSet("fo wrap archlint", flag.ContinueOnError)
		fs.SetOutput(stderr)
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if err := wraparchlint.Convert(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "fo wrap archlint: %v\n", err)
			return 2
		}
		return 0
	case "jscpd":
		fs := flag.NewFlagSet("fo wrap jscpd", flag.ContinueOnError)
		fs.SetOutput(stderr)
		if err := fs.Parse(args[1:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			return 2
		}
		if err := wrapjscpd.Convert(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "fo wrap jscpd: %v\n", err)
			return 2
		}
		return 0
	case "diag":
		return runWrapDiag(args[1:], stdin, stdout, stderr)
	}

	fmt.Fprintf(stderr, "fo wrap: unknown wrapper %q\n\nAvailable wrappers: %s\n",
		name, strings.Join(wrapNames, ", "))
	return 2
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

func runWrapHelp(stderr io.Writer) int {
	fmt.Fprintf(stderr, "fo wrap: convert tool output to SARIF\n\n")
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
