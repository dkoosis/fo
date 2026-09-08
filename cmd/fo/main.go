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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"golang.org/x/term"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/metrics"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/state"
	"github.com/dkoosis/fo/pkg/status"
	"github.com/dkoosis/fo/pkg/tally"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/wrapper/wrapleaderboard"
)

const (
	formatHuman  = "human"
	formatLLM    = "llm"
	formatJSON   = "json"
	formatGitHub = "github"
	// formatCast emits an asciinema v2 recording. It is Scene-native:
	// only `# fo:scene` input animates, so other renderers reject it.
	formatCast = "cast"
)

// CLI flag and subcommand names. Centralized to satisfy goconst and so
// help/usage strings stay aligned with parsing.
const (
	flagFormat    = "--format"
	flagStateFile = "--state-file"
	flagNoState   = "--no-state"
	flagRule      = "--rule"
	flagTool      = "--tool"
	flagHelp      = "--help"

	subState       = "state"
	subSuppress    = "suppress"
	subWatch       = "watch"
	subExplain     = "explain"
	subTrend       = "trend"
	subReplay      = "replay"
	subWrap        = "wrap"
	subDiag        = "diag"
	subLeaderboard = "leaderboard"
	subArchlint    = "archlint"
	subJSCPD       = "jscpd"
	subGofmt       = "gofmt"
)

// version is the build version. Override with -ldflags "-X main.version=v1.2.3".
// When unset and the binary was installed via `go install`, falls back to the
// module version reported by debug.ReadBuildInfo.
var version = "dev"

