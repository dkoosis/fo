# Wrapper DX Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make fo's wrapper plugin interface easier to implement correctly by separating concerns, adding validation, and providing contract tests as a safety net.

**Architecture:** Split the monolithic `Wrap(args, r, w)` method into `RegisterFlags(fs)` + `Convert(r, w)` so the framework owns orchestration (flag parsing, error handling, help text) and wrapper authors only implement the transformation. Add builder validation to catch invalid SARIF at construction time. Add contract tests that verify invariants across all registered wrappers.

**Tech Stack:** Go 1.24+, stdlib `flag`, stdlib `testing`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `pkg/wrapper/contract_test.go` | Cross-wrapper invariant tests |
| Modify | `pkg/sarif/builder.go` | Add validation (empty driver name, invalid level) |
| Modify | `pkg/sarif/builder_test.go` | Tests for validation errors |
| Modify | `pkg/wrapper/wrapper.go` | New interface: `RegisterFlags` + `Convert` |
| Modify | `pkg/wrapper/registry.go` | Add description field, `Description()` accessor |
| Modify | `pkg/wrapper/wrapdiag/diag.go` | Implement new interface |
| Modify | `pkg/wrapper/wrapdiag/diag_test.go` | Update for `RegisterFlags`/`Convert` API |
| Modify | `pkg/wrapper/wrapjscpd/jscpd.go` | Implement new interface |
| Modify | `pkg/wrapper/wrapjscpd/jscpd_test.go` | Update for `Convert` API |
| Modify | `pkg/wrapper/wraparchlint/archlint.go` | Implement new interface |
| Modify | `pkg/wrapper/wraparchlint/archlint_test.go` | Update for `Convert` API |
| Modify | `cmd/fo/main.go` | Framework orchestration in `runWrap`, dynamic help |
| Modify | `cmd/fo/main_test.go` | Existing `runWrap` tests still pass (black-box) |

---

### Task 1: Contract Tests for Existing Wrappers

> **Rollback boundary:** Independently revertible. Single new file, no modifications.

Safety net before any interface changes. Tests the current `Wrap` API.

**Files:**
- Create: `pkg/wrapper/contract_test.go`

- [ ] **Step 1: Write contract test file**

This test lives in `package wrapper_test` (external test package) so it can blank-import the sub-packages the same way `cmd/fo/main.go` does.

```go
package wrapper_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/wrapper"
	_ "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

func TestAllWrappers_Registered(t *testing.T) {
	names := wrapper.Names()
	expected := []string{"archlint", "diag", "jscpd"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d wrappers, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected wrapper[%d] = %q, got %q", i, name, names[i])
		}
	}
}

func TestAllWrappers_OutputFormat(t *testing.T) {
	valid := map[wrapper.Format]bool{
		wrapper.FormatSARIF:    true,
		wrapper.FormatTestJSON: true,
	}
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		if w == nil {
			t.Errorf("Get(%q) returned nil", name)
			continue
		}
		if !valid[w.OutputFormat()] {
			t.Errorf("wrapper %q: invalid OutputFormat %q", name, w.OutputFormat())
		}
	}
}

func TestAllWrappers_EmptyInputNoPanic(t *testing.T) {
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			// No wrapper-specific args — tests the interface contract, not business logic.
			// Wrappers with required flags (e.g. diag --tool) will return an error, which is fine.
			// The contract: empty input must not panic, regardless of flag state.
			_ = w.Wrap([]string{}, strings.NewReader(""), &buf)
		})
	}
}

func TestAllWrappers_GetNilForUnknown(t *testing.T) {
	if w := wrapper.Get("nonexistent"); w != nil {
		t.Error("expected nil for unknown wrapper")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test -v ./pkg/wrapper/`
Expected: all 4 tests PASS. This confirms the contract tests work against the current API.

- [ ] **Step 3: Commit**

```bash
git add pkg/wrapper/contract_test.go
git commit -m "test(wrapper): add contract tests for all registered wrappers"
```

---

### Task 2: Builder Validation

> **Rollback boundary:** Independently revertible. Only touches `pkg/sarif/builder.go` and its test file. No downstream API changes.

Add validation to `sarif.Builder` so empty driver names and invalid levels are caught at write time. Uses a deferred-error pattern to preserve the chaining API.

**Files:**
- Modify: `pkg/sarif/builder.go`
- Modify: `pkg/sarif/builder_test.go`

