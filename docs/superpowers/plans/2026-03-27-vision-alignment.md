# Vision Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align fo's code and docs with its simplified vision: two native input formats (SARIF, go-test-json), compiled-in wrapper plugin system for everything else, fo-metrics/v1 eliminated.

**Architecture:** Add `pkg/wrapper/` with a `Wrapper` interface + flat registry. Move existing wrap logic from `cmd/fo/` into wrapper packages. Cut `internal/fometrics`. Simplify `internal/report` regex and `pkg/mapper/report.go` to only handle SARIF and testjson sections. Update testdata and tests.

**Tech Stack:** Go 1.24+, `pkg/sarif.Builder` for SARIF construction, standard library only.

---

### Task 1: Create Wrapper Interface and Registry

**Files:**
- Create: `pkg/wrapper/wrapper.go`
- Create: `pkg/wrapper/registry.go`

- [ ] **Step 1: Create `pkg/wrapper/wrapper.go`**

```go
// Package wrapper defines the plugin interface for converting tool output
// into fo-native formats (SARIF or go-test-json).
package wrapper

import "io"

// Format identifies a fo-native output format.
type Format string

const (
	FormatSARIF    Format = "sarif"
	FormatTestJSON Format = "testjson"
)

// Wrapper converts tool-specific output into a fo-native format.
type Wrapper interface {
	// OutputFormat returns the native format this wrapper produces.
	OutputFormat() Format

	// Wrap reads tool output from r, writes fo-native output to w.
	// args contains any flags passed after "fo wrap <name>".
	Wrap(args []string, r io.Reader, w io.Writer) error
}
```

- [ ] **Step 2: Create `pkg/wrapper/registry.go`** (initially empty registry — wrappers added in later tasks)

```go
package wrapper

import "sort"

var registry = map[string]Wrapper{}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
	return registry[name]
}

// Names returns all registered wrapper names, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./pkg/wrapper/...`
Expected: clean build, no errors.

- [ ] **Step 4: Commit**

```bash
git add pkg/wrapper/wrapper.go pkg/wrapper/registry.go
git commit -m "feat(wrapper): add Wrapper interface and registry"
```

---

### Task 2: Create wrapdiag Wrapper

Move `parseDiagLine` and the sarif wrapping logic from `cmd/fo/main.go` into `pkg/wrapper/wrapdiag/`.

**Files:**
- Create: `pkg/wrapper/wrapdiag/diag.go`
- Create: `pkg/wrapper/wrapdiag/diag_test.go`
- Modify: `pkg/wrapper/registry.go` (add import + registration)

- [ ] **Step 1: Write `pkg/wrapper/wrapdiag/diag_test.go`**

```go
package wrapdiag

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestDiag_OutputFormat(t *testing.T) {
	d := New()
	if d.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", d.OutputFormat())
	}
}

func TestDiag_FileLineColMessage(t *testing.T) {
	input := "main.go:15:3: unreachable code after return\npkg/util.go:42: unused variable x\n"
	var buf bytes.Buffer
	if err := New().Wrap([]string{"--tool", "govet"}, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", doc.Version)
	}
	if doc.Runs[0].Tool.Driver.Name != "govet" {
		t.Errorf("expected tool govet, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.Locations[0].PhysicalLocation.Region.StartLine != 15 {
		t.Errorf("expected line 15, got %d", r.Locations[0].PhysicalLocation.Region.StartLine)
	}
}

func TestDiag_FileOnly(t *testing.T) {
	input := "pkg/handler.go\nmain.go\n"
	var buf bytes.Buffer
	err := New().Wrap([]string{"--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"}, strings.NewReader(input), &buf)
	if err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(doc.Runs[0].Results))
	}
	if doc.Runs[0].Results[0].RuleID != "needs-formatting" {
		t.Errorf("expected rule needs-formatting, got %s", doc.Runs[0].Results[0].RuleID)
	}
}

func TestDiag_MissingToolFlag(t *testing.T) {
	var buf bytes.Buffer
	err := New().Wrap([]string{}, strings.NewReader("x.go:1: msg\n"), &buf)
	if err == nil {
		t.Error("expected error for missing --tool flag")
	}
}

func TestDiag_WindowsDriveLetter(t *testing.T) {
	input := `C:\Users\dev\main.go:15:3: unreachable code` + "\n"
	var buf bytes.Buffer
	if err := New().Wrap([]string{"--tool", "govet"}, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	uri := doc.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI
	if uri != `C:\Users\dev\main.go` {
		t.Errorf("expected Windows path, got %q", uri)
	}
}

func TestParseDiagLine(t *testing.T) {
	tests := []struct {
		input            string
		wantFile         string
		wantLine, wantCol int
		wantMsg          string
	}{
		{"main.go:15:3: unreachable code", "main.go", 15, 3, "unreachable code"},
		{"pkg/util.go:42: unused variable x", "pkg/util.go", 42, 0, "unused variable x"},
		{"pkg/handler.go", "pkg/handler.go", 0, 0, "needs formatting"},
		{`C:\Users\dev\main.go:15:3: unreachable code`, `C:\Users\dev\main.go`, 15, 3, "unreachable code"},
		{`D:\proj\util.go:42: unused`, `D:\proj\util.go`, 42, 0, "unused"},
		{"not a diagnostic", "", 0, 0, ""},
	}
	for _, tt := range tests {
		file, ln, col, msg := parseDiagLine(tt.input)
		if file != tt.wantFile || ln != tt.wantLine || col != tt.wantCol || msg != tt.wantMsg {
			t.Errorf("parseDiagLine(%q) = (%q,%d,%d,%q), want (%q,%d,%d,%q)",
				tt.input, file, ln, col, msg, tt.wantFile, tt.wantLine, tt.wantCol, tt.wantMsg)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/wrapper/wrapdiag/ -v`
