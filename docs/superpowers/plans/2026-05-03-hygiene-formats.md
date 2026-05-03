# Hygiene Formats Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Auto-detect and render the structured outputs of the Go tools dk uses across trixi/snipe/next, plus two new fo-defined formats (`# fo:status`, `# fo:metrics`) that dk's tools will emit in place of ad-hoc `printf|awk` tables.

**Architecture:**
- Two new packages mirroring `pkg/tally`: `pkg/status` (PASS/FAIL contract tables) and `pkg/metrics` (key/value with optional unit and delta-vs-last-run).
- Wrappers under `pkg/wrapper/`: `wrapgobench` (`go test -bench` text → metrics), `wrapcover` (`go tool cover -func` → metrics), `wraparchlinttext` (go-arch-lint text → diag SARIF). A real `benchstat` (golang.org/x/perf) tabular-comparison wrapper is filed as a follow-up bead.
- `cmd/fo/main.go` sniffs the new `# fo:status` / `# fo:metrics` headers on stdin (parallel to tally) and adds `--as <kind>` hint flag.
- Renderers extend `pkg/view`: status → tabular PASS/FAIL view; metrics → labeled value list with delta sparklines when sidecar state has prior runs.
- Sidecar state (`pkg/state`) gets a metrics history slot keyed by metric name for delta classification.

**Tech Stack:** Go 1.24, lipgloss, x/term. No new external deps.

**TDD discipline (applies to every task that adds tests + code).** Even when a task collapses "write tests" and "implement" into adjacent steps to keep the plan readable, the executor must keep the red→green discipline: write the test, run it, observe the compile/run failure, *then* write the implementation. Tasks 4, 6, 9, 10, 11 list these as adjacent steps; do not skip the failing-run between them. Task 1 spells the cycle out explicitly — use it as the template.

**Audit reference:** Tool census across trixi/snipe/next Makefiles in agent run from 2026-05-03. Captured tools: govet, golangci-lint, go test, go build, jscpd, govulncheck, nilaway, go-arch-lint, benchstat, jq, snipe, gh, gomarkdoc, awk-formatted status tables. Of these, only benchstat, go-arch-lint text, govulncheck text, coverage, and dk's printf-table outputs are not yet routed through fo.

---

## File Structure

| Path | Role | Status |
|---|---|---|
| `pkg/status/status.go` | `# fo:status` parser, IsHeader sniffer, ToView | Create |
| `pkg/status/status_test.go` | Parser tests | Create |
| `pkg/metrics/metrics.go` | `# fo:metrics` parser, IsHeader, ToView, delta merge | Create |
| `pkg/metrics/metrics_test.go` | Parser tests | Create |
| `pkg/view/status.go` | Status view renderer (human + llm) | Create |
| `pkg/view/metrics.go` | Metrics view renderer (human + llm + delta sparkline) | Create |
| `pkg/state/metrics_history.go` | Sidecar history for metrics deltas | Create |
| `pkg/wrapper/wrapgobench/wrapgobench.go` | `go test -bench` text → metrics | Create |
| `pkg/wrapper/wrapcover/wrapcover.go` | `go tool cover -func` → metrics | Create |
| `pkg/wrapper/wraparchlinttext/wraparchlinttext.go` | go-arch-lint text → diag SARIF | Create |
| `cmd/fo/main.go` | Add status/metrics sniffers, `--as` flag, wrap dispatch entries | Modify |
| `cmd/fo/testdata/help/*.golden` | Help output goldens | Update |
| `docs/guides/hygiene-formats.md` | User-facing format spec + Makefile migration | Create |

---

## Task 1: Status Format Parser

**Files:**
- Create: `pkg/status/status.go`
- Test: `pkg/status/status_test.go`

Format spec (TSV after the state token — chosen so multi-word labels don't need quoting and `printf "ok\t%s\t%s\t%s\n"` is the natural producer):
```
# fo:status [tool=<name>]
<state><TAB><label>[<TAB><value>[<TAB><note>]]
```
or, when the producer has no tabs to spare, single-token labels separated by runs of spaces:
```
<state>  <label>
```
`<state>` is one of `ok|pass|fail|error|warn|warning|skip` (case-insensitive). Comment lines (`#`), blank lines, and the header must all be tolerated identically to `pkg/tally`.

Parser rule: state = first whitespace-delimited token. Remainder = rest of line. If remainder contains a tab, split remainder on `\t` into `[label, value, note]`. Else label = trimmed remainder, value/note empty. This lets shell producers always write `printf "ok\tlabel with spaces\tvalue\tnote\n"` without a label-quoting dance, and lets the simplest case `ok foo` still work.

- [ ] **Step 1: Write failing tests for IsHeader**

```go
package status

import "testing"

func TestIsHeader(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# fo:status\nok foo\n", true},
		{"# fo:status tool=doctor\n", true},
		{"  # fo:status\n", true},
		{"# fo:tally\n", false},
		{"ok foo\n", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHeader([]byte(c.in)); got != c.want {
			t.Errorf("IsHeader(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run tests; expect compile failure**

Run: `go test ./pkg/status/...`
Expected: undefined: IsHeader.

- [ ] **Step 3: Implement IsHeader + scaffolding**

```go
// Package status parses fo's status input format — labeled rows with
// PASS/FAIL/WARN/SKIP state, used for contract tables, doctor checks,
// module gates, and any "list of named conditions" output that today
// gets handed to printf|awk.
//
// Format:
//
//	# fo:status [tool=<name>]
//	<state>  <label>  [value]  [note...]
//
// State is one of: ok | fail | warn | skip (case-insensitive). Lines
// beginning with # after the header are comments. Blank lines and
// leading whitespace on data rows are tolerated.
package status

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

const HeaderPrefix = "# fo:status"

type State string

const (
	StateOK   State = "ok"
	StateFail State = "fail"
	StateWarn State = "warn"
	StateSkip State = "skip"
)

type Row struct {
	State State  `json:"state"`
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
	Note  string `json:"note,omitempty"`
}

type Status struct {
	Tool string `json:"tool,omitempty"`
	Rows []Row  `json:"rows"`
}

func IsHeader(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte(HeaderPrefix))
}

var (
	ErrNoHeader     = errors.New("status: missing '# fo:status' header")
	ErrNoRows       = errors.New("status: no data rows")
	ErrMalformedRow = errors.New("status: malformed row")
	ErrBadState     = errors.New("status: bad state token")
)
```

- [ ] **Step 4: Run; expect IsHeader tests to pass**

Run: `go test ./pkg/status/... -run TestIsHeader`
Expected: PASS.

- [ ] **Step 5: Write failing tests for Parse**

```go
func TestParse_basic(t *testing.T) {
	// Mix space-only and TSV rows. Space-only rows put everything after
	// state into Label; TSV rows split into label/value/note.
	in := strings.NewReader("# fo:status tool=doctor\nok\tenv loaded\nfail\tdolt missing\t\tnot-installed\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Tool != "doctor" {
		t.Errorf("Tool = %q", s.Tool)
	}
	if len(s.Rows) != 2 {
		t.Fatalf("rows = %d", len(s.Rows))
	}
	if s.Rows[0] != (Row{State: StateOK, Label: "env loaded"}) {
		t.Errorf("row0 = %+v", s.Rows[0])
	}
	if s.Rows[1].State != StateFail || s.Rows[1].Label != "dolt missing" || s.Rows[1].Note != "not-installed" {
		t.Errorf("row1 = %+v", s.Rows[1])
	}
}