- [ ] **Step 1: Write failing tests for builder validation**

Add to `pkg/sarif/builder_test.go`:

```go
func TestBuilder_EmptyDriverNameError(t *testing.T) {
	b := NewBuilder("", "1.0")
	b.AddResult("r1", "warning", "msg", "f.go", 1, 0)

	var buf bytes.Buffer
	_, err := b.WriteTo(&buf)
	if err == nil {
		t.Error("expected error for empty driver name")
	}
}

func TestBuilder_InvalidLevelError(t *testing.T) {
	b := NewBuilder("tool", "1.0")
	b.AddResult("r1", "critical", "msg", "f.go", 1, 0)

	var buf bytes.Buffer
	_, err := b.WriteTo(&buf)
	if err == nil {
		t.Error("expected error for invalid SARIF level")
	}
}

func TestBuilder_ValidLevels(t *testing.T) {
	for _, level := range []string{"error", "warning", "note", "none"} {
		b := NewBuilder("tool", "1.0")
		b.AddResult("r1", level, "msg", "f.go", 1, 0)

		var buf bytes.Buffer
		if _, err := b.WriteTo(&buf); err != nil {
			t.Errorf("level %q should be valid, got error: %v", level, err)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v -run TestBuilder_EmptyDriverName\|TestBuilder_InvalidLevel\|TestBuilder_ValidLevels ./pkg/sarif/`
Expected: `EmptyDriverNameError` and `InvalidLevelError` FAIL (no validation yet). `ValidLevels` passes.

- [ ] **Step 3: Implement validation in builder.go**

Edit `pkg/sarif/builder.go`. Add an `err` field and a `validLevel` helper. Validate in `AddResult` and `WriteTo`:

```go
package sarif

import (
	"encoding/json"
	"fmt"
	"io"
)

// Builder constructs valid SARIF 2.1.0 documents.
// Designed for fo wrap and as an importable library.
type Builder struct {
	doc *Document
	err error
}

// NewBuilder creates a SARIF builder for the given tool.
func NewBuilder(toolName, toolVersion string) *Builder {
	return &Builder{
		doc: &Document{
			Version: "2.1.0",
			Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
			Runs: []Run{{
				Tool: Tool{
					Driver: Driver{
						Name:    toolName,
						Version: toolVersion,
					},
				},
			}},
		},
	}
}

// validLevel reports whether level is a valid SARIF 2.1.0 level.
func validLevel(level string) bool {
	switch level {
	case "error", "warning", "note", "none":
		return true
	}
	return false
}

// AddResult adds a diagnostic result to the current run.
func (b *Builder) AddResult(ruleID, level, message, file string, line, col int) *Builder {
	if b.err != nil {
		return b
	}
	if !validLevel(level) {
		b.err = fmt.Errorf("sarif: invalid level %q (want error|warning|note|none)", level)
		return b
	}
	r := Result{
		RuleID:  ruleID,
		Level:   level,
		Message: Message{Text: message},
	}
	if file != "" {
		r.Locations = []Location{{
			PhysicalLocation: PhysicalLocation{
				ArtifactLocation: ArtifactLocation{URI: file},
				Region: Region{
					StartLine:   line,
					StartColumn: col,
				},
			},
		}}
	}
	b.doc.Runs[0].Results = append(b.doc.Runs[0].Results, r)
	return b
}

// Document returns the constructed SARIF document without validation.
// Use WriteTo for production output — it validates driver name and levels.
// This method is the "I know what I'm doing" escape hatch for tests and inspection.
func (b *Builder) Document() *Document {
	return b.doc
}

// WriteTo writes the SARIF document as JSON to w.
func (b *Builder) WriteTo(w io.Writer) (int64, error) {
	if b.doc.Runs[0].Tool.Driver.Name == "" {
		return 0, fmt.Errorf("sarif: driver name is required")
	}
	if b.err != nil {
		return 0, b.err
	}
	data, err := json.MarshalIndent(b.doc, "", "  ")
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	n, err := w.Write(data)
	return int64(n), err
}
```

- [ ] **Step 4: Run all sarif tests to verify they pass**

Run: `go test -v ./pkg/sarif/`
Expected: ALL pass, including the new validation tests and existing tests.

- [ ] **Step 5: Run full wrapper tests to verify no regressions**