Expected: compilation failure — `New` not defined.

- [ ] **Step 3: Write `pkg/wrapper/wrapdiag/diag.go`**

Move `parseDiagLine` from `cmd/fo/main.go` and `runWrapSarif` logic. Adapt to `Wrapper` interface:

```go
// Package wrapdiag converts line-based Go diagnostics into SARIF 2.1.0.
//
// Input formats:
//   - file.go:line:col: message
//   - file.go:line: message
//   - file.go (file-only, e.g. gofmt -l)
package wrapdiag

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

// Diag converts line-based diagnostics to SARIF.
type Diag struct{}

// New returns a new Diag wrapper.
func New() *Diag { return &Diag{} }

// OutputFormat returns FormatSARIF.
func (d *Diag) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// Wrap parses line diagnostics from r and writes SARIF to w.
// Required flag: --tool <name>. Optional: --rule, --level, --version.
func (d *Diag) Wrap(args []string, r io.Reader, w io.Writer) error {
	fs := flag.NewFlagSet("fo wrap diag", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	toolName := fs.String("tool", "", "Tool name for SARIF driver.name (required)")
	ruleID := fs.String("rule", "finding", "Default rule ID")
	level := fs.String("level", "warning", "Default severity: error|warning|note")
	version := fs.String("version", "", "Tool version string")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *toolName == "" {
		return fmt.Errorf("--tool is required")
	}

	b := sarif.NewBuilder(*toolName, *version)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		file, ln, col, msg := parseDiagLine(line)
		if file == "" {
			continue
		}
		b.AddResult(*ruleID, *level, msg, file, ln, col)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	_, err := b.WriteTo(w)
	return err
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

	if len(rest) >= 3 && rest[1] == ':' && (rest[2] == '\\' || rest[2] == '/') {
		prefix = rest[:2]
		rest = rest[2:]
	}

	parts := strings.SplitN(rest, ":", 4)
	if len(parts) >= 4 {
		if l, err := strconv.Atoi(parts[1]); err == nil {
			if c, err := strconv.Atoi(parts[2]); err == nil {
				return prefix + parts[0], l, c, strings.TrimSpace(parts[3])
			}
		}
	}

	if len(parts) >= 3 {
		if l, err := strconv.Atoi(parts[1]); err == nil {
			return prefix + parts[0], l, 0, strings.TrimSpace(strings.Join(parts[2:], ":"))
		}
	}

	trimmed := strings.TrimSpace(line)
	if strings.HasSuffix(trimmed, ".go") || strings.Contains(trimmed, "/") {
		if !strings.Contains(trimmed, " ") {
			return trimmed, 0, 0, "needs formatting"
		}
	}

	return "", 0, 0, ""
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/wrapper/wrapdiag/ -v`
Expected: all tests PASS.

- [ ] **Step 5: Register in registry**

Edit `pkg/wrapper/registry.go`:

```go
package wrapper

import (
	"sort"

	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
)

var registry = map[string]Wrapper{
	"diag": wrapdiag.New(),
}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
	return registry[name]
}

// Names returns all registered wrapper names, sorted.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./pkg/wrapper/...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add pkg/wrapper/
git commit -m "feat(wrapper): add wrapdiag wrapper (line diagnostics -> SARIF)"
```

---

### Task 3: Create wrapjscpd Wrapper

Move jscpd parser from `internal/jscpd/` into the wrapper package and convert output from fo-metrics to SARIF.

**Files:**
- Create: `pkg/wrapper/wrapjscpd/jscpd.go` (parser + wrapper)
- Create: `pkg/wrapper/wrapjscpd/jscpd_test.go`
- Modify: `pkg/wrapper/registry.go` (add import + registration)

- [ ] **Step 1: Write `pkg/wrapper/wrapjscpd/jscpd_test.go`**

