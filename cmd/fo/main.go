// fo renders build tool output as information-dense terminal visualizations.
//
// Usage:
//
//	golangci-lint run --out-format sarif ./... | fo
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
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/dkoosis/fo/internal/detect"
	"github.com/dkoosis/fo/pkg/mapper"
	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/testjson"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// Check for subcommands before flag parsing
	if len(args) > 0 && args[0] == "wrap" {
		return runWrap(args[1:], stdin, stdout, stderr)
	}

	fs := flag.NewFlagSet("fo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	formatFlag := fs.String("format", "auto", "Output format: auto, terminal, llm, json")
	themeFlag := fs.String("theme", "default", "Theme: default, orca, mono")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	// Read all stdin
	input, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "fo: reading stdin: %v\n", err)
		return 2
	}
	if len(input) == 0 {
		fmt.Fprintf(stderr, "fo: no input on stdin\n")
		return 2
	}

	// Detect format
	format := detect.Sniff(input)

	// Parse and map to patterns
	var patterns []pattern.Pattern
	switch format {
	case detect.SARIF:
		doc, parseErr := sarif.ReadBytes(input)
		if parseErr != nil {
			fmt.Fprintf(stderr, "fo: parsing SARIF: %v\n", parseErr)
			return 2
		}
		patterns = mapper.FromSARIF(doc)

	case detect.GoTestJSON:
		results, parseErr := testjson.ParseBytes(input)
		if parseErr != nil {
			fmt.Fprintf(stderr, "fo: parsing go test -json: %v\n", parseErr)
			return 2
		}
		patterns = mapper.FromTestJSON(results)

	default:
		fmt.Fprintf(stderr, "fo: unrecognized input format (expected SARIF or go test -json)\n")
		return 2
	}

	// Select renderer
	renderer := selectRenderer(*formatFlag, *themeFlag, stdout)

	// Render and output
	output := renderer.Render(patterns)
	fmt.Fprint(stdout, output)

	return exitCode(patterns)
}

func selectRenderer(format, themeName string, w io.Writer) render.Renderer {
	mode := resolveFormat(format, w)
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
			if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
				width = w
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
		if term.IsTerminal(int(f.Fd())) {
			return "terminal"
		}
	}
	return "llm"
}

// exitCode returns 0 for clean, 1 for failures present.
func exitCode(patterns []pattern.Pattern) int {
	for _, p := range patterns {
		switch v := p.(type) {
		case *pattern.TestTable:
			for _, r := range v.Results {
				if r.Status == "fail" {
					return 1
				}
			}
		case *pattern.Summary:
			for _, m := range v.Metrics {
				if m.Kind == "error" {
					return 1
				}
			}
		}
	}
	return 0
}

// --- fo wrap sarif subcommand ---

func runWrap(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "sarif" {
		fmt.Fprintf(stderr, "fo wrap: unknown subcommand (expected 'sarif')\n")
		fmt.Fprintf(stderr, "Usage: fo wrap sarif --tool <name> [--rule <id>] [--level <level>]\n")
		return 2
	}

	fs := flag.NewFlagSet("fo wrap sarif", flag.ContinueOnError)
	fs.SetOutput(stderr)
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
func parseDiagLine(line string) (file string, ln, col int, msg string) {
	// Try file:line:col: message
	parts := strings.SplitN(line, ":", 4)
	if len(parts) >= 4 {
		var l, c int
		if _, err := fmt.Sscanf(parts[1], "%d", &l); err == nil {
			if _, err := fmt.Sscanf(parts[2], "%d", &c); err == nil {
				return parts[0], l, c, strings.TrimSpace(parts[3])
			}
		}
	}

	// Try file:line: message
	if len(parts) >= 3 {
		var l int
		if _, err := fmt.Sscanf(parts[1], "%d", &l); err == nil {
			return parts[0], l, 0, strings.TrimSpace(strings.Join(parts[2:], ":"))
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