Run: `go test ./pkg/wrapper/...`
Expected: ALL pass. No existing wrapper passes empty driver names or invalid levels.

- [ ] **Step 6: Commit**

```bash
git add pkg/sarif/builder.go pkg/sarif/builder_test.go
git commit -m "feat(sarif): validate driver name and result level in Builder"
```

---

### Task 3: Interface Split — RegisterFlags + Convert

> **Rollback boundary:** This task is all-or-nothing. Interface, all three wrappers, framework orchestration, and all tests must land in a single commit. A partial application breaks the build. If any step fails, revert the entire task and debug before retrying.

Replace `Wrap(args, r, w)` with `RegisterFlags(fs)` + `Convert(r, w)`. The framework creates the `flag.FlagSet`, calls `RegisterFlags`, parses flags, then calls `Convert`. Wrapper authors only implement the transformation.

> **Note on `errors` import:** The updated `runWrap` uses `errors.Is(err, flag.ErrHelp)`. The current `cmd/fo/main.go` already imports `"errors"` (line 23). Preserve this import — do not remove during cleanup.

**Files:**
- Modify: `pkg/wrapper/wrapper.go`
- Modify: `pkg/wrapper/wrapdiag/diag.go`
- Modify: `pkg/wrapper/wrapjscpd/jscpd.go`
- Modify: `pkg/wrapper/wraparchlint/archlint.go`
- Modify: `cmd/fo/main.go` (runWrap)
- Modify: `pkg/wrapper/contract_test.go`
- Modify: `pkg/wrapper/wrapdiag/diag_test.go`
- Modify: `pkg/wrapper/wrapjscpd/jscpd_test.go`
- Modify: `pkg/wrapper/wraparchlint/archlint_test.go`

- [ ] **Step 1: Update the Wrapper interface**

Replace the contents of `pkg/wrapper/wrapper.go`:

```go
// Package wrapper defines the plugin interface for converting tool output
// into fo-native formats (SARIF or go-test-json).
package wrapper

import (
	"flag"
	"io"
)

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

	// RegisterFlags adds wrapper-specific flags to the provided FlagSet.
	// Called by the framework before flag parsing. Implementations store
	// flag value pointers for use in Convert. No-op if no flags needed.
	RegisterFlags(fs *flag.FlagSet)

	// Convert reads tool output from r and writes fo-native output to w.
	// Must be called after RegisterFlags + FlagSet.Parse — implementations
	// read parsed flag values via stored pointers. Calling Convert without
	// prior registration may panic on nil pointer dereference.
	// Return an error for invalid flag values (e.g. missing required flags)
	// or conversion failures.
	Convert(r io.Reader, w io.Writer) error
}
```

- [ ] **Step 2: Verify the build fails**

Run: `go build ./...`
Expected: compile errors in all three wrappers and `cmd/fo/main.go` — they still implement the old `Wrap` method.

- [ ] **Step 3: Update wrapdiag**

Replace `pkg/wrapper/wrapdiag/diag.go`:

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
type Diag struct {
	toolName *string
	ruleID   *string
	level    *string
	version  *string
}

// New returns a new Diag wrapper.
func New() *Diag { return &Diag{} }

func init() {
	wrapper.Register("diag", New())
}

// OutputFormat returns FormatSARIF.
func (d *Diag) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// RegisterFlags adds diag-specific flags.
func (d *Diag) RegisterFlags(fs *flag.FlagSet) {
	d.toolName = fs.String("tool", "", "Tool name for SARIF driver.name (required)")
	d.ruleID = fs.String("rule", "finding", "Default rule ID")
	d.level = fs.String("level", "warning", "Default severity: error|warning|note")
	d.version = fs.String("version", "", "Tool version string")
}

// Convert reads line diagnostics from r and writes SARIF to w.
func (d *Diag) Convert(r io.Reader, w io.Writer) error {
	if d.toolName == nil || *d.toolName == "" {
		return fmt.Errorf("--tool is required")
	}

	b := sarif.NewBuilder(*d.toolName, *d.version)
	scanner := bufio.NewScanner(r)
	// Same 1 MiB limit as testjson.ParseStream — see BUG note there.
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
		b.AddResult(*d.ruleID, *d.level, msg, file, ln, col)
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
	if strings.HasSuffix(trimmed, ".go") && !strings.Contains(trimmed, " ") {
		return trimmed, 0, 0, "needs formatting"
	}

	return "", 0, 0, ""
}
```

- [ ] **Step 4: Update wrapjscpd**

Replace `pkg/wrapper/wrapjscpd/jscpd.go`:

```go
// Package wrapjscpd converts jscpd JSON duplication reports into SARIF 2.1.0.
package wrapjscpd