```go
package wrapjscpd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestJscpd_OutputFormat(t *testing.T) {
	w := New()
	if w.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", w.OutputFormat())
	}
}

func TestJscpd_EmptyDuplicates(t *testing.T) {
	input := `{"duplicates":[],"statistics":{}}`
	var buf bytes.Buffer
	if err := New().Wrap(nil, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", doc.Version)
	}
	if doc.Runs[0].Tool.Driver.Name != "jscpd" {
		t.Errorf("expected tool jscpd, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results for empty duplicates, got %d", len(doc.Runs[0].Results))
	}
}

func TestJscpd_WithClones(t *testing.T) {
	input := `{"duplicates":[{
		"format":"go",
		"lines":22,
		"firstFile":{"name":"a.go","startLoc":{"line":1},"endLoc":{"line":22}},
		"secondFile":{"name":"b.go","startLoc":{"line":10},"endLoc":{"line":31}}
	}],"statistics":{}}`
	var buf bytes.Buffer
	if err := New().Wrap(nil, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.RuleID != "code-clone" {
		t.Errorf("expected ruleId code-clone, got %s", r.RuleID)
	}
	if r.Level != "warning" {
		t.Errorf("expected level warning, got %s", r.Level)
	}
	if !strings.Contains(r.Message.Text, "b.go") {
		t.Errorf("expected message to reference b.go, got %q", r.Message.Text)
	}
	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "a.go" {
		t.Errorf("expected location a.go, got %s", loc.ArtifactLocation.URI)
	}
	if loc.Region.StartLine != 1 {
		t.Errorf("expected start line 1, got %d", loc.Region.StartLine)
	}
}

func TestJscpd_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	err := New().Wrap(nil, strings.NewReader("not json"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJscpd_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := New().Wrap(nil, strings.NewReader(""), &buf)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseClones(t *testing.T) {
	input := []byte(`{"duplicates":[{
		"format":"go","lines":10,
		"firstFile":{"name":"x.go","startLoc":{"line":5},"endLoc":{"line":14}},
		"secondFile":{"name":"y.go","startLoc":{"line":20},"endLoc":{"line":29}}
	}],"statistics":{}}`)
	clones, err := parseClones(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(clones) != 1 {
		t.Fatalf("expected 1 clone, got %d", len(clones))
	}
	c := clones[0]
	if c.FileA != "x.go" || c.FileB != "y.go" || c.Lines != 10 {
		t.Errorf("unexpected clone: %+v", c)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/wrapper/wrapjscpd/ -v`
Expected: compilation failure — `New` not defined.

- [ ] **Step 3: Write `pkg/wrapper/wrapjscpd/jscpd.go`**

```go
// Package wrapjscpd converts jscpd JSON duplication reports into SARIF 2.1.0.
package wrapjscpd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

// clone records a single code duplication instance.
type clone struct {
	Format string
	Lines  int
	FileA  string
	StartA int
	EndA   int
	FileB  string
	StartB int
	EndB   int
}

// Jscpd converts jscpd JSON to SARIF.
type Jscpd struct{}

// New returns a new Jscpd wrapper.
func New() *Jscpd { return &Jscpd{} }

// OutputFormat returns FormatSARIF.
func (j *Jscpd) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// Wrap reads jscpd JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for jscpd reports (typically <1MB).
func (j *Jscpd) Wrap(args []string, r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	clones, err := parseClones(data)
	if err != nil {
		return err
	}

	b := sarif.NewBuilder("jscpd", "")
	for _, c := range clones {
		msg := fmt.Sprintf("%d lines duplicated with %s:%d-%d", c.Lines, c.FileB, c.StartB, c.EndB)
		b.AddResult("code-clone", "warning", msg, c.FileA, c.StartA, 0)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseClones decodes jscpd JSON report into a slice of clones.
func parseClones(data []byte) ([]clone, error) {
	var raw struct {
		Duplicates []struct {
			Format     string `json:"format"`
			Lines      int    `json:"lines"`
			FirstFile  struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"firstFile"`
			SecondFile struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"secondFile"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("jscpd: %w", err)
	}

	clones := make([]clone, 0, len(raw.Duplicates))
	for _, d := range raw.Duplicates {
		clones = append(clones, clone{
			Format: d.Format, Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line, EndA: d.FirstFile.EndLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return clones, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/wrapper/wrapjscpd/ -v`
Expected: all tests PASS.

- [ ] **Step 5: Register in registry**

Add import and entry to `pkg/wrapper/registry.go`:

```go
import (
	"sort"

	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	"github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

var registry = map[string]Wrapper{
	"diag":  wrapdiag.New(),
	"jscpd": wrapjscpd.New(),
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./pkg/wrapper/...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add pkg/wrapper/
git commit -m "feat(wrapper): add wrapjscpd wrapper (jscpd JSON -> SARIF)"
```

---

### Task 4: Create wraparchlint Wrapper

Move archlint parser from `internal/archlint/` into the wrapper package and convert output from fo-metrics to SARIF.

**Files:**
- Create: `pkg/wrapper/wraparchlint/archlint.go` (parser + wrapper)
- Create: `pkg/wrapper/wraparchlint/archlint_test.go`
- Modify: `pkg/wrapper/registry.go` (add import + registration)

- [ ] **Step 1: Write `pkg/wrapper/wraparchlint/archlint_test.go`**

```go
package wraparchlint

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

func TestArchlint_OutputFormat(t *testing.T) {
	w := New()
	if w.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", w.OutputFormat())
	}
}

func TestArchlint_Clean(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var buf bytes.Buffer
	if err := New().Wrap(nil, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc.Runs[0].Tool.Driver.Name != "go-arch-lint" {
		t.Errorf("expected tool go-arch-lint, got %s", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results for clean output, got %d", len(doc.Runs[0].Results))
	}
}

func TestArchlint_WithViolations(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"search","FileRelativePath":"pkg/search/search.go","ResolvedImportName":"embedder"}
	],"ArchWarningsNotMatched":[],"ArchWarningsDeepScan":[],"OmittedCount":0,"Qualities":[]}}`
	var buf bytes.Buffer
	if err := New().Wrap(nil, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if r.RuleID != "dependency-violation" {
		t.Errorf("expected ruleId dependency-violation, got %s", r.RuleID)
	}
	if r.Level != "error" {
		t.Errorf("expected level error, got %s", r.Level)
	}
	if !strings.Contains(r.Message.Text, "search") || !strings.Contains(r.Message.Text, "embedder") {
		t.Errorf("expected message to reference search and embedder, got %q", r.Message.Text)
	}
	loc := r.Locations[0].PhysicalLocation
	if loc.ArtifactLocation.URI != "pkg/search/search.go" {
		t.Errorf("expected location pkg/search/search.go, got %s", loc.ArtifactLocation.URI)
	}
}

func TestArchlint_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	err := New().Wrap(nil, strings.NewReader("bad"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestArchlint_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := New().Wrap(nil, strings.NewReader(""), &buf)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestArchlint_FullImportPath(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"agentSupervisor","FileRelativePath":"/internal/agent/supervisor/supervisor.go","ResolvedImportName":"github.com/example/project/internal/agent/shell"}
	],"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var buf bytes.Buffer
	if err := New().Wrap(nil, strings.NewReader(input), &buf); err != nil {
		t.Fatal(err)
	}

	var doc sarif.Document
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Runs[0].Results))
	}
	r := doc.Runs[0].Results[0]
	if !strings.Contains(r.Message.Text, "github.com/example/project/internal/agent/shell") {
		t.Errorf("expected full import path in message, got %q", r.Message.Text)
	}
}

func TestParseResult(t *testing.T) {
	input := []byte(`{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"a","FileRelativePath":"a.go","ResolvedImportName":"b"},
		{"ComponentName":"c","FileRelativePath":"c.go","ResolvedImportName":"d"}
	],"Qualities":[{"ID":"q1","Used":true},{"ID":"q2","Used":false}]}}`)
	result, err := parseResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.HasWarnings {
		t.Error("expected HasWarnings true")
	}
	if len(result.Violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(result.Violations))
	}
	if result.CheckCount != 2 {
		t.Errorf("expected 2 checks, got %d", result.CheckCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/wrapper/wraparchlint/ -v`