func TestParse_spaceOnly(t *testing.T) {
	// No tabs anywhere — entire remainder of line is the label.
	in := strings.NewReader("# fo:status\nok build green\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Rows[0].Label != "build green" || s.Rows[0].Value != "" || s.Rows[0].Note != "" {
		t.Errorf("row = %+v", s.Rows[0])
	}
}

func TestParse_valueAndNote(t *testing.T) {
	in := strings.NewReader("# fo:status\nok\tbuild\t2.3s\tgreen\n")
	s, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if s.Rows[0].Value != "2.3s" || s.Rows[0].Note != "green" {
		t.Errorf("row = %+v", s.Rows[0])
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"no header", "ok foo\n", ErrNoHeader},
		{"no rows", "# fo:status\n", ErrNoRows},
		{"bad state", "# fo:status\nbogus foo\n", ErrBadState},
		{"missing label", "# fo:status\nok\n", ErrMalformedRow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(c.in))
			if !errors.Is(err, c.want) {
				t.Errorf("err = %v, want Is %v", err, c.want)
			}
		})
	}
}
```

- [ ] **Step 6: Implement Parse**

Append to `pkg/status/status.go`:

```go
func Parse(r io.Reader) (Status, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	var s Status
	headerSeen := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !headerSeen {
			if !strings.HasPrefix(line, HeaderPrefix) {
				return Status{}, ErrNoHeader
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, HeaderPrefix))
			s.Tool = parseAttr(rest, "tool")
			headerSeen = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		row, err := parseRow(line)
		if err != nil {
			return Status{}, fmt.Errorf("status: line %d: %w", lineNo, err)
		}
		s.Rows = append(s.Rows, row)
	}
	if err := sc.Err(); err != nil {
		return Status{}, fmt.Errorf("status: read: %w", err)
	}
	if !headerSeen {
		return Status{}, ErrNoHeader
	}
	if len(s.Rows) == 0 {
		return Status{}, ErrNoRows
	}
	return s, nil
}

func parseRow(line string) (Row, error) {
	// State is the first whitespace-delimited token; remainder is everything after.
	idx := strings.IndexAny(line, " \t")
	if idx <= 0 {
		return Row{}, fmt.Errorf("%w: expected '<state> <label> ...', got %q", ErrMalformedRow, line)
	}
	st, err := parseState(line[:idx])
	if err != nil {
		return Row{}, err
	}
	rest := strings.TrimLeft(line[idx:], " \t")
	if rest == "" {
		return Row{}, fmt.Errorf("%w: missing label, got %q", ErrMalformedRow, line)
	}
	row := Row{State: st}
	if strings.ContainsRune(rest, '\t') {
		// TSV form: split into label / value / note. Empty fields tolerated.
		parts := strings.SplitN(rest, "\t", 3)
		row.Label = strings.TrimSpace(parts[0])
		if len(parts) >= 2 {
			row.Value = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			row.Note = strings.TrimSpace(parts[2])
		}
	} else {
		// Space-only form: whole remainder is the label.
		row.Label = strings.TrimSpace(rest)
	}
	if row.Label == "" {
		return Row{}, fmt.Errorf("%w: missing label, got %q", ErrMalformedRow, line)
	}
	return row, nil
}

func parseState(tok string) (State, error) {
	switch strings.ToLower(tok) {
	case "ok", "pass":
		return StateOK, nil
	case "fail", "error":
		return StateFail, nil
	case "warn", "warning":
		return StateWarn, nil
	case "skip":
		return StateSkip, nil
	}
	return "", fmt.Errorf("%w: %q", ErrBadState, tok)
}

func parseAttr(tail, key string) string {
	for tok := range strings.FieldsSeq(tail) {
		if eq := strings.IndexByte(tok, '='); eq > 0 && tok[:eq] == key {
			return tok[eq+1:]
		}
	}
	return ""
}
```

> Note: TSV after the state token. Producers that want multi-field rows write tabs; producers that just want a labeled state token write spaces. No quoting machinery needed.

- [ ] **Step 7: Run all status tests**

Run: `go test ./pkg/status/...`
Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/status/
git commit -m "feat(status): add fo:status parser for PASS/FAIL contract tables"
```

---

## Task 2: Status View Renderer

**Files:**
- Create: `pkg/view/status.go`
- Modify: `pkg/view/view.go` (add Status case to render dispatch if one exists; otherwise expose `RenderStatus`)
- Test: `pkg/view/status_test.go`

- [ ] **Step 1: Inspect view.go to find existing render dispatch shape**

Run: `rg -n 'func Render|switch' pkg/view/view.go pkg/view/render.go`
Read whichever entry-point exists (likely `render.go`).

- [ ] **Step 2: Write failing test for human render**

```go
package view

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderStatus_human(t *testing.T) {
	rows := []StatusRow{
		{State: "ok", Label: "env-loaded"},
		{State: "fail", Label: "dolt-installed", Note: "not on PATH"},
		{State: "warn", Label: "snipe-fresh", Value: "2h-old"},
	}
	var buf bytes.Buffer
	if err := RenderStatusHuman(&buf, "doctor", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"doctor", "env-loaded", "dolt-installed", "not on PATH", "2h-old"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderStatus_llm(t *testing.T) {
	rows := []StatusRow{{State: "ok", Label: "a"}, {State: "fail", Label: "b"}}
	var buf bytes.Buffer
	if err := RenderStatusLLM(&buf, "tool", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "ok   a") || !strings.Contains(got, "fail b") {
		t.Errorf("llm output unexpected:\n%s", got)
	}
}
```

- [ ] **Step 3: Run; expect compile failure**

Run: `go test ./pkg/view/... -run TestRenderStatus`
Expected: undefined.

- [ ] **Step 4: Implement renderers**

Create `pkg/view/status.go`:

```go
package view

import (
	"fmt"
	"io"
	"strings"
)

type StatusRow struct {
	State string
	Label string
	Value string
	Note  string
}

// RenderStatusLLM emits one row per line: aligned state token, label,
// optional value, optional note. Token-dense; no decoration.
func RenderStatusLLM(w io.Writer, tool string, rows []StatusRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "# %s\n", tool); err != nil {
			return err
		}
	}
	labelMax := 0
	for _, r := range rows {
		if l := len(r.Label); l > labelMax {
			labelMax = l
		}
	}
	for _, r := range rows {
		extra := strings.TrimSpace(r.Value + " " + r.Note)
		if _, err := fmt.Fprintf(w, "%-4s %-*s  %s\n", r.State, labelMax, r.Label, extra); err != nil {
			return err
		}
	}
	return nil
}

// RenderStatusHuman emits a colored, aligned table when the renderer's
// theme is color; mono falls back to plain text identical to LLM mode
// plus a header banner.
func RenderStatusHuman(w io.Writer, tool string, rows []StatusRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "── %s ──\n", tool); err != nil {
			return err
		}
	}
	// Counts header: 3 ok · 1 fail · 1 warn
	var ok, fail, warn, skip int
	for _, r := range rows {
		switch r.State {
		case "ok":
			ok++
		case "fail":
			fail++
		case "warn":
			warn++
		case "skip":
			skip++
		}
	}
	if _, err := fmt.Fprintf(w, "%d ok · %d fail · %d warn · %d skip\n\n", ok, fail, warn, skip); err != nil {
		return err
	}
	return RenderStatusLLM(w, "", rows)
}
```

- [ ] **Step 5: Run tests; expect PASS**

Run: `go test ./pkg/view/... -run TestRenderStatus`

