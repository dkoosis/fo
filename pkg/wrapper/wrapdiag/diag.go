// Package wrapdiag converts line-based Go diagnostics into SARIF 2.1.0.
//
// Input formats:
//   - file.go:line:col: message
//   - file.go:line: message
//   - file.go (file-only, e.g. gofmt -l)
package wrapdiag

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dkoosis/fo/pkg/sarif"
)

var (
	errDiagNotInitialized = errors.New("diag: options not initialized")
	errToolRequired       = errors.New("--tool is required")
	errInvalidLevel       = errors.New("--level: must be error, warning, note, or none")
)

// diag converts line-based diagnostics to SARIF.
type diag struct {
	toolName *string
	ruleID   *string
	level    *string
	version  *string
}

// Convert reads line diagnostics from r and writes SARIF to w.
func (d *diag) Convert(r io.Reader, w io.Writer) error {
	if d.toolName == nil {
		return errDiagNotInitialized
	}
	if *d.toolName == "" {
		return errToolRequired
	}
	switch *d.level {
	case "error", "warning", "note", "none":
	default:
		return fmt.Errorf("%w: %q", errInvalidLevel, *d.level)
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
		fixCmd := fixCommandFor(*d.toolName, *d.ruleID, file)
		b.AddResultWithFix(*d.ruleID, *d.level, msg, file, ln, col, fixCmd)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	_, err := b.WriteTo(w)
	return err
}

// fixCommandFor returns a best-effort shell command the user can run to
// fix the finding, or "" if the source tool has no known autofix idiom.
// Dispatch is by the --tool name passed to the wrapper (e.g. "golangci-lint",
// "gofmt"); unknown tools return "" so renderers omit the hint.
func fixCommandFor(tool, ruleID, file string) string {
	switch tool {
	case "golangci-lint":
		// Always emit: we're not gating on whether the rule is autofixable.
		// If the rule isn't supported, golangci-lint will print a no-op.
		if ruleID == "" || ruleID == "finding" {
			return "golangci-lint run --fix " + file
		}
		return fmt.Sprintf("golangci-lint run --fix --enable-only=%s %s", ruleID, file)
	case "gofmt":
		return "gofmt -w " + file
	case "goimports":
		return "goimports -w " + file
	default:
		// govulncheck needs a fixed-version we don't have here; generic tools
		// get no hint. Renderer treats "" as "omit".
		return ""
	}
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