Expected: compilation failure — `New` not defined.

- [ ] **Step 3: Write `pkg/wrapper/wraparchlint/archlint.go`**

```go
// Package wraparchlint converts go-arch-lint JSON output into SARIF 2.1.0.
package wraparchlint

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

type violation struct {
	From     string
	To       string
	FileFrom string
}

type result struct {
	HasWarnings bool
	Violations  []violation
	CheckCount  int
}

// Archlint converts go-arch-lint JSON to SARIF.
type Archlint struct{}

// New returns a new Archlint wrapper.
func New() *Archlint { return &Archlint{} }

// OutputFormat returns FormatSARIF.
func (a *Archlint) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// Wrap reads go-arch-lint JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for arch-lint reports (typically <100KB).
func (a *Archlint) Wrap(args []string, r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	res, err := parseResult(data)
	if err != nil {
		return err
	}

	b := sarif.NewBuilder("go-arch-lint", "")
	for _, v := range res.Violations {
		msg := fmt.Sprintf("%s \u2192 %s", v.From, v.To)
		b.AddResult("dependency-violation", "error", msg, v.FileFrom, 0, 0)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseResult decodes go-arch-lint --json output.
func parseResult(data []byte) (*result, error) {
	var raw struct {
		Payload struct {
			ArchHasWarnings  bool `json:"ArchHasWarnings"`
			ArchWarningsDeps []struct {
				ComponentName      string `json:"ComponentName"`
				FileRelativePath   string `json:"FileRelativePath"`
				ResolvedImportName string `json:"ResolvedImportName"`
			} `json:"ArchWarningsDeps"`
			Qualities []struct {
				ID   string `json:"ID"`
				Used bool   `json:"Used"`
			} `json:"Qualities"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("archlint: %w", err)
	}

	r := &result{HasWarnings: raw.Payload.ArchHasWarnings}
	for _, d := range raw.Payload.ArchWarningsDeps {
		r.Violations = append(r.Violations, violation{
			From:     d.ComponentName,
			To:       d.ResolvedImportName,
			FileFrom: d.FileRelativePath,
		})
	}
	r.CheckCount = len(raw.Payload.Qualities)
	return r, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/wrapper/wraparchlint/ -v`
Expected: all tests PASS.

- [ ] **Step 5: Register in registry**

Add import and entry to `pkg/wrapper/registry.go`:

```go
import (
	"sort"

	"github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	"github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	"github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

var registry = map[string]Wrapper{
	"archlint": wraparchlint.New(),
	"diag":     wrapdiag.New(),
	"jscpd":    wrapjscpd.New(),
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./pkg/wrapper/...`
Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add pkg/wrapper/
git commit -m "feat(wrapper): add wraparchlint wrapper (go-arch-lint JSON -> SARIF)"
```

---

### Task 5: Wire Wrapper Dispatch into cmd/fo

Replace the format-specific `runWrap` dispatch with the generic wrapper registry lookup.

**Files:**
- Modify: `cmd/fo/main.go` (replace `runWrap`, `runWrapSarif`, remove `parseDiagLine`)
- Delete: `cmd/fo/wrap_jscpd.go`
- Delete: `cmd/fo/wrap_archlint.go`

- [ ] **Step 1: Update tests that call `runWrap` with old wrapper names**

In `cmd/fo/main_test.go`, update wrapper name references. Replace `"sarif"` as the first `runWrap` arg with `"diag"`:

- `TestJTBD_WrapSARIFConvertsLineDiagnostics`: `runWrap([]string{"sarif", "--tool", "govet"}` → `runWrap([]string{"diag", "--tool", "govet"}`
- `TestJTBD_WrapSARIFFileOnly`: `runWrap([]string{"sarif", "--tool", "gofmt", ...}` → `runWrap([]string{"diag", "--tool", "gofmt", ...}`
- `TestJTBD_WrapSARIFMissingToolFlag`: `runWrap([]string{"sarif"}` → `runWrap([]string{"diag"}`
- `TestJTBD_WrapSARIFLongLine`: `runWrap([]string{"sarif", "--tool", "big"}` → `runWrap([]string{"diag", "--tool", "big"}`

Delete the old wrapper test files entirely — their coverage is replaced by tests in `pkg/wrapper/wrapjscpd/` (Task 3) and `pkg/wrapper/wraparchlint/` (Task 4). Use `git rm` so the commit step doesn't fail:

```bash
git rm cmd/fo/wrap_jscpd_test.go cmd/fo/wrap_archlint_test.go
```

- [ ] **Step 2: Replace `runWrap` in `cmd/fo/main.go`**

Replace the entire `runWrap` function and remove the `runWrapSarif` and `parseDiagLine` functions. Add import for `wrapper` package. Remove imports for `strconv`, `bufio` (if no longer needed — `bufio` is still used in `run()`), and the SARIF-specific wrap code.

New `runWrap`:

```go
func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "fo wrap: wrapper name required\n\nAvailable wrappers: %s\n",
			strings.Join(wrapper.Names(), ", "))
		return 2
	}
	w := wrapper.Get(args[0])
	if w == nil {
		fmt.Fprintf(stderr, "fo wrap: unknown wrapper %q\n\nAvailable wrappers: %s\n",
			args[0], strings.Join(wrapper.Names(), ", "))
		return 2
	}
	if err := w.Wrap(args[1:], stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "fo wrap %s: %v\n", args[0], err)
		return 2
	}
	return 0
}
```

Remove from `main.go`:
- The entire `runWrapSarif` function
- The entire `parseDiagLine` function
- The `wrapUsage` const
- Import `strconv` (only used by `parseDiagLine`)

Add to imports:
- `"github.com/dkoosis/fo/pkg/wrapper"`

Remove from imports:
- `"strconv"` (only used by `parseDiagLine`, which is now in `wrapdiag`)

Keep these imports — they're still used by `run()`/`parseInput`:
- `"bufio"` (stdin peeking in `run()`)
- `"github.com/dkoosis/fo/pkg/sarif"` (used for `sarif.ReadBytes` in `parseInput`)

- [ ] **Step 3: Delete `cmd/fo/wrap_jscpd.go`**

```bash
rm cmd/fo/wrap_jscpd.go
```

- [ ] **Step 4: Delete `cmd/fo/wrap_archlint.go`**

```bash
rm cmd/fo/wrap_archlint.go
```

- [ ] **Step 5: Verify no stale references to deleted functions**

Run: `rg 'runWrapSarif|runWrapJscpd|runWrapArchlint|parseDiagLine' --type go`
Expected: no matches. If any remain, fix them before proceeding.

- [ ] **Step 6: Run tests to verify**

Run: `go test ./cmd/fo/ -v -run TestJTBD_Wrap`
Expected: all wrap tests PASS.

Run: `go test ./cmd/fo/ -v -run TestWrap`
Expected: all wrapper integration tests PASS.

- [ ] **Step 7: Run full test suite**

Run: `go test ./... 2>&1 | tail -20`
Expected: all tests PASS. Some tests may fail due to testdata files with old formats — those are fixed in Task 7.

- [ ] **Step 8: Commit**

```bash
git rm cmd/fo/wrap_jscpd.go cmd/fo/wrap_archlint.go
git add cmd/fo/main.go cmd/fo/main_test.go
git commit -m "refactor(cmd/fo): replace format-specific wrap with generic wrapper dispatch"
```

Note: `wrap_jscpd_test.go` and `wrap_archlint_test.go` were already `git rm`'d in Step 1.

---

### Task 6: Simplify Report Format

Narrow the report delimiter regex to only allow `sarif` and `testjson`. Simplify the mapper.

**Files:**
- Modify: `internal/report/report.go` (narrow regex)
- Modify: `internal/report/report_test.go` (update tests)
- Modify: `pkg/mapper/report.go` (delete metrics/archlint/jscpd/text handlers)
- Modify: `pkg/mapper/report_test.go` (delete tests for removed handlers)

- [ ] **Step 1: Update regex in `internal/report/report.go`**

Change the `delimiterRe` from:
```go
var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson|text|metrics|archlint|jscpd)(?: status:(pass|fail))? ---$`,
)
```

To:
```go
var delimiterRe = regexp.MustCompile(
	`^--- tool:(\w[\w-]*) format:(sarif|testjson)(?: status:(pass|fail))? ---$`,
)
```

Also update the `Section.Format` field comment from `"sarif", "testjson", "text", "metrics", "archlint", "jscpd"` to `"sarif" or "testjson"`.

- [ ] **Step 2: Update `internal/report/report_test.go`**

Remove any tests that use `format:text`, `format:metrics`, `format:archlint`, or `format:jscpd` in delimiters. Specifically:
- `TestParse_MultipleSections`: remove the `format:text` section, expect 2 sections not 3
- `TestIsDelimiter`: `"--- tool:arch format:text status:pass ---"` → `false`, `"--- tool:m format:metrics ---"` → `false`

Add a test verifying the old formats are no longer recognized as delimiters (they become preamble/content, and Parse returns `ErrNoSections` when no valid delimiters exist):

```go
func TestParse_OldFormatsNotRecognized(t *testing.T) {
	for _, format := range []string{"text", "metrics", "archlint", "jscpd"} {
		input := []byte(fmt.Sprintf("--- tool:x format:%s ---\ncontent\n", format))
		sections, err := Parse(input)
		// Old format delimiter is not recognized by the narrowed regex.
		// It becomes preamble, and with no valid delimiters, Parse returns ErrNoSections.
		if err == nil {
			t.Errorf("format:%s — expected ErrNoSections, got %d sections", format, len(sections))
		}
		if !errors.Is(err, ErrNoSections) {
			t.Errorf("format:%s — expected ErrNoSections, got %v", format, err)
		}
	}
}
```
```

- [ ] **Step 3: Run report tests**

Run: `go test ./internal/report/ -v`
Expected: PASS.

- [ ] **Step 4: Simplify `pkg/mapper/report.go`**

Delete these functions:
- `mapMetricsSection`
- `mapArchLintSection`
- `mapJSCPDSection`
- `mapTextSection`
- `mapMetricsStatus`
- `buildMetricsLabel`
- `mapDetailSeverity`

Simplify `mapSection` to:

```go
func mapSection(sec report.Section) ([]pattern.Pattern, pattern.ItemKind, string) {
	switch sec.Format {
	case "sarif":
		return mapSARIFSection(sec)
	case "testjson":
		return mapTestJSONSection(sec)
	default:
		return sectionError(sec.Tool, fmt.Errorf("unknown format %q", sec.Format)),
			pattern.KindError, fmt.Sprintf("unknown format %q", sec.Format)
	}
}
```

Remove imports that are no longer used:
- `"github.com/dkoosis/fo/internal/archlint"`
- `"github.com/dkoosis/fo/internal/fometrics"`
- `"github.com/dkoosis/fo/internal/jscpd"`

- [ ] **Step 5: Update `pkg/mapper/report_test.go`**

Delete tests for removed functionality:
- `TestFromReport_TextPassSection`
- `TestFromReport_AllPassLabel` (uses `format:text`)
- `TestFromReport_FailLabel` (uses `format:text`)
- `TestFromReport_MetricsPassSection`
- `TestFromReport_MetricsFailSection`
- `TestFromReport_MetricsWarnSection`
- `TestFromReport_MetricsFailNoDetails`
- `TestFromReport_MetricsEmptyMetrics`
- `TestFromReport_MetricsWithUnit`
- `TestFromReport_MetricsInvalidJSON`
- `TestFromReport_MetricsDetailCategories`

Update `TestFromReport_MultiSection` — replace the `format:text` section with a `format:sarif` section:

```go
func TestFromReport_MultiSection(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "lint", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	sum, ok := patterns[0].(*pattern.Summary)
	if !ok {
		t.Fatalf("patterns[0] is %T, want *pattern.Summary", patterns[0])
	}
	if len(sum.Metrics) != 2 {
		t.Errorf("expected 2 tool metrics, got %d", len(sum.Metrics))
	}
}
```

Add tests for all-pass and fail labels using SARIF sections:

```go
func TestFromReport_AllPassLabel(t *testing.T) {
	sarifDoc := `{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"tool"}},"results":[]}]}`
	sections := []report.Section{
		{Tool: "vet", Format: "sarif", Content: []byte(sarifDoc)},
		{Tool: "lint", Format: "sarif", Content: []byte(sarifDoc)},
	}
	patterns := FromReport(sections)
	sum := patterns[0].(*pattern.Summary)
	if !strings.Contains(sum.Label, "all pass") {
		t.Errorf("expected 'all pass' in label, got %q", sum.Label)
	}
}
```