- [ ] **Step 6: Add ToView convenience on pkg/status**

Append to `pkg/status/status.go`:

```go
// ToViewRows converts to the renderer's row shape; renderer is in
// pkg/view to avoid pulling pkg/status into pkg/view.
func (s Status) ToViewRows() []ViewRow {
	out := make([]ViewRow, len(s.Rows))
	for i, r := range s.Rows {
		out[i] = ViewRow{State: string(r.State), Label: r.Label, Value: r.Value, Note: r.Note}
	}
	return out
}

// ViewRow is the renderer-facing shape; mirrors view.StatusRow without
// the import (view depends on no domain pkg).
type ViewRow struct {
	State string
	Label string
	Value string
	Note  string
}
```

- [ ] **Step 7: Run full pkg test**

Run: `go test ./pkg/status/... ./pkg/view/...`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/view/status.go pkg/view/status_test.go pkg/status/status.go
git commit -m "feat(view): render fo:status tables (human + llm)"
```

---

## Task 3: Wire Status into cmd/fo

**Files:**
- Modify: `cmd/fo/main.go`

**Sniffer dispatch order (canonical — preserve as new sniffers land):**

1. `report.HasDelimiter` (multiplex `--- tool:`)
2. `sniffSARIF`
3. `sniffGoTestJSON`
4. `tally.IsHeader`
5. `status.IsHeader`   ← added in this task
6. `metrics.IsHeader`  ← added in Task 7
7. `sniffBareTally`    ← added in Task 12 (fuzzy, runs last)
8. otherwise → unrecognized-input error

`# fo:*` headers are cheap exact-prefix checks; keep them ahead of the fuzzy bare-tally sniffer so a malformed `# fo:status` never falls through into leaderboard.

- [ ] **Step 1: Find tally sniff site + verify golden harness**

Run: `rg -n 'tally.IsHeader' cmd/fo/main.go`
Read 5 lines around the match.

Run once: `rg -n -- "-update" cmd/fo/*_test.go cmd/fo/testdata/`
Confirm whether the e2e/help golden harness honors `-update`. If it does not, every "regenerate golden" step in this plan becomes "hand-edit `cmd/fo/testdata/help/*.golden`". Note the result here so later tasks don't stall.

- [ ] **Step 2: Write failing e2e test**

Append to `cmd/fo/e2e_test.go`:

```go
func TestE2E_statusFormat(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())
	in := "# fo:status tool=doctor\nok\tenv\nfail\tdolt\n"
	out, _, code := runFo(t, in, "--format", "llm")
	if code != 0 {
		t.Fatalf("exit = %d, out=%s", code, out)
	}
	if !strings.Contains(out, "ok") || !strings.Contains(out, "fail") {
		t.Errorf("missing rows in output:\n%s", out)
	}
}
```

(Adapt `runFo` helper name to the actual one in e2e_test.go.)

- [ ] **Step 3: Run; expect failure (status not recognized)**

Run: `go test ./cmd/fo/... -run TestE2E_statusFormat`
Expected: exit 2 or unrecognized-input error.

- [ ] **Step 4: Add sniffer + render path**

In `cmd/fo/main.go`, locate the block that handles `tally.IsHeader(input)`. Add a parallel block above or below it:

```go
if status.IsHeader(input) {
	return renderStatus(input, stdout, stderr, format)
}
```

Add the renderer function (next to `renderTally`):

```go
import "github.com/dkoosis/fo/pkg/status"

func renderStatus(input []byte, stdout, stderr io.Writer, format string) int {
	s, err := status.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing status: %v\n", err)
		return 2
	}
	rows := make([]view.StatusRow, len(s.Rows))
	for i, r := range s.Rows {
		rows[i] = view.StatusRow{State: string(r.State), Label: r.Label, Value: r.Value, Note: r.Note}
	}
	switch format {
	case "json":
		return emitStatusJSON(s, stdout, stderr)
	case "llm":
		if err := view.RenderStatusLLM(stdout, s.Tool, rows); err != nil {
			fmt.Fprintf(stderr, "fo: render: %v\n", err)
			return 2
		}
	default:
		if err := view.RenderStatusHuman(stdout, s.Tool, rows); err != nil {
			fmt.Fprintf(stderr, "fo: render: %v\n", err)
			return 2
		}
	}
	return 0
}

func emitStatusJSON(s status.Status, stdout, stderr io.Writer) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		fmt.Fprintf(stderr, "fo: json: %v\n", err)
		return 2
	}
	return 0
}
```

- [ ] **Step 5: Run; expect PASS**

Run: `go test ./cmd/fo/... -run TestE2E_statusFormat`

- [ ] **Step 6: Update help golden if needed**

Run: `go test ./cmd/fo/... -run Help -update` (if the golden harness uses `-update`; otherwise edit `cmd/fo/testdata/help/root.golden` to add a `status` line in the wrap or formats section as appropriate).

- [ ] **Step 7: Commit**

```bash
git add cmd/fo/main.go cmd/fo/e2e_test.go cmd/fo/testdata/
git commit -m "feat(cli): auto-route fo:status streams to status renderer"
```

---

## Task 4: Metrics Format Parser

**Files:**
- Create: `pkg/metrics/metrics.go`
- Test: `pkg/metrics/metrics_test.go`

Format spec:
```
# fo:metrics [tool=<name>]
<key>  <value>  [unit]
```
Value parses as float64. Unit is optional (e.g. `ms`, `%`, `MB`, `count`).

- [ ] **Step 1: Mirror tally test layout for IsHeader, Parse, errors**

Tests follow exactly the shape of `pkg/tally/tally_test.go` and the status tests above. Three error sentinels: `ErrNoHeader`, `ErrNoRows`, `ErrMalformedRow`.

- [ ] **Step 2: Write the tests**

```go
package metrics

import (
	"errors"
	"strings"
	"testing"
)

func TestIsHeader(t *testing.T) {
	if !IsHeader([]byte("# fo:metrics\n")) {
		t.Error("expected header detected")
	}
	if IsHeader([]byte("# fo:status\n")) {
		t.Error("status should not match")
	}
}

func TestParse_basic(t *testing.T) {
	in := strings.NewReader("# fo:metrics tool=cover\npkg/x 87.3 %\npkg/y 100 %\n")
	m, err := Parse(in)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Tool != "cover" {
		t.Errorf("tool = %q", m.Tool)
	}
	if len(m.Rows) != 2 {
		t.Fatalf("rows = %d", len(m.Rows))
	}
	if m.Rows[0] != (Row{Key: "pkg/x", Value: 87.3, Unit: "%"}) {
		t.Errorf("row0 = %+v", m.Rows[0])
	}
}

func TestParse_noUnit(t *testing.T) {
	m, err := Parse(strings.NewReader("# fo:metrics\nbuild_time 2.3\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Rows[0].Unit != "" || m.Rows[0].Value != 2.3 {
		t.Errorf("row = %+v", m.Rows[0])
	}
}

func TestParse_errors(t *testing.T) {
	cases := []struct {
		in   string
		want error
	}{
		{"x 1\n", ErrNoHeader},
		{"# fo:metrics\n", ErrNoRows},
		{"# fo:metrics\nbad\n", ErrMalformedRow},
		{"# fo:metrics\nx not-a-number\n", ErrMalformedRow},
	}
	for _, c := range cases {
		_, err := Parse(strings.NewReader(c.in))
		if !errors.Is(err, c.want) {
			t.Errorf("err = %v, want Is %v", err, c.want)
		}
	}
}
```