import (
	"encoding/json"
	"flag"
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

func init() {
	wrapper.Register("jscpd", New())
}

// OutputFormat returns FormatSARIF.
func (j *Jscpd) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// RegisterFlags is a no-op — jscpd takes no flags.
func (j *Jscpd) RegisterFlags(_ *flag.FlagSet) {}

// Convert reads jscpd JSON from r and writes SARIF to w.
func (j *Jscpd) Convert(r io.Reader, w io.Writer) error {
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
			Format    string `json:"format"`
			Lines     int    `json:"lines"`
			FirstFile struct {
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

- [ ] **Step 5: Update wraparchlint**

Replace `pkg/wrapper/wraparchlint/archlint.go`:

```go
// Package wraparchlint converts go-arch-lint JSON output into SARIF 2.1.0.
package wraparchlint

import (
	"encoding/json"
	"flag"
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

// Archlint converts go-arch-lint JSON to SARIF.
type Archlint struct{}

// New returns a new Archlint wrapper.
func New() *Archlint { return &Archlint{} }

func init() {
	wrapper.Register("archlint", New())
}

// OutputFormat returns FormatSARIF.
func (a *Archlint) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// RegisterFlags is a no-op — archlint takes no flags.
func (a *Archlint) RegisterFlags(_ *flag.FlagSet) {}

// Convert reads go-arch-lint JSON from r and writes SARIF to w.
func (a *Archlint) Convert(r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	violations, err := parseResult(data)
	if err != nil {
		return err
	}

	b := sarif.NewBuilder("go-arch-lint", "")
	for _, v := range violations {
		msg := fmt.Sprintf("%s → %s", v.From, v.To)
		b.AddResult("dependency-violation", "error", msg, v.FileFrom, 0, 0)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseResult decodes go-arch-lint --json output.
func parseResult(data []byte) ([]violation, error) {
	var raw struct {
		Payload struct {
			ArchWarningsDeps []struct {
				ComponentName      string `json:"ComponentName"`
				FileRelativePath   string `json:"FileRelativePath"`
				ResolvedImportName string `json:"ResolvedImportName"`
			} `json:"ArchWarningsDeps"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("archlint: %w", err)
	}

	vs := make([]violation, len(raw.Payload.ArchWarningsDeps))
	for i, d := range raw.Payload.ArchWarningsDeps {
		vs[i] = violation{
			From:     d.ComponentName,
			To:       d.ResolvedImportName,
			FileFrom: d.FileRelativePath,
		}
	}
	return vs, nil
}
```

- [ ] **Step 6: Update framework orchestration in cmd/fo/main.go**

Replace the `runWrap` function in `cmd/fo/main.go`:

```go
func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, "fo wrap: wrapper name required\n\nAvailable wrappers: %s\n",
			strings.Join(wrapper.Names(), ", "))
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintf(stderr, "fo wrap: convert tool output to SARIF or go-test-json\n\nAvailable wrappers: %s\n",
			strings.Join(wrapper.Names(), ", "))
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
```

Also remove the `"flag"` import if it's not used elsewhere in main.go. (It's already imported — `flag` is used by the main `fs` too, so no change needed.)

- [ ] **Step 7: Verify build compiles**

Run: `go build ./...`
Expected: clean compile.

- [ ] **Step 8: Update wrapper unit tests for new API**

> **Important:** Apply these as targeted edits, not full file replacements. The test files may contain additional tests not shown here — preserve them. The key changes: (1) add `diagConvert` helper, (2) replace `Wrap(args, r, w)` calls with `RegisterFlags+Parse+Convert`, (3) replace `Wrap(nil, r, w)` with `Convert(r, w)` in jscpd/archlint.

In `pkg/wrapper/wrapdiag/diag_test.go`, replace calls to `Wrap(args, r, w)` with `RegisterFlags(fs)` + `fs.Parse(args)` + `Convert(r, w)`. Add a helper to reduce repetition:

```go
package wrapdiag