- [ ] **Step 6: Run mapper tests**

Run: `go test ./pkg/mapper/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/report/report.go internal/report/report_test.go
git add pkg/mapper/report.go pkg/mapper/report_test.go
git commit -m "refactor(report): narrow to sarif+testjson sections only"
```

---

### Task 7: Update Testdata and E2E Tests

Update report testdata files and E2E tests that reference old formats.

**Files:**
- Modify: `cmd/fo/testdata/clean.report` (remove `format:text` section)
- Modify: `cmd/fo/testdata/full.report` (remove metrics/archlint/jscpd sections)
- Modify: `cmd/fo/testdata/failing.report` (remove `format:text` section)
- Modify: `cmd/fo/main_test.go` (update E2E tests)

- [ ] **Step 1: Update `cmd/fo/testdata/clean.report`**

Remove the `format:text` section:

```
--- tool:vet format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}
--- tool:test format:testjson ---
{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}
{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.05}
{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.1}
```

- [ ] **Step 2: Update `cmd/fo/testdata/full.report`**

Remove `format:metrics`, `format:archlint`, `format:jscpd` sections. Keep only sarif and testjson:

```
--- tool:vet format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}
--- tool:lint format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"golangci-lint"}},"results":[]}]}
--- tool:test format:testjson ---
{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}
{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.05}
{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.1}
--- tool:vuln format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govulncheck"}},"results":[]}]}
```

