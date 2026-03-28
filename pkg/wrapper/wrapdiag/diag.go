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

// RegisterFlags adds diag-specific flags to the provided FlagSet.
func (d *Diag) RegisterFlags(fs *flag.FlagSet) {
	d.toolName = fs.String("tool", "", "Tool name for SARIF driver.name (required)")
	d.ruleID = fs.String("rule", "finding", "Default rule ID")
	d.level = fs.String("level", "warning", "Default severity: error|warning|note")
	d.version = fs.String("version", "", "Tool version string")
}

// Convert reads line diagnostics from r and writes SARIF to w.
// Must be called after RegisterFlags + FlagSet.Parse.
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