import (
	"bytes"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

// diagConvert creates a Diag, registers+parses flags, and calls Convert.
func diagConvert(t *testing.T, args []string, input string) (bytes.Buffer, error) {
	t.Helper()
	d := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	d.RegisterFlags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("flag parse: %v", err)
	}
	var buf bytes.Buffer
	err := d.Convert(strings.NewReader(input), &buf)
	return buf, err
}

func TestDiag_OutputFormat(t *testing.T) {
	d := New()
	if d.OutputFormat() != wrapper.FormatSARIF {
		t.Errorf("expected FormatSARIF, got %q", d.OutputFormat())
	}
}

func TestDiag_FileLineColMessage(t *testing.T) {
	input := "main.go:15:3: unreachable code after return\npkg/util.go:42: unused variable x\n"
	buf, err := diagConvert(t, []string{"--tool", "govet"}, input)
	if err != nil {
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
	buf, err := diagConvert(t, []string{"--tool", "gofmt", "--rule", "needs-formatting", "--level", "warning"}, input)
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
	d := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	d.RegisterFlags(fs)
	_ = fs.Parse([]string{})
	var buf bytes.Buffer
	err := d.Convert(strings.NewReader("x.go:1: msg\n"), &buf)
	if err == nil {
		t.Error("expected error for missing --tool flag")
	}
}

func TestDiag_WindowsDriveLetter(t *testing.T) {
	input := "C:\\Users\\dev\\main.go:15:3: unreachable code\n"
	buf, err := diagConvert(t, []string{"--tool", "govet"}, input)
	if err != nil {
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
	if uri != "C:\\Users\\dev\\main.go" {
		t.Errorf("expected Windows path, got %q", uri)
	}
}

func TestParseDiagLine(t *testing.T) {
	tests := []struct {
		input             string
		wantFile          string
		wantLine, wantCol int
		wantMsg           string
	}{
		{"main.go:15:3: unreachable code", "main.go", 15, 3, "unreachable code"},
		{"pkg/util.go:42: unused variable x", "pkg/util.go", 42, 0, "unused variable x"},
		{"pkg/handler.go", "pkg/handler.go", 0, 0, "needs formatting"},
		{"C:\\Users\\dev\\main.go:15:3: unreachable code", "C:\\Users\\dev\\main.go", 15, 3, "unreachable code"},
		{"D:\\proj\\util.go:42: unused", "D:\\proj\\util.go", 42, 0, "unused"},
		{"not a diagnostic", "", 0, 0, ""},
		{"src/main.rs", "", 0, 0, ""},
		{"some/path/to/file.txt", "", 0, 0, ""},
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

In `pkg/wrapper/wrapjscpd/jscpd_test.go`, replace `Wrap(nil, ...)` with `Convert(...)`:

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
	if err := New().Convert(strings.NewReader(input), &buf); err != nil {
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
	if err := New().Convert(strings.NewReader(input), &buf); err != nil {
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
	err := New().Convert(strings.NewReader("not json"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestJscpd_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := New().Convert(strings.NewReader(""), &buf)
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

In `pkg/wrapper/wraparchlint/archlint_test.go`, replace `Wrap(nil, ...)` with `Convert(...)`:

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
	if err := New().Convert(strings.NewReader(input), &buf); err != nil {
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
	if err := New().Convert(strings.NewReader(input), &buf); err != nil {
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
	err := New().Convert(strings.NewReader("bad"), &buf)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestArchlint_EmptyInput(t *testing.T) {
	var buf bytes.Buffer
	err := New().Convert(strings.NewReader(""), &buf)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestArchlint_FullImportPath(t *testing.T) {
	input := `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[
		{"ComponentName":"agentSupervisor","FileRelativePath":"/internal/agent/supervisor/supervisor.go","ResolvedImportName":"github.com/example/project/internal/agent/shell"}
	],"Qualities":[{"ID":"component_imports","Used":true}]}}`
	var buf bytes.Buffer
	if err := New().Convert(strings.NewReader(input), &buf); err != nil {
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
	vs, err := parseResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Errorf("expected 2 violations, got %d", len(vs))
	}
}
```

- [ ] **Step 9: Update contract tests for new API**

Replace `pkg/wrapper/contract_test.go`:

```go
package wrapper_test

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
	_ "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

func TestAllWrappers_Registered(t *testing.T) {
	names := wrapper.Names()
	expected := []string{"archlint", "diag", "jscpd"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d wrappers, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected wrapper[%d] = %q, got %q", i, name, names[i])
		}
	}
}

func TestAllWrappers_OutputFormat(t *testing.T) {
	valid := map[wrapper.Format]bool{
		wrapper.FormatSARIF:    true,
		wrapper.FormatTestJSON: true,
	}
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		if w == nil {
			t.Errorf("Get(%q) returned nil", name)
			continue
		}
		if !valid[w.OutputFormat()] {
			t.Errorf("wrapper %q: invalid OutputFormat %q", name, w.OutputFormat())
		}
	}
}

func TestAllWrappers_RegisterFlagsNoPanic(t *testing.T) {
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		t.Run(name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			w.RegisterFlags(fs) // must not panic
		})
	}
}

func TestAllWrappers_EmptyInputNoPanic(t *testing.T) {
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		t.Run(name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			w.RegisterFlags(fs)
			_ = fs.Parse([]string{}) // no wrapper-specific args
			var buf bytes.Buffer
			// No special cases per wrapper. Error is acceptable (e.g. diag missing --tool).
			// The contract: Convert must not panic, regardless of flag state.
			_ = w.Convert(strings.NewReader(""), &buf)
		})
	}
}

// TestAllWrappers_HappyPathProducesValidSARIF feeds each wrapper its happy-path
// input from testdata/ and verifies the output parses as valid SARIF.
// This catches the seam where wrapper output drifts from what fo's parser expects.
func TestAllWrappers_HappyPathProducesValidSARIF(t *testing.T) {
	fixtures := map[string]struct {
		args  []string
		input string
	}{
		"diag": {
			args:  []string{"--tool", "govet"},
			input: "main.go:15:3: unreachable code\npkg/util.go:42: unused variable\n",
		},
		"jscpd": {
			args: nil,
			input: `{"duplicates":[{"format":"go","lines":10,` +
				`"firstFile":{"name":"a.go","startLoc":{"line":1},"endLoc":{"line":10}},` +
				`"secondFile":{"name":"b.go","startLoc":{"line":5},"endLoc":{"line":14}}}],` +
				`"statistics":{}}`,
		},
		"archlint": {
			args: nil,
			input: `{"Type":"models.Check","Payload":{"ArchHasWarnings":true,"ArchWarningsDeps":[` +
				`{"ComponentName":"a","FileRelativePath":"a.go","ResolvedImportName":"b"}` +
				`],"Qualities":[]}}`,
		},
	}

	for _, name := range wrapper.Names() {
		fix, ok := fixtures[name]
		if !ok {
			t.Errorf("no happy-path fixture for wrapper %q — add one", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			w := wrapper.Get(name)
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			w.RegisterFlags(fs)
			if fix.args != nil {
				if err := fs.Parse(fix.args); err != nil {
					t.Fatalf("flag parse: %v", err)
				}
			}
			var buf bytes.Buffer
			if err := w.Convert(strings.NewReader(fix.input), &buf); err != nil {
				t.Fatalf("Convert: %v", err)
			}
			// Verify output is valid SARIF by parsing it with fo's own reader.
			if _, err := sarif.ReadBytes(buf.Bytes()); err != nil {
				t.Errorf("output is not valid SARIF: %v\nraw output:\n%s", err, buf.String())
			}
		})
	}
}

func TestAllWrappers_GetNilForUnknown(t *testing.T) {
	if w := wrapper.Get("nonexistent"); w != nil {
		t.Error("expected nil for unknown wrapper")
	}
}
```

- [ ] **Step 10: Run all tests**

Run: `go test ./...`
Expected: ALL pass. No regressions.

- [ ] **Step 11: Commit**

```bash
git add pkg/wrapper/wrapper.go pkg/wrapper/wrapdiag/diag.go pkg/wrapper/wrapjscpd/jscpd.go pkg/wrapper/wraparchlint/archlint.go cmd/fo/main.go pkg/wrapper/contract_test.go pkg/wrapper/wrapdiag/diag_test.go pkg/wrapper/wrapjscpd/jscpd_test.go pkg/wrapper/wraparchlint/archlint_test.go
git commit -m "refactor(wrapper): split Wrap into RegisterFlags+Convert, framework owns orchestration"
```

---

### Task 4: Registry Descriptions and Dynamic Help

> **Atomic commit required:** This task changes the `Register()` signature from 2 args to 3 args. The registry change and all three wrapper init() updates must land in the same commit — a partial application breaks the build. Tasks 1, 2, and 3 are independently revertible; this task is not splittable.

Add a description string to `Register()` so `fo wrap --help` generates help text dynamically from the registry + each wrapper's FlagSet.

**Files:**
- Modify: `pkg/wrapper/registry.go`
- Modify: `pkg/wrapper/wrapdiag/diag.go` (Register call)
- Modify: `pkg/wrapper/wrapjscpd/jscpd.go` (Register call)
- Modify: `pkg/wrapper/wraparchlint/archlint.go` (Register call)
- Modify: `cmd/fo/main.go` (runWrap help, usage string)
- Modify: `cmd/fo/main_test.go` (optional: test dynamic help)

- [ ] **Step 1: Write test for Description accessor**

Add to `pkg/wrapper/contract_test.go`:

```go
func TestAllWrappers_HaveDescriptions(t *testing.T) {
	for _, name := range wrapper.Names() {
		desc := wrapper.Description(name)
		if desc == "" {
			t.Errorf("wrapper %q has no description", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestAllWrappers_HaveDescriptions ./pkg/wrapper/`
Expected: compile error — `wrapper.Description` doesn't exist yet.

- [ ] **Step 3: Update registry to store descriptions**

Replace `pkg/wrapper/registry.go`:

```go
package wrapper

import (
	"fmt"
	"sort"
)

type entry struct {
	wrapper     Wrapper
	description string
}

var registry = map[string]entry{}

// Register adds a wrapper to the global registry under the given name.
// Intended for use in sub-package init() functions.
func Register(name, description string, w Wrapper) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("wrapper: duplicate registration %q", name))
	}
	registry[name] = entry{wrapper: w, description: description}
}

// Get returns the named wrapper, or nil if not found.
func Get(name string) Wrapper {
	e, ok := registry[name]
	if !ok {
		return nil
	}
	return e.wrapper
}

// Description returns the description for the named wrapper, or "" if not found.
func Description(name string) string {
	return registry[name].description
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

// TODO: add ResetForTesting() to clear the global registry for test isolation.
// Not urgent with init()-based registration, but needed if tests ever register
// wrappers dynamically. See wrapper DX discussion #10.
```

- [ ] **Step 4: Update Register calls in all wrappers**

In `pkg/wrapper/wrapdiag/diag.go`, change the init:

```go
func init() {
	wrapper.Register("diag", "Convert line diagnostics (file:line:col: msg) to SARIF", New())
}
```

In `pkg/wrapper/wrapjscpd/jscpd.go`:

```go
func init() {
	wrapper.Register("jscpd", "Convert jscpd JSON duplication report to SARIF", New())
}
```

In `pkg/wrapper/wraparchlint/archlint.go`:

```go
func init() {
	wrapper.Register("archlint", "Convert go-arch-lint JSON to SARIF", New())
}
```

- [ ] **Step 5: Verify build compiles and description test passes**

Run: `go test -v -run TestAllWrappers_HaveDescriptions ./pkg/wrapper/`
Expected: PASS.

- [ ] **Step 6: Update runWrap to generate dynamic help**

In `cmd/fo/main.go`, replace the help branch in `runWrap`:

```go
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
```

No helper needed — `VisitAll` is a no-op if the FlagSet has no flags.

- [ ] **Step 7: Trim the hardcoded wrapper docs from the usage string**

In the `usage` constant in `cmd/fo/main.go`, replace the wrapper-specific section:

```
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
```

With:

```
SUBCOMMANDS
  fo wrap <name>     Convert tool output to SARIF or go-test-json
  fo wrap --help     Show available wrappers and their flags
```

- [ ] **Step 8: Run all tests**

Run: `go test ./...`
Expected: ALL pass.

- [ ] **Step 9: Run make qa**

Run: `make qa`
Expected: clean — build, test, vet, lint all pass.

- [ ] **Step 10: Commit**

```bash
git add pkg/wrapper/registry.go pkg/wrapper/wrapdiag/diag.go pkg/wrapper/wrapjscpd/jscpd.go pkg/wrapper/wraparchlint/archlint.go pkg/wrapper/contract_test.go cmd/fo/main.go
git commit -m "feat(wrapper): add descriptions to registry, generate help dynamically"
```