- [ ] **Step 3: Update `cmd/fo/testdata/failing.report`**

Remove the `format:text` section:

```
--- tool:vet format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}
--- tool:lint format:sarif ---
{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"golangci-lint"}},"results":[{"ruleId":"errcheck","level":"error","message":{"text":"error return value not checked"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"internal/store/store.go"},"region":{"startLine":42,"startColumn":5}}}]}]}]}
--- tool:test format:testjson ---
{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestParser"}
{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg","Test":"TestParser","Output":"    parser_test.go:20: expected nil, got error: unexpected token\n"}
{"Time":"2024-01-01T00:00:01Z","Action":"fail","Package":"example.com/pkg","Test":"TestParser","Elapsed":0.02}
{"Time":"2024-01-01T00:00:01Z","Action":"fail","Package":"example.com/pkg","Elapsed":0.5}
```

- [ ] **Step 4: Update E2E tests in `cmd/fo/main_test.go`**

Update `TestRun_ReportFormat`: remove the `format:text` section from the inline report string:

```go
func TestRun_ReportFormat(t *testing.T) {
	input := "--- tool:vet format:sarif ---\n" +
		`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"govet"}},"results":[]}]}` + "\n" +
		"--- tool:test format:sarif ---\n" +
		`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"test"}},"results":[]}]}` + "\n"
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "REPORT:") {
		t.Errorf("output should contain REPORT header, got:\n%s", stdout.String())
	}
}
```