- [ ] **Step 3: Implement**

Create `pkg/metrics/metrics.go`. Structure mirrors `pkg/status/status.go` and `pkg/tally/tally.go` exactly — header sniff, scanner loop, parseRow with `<key> <value> [unit]` split via `strings.Fields`, value via `strconv.ParseFloat`, sentinels for errors.

```go
// Package metrics parses fo's metrics input format — keyed numeric
// values used for hygiene rollups (coverage %, LOC counts, build time,
// benchmark deltas, dependency counts). Renders as a labeled value list
// with delta sparklines when sidecar history is present.
//
// Format:
//
//	# fo:metrics [tool=<name>]
//	<key>  <value>  [unit]
package metrics

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const HeaderPrefix = "# fo:metrics"

type Row struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type Metrics struct {
	Tool string `json:"tool,omitempty"`
	Rows []Row  `json:"rows"`
}

func IsHeader(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte(HeaderPrefix))
}

var (
	ErrNoHeader     = errors.New("metrics: missing '# fo:metrics' header")
	ErrNoRows       = errors.New("metrics: no data rows")
	ErrMalformedRow = errors.New("metrics: malformed row")
)

func Parse(r io.Reader) (Metrics, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	var m Metrics
	headerSeen := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !headerSeen {
			if !strings.HasPrefix(line, HeaderPrefix) {
				return Metrics{}, ErrNoHeader
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, HeaderPrefix))
			m.Tool = parseAttr(rest, "tool")
			headerSeen = true
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		row, err := parseRow(line)
		if err != nil {
			return Metrics{}, fmt.Errorf("metrics: line %d: %w", lineNo, err)
		}
		m.Rows = append(m.Rows, row)
	}
	if err := sc.Err(); err != nil {
		return Metrics{}, fmt.Errorf("metrics: read: %w", err)
	}
	if !headerSeen {
		return Metrics{}, ErrNoHeader
	}
	if len(m.Rows) == 0 {
		return Metrics{}, ErrNoRows
	}
	return m, nil
}

func parseRow(line string) (Row, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return Row{}, fmt.Errorf("%w: expected '<key> <value> [unit]', got %q", ErrMalformedRow, line)
	}
	v, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Row{}, fmt.Errorf("%w: non-numeric value %q", ErrMalformedRow, fields[1])
	}
	row := Row{Key: fields[0], Value: v}
	if len(fields) >= 3 {
		row.Unit = fields[2]
	}
	return row, nil
}

func parseAttr(tail, key string) string {
	for tok := range strings.FieldsSeq(tail) {
		if eq := strings.IndexByte(tok, '='); eq > 0 && tok[:eq] == key {
			return tok[eq+1:]
		}
	}
	return ""
}
```

- [ ] **Step 4: Run tests; expect PASS**

Run: `go test ./pkg/metrics/...`

- [ ] **Step 5: Commit**

```bash
git add pkg/metrics/
git commit -m "feat(metrics): add fo:metrics parser for hygiene rollups"
```

---

## Task 5: Metrics History (Sidecar Delta)

**Files:**
- Create: `pkg/state/metrics_history.go`
- Test: `pkg/state/metrics_history_test.go`
- Modify: `pkg/state/` — extend the existing sidecar shape so metrics rows persist alongside finding fingerprints.

- [ ] **Step 1: Read existing state shape**

Run: `rg -n 'type Envelope|type Item|func Save|func Load' pkg/state/*.go`
Read the file that defines `Envelope`.

- [ ] **Step 2: Write failing test**

```go
package state

import (
	"path/filepath"
	"testing"
)

func TestMetricsHistory_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	curr := []MetricSample{
		{Tool: "cover", Key: "pkg/x", Value: 87.3, Unit: "%"},
		{Tool: "cover", Key: "pkg/y", Value: 100, Unit: "%"},
	}
	if err := SaveMetrics(path, curr); err != nil {
		t.Fatalf("save: %v", err)
	}
	prev, err := LoadMetrics(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(prev) != 2 || prev[0].Value != 87.3 {
		t.Errorf("got %+v", prev)
	}
}

func TestMetricsHistory_diff(t *testing.T) {
	prev := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 80}}
	curr := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 87.3}}
	d := DiffMetrics(prev, curr)
	if len(d) != 1 || d[0].Delta != 7.3 || d[0].New {
		t.Errorf("diff = %+v", d)
	}
}

func TestMetricsHistory_newRow(t *testing.T) {
	curr := []MetricSample{{Tool: "cover", Key: "pkg/new", Value: 42}}
	d := DiffMetrics(nil, curr)
	if len(d) != 1 || !d[0].New || d[0].Delta != 0 {
		t.Errorf("expected New=true, Delta=0; got %+v", d)
	}
}

func TestMetricsHistory_keyOnlyFallback(t *testing.T) {
	// Prev has tool="cover"; curr has empty tool (e.g. --as metrics).
	// Should match by key alone and emit a real Delta, not New.
	prev := []MetricSample{{Tool: "cover", Key: "pkg/x", Value: 80}}
	curr := []MetricSample{{Tool: "", Key: "pkg/x", Value: 90}}
	d := DiffMetrics(prev, curr)
	if len(d) != 1 || d[0].Delta != 10 || d[0].New {
		t.Errorf("expected key-only match Delta=10, got %+v", d)
	}
}
```

- [ ] **Step 3: Implement**

Create `pkg/state/metrics_history.go`:

```go
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type MetricSample struct {
	Tool  string  `json:"tool,omitempty"`
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type MetricDelta struct {
	Sample MetricSample `json:"sample"`
	Prior  float64      `json:"prior"`
	Delta  float64      `json:"delta"`
	New    bool         `json:"new,omitempty"` // no prior sample matched
}

func SaveMetrics(path string, samples []MetricSample) error {
	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		return fmt.Errorf("metrics: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("metrics: write %s: %w", path, err)
	}
	return nil
}

func LoadMetrics(path string) ([]MetricSample, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: read %s: %w", path, err)
	}
	var out []MetricSample
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("metrics: unmarshal: %w", err)
	}
	return out, nil
}

func DiffMetrics(prev, curr []MetricSample) []MetricDelta {
	// Index prior by tool+key AND by key alone, so we can fall back when
	// either side has an empty Tool tag (e.g. when --as metrics injects a
	// bare header). Tool-qualified match wins when both have a tool.
	priorTK := make(map[string]float64, len(prev))
	priorK := make(map[string]float64, len(prev))
	for _, s := range prev {
		priorTK[s.Tool+"\x00"+s.Key] = s.Value
		priorK[s.Key] = s.Value
	}
	out := make([]MetricDelta, 0, len(curr))
	for _, s := range curr {
		if p, ok := priorTK[s.Tool+"\x00"+s.Key]; ok {
			out = append(out, MetricDelta{Sample: s, Prior: p, Delta: s.Value - p})
			continue
		}
		if p, ok := priorK[s.Key]; ok && s.Tool == "" {
			// Current row has no tool; match by key alone.
			out = append(out, MetricDelta{Sample: s, Prior: p, Delta: s.Value - p})
			continue
		}
		out = append(out, MetricDelta{Sample: s, Prior: 0, Delta: 0, New: true})
	}
	return out
}
```

- [ ] **Step 4: Run tests; expect PASS**

Run: `go test ./pkg/state/...`

- [ ] **Step 5: Commit**

