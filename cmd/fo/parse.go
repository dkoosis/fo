package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/internal/lineread"
	"github.com/dkoosis/fo/pkg/multiplex"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	"github.com/dkoosis/fo/pkg/wrapper/wrapleaderboard"
)

// coerceAs converts headerless stdin into the requested format by either
// prepending the canonical fo header (status/metrics) or running it
// through the corresponding wrapper (tally/diag). Returns the coerced
// input or a non-zero exit code on usage error.
func coerceAs(kind string, input []byte, stderr io.Writer) ([]byte, int) {
	switch kind {
	case "tally":
		var buf bytes.Buffer
		if err := wrapleaderboard.Convert(bytes.NewReader(input), &buf, wrapleaderboard.Opts{}); err != nil {
			fmt.Fprintf(stderr, "fo: --as tally: %v\n", err)
			return nil, 2
		}
		return buf.Bytes(), 0
	case "status":
		return append([]byte("# fo:status\n"), input...), 0
	case "metrics":
		return append([]byte("# fo:metrics\n"), input...), 0
	case subDiag:
		var buf bytes.Buffer
		if err := wrapdiag.Convert(bytes.NewReader(input), &buf, wrapdiag.DiagOpts{Tool: subDiag, Rule: "finding", Level: sarif.LevelWarning}); err != nil {
			fmt.Fprintf(stderr, "fo: --as diag: %v\n", err)
			return nil, 2
		}
		return buf.Bytes(), 0
	}
	fmt.Fprintf(stderr, "fo: --as: unknown kind %q (want tally|status|metrics|diag)\n", kind)
	return nil, 2
}

// sniffBareTally returns true when every non-blank/non-comment line
// looks like "<number> <label>". Conservative — requires ≥2 rows so a
// single stray "404 not_found" log line never triggers leaderboard.
func sniffBareTally(data []byte) bool {
	br := bufio.NewReaderSize(bytes.NewReader(data), 64*1024)
	rows := 0
	for {
		raw, oversize, err := lineread.Read(br)
		if !oversize {
			ok, counted := tallyRowOK(string(raw))
			if !ok {
				return false
			}
			if counted {
				rows++
			}
		}
		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		return false
	}
	return rows >= 2
}

// tallyRowOK classifies one raw line for sniffBareTally. It returns
// ok=false when the line breaks the "<number> <label>" shape (aborting the
// sniff), and counted=true when the line is a valid tally row that should
// increment the row count. Blank and comment lines are ok but not counted.
func tallyRowOK(raw string) (ok, counted bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return true, false
	}
	idx := strings.IndexAny(line, " \t")
	if idx <= 0 {
		return false, false
	}
	if _, err := strconv.ParseFloat(line[:idx], 64); err != nil {
		return false, false
	}
	if strings.TrimSpace(line[idx:]) == "" {
		return false, false
	}
	return true, true
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
	if multiplex.HasDelimiter(input) {
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
	// Not SARIF — try go test -json. The tolerant path is a strict superset of
	// sniffGoTestJSON-then-ParseBytes: it accepts wrapped banners, surfaces
	// truncated-stream diagnostics distinctly, and emits the same malformed
	// warning. No need for a separate strict arm.
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
// raw line-diagnostic output (fo-tl4). The first 80 bytes of input are
// appended (Go-escaped) so callers can self-diagnose without re-running
// the producer (fo-g3y).
func unrecognizedInputErr(input []byte) error {
	preview := previewBytes(input, 80)
	if looksLikeLineDiagnostics(input) {
		return fmt.Errorf(
			"%w; first bytes: %s\nhint: input looks like line diagnostics — try piping through: fo wrap diag --tool <name>",
			errUnrecognizedInput, preview,
		)
	}
	return fmt.Errorf("%w; first bytes: %s", errUnrecognizedInput, preview)
}

// previewBytes returns up to n bytes of input as a Go-quoted string with a
// "...(truncated)" suffix when the input was longer. Empty input renders as
// `""`.
func previewBytes(input []byte, n int) string {
	truncated := false
	if len(input) > n {
		input = input[:n]
		truncated = true
	}
	q := fmt.Sprintf("%q", input)
	if truncated {
		q += " (truncated)"
	}
	return q
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
	sections, prelude, err := multiplex.ParseSections(input)
	if err != nil {
		var ufe *multiplex.UnknownFormatError
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
func sectionStatusFinding(sec multiplex.Section) (report.Finding, bool) {
	switch sec.Status {
	case multiplex.StatusTimeout:
		return report.Finding{
			RuleID:   "fo/section-timeout",
			Severity: report.SeverityError,
			Message:  fmt.Sprintf("tool=%s timed out before producing output", sec.Tool),
		}, true
	case multiplex.StatusError:
		return report.Finding{
			RuleID:   "fo/section-error",
			Severity: report.SeverityError,
			Message:  fmt.Sprintf("tool=%s exited with an error", sec.Tool),
		}, true
	case multiplex.StatusPartial:
		return report.Finding{
			RuleID:   "fo/section-partial",
			Severity: report.SeverityWarning,
			Message:  fmt.Sprintf("tool=%s produced partial output (may have been interrupted)", sec.Tool),
		}, true
	case multiplex.StatusSkipped:
		return report.Finding{
			RuleID:   "fo/section-skipped",
			Severity: report.SeverityNote,
			Message:  fmt.Sprintf("tool=%s was skipped", sec.Tool),
		}, true
	default:
		return report.Finding{}, false
	}
}

func parseSection(sec multiplex.Section, body []byte, stderr io.Writer) (*report.Report, error) {
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