Update `TestRun_ReportFullFormats`: change expected tool count from 7 to 4 (vet, lint, test, vuln):

```go
func TestRun_ReportFullFormats(t *testing.T) {
	input, err := os.ReadFile("testdata/full.report")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "llm"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Errorf("full report exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	// Coupled to renderer phrasing — update if Summary label format changes.
	if !strings.Contains(out, "4 tools") {
		t.Errorf("expected '4 tools' in output:\n%s", out)
	}
	if !strings.Contains(out, "all pass") {
		t.Errorf("expected 'all pass' in output:\n%s", out)
	}
	for _, tool := range []string{"vet:", "lint:", "test:", "vuln:"} {
		if !strings.Contains(out, tool) {
			t.Errorf("expected tool %q in output:\n%s", tool, out)
		}
	}
}
```

Delete these E2E tests entirely:
- `TestRun_ReportWithFoMetrics`
- `TestRun_ReportWithFoMetricsFailing`
- `TestRun_FoMetricsFailNoDetails`
- `TestRun_FoMetricsRejectsV2`

- [ ] **Step 5: Run all tests**

Run: `go test ./... 2>&1 | tail -20`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/fo/testdata/ cmd/fo/main_test.go
git commit -m "test: update testdata and E2E tests for sarif+testjson-only reports"
```

---

### Task 8: Delete Obsolete Packages

Remove `internal/fometrics`, `internal/jscpd`, and `internal/archlint` (parsers moved into wrapper packages).

**Files:**
- Delete: `internal/fometrics/fometrics.go`
- Delete: `internal/fometrics/fometrics_test.go`
- Delete: `internal/jscpd/jscpd.go`
- Delete: `internal/jscpd/jscpd_test.go`
- Delete: `internal/archlint/archlint.go`
- Delete: `internal/archlint/archlint_test.go`

- [ ] **Step 1: Verify no remaining imports of deleted packages**

Run: `rg 'internal/fometrics|internal/jscpd|internal/archlint' --type go`
Expected: no matches (all references removed in earlier tasks).

- [ ] **Step 2: Delete the packages**

```bash
rm -rf internal/fometrics internal/jscpd internal/archlint
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 4: Tidy module dependencies**

Run: `go mod tidy`
Check: `git diff go.mod go.sum` — if any dependencies were removed, that's expected.

- [ ] **Step 5: Commit**

```bash
git rm -r internal/fometrics internal/jscpd internal/archlint
git add go.mod go.sum
git commit -m "refactor: delete obsolete internal/fometrics, internal/jscpd, internal/archlint"
```