```bash
git add pkg/state/metrics_history.go pkg/state/metrics_history_test.go
git commit -m "feat(state): add metrics sidecar for delta classification"
```

---

## Task 6: Metrics View Renderer

**Files:**
- Create: `pkg/view/metrics.go`
- Test: `pkg/view/metrics_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRenderMetrics_human(t *testing.T) {
	rows := []MetricRow{
		{Key: "pkg/x", Value: 87.3, Unit: "%", Delta: 7.3},
		{Key: "pkg/y", Value: 100, Unit: "%", Delta: 0},
	}
	var buf bytes.Buffer
	if err := RenderMetricsHuman(&buf, "cover", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"cover", "pkg/x", "87.3", "%", "+7.3"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRenderMetrics_llm(t *testing.T) {
	rows := []MetricRow{{Key: "k", Value: 1.5, Unit: "s"}}
	var buf bytes.Buffer
	if err := RenderMetricsLLM(&buf, "tool", rows); err != nil {
		t.Fatalf("render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "k 1.5 s") {
		t.Errorf("got: %q", got)
	}
}
```

- [ ] **Step 2: Implement**

Create `pkg/view/metrics.go`:

```go
package view

import (
	"fmt"
	"io"
	"strconv"
)

type MetricRow struct {
	Key   string
	Value float64
	Unit  string
	Delta float64 // 0 if New, or genuinely unchanged
	New   bool    // true when no prior sample matched — render "(new)"
}

func RenderMetricsLLM(w io.Writer, tool string, rows []MetricRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "# %s\n", tool); err != nil {
			return err
		}
	}
	for _, r := range rows {
		v := strconv.FormatFloat(r.Value, 'f', -1, 64)
		if r.Unit != "" {
			if _, err := fmt.Fprintf(w, "%s %s %s\n", r.Key, v, r.Unit); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(w, "%s %s\n", r.Key, v); err != nil {
			return err
		}
	}
	return nil
}

func RenderMetricsHuman(w io.Writer, tool string, rows []MetricRow) error {
	if tool != "" {
		if _, err := fmt.Fprintf(w, "── %s ──\n", tool); err != nil {
			return err
		}
	}
	keyMax := 0
	for _, r := range rows {
		if l := len(r.Key); l > keyMax {
			keyMax = l
		}
	}
	for _, r := range rows {
		v := strconv.FormatFloat(r.Value, 'f', -1, 64)
		delta := ""
		switch {
		case r.New:
			delta = "  (new)"
		case r.Delta != 0:
			sign := "+"
			if r.Delta < 0 {
				sign = ""
			}
			delta = fmt.Sprintf("  (%s%s)", sign, strconv.FormatFloat(r.Delta, 'f', -1, 64))
		}
		unit := ""
		if r.Unit != "" {
			unit = " " + r.Unit
		}
		if _, err := fmt.Fprintf(w, "%-*s  %s%s%s\n", keyMax, r.Key, v, unit, delta); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: Run; expect PASS**

Run: `go test ./pkg/view/... -run TestRenderMetrics`

- [ ] **Step 4: Commit**

```bash
git add pkg/view/metrics.go pkg/view/metrics_test.go
git commit -m "feat(view): render fo:metrics with delta annotation"
```

---

## Task 7: Wire Metrics into cmd/fo (with Sidecar Delta)

**Files:**
- Modify: `cmd/fo/main.go`
- Modify: `cmd/fo/state.go` (or wherever sidecar path resolution lives)

- [ ] **Step 1: Find state path resolution**

Run: `rg -n 'last-run|\.fo/' cmd/fo/*.go`
Identify the function that resolves the sidecar path.

- [ ] **Step 2: Write e2e test**

Append to `cmd/fo/e2e_test.go`:

```go
func TestE2E_metricsFormat(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())
	in := "# fo:metrics tool=cover\npkg/x 87.3 %\npkg/y 100 %\n"
	out, _, code := runFo(t, in, "--format", "llm")
	if code != 0 {
		t.Fatalf("exit = %d, out=%s", code, out)
	}
	if !strings.Contains(out, "pkg/x 87.3 %") {
		t.Errorf("missing row:\n%s", out)
	}
}
```

- [ ] **Step 3: Run; expect failure**

Run: `go test ./cmd/fo/... -run TestE2E_metricsFormat`

- [ ] **Step 4: Add sniffer + render path**

In `cmd/fo/main.go`, add after the status sniff:

```go
if metrics.IsHeader(input) {
	return renderMetrics(input, stdout, stderr, format)
}
```

Implement `renderMetrics`. It loads sidecar history, computes deltas, renders, and saves the new sample set:

```go
import (
	"github.com/dkoosis/fo/pkg/metrics"
	"github.com/dkoosis/fo/pkg/state"
)

func renderMetrics(input []byte, stdout, stderr io.Writer, format string) int {
	m, err := metrics.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing metrics: %v\n", err)
		return 2
	}
	curr := make([]state.MetricSample, len(m.Rows))
	for i, r := range m.Rows {
		curr[i] = state.MetricSample{Tool: m.Tool, Key: r.Key, Value: r.Value, Unit: r.Unit}
	}
	histPath := metricsHistoryPath() // returns ".fo/metrics-history.json"
	prev, _ := state.LoadMetrics(histPath)
	deltas := state.DiffMetrics(prev, curr)

	rows := make([]view.MetricRow, len(deltas))
	for i, d := range deltas {
		rows[i] = view.MetricRow{
			Key: d.Sample.Key, Value: d.Sample.Value, Unit: d.Sample.Unit, Delta: d.Delta, New: d.New,
		}
	}

	switch format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Tool   string              `json:"tool,omitempty"`
			Deltas []state.MetricDelta `json:"deltas"`
		}{Tool: m.Tool, Deltas: deltas}); err != nil {
			fmt.Fprintf(stderr, "fo: json: %v\n", err)
			return 2
		}
	case "llm":
		if err := view.RenderMetricsLLM(stdout, m.Tool, rows); err != nil {
			fmt.Fprintf(stderr, "fo: render: %v\n", err)
			return 2
		}
	default:
		if err := view.RenderMetricsHuman(stdout, m.Tool, rows); err != nil {
			fmt.Fprintf(stderr, "fo: render: %v\n", err)
			return 2
		}
	}

	// Save current as new prior. Soft-fail with a Notice on the next run if
	// save fails (matches existing fo behavior for sidecar Save failures).
	if err := state.SaveMetrics(histPath, curr); err != nil {
		fmt.Fprintf(stderr, "fo: save metrics history: %v\n", err)
	}
	return 0
}

func metricsHistoryPath() string {
	// Same .fo/ dir used by last-run.json; helper resolves it.
	return filepath.Join(stateDir(), "metrics-history.json")
}
```

**Unify FO_STATE_DIR in `stateDir()`.** Do not bolt the env var into `metricsHistoryPath` only — that would let `last-run.json` and `metrics-history.json` resolve to different directories in tests. Edit the existing `stateDir()` helper (or, if there is no helper today, extract one) so it consults `os.Getenv("FO_STATE_DIR")` first and falls back to the current `.fo/` resolution. Then both `last-run.json` and `metrics-history.json` share the same root automatically.

- [ ] **Step 5: Run; expect PASS**

Run: `go test ./cmd/fo/... -run TestE2E_metricsFormat`

- [ ] **Step 6: Add a delta test**

```go
func TestE2E_metricsDelta(t *testing.T) {
	t.Setenv("FO_STATE_DIR", t.TempDir())
	in := "# fo:metrics\nx 10\n"
	runFo(t, in, "--format", "llm")
	in2 := "# fo:metrics\nx 12\n"
	out, _, _ := runFo(t, in2, "--format", "human")
	if !strings.Contains(out, "+2") {
		t.Errorf("expected delta +2 in:\n%s", out)
	}
}
```

The `FO_STATE_DIR` honoring lives in `stateDir()` per Step 4 above — every sidecar writer (last-run, metrics-history, future) inherits it for free. Set it via `t.Setenv("FO_STATE_DIR", t.TempDir())` in **every** status/metrics e2e test, not just delta tests, to keep test runs from sharing a `.fo/` directory.

- [ ] **Step 7: Run; expect PASS**

Run: `go test ./cmd/fo/... -run TestE2E_metricsDelta`

- [ ] **Step 8: Commit**

```bash
git add cmd/fo/ pkg/state/
git commit -m "feat(cli): auto-route fo:metrics with sidecar delta tracking"
```

---

## Task 8: `--as <kind>` Hint Flag

**Files:**
- Modify: `cmd/fo/main.go`

- [ ] **Step 1: Write failing test**

```go
func TestE2E_asHint_tally(t *testing.T) {
	// Bare uniq -c output, no fo header.
	in := "  10 a\n   3 b\n"
	out, _, code := runFo(t, in, "--as", "tally", "--format", "llm")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "a") || !strings.Contains(out, "10") {
		t.Errorf("missing rows:\n%s", out)
	}
}

func TestE2E_asHint_unknown(t *testing.T) {
	_, errOut, code := runFo(t, "x\n", "--as", "bogus")
	if code != 2 || !strings.Contains(errOut, "--as") {
		t.Errorf("expected usage error, got code=%d err=%s", code, errOut)
	}
}
```

- [ ] **Step 2: Run; expect failure**

- [ ] **Step 3: Add flag + dispatch**

In `cmd/fo/main.go` near the other flag declarations:

```go
asFlag := fs.String("as", "", "Hint format when auto-detection is ambiguous: tally|status|metrics|diag")
```

After reading stdin and before the existing sniff chain:

```go
if *asFlag != "" {
	switch *asFlag {
	case "tally":
		// Wrap input through wrapleaderboard to add the header, then route
		// to the existing tally path.
		var buf bytes.Buffer
		if err := wrapleaderboard.Convert(bytes.NewReader(input), &buf, wrapleaderboard.Opts{}); err != nil {
			fmt.Fprintf(stderr, "fo: --as tally: %v\n", err)
			return 2
		}
		input = buf.Bytes()
	case "status":
		// Prepend header so existing path handles it.
		input = append([]byte("# fo:status\n"), input...)
	case "metrics":
		input = append([]byte("# fo:metrics\n"), input...)
	case "diag":
		// Route through wrapdiag, then SARIF path.
		var buf bytes.Buffer
		if err := wrapdiag.Convert(bytes.NewReader(input), &buf, wrapdiag.Opts{}); err != nil {
			fmt.Fprintf(stderr, "fo: --as diag: %v\n", err)
			return 2
		}
		input = buf.Bytes()
	default:
		fmt.Fprintf(stderr, "fo: --as: unknown kind %q (want tally|status|metrics|diag)\n", *asFlag)
		return 2
	}
}
```

- [ ] **Step 4: Run; expect PASS**

Run: `go test ./cmd/fo/... -run TestE2E_asHint`

- [ ] **Step 5: Update `--help` golden**

The flag list rendered by `--help` must include `--as`. Either regenerate goldens (if the harness has `-update`) or hand-edit `cmd/fo/testdata/help/root.golden`.

- [ ] **Step 6: Commit**

```bash
git add cmd/fo/
git commit -m "feat(cli): add --as hint flag for ambiguous stdin formats"
```

---

## Task 9: Wrapper — go-arch-lint text → diag SARIF

**Files:**
- Create: `pkg/wrapper/wraparchlinttext/wraparchlinttext.go`
- Test: `pkg/wrapper/wraparchlinttext/wraparchlinttext_test.go`
- Modify: `cmd/fo/main.go` (register wrap dispatch case)

go-arch-lint's text output (as run in `trixi/Makefile` `arch-lint` target) emits lines like:

```
[Warning] Component "internal/foo" shouldn't import component "internal/bar"
  internal/foo/x.go: imports forbidden component
total notices: 3
```

The wrapper extracts the first-line component pair into a finding and discards the trailing total. The `internal/foo/x.go` next-line gives file:line if present.

- [ ] **Step 1: Write failing test**

```go
package wraparchlinttext

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvert_basic(t *testing.T) {
	in := `[Warning] Component "internal/foo" shouldn't import component "internal/bar"
  internal/foo/x.go: imports forbidden component
[Warning] Component "internal/baz" shouldn't import component "internal/qux"
  internal/baz/y.go: imports forbidden component
total notices: 2
`
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\"version\": \"2.1.0\"") {
		t.Errorf("not SARIF: %s", got)
	}
	if !strings.Contains(got, "internal/foo") {
		t.Errorf("missing rule context:\n%s", got)
	}
}
```

- [ ] **Step 2: Implement**

Pattern after `pkg/wrapper/wrapdiag` — read lines, build SARIF results, emit. Ruleset: a single `arch-lint/forbidden-import` rule. Each `[Warning]`/`[Error]` line opens a result; the indented continuation line provides location.

**Reuse `pkg/sarif`'s builder rather than redeclaring SARIF struct types.** Per the architecture doc, `pkg/sarif` is the canonical home for "SARIF 2.1.0 types, reader, builder". Inspect what `pkg/wrapper/wrapdiag` does today: if it uses `pkg/sarif` types or a builder, mirror that. Only fall back to private struct copies (the inline form below) if `wrapdiag` itself does — and if so, file a follow-up bead to consolidate both wrappers onto `pkg/sarif`.

The struct definitions below are a fallback for when consolidation onto `pkg/sarif` is too invasive in this task.

```go
// Package wraparchlinttext converts go-arch-lint plain-text output into
// SARIF. The JSON output of go-arch-lint is already handled by
// wraparchlint; this wrapper exists for setups that pipe the text form
// (e.g. when the `--json` flag is unavailable or undesired in CI).
package wraparchlinttext

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var headerRe = regexp.MustCompile(`^\[(Warning|Error)\] Component "([^"]+)" shouldn't import component "([^"]+)"`)

type sarifReport struct {
	Schema  string     `json:"$schema,omitempty"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}
type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}
type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}
type sarifDriver struct {
	Name string `json:"name"`
}
type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}
type sarifMessage struct {
	Text string `json:"text"`
}
type sarifLocation struct {
	PhysicalLocation sarifPhys `json:"physicalLocation"`
}
type sarifPhys struct {
	ArtifactLocation sarifArt    `json:"artifactLocation"`
	Region           sarifRegion `json:"region,omitempty"`
}
type sarifArt struct {
	URI string `json:"uri"`
}
type sarifRegion struct {
	StartLine int `json:"startLine,omitempty"`
}

func Convert(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	var results []sarifResult
	var pending *sarifResult
	for sc.Scan() {
		line := sc.Text()
		if m := headerRe.FindStringSubmatch(line); m != nil {
			if pending != nil {
				results = append(results, *pending)
			}
			level := "warning"
			if m[1] == "Error" {
				level = "error"
			}
			pending = &sarifResult{
				RuleID:  "arch-lint/forbidden-import",
				Level:   level,
				Message: sarifMessage{Text: fmt.Sprintf("%s shouldn't import %s", m[2], m[3])},
			}
			continue
		}
		// Indented continuation: "  path/file.go: msg"
		if pending != nil && strings.HasPrefix(line, "  ") {
			trimmed := strings.TrimSpace(line)
			if idx := strings.IndexByte(trimmed, ':'); idx > 0 {
				pending.Locations = []sarifLocation{{
					PhysicalLocation: sarifPhys{
						ArtifactLocation: sarifArt{URI: trimmed[:idx]},
					},
				}}
			}
		}
	}
	if pending != nil {
		results = append(results, *pending)
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("archlinttext: read: %w", err)
	}
	rep := sarifReport{
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: "go-arch-lint"}},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rep); err != nil {
		return fmt.Errorf("archlinttext: encode: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Run tests; expect PASS**

Run: `go test ./pkg/wrapper/wraparchlinttext/...`

- [ ] **Step 4: Register wrap dispatch**

In `cmd/fo/main.go`, find `wrapNames` slice and the wrap dispatch switch. Add `archlint-text` entry mirroring `diag`:

```go
var wrapNames = []string{"archlint", "archlint-text", "diag", "jscpd", "leaderboard"}

// ...descriptions map...
"archlint-text": "Convert go-arch-lint text output to SARIF",

// ...switch cases...
case "archlint-text":
	return runWrapArchlintText(args[1:], stdin, stdout, stderr)
```

Add `runWrapArchlintText` modeled on `runWrapDiag` (no flags needed beyond shared ones):

```go
func runWrapArchlintText(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo wrap archlint-text", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := wraparchlinttext.Convert(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "fo wrap archlint-text: %v\n", err)
		return 2
	}
	return 0
}
```

- [ ] **Step 5: Update help golden + commit**

Run: `go test ./cmd/fo/... -run Help -update`
```bash
git add pkg/wrapper/wraparchlinttext/ cmd/fo/
git commit -m "feat(wrap): add archlint-text wrapper for go-arch-lint plain output"
```

---

## Task 10: Wrapper — `go test -bench` text → metrics  (`fo wrap gobench`)

**Files:**
- Create: `pkg/wrapper/wrapgobench/wrapgobench.go`
- Test: `pkg/wrapper/wrapgobench/wrapgobench_test.go`
- Modify: `cmd/fo/main.go`

> **Naming note.** This wrapper consumes the **raw `go test -bench`** output shape (`BenchmarkFoo-10  1234 ns/op  56 B/op  2 allocs/op`) — not benchstat's tabular comparison output (`golang.org/x/perf/cmd/benchstat`), which has columns for sec/op, geomean, confidence intervals, and a delta column. A real `benchstat` wrapper is a separate, larger task and is filed as a follow-up bead. Calling this wrapper `benchstat` would mis-advertise its input.

`go test -bench` text output looks like:

```
goos: darwin
goarch: arm64
pkg: github.com/x/y
BenchmarkFoo-10        1234 ns/op     56 B/op     2 allocs/op
BenchmarkBar-10        2345 ns/op    100 B/op     5 allocs/op
```

The wrapper emits one metrics row per `BenchmarkX/<metric>` (so a single benchmark line yields three rows: ns/op, B/op, allocs/op). Header (`goos:`, `goarch:`, `pkg:`) lines are dropped.

- [ ] **Step 1: Write failing test**

```go
func TestConvert_basic(t *testing.T) {
	in := `goos: darwin
pkg: x
BenchmarkFoo-10        1234 ns/op     56 B/op     2 allocs/op
`
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# fo:metrics tool=gobench", "BenchmarkFoo/ns_op 1234 ns/op", "BenchmarkFoo/allocs_op 2 allocs/op"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Implement**

```go
package wrapgobench

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Pairs each value with the unit token that follows it.
var benchRe = regexp.MustCompile(`^(Benchmark[\w/.-]+)\s+\d+\s+(.+)$`)

// Strips the trailing GOMAXPROCS suffix that the runtime appends to bench names
// (e.g. "BenchmarkFoo-10" → "BenchmarkFoo"). Anchored so it only fires on a
// trailing "-<digits>" group; benchmark names with embedded hyphens are safe.
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
		// Tail like: "1234 ns/op     56 B/op     2 allocs/op"
		// Iterate value/unit pairs.
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
	// "ns/op" → "ns_op"
	return strings.NewReplacer("/", "_").Replace(u)
}
```

> Caveat: this wrapper handles the common `<count> <unit> [...]` shape from `go test -bench`. A real benchstat-tabular wrapper (with delta column, geomean rows, confidence intervals) is a separate, larger task — file as a follow-up bead.

- [ ] **Step 3: Run; expect PASS**

- [ ] **Step 4: Register wrap dispatch + commit**

Same pattern as Task 9; add `gobench` to `wrapNames`, description, switch case, `runWrapGobench`. Update help golden.

```bash
git add pkg/wrapper/wrapgobench/ cmd/fo/
git commit -m "feat(wrap): add gobench wrapper for go test -bench → fo:metrics"
```

---

## Task 11: Wrapper — `go tool cover -func` → metrics

**Files:**
- Create: `pkg/wrapper/wrapcover/wrapcover.go`
- Test: `pkg/wrapper/wrapcover/wrapcover_test.go`
- Modify: `cmd/fo/main.go`

`go tool cover -func=cover.out` emits:

```
github.com/x/y/foo.go:12:	Foo	100.0%
github.com/x/y/foo.go:20:	Bar	75.0%
total:				(statements)	87.3%
```

Wrapper aggregates per-package: groups by import path, takes the package's share of `total:` lines, and emits one metrics row per package + a final `total` row. Or — simpler and matches what dk likely wants for a hygiene rollup — emit the per-function rows as keyed metrics (key = `path:line:func`), plus the trailing `total`.

For first cut, choose the simpler form: emit per-function rows + total.

- [ ] **Step 1: Write failing test**

```go
func TestConvert_basic(t *testing.T) {
	in := `github.com/x/y/foo.go:12:	Foo	100.0%
github.com/x/y/foo.go:20:	Bar	75.0%
total:				(statements)	87.3%
`
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# fo:metrics tool=cover", "github.com/x/y/foo.go:12:Foo 100", "total 87.3 %"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Implement**

```go
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
		// Last token is "<num>%". Strip % and parse.
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
			// "<path:line>:" "<func>" "<pct>"
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
```

- [ ] **Step 3: Run; expect PASS**

- [ ] **Step 4: Register wrap dispatch + commit**

```bash
git add pkg/wrapper/wrapcover/ cmd/fo/
git commit -m "feat(wrap): add cover wrapper for go tool cover -func"
```

---

## Task 12: Auto-Sniff Bare `count<sp>label` Tally

**Files:**
- Modify: `cmd/fo/main.go`

dk's request: `sort | uniq -c | sort -rn | fo` should work without `| fo wrap leaderboard`. Add a conservative sniffer that fires only when:
- input is non-SARIF, non-test-json, non-multiplex, non-`# fo:*`,
- every non-blank, non-`#` line matches `^\s*\d+(\.\d+)?\s+\S`,
- there are at least 2 such lines (avoid one-shot false positives).

- [ ] **Step 1: Write failing test**

```go
func TestE2E_bareTallyAutoDetect(t *testing.T) {
	in := "  10 alpha\n   3 beta\n   1 gamma\n"
	out, _, code := runFo(t, in, "--format", "llm")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("expected leaderboard render, got:\n%s", out)
	}
}

func TestE2E_bareTally_notRecognized(t *testing.T) {
	// Mixed content — must NOT auto-detect as tally.
	in := "10 alpha\nsomething else\n"
	_, errOut, code := runFo(t, in)
	if code == 0 {
		t.Errorf("expected unrecognized error, got success errOut=%s", errOut)
	}
}
```

- [ ] **Step 2: Implement sniffer**

In `cmd/fo/main.go`, add helper near the existing sniffers:

```go
func sniffBareTally(data []byte) bool {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	rows := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexAny(line, " \t")
		if idx <= 0 {
			return false
		}
		if _, err := strconv.ParseFloat(line[:idx], 64); err != nil {
			return false
		}
		if strings.TrimSpace(line[idx:]) == "" {
			return false
		}
		rows++
	}
	return rows >= 2
}
```

Add the dispatch (after existing `tally.IsHeader` check, before the unrecognized-input error):

```go
if sniffBareTally(input) {
	var buf bytes.Buffer
	if err := wrapleaderboard.Convert(bytes.NewReader(input), &buf, wrapleaderboard.Opts{}); err != nil {
		fmt.Fprintf(stderr, "fo: tally auto-detect: %v\n", err)
		return 2
	}
	input = buf.Bytes()
	// Fall through to tally.IsHeader path next.
	if tally.IsHeader(input) {
		return renderTally(input, stdout, stderr, format)
	}
}
```

**Concrete refactor for the dispatch.** Read `cmd/fo/main.go` first and identify the function that currently does the sniff-then-render decision (likely `runRender` or similar around the `tally.IsHeader` site). Replace the in-place `if tally.IsHeader(input) { return renderTally(...) }` with a one-liner that calls a small helper, e.g.:

```go
func dispatchTally(input []byte, stdout, stderr io.Writer, format string) int {
    return renderTally(input, stdout, stderr, format)
}
```

then both the explicit `tally.IsHeader` branch and the `sniffBareTally` branch call `dispatchTally(input, stdout, stderr, format)` after the input has been normalized (i.e., after `wrapleaderboard.Convert` produces a `# fo:tally` header). This avoids the "fall through to tally.IsHeader path next" awkwardness and keeps the sniffer order block (Task 3) honest. Keep `renderTally` as-is — only add `dispatchTally` as a thin shim if the existing code shape makes the in-line `return renderTally(...)` unreachable from the new branch.

- [ ] **Step 3: Run; expect both tests PASS**

- [ ] **Step 4: Commit**

```bash
git add cmd/fo/
git commit -m "feat(cli): auto-detect bare 'count label' tally on stdin"
```

---

## Task 13: Migration Guide + README Update

**Files:**
- Create: `docs/guides/hygiene-formats.md`
- Modify: `README.md` (add new formats to the supported-inputs section)

- [ ] **Step 1: Write the guide**

Cover four sections:
1. **What fo accepts on stdin** — table mapping each input shape to its renderer (SARIF, go test -json, `# fo:tally`, `# fo:status`, `# fo:metrics`, multiplex).
2. **Hint flag** — `--as tally|status|metrics|diag` for ambiguous inputs.
3. **Wrappers** — table of `fo wrap <name>` invocations, what they consume, what they emit.
4. **Migration recipes for trixi/snipe/next** — concrete diffs:
   - trixi `arch-lint` target: replace `grep ... | awk` with `go-arch-lint check ./... | fo wrap archlint-text | fo`.
   - trixi `doctor` target: replace `printf "%-30s %s\n" ... ok|MISSING` with a script that emits `# fo:status\nok foo\nfail bar  not-installed\n` and pipes to `fo`.
   - trixi `eval:trend` target: emit `# fo:metrics tool=eval` with one row per metric; `fo` shows deltas vs the prior run.
   - snipe / next sandbox build size: replace `du -h ... | sort -rh` with a script emitting `# fo:metrics tool=size\n<binary> <bytes>\n…` piped through fo.
   - All three projects' `bench` targets: `go test -bench=. -count=5 ./... | fo wrap gobench | fo`. (A real benchstat-tabular wrapper is on the deferred bead list; until it lands, use raw `go test -bench` text.)

- [ ] **Step 2: Update README.md**

Add the three new format markers (`# fo:status`, `# fo:metrics`, plus the bare-tally auto-detect note) to whichever section enumerates accepted inputs. Add the `--as` flag.

- [ ] **Step 3: Run all tests one more time**

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add docs/ README.md
git commit -m "docs: hygiene formats guide + Makefile migration recipes"
```

---

## Self-Review

**Spec coverage:**
- "Auto-detect Go-tool outputs" → covered by Tasks 9 (archlint-text), 10 (benchstat), 11 (cover); existing wrappers cover vet/build/golangci/jscpd/test-json/SARIF.
- "fo defines extra formats dk's tools emit" → covered by Tasks 1–7 (status + metrics with delta).
- "Accept hints" → Task 8 (`--as`).
- "uniq -c without explicit wrap" → Task 12.

**Gaps deliberately not addressed (file as separate beads if/when needed):**
- govulncheck text output — the SARIF route already works (`-format sarif`), and the text form is rarely worth a wrapper.
- nilaway text — already handled by `wrapdiag` since it's `file:line:col` shaped.
- Real benchstat (golang.org/x/perf) tabular-comparison wrapper — Task 10 ships `gobench` for raw `go test -bench` only.
- pprof tabular output — out of scope; not used in any of the three Makefiles.
- **Multiplex protocol extension for status/metrics.** `pkg/report` defines a `--- tool: --- protocol` delimiter for findings + tests in one stream. Status and metrics live in parallel rendering paths (separate sniffers, separate renderers, no Report IR). If `make ci-report` later wants findings + status + metrics in one piped stream, the delimiter protocol needs to learn the new tool kinds. File a bead when that need shows up.
- **`fo --print-schema` only emits Report.** It dumps `pkg/report.Schema`. After this plan ships, also expose `pkg/status.Schema` and `pkg/metrics.Schema` (or document explicitly in `--help` that `--print-schema` is Report-only). File a bead for one or the other.
- **Theme integration.** `pkg/theme` (color/mono) exists per arch doc. Status and metrics renderers in this plan print `── tool ──` headers directly without routing through theme. By design for first cut: hygiene formats render mono-only. A follow-up bead can route them through `pkg/theme` once the visual treatment is settled.
- **Sidecar consolidation.** `pkg/state` will hold both `last-run.json` (findings/tests fingerprints) and `metrics-history.json` after Task 5. Eventually merge into a single envelope to avoid two-file atomicity windows. Not blocking.

**Type consistency check:**
- `view.StatusRow{State,Label,Value,Note}` matches `pkg/status.Row` field-for-field via `ToViewRows`.
- `view.MetricRow{Key,Value,Unit,Delta}` carries Delta; `state.MetricDelta{Sample,Prior,Delta}` is the persisted form, converted at the seam in Task 7 step 4.
- `wrapNames` slice in Task 9/10/11 stays consistent — additions append, no renames.

**Placeholders:** none — every step contains real code or a real command.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-03-hygiene-formats.md`. Two execution options:

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using executing-plans, batch with checkpoints.

Recommend filing one bead per task as children of fo-co5 (or a new epic) so progress is visible in `bd ready`. Tasks 1–3 (status), 4–7 (metrics), 8 (--as), 9–11 (wrappers), 12 (auto-sniff), 13 (docs) form natural shipping increments — each pair/triple is independently mergeable.