var (
	errUnrecognizedInput    = errors.New("unrecognized input (expected SARIF or go test -json)")
	errTruncatedTestJSON    = errors.New("no complete events recovered (truncated stream?)")
	errUnknownFormat        = errors.New("unknown format (expected auto, human, llm, json, github)")
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
  tally           Count→label distribution (# fo:tally header) → leaderboard
  status          PASS/FAIL/WARN/SKIP rows (# fo:status header) → table
  metrics         Keyed numeric values (# fo:metrics header) → list with deltas
  scene           Narrated multi-actor walk-through (# fo:scene header) → story

OUTPUT FORMATS (--format)
  auto            TTY → human, piped → llm (default)
  human           Tufte-Swiss styled terminal output
  llm             Token-dense plain text, no ANSI
  json            Machine-parseable Report JSON
  github          GitHub Actions annotations (::error/::warning/::notice),
                  scoped to new findings when a diff baseline exists

FLAGS
  --format <mode>     auto | human | llm | json | github (default: auto)
  --theme <name>      color | mono (default: auto — color on TTY, mono otherwise)
  --state-file <path> Sidecar state file (default: .fo/last-run.json)
  --no-state          Skip diff classification and sidecar I/O
  --state-strict      Exit non-zero (2) if sidecar Save fails
  --stream            Stream go test -json incrementally (avoids 256 MiB
                      input cap; enabled automatically on TTY+auto)
  --as <kind>         Hint format when stdin lacks a fo header
                      (tally|status|metrics|diag)

SUBCOMMANDS
  fo wrap <name>             Convert tool output to SARIF
  fo wrap list               List wrappers (--json for machine output)
  fo wrap --help             Show available wrappers
  fo watch -- <cmd>          Run <cmd>, render output, rerun on stdin newline (A.1)
  fo explain <id>            Expand a handle (F-7a2/T-3f1) from the last run
  fo trend <rule-id>         Chart a rule's count across recorded runs (sparkline)
  fo replay [--since=<dur>]   List recent runs with headline counts
  fo suppress add|list|rm    Manage .fo/ignore suppressions (rule-id, glob, expiry)
  fo state reset             Clear diff classification baseline
  fo --version               Print build version and exit
  fo --print-schema          Print JSON Schema for Report (--format json output) and exit

EXAMPLES
  # Static analysis (SARIF) — golangci-lint v2
  golangci-lint run --output.sarif.path=stdout ./... | fo

  # Test stream
  go test -json ./... | fo

  # Streaming for large CI runs (bypasses 256 MiB cap)
  go test -json ./... | fo --stream

  # Raw line diagnostics → SARIF → fo
  go vet ./... 2>&1 | fo wrap diag --tool govet | fo

  # Count→label tally → leaderboard
  sort | uniq -c | sort -rn | fo wrap leaderboard --tool kg-types | fo

  # Force a format regardless of TTY
  go test -json ./... | fo --format llm
  go test -json ./... | fo --format json

  # Reset diff classification baseline
  fo state reset

EXIT CODES
  0   Clean — no errors or test failures
  1   Failures — lint errors or test failures present
  2   Usage error — bad flags, unrecognized input, stdin problems
`

// run is the top-level dispatcher: subcommand routing, flag parsing,
// format sniffing, the hygiene-format dispatch chain, the general
// SARIF/go-test parse path, diff classification, and rendering. Each
// phase is a named helper below so it can be read and tested on its
// own; run() itself is just the sequencing plus early-return plumbing.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if code, handled := dispatchSubcommand(args, stdin, stdout, stderr); handled {
		return code
	}

	rf, code, ok := parseRunFlags(args, stderr)
	if !ok {
		return code
	}

	br, peeked, code, ok := peekStdin(stdin, stderr)
	if !ok {
		return code
	}

	mode, policy, code, ok := resolveModeAndPolicy(rf, stdout, stderr)
	if !ok {
		return code
	}

	// Streaming dispatch: go test -json input only.
	//   - TTY + format=auto → incremental render (existing path).
	//   - --stream (any format) → incremental parse, single batch render.
	// Non-go-test input (SARIF, multiplex) ignores --stream and falls
	// through to the batch path.
	if code, handled := dispatchGoTestStream(peeked, mode, rf, policy, stdin, br, stdout, stderr); handled {
		return code
	}

	input, rawInput, code, ok := readInput(br, rf.as, stderr)
	if !ok {
		return code
	}

	if code, handled := dispatchHygieneFormat(input, mode, rf.theme, stdout, stderr); handled {
		return code
	}

	r, code, ok := buildReport(input, stderr)
	if !ok {
		return code
	}
	saveErr := classifyAndRecord(r, rawInput, rf.stateFile, policy, stderr)

	warnUnknownExpandIDs(mode, r, rf.expand, stderr)

	return finishRender(mode, r, stdout, rf.theme, rf.expand, saveErr, policy, stderr)
}

// dispatchSubcommand handles the args that precede normal flag parsing:
// known subcommands (wrap, state, suppress, watch, explain, trend,
// replay), the help/version/schema shortcuts, and rejection of an
// unknown non-flag positional arg. handled is false when args should
// fall through to normal flag parsing (empty args, or args[0] is a flag).
func dispatchSubcommand(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int, handled bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case subWrap:
		return runWrap(args[1:], stdin, stdout, stderr), true
	case subState:
		return runState(args[1:], stdout, stderr), true
	case subSuppress:
		return runSuppress(args[1:], stdout, stderr), true
	case subWatch:
		return runWatch(args[1:], stdin, stdout, stderr), true
	case subExplain:
		return runExplain(args[1:], stdout, stderr), true
	case subTrend:
		return runTrend(args[1:], stdout, stderr), true
	case subReplay:
		return runReplay(args[1:], stdout, stderr), true
	case "help", "-h", flagHelp:
		fmt.Fprint(stderr, usage)
		return 0, true
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, resolveVersion())
		return 0, true
	case "-print-schema", "--print-schema":
		fmt.Fprint(stdout, report.Schema())
		return 0, true
	}
	// Reject unknown non-flag positional args (e.g. typos like
	// `fo nonsense`). Otherwise the flag parser stops at the arg,
	// stdin is empty, and the user gets a misleading "no input" error.
	if !strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(stderr, "fo: unknown subcommand %q\n\nRun 'fo --help' for usage.\n", args[0])
		return 2, true
	}
	return 0, false
}

// runFlags holds the parsed flags for the default (non-subcommand) run
// path, so the pipeline stages below can pass them around as a unit
// instead of a long parameter list of pointers.
type runFlags struct {
	format      string
	theme       string
	stateFile   string
	noState     bool
	stateStrict bool
	stream      bool
	as          string
	expand      []string
}

// parseRunFlags parses the default run flag set. ok is false when the
// caller should return code directly (help was printed, or a parse
// error occurred).
func parseRunFlags(args []string, stderr io.Writer) (rf *runFlags, code int, ok bool) {
	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	rf = &runFlags{}
	fs.StringVar(&rf.format, "format", "auto", "Output format: auto, human, llm, json, github")
	fs.StringVar(&rf.theme, "theme", "auto", "Theme: auto, color, mono")
	fs.StringVar(&rf.stateFile, "state-file", state.Path(), "Sidecar state file path")
	fs.BoolVar(&rf.noState, "no-state", false, "Skip diff classification and sidecar I/O")
	fs.BoolVar(&rf.stateStrict, "state-strict", false, "Exit non-zero if sidecar Save fails")
	fs.BoolVar(&rf.stream, "stream", false, "Stream go test -json incrementally (avoids 256 MiB cap)")
	fs.StringVar(&rf.as, "as", "", "Hint format when auto-detection is ambiguous: tally|status|metrics|diag")
	fs.Func("expand", "Reveal cluster members; value is a cluster ID or 'all'. Repeatable.", func(v string) error {
		rf.expand = append(rf.expand, v)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, 0, false
		}
		return nil, 2, false
	}
	return rf, 0, true
}

// peekStdin validates that stdin is piped rather than an interactive
// terminal, then buffers it and peeks the bytes used for format
// sniffing. ok is false when the caller should return code directly.
func peekStdin(stdin io.Reader, stderr io.Writer) (br *bufio.Reader, peeked []byte, code int, ok bool) {
	// Short-circuit when stdin is a terminal: Peek would block waiting for
	// EOF (Ctrl-D) and the user sees a hang. fo only consumes piped input.
	if f, isFile := stdin.(*os.File); isFile && term.IsTerminal(int(f.Fd())) {
		fmt.Fprintf(stderr, "fo: no input on stdin (pipe data in or run 'fo --help')\n")
		return nil, nil, 2, false
	}

	br = bufio.NewReaderSize(stdin, 8*1024)
	peeked, peekErr := br.Peek(4096)
	if len(peeked) == 0 {
		if peekErr != nil && peekErr != io.EOF {
			fmt.Fprintf(stderr, "fo: reading stdin: %v\n", peekErr)
		} else {
			fmt.Fprintf(stderr, "fo: no input on stdin\n")
		}
		return nil, nil, 2, false
	}
	return br, peeked, 0, true
}

// resolveModeAndPolicy resolves the output mode (auto→TTY-detected) and
// the sidecar state policy from the parsed flags. ok is false when the
// caller should return code directly.
func resolveModeAndPolicy(rf *runFlags, stdout io.Writer, stderr io.Writer) (mode string, policy statePolicy, code int, ok bool) {
	mode, err := resolveFormat(rf.format, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return "", 0, 2, false
	}
	policy, perr := resolveStatePolicy(rf.noState, rf.stateStrict)
	if perr != nil {
		fmt.Fprintf(stderr, "fo: %v\n", perr)
		return "", 0, 2, false
	}
	return mode, policy, 0, true
}

// dispatchGoTestStream handles the two streaming fast paths for
// go test -json input: TTY+format=auto renders incrementally, and
// --stream (any format) parses incrementally with a single batch
// render. handled is false when input isn't go-test JSON, or is but
// matches neither case (falls through to the batch parse path).
func dispatchGoTestStream(peeked []byte, mode string, rf *runFlags, policy statePolicy, stdin io.Reader, br *bufio.Reader, stdout, stderr io.Writer) (code int, handled bool) {
	if !sniffGoTestJSON(peeked) {
		return 0, false
	}
	ttyAuto := rf.format == "auto" && isTTYWriter(stdout)
	switch {
	case ttyAuto:
		return runStream(streamOpts{
			stdin: stdin, br: br, stdout: stdout, stderr: stderr,
			theme: resolveTheme(rf.theme, stdout), stateFile: rf.stateFile, policy: policy,
		}), true
	case rf.stream:
		return runStreamBatch(streamOpts{
			stdin: stdin, br: br, stdout: stdout, stderr: stderr,
			mode: mode, themeName: rf.theme, stateFile: rf.stateFile, policy: policy,
		}), true
	}
	return 0, false
}

// readInput reads the full bounded stdin body and applies the optional
// --as coercion hint. ok is false when the caller should return code
// directly.
func readInput(br *bufio.Reader, as string, stderr io.Writer) (input, rawInput []byte, code int, ok bool) {
	input, err := boundread.All(br, 0)
	if err != nil {
		if errors.Is(err, boundread.ErrInputTooLarge) {
			fmt.Fprintf(stderr, "fo: %v (pass --stream for go test -json to bypass this cap)\n", err)
		} else {
			fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		}
		return nil, nil, 2, false
	}

	rawInput = input
	if as != "" {
		coerced, code := coerceAs(as, input, stderr)
		if code != 0 {
			return nil, nil, code, false
		}
		input = coerced
	}
	return input, rawInput, 0, true
}

// dispatchHygieneFormat renders the small structured "hygiene" formats
// (tally, status, metrics, scene) and the cast/bare-tally special
// cases — all of which bypass the general SARIF/go-test Report
// pipeline. handled is false when input matches none of these and
// should continue to the general parse path.
func dispatchHygieneFormat(input []byte, mode string, themeFlag string, stdout, stderr io.Writer) (code int, handled bool) {
	// cast animates a scene; nothing else has a time axis to record.
	if mode == formatCast && !scene.IsHeader(input) {
		fmt.Fprintln(stderr, "fo: --format cast requires # fo:scene input")
		return 2, true
	}

	if tally.IsHeader(input) {
		return renderTally(input, stdout, stderr, mode, themeFlag), true
	}

	if status.IsHeader(input) {
		return renderStatus(input, stdout, stderr, mode), true
	}

	if metrics.IsHeader(input) {
		return renderMetrics(input, stdout, stderr, mode), true
	}

	if scene.IsHeader(input) {
		return renderScene(input, stdout, stderr, mode), true
	}

	if sniffBareTally(input) {
		var buf bytes.Buffer
		if err := wrapleaderboard.Convert(bytes.NewReader(input), &buf, wrapleaderboard.Opts{Stderr: stderr}); err != nil {
			fmt.Fprintf(stderr, "fo: tally auto-detect: %v\n", err)
			return 2, true
		}
		return renderTally(buf.Bytes(), stdout, stderr, mode, themeFlag), true
	}

	return 0, false
}

// buildReport parses input into a Report. ok is false when the caller
// should return code directly.
func buildReport(input []byte, stderr io.Writer) (r *report.Report, code int, ok bool) {
	r, err := parseToReport(input, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return nil, 2, false
	}
	return r, 0, true
}

// classifyAndRecord runs the post-parse pipeline on r: suppression,
// full-log recording, diff classification against the sidecar, and
// handle assignment/run-log recording. Returns the sidecar's save
// error, if any — surfaced to the caller for --state-strict handling
// rather than treated as fatal here.
func classifyAndRecord(r *report.Report, rawInput []byte, stateFile string, policy statePolicy, stderr io.Writer) error {
	applySuppress(r, suppressPath(), stderr)
	recordFullLog(r, rawInput, policy, stderr)

	saveErr := attachDiff(r, stateFile, policy, stderr)

	assignAndPersistIDs(r, policy, stderr)
	recordRun(r, policy, stderr)

	return saveErr
}

// warnUnknownExpandIDs warns on --expand values that don't match any
// cluster ID in the report. Human mode only; LLM mode ignores --expand
// since clusters always render fully there.
func warnUnknownExpandIDs(mode string, r *report.Report, expandValues []string, stderr io.Writer) {
	if mode == formatLLM || len(expandValues) == 0 {
		return
	}
	known := map[string]struct{}{}
	for _, c := range r.Clusters {
		known[c.ID] = struct{}{}
	}
	for _, v := range expandValues {
		if v == "all" {
			continue
		}
		if _, ok := known[v]; !ok {
			fmt.Fprintf(stderr, "fo: --expand=%s not found\n", v)
		}
	}
}

// finishRender renders the report and computes the final exit code,
// honoring --state-strict: a failed sidecar save escalates to exit 2
// even when the report itself is clean.
func finishRender(mode string, r *report.Report, stdout io.Writer, themeFlag string, expandValues []string, saveErr error, policy statePolicy, stderr io.Writer) int {
	if err := renderMode(mode, r, stdout, themeFlag, expandValues); err != nil {
		fmt.Fprintf(stderr, "fo: %v\n", err)
		return 2
	}
	if saveErr != nil && policy == stateStrict {
		return 2
	}
	return exitCodeReport(r)
}

// resolveStatePolicy translates the (noState, strict) flag pair to a
// single statePolicy. The two flags are mutually exclusive: --no-state
// disables I/O, so --state-strict cannot escalate a save that never
// happened. Rejecting the combination here keeps the policy unambiguous.
var errStateFlagsConflict = errors.New("--no-state and --state-strict are mutually exclusive")

func resolveStatePolicy(noState, strict bool) (statePolicy, error) {
	if noState && strict {
		return stateOff, errStateFlagsConflict
	}
	switch {
	case noState:
		return stateOff, nil
	case strict:
		return stateStrict, nil
	}
	return stateOn, nil
}

func resolveFormat(format string, w io.Writer) (string, error) {
	switch format {
	case "auto":
		if isTTYWriter(w) {
			return formatHuman, nil
		}
		return formatLLM, nil
	case formatHuman, formatLLM, formatJSON, formatCast, formatGitHub:
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
		return theme.Default(theme.OutputKindFromTTY(isTTYWriter(w)))
	}
}

func isTTYWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func termSize(w io.Writer) int {
	width := 80
	if f, ok := w.(*os.File); ok {
		if tw, _, err := term.GetSize(int(f.Fd())); err == nil {
			if tw > 0 {
				width = tw
			}
		}
	}
	return width
}

// runState handles `fo state <subcommand>`. Currently only `reset`.
func runState(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo state", flag.ContinueOnError)
	fs.SetOutput(stderr)
	stateFile := fs.String("state-file", state.Path(), "Sidecar state file path")
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