---

### Task 9: Update Usage Text and Docs

Update CLI help, CLAUDE.md, and usage examples.

**Files:**
- Modify: `cmd/fo/main.go` (usage string)
- Modify: `.claude/rules/CLAUDE.md`

- [ ] **Step 1: Update usage string in `cmd/fo/main.go`**

Replace the `usage` const with:

```go
const usage = `fo — focused build output renderer

USAGE
  <input-command> | fo [FLAGS]
  <tool-output>   | fo wrap <name> [FLAGS]

INPUT FORMATS (auto-detected from stdin)
  SARIF 2.1.0     Static analysis results (golangci-lint, gosec, etc.)
  go test -json   Test execution stream (supports live + batch)

OUTPUT FORMATS (--format)
  auto            TTY → terminal, piped → llm (default)
  terminal        Styled Unicode with color and sparklines
  llm             Terse plain text, no ANSI — optimized for AI consumption
  json            Structured JSON for automation

FLAGS
  --format <mode>   Output format: auto | terminal | llm | json (default: auto)
  --theme <name>    Color theme: default | orca | mono (default: default)

SUBCOMMANDS
  fo wrap <name>     Convert tool output to SARIF or go-test-json

  Available wrappers:
    diag             Convert line diagnostics (file:line:col: msg) to SARIF
      --tool <name>    Tool name for SARIF driver (required)
      --rule <id>      Default rule ID (default: finding)
      --level <level>  Severity: error | warning | note (default: warning)
      --version <str>  Tool version string
    archlint         Convert go-arch-lint JSON to SARIF
    jscpd            Convert jscpd JSON report to SARIF

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
  - TTY auto-detection: terminal style when stdout is a TTY, LLM mode when piped
  - Live streaming mode activates for go test -json when stdout is a TTY
  - NO_COLOR env var forces mono theme
  - SARIF input supports multiple runs (multiple tools in one document)
`
```

Also update the `wrapUsage` const (or remove it — the generic dispatcher prints wrapper names from the registry). Since `wrapUsage` is no longer referenced after Task 5, verify it was already removed. If not, remove it now.

- [ ] **Step 2: Update `.claude/rules/CLAUDE.md`**

Update the package structure section. Replace the existing package list with:

```markdown
## Package Structure

- `cmd/fo/` — CLI entry, flags, subcommands
- `pkg/wrapper/` — Wrapper plugin interface, registry
- `pkg/wrapper/wrapdiag/` — Line diagnostics → SARIF
- `pkg/wrapper/wrapjscpd/` — jscpd JSON → SARIF
- `pkg/wrapper/wraparchlint/` — go-arch-lint JSON → SARIF
- `pkg/pattern/` — Pure data structs: Summary, Leaderboard, TestTable, Sparkline, Comparison
- `pkg/sarif/` — SARIF types, reader, stats, builder
- `pkg/testjson/` — go test -json stream parser
- `pkg/mapper/` — SARIF → patterns, testjson → patterns
- `pkg/render/` — Renderer interface + terminal, llm, json implementations + themes
- `internal/detect/` — Format sniffing (SARIF vs go test -json)
- `internal/report/` — Report delimiter protocol (multiplexer for multi-tool pipelines)
```

Add to key design decisions:

```markdown
- Wrappers are compiled-in plugins behind a `Wrapper` interface (pkg/wrapper)
- Adding a new wrapper: implement interface, register in registry.go
```

- [ ] **Step 3: Verify build and tests**

Run: `go build ./cmd/fo/ && go test ./...`
Expected: clean build, all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/fo/main.go .claude/rules/CLAUDE.md
git commit -m "docs: update usage text and CLAUDE.md for wrapper system"
```

---

### Task 10: QA and Final Verification

**Files:** None (verification only)

- [ ] **Step 1: Run full QA**

Run: `make qa`
Expected: clean pass. (`make qa` = build + test + lint — it does not produce or consume report-format output, so the regex narrowing has no effect here.)

- [ ] **Step 2: Verify wrapper round-trip**

Build fo, then test each wrapper produces valid SARIF that fo can render:

```bash
go build -o /tmp/fo ./cmd/fo
echo 'main.go:15:3: unreachable code' | /tmp/fo wrap diag --tool govet | /tmp/fo --format llm
echo '{"duplicates":[],"statistics":{}}' | /tmp/fo wrap jscpd | /tmp/fo --format llm
echo '{"Type":"models.Check","Payload":{"ArchHasWarnings":false,"ArchWarningsDeps":[],"Qualities":[]}}' | /tmp/fo wrap archlint | /tmp/fo --format llm
rm /tmp/fo
```

Expected: each produces clean fo output. First should show the diagnostic. Second and third should produce clean SARIF (no findings = empty output or clean summary).

- [ ] **Step 3: Verify test coverage hasn't regressed on surviving code**

Run: `go test -cover ./pkg/wrapper/... ./pkg/mapper/ ./internal/report/`
Expected: wrapper packages have decent coverage from new tests. Mapper and report coverage should be comparable to before (minus deleted code).

- [ ] **Step 4: Commit any fixes**

If any issues found, fix and commit.
