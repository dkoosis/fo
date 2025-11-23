# Stream Adapter Examples

Stream adapters automatically detect and parse structured command output into rich visualization patterns.

## Overview

Many modern tools emit structured output (JSON, XML, etc.) that contains rich semantic information. Stream adapters detect these formats and parse them into appropriate patterns for better visualization.

## Supported Formats

### Go Test JSON

Go's test runner can output JSON with `-json` flag:

```bash
go test -json ./...
```

The `GoTestJSONAdapter` automatically detects this format and parses it into a `TestTable` pattern, extracting:
- Package names
- Test counts
- Pass/fail/skip status
- Durations
- Failure details

## Usage

### Programmatic Usage

```go
import (
    "os/exec"
    "github.com/dkoosis/fo/pkg/adapter"
    "github.com/dkoosis/fo/pkg/design"
)

// Create adapter registry
registry := adapter.NewRegistry()

// Execute command
cmd := exec.Command("go", "test", "-json", "./...")
output, _ := cmd.Output()

// Detect format from first few lines
firstLines := strings.Split(string(output[:200]), "\n")
streamAdapter := registry.Detect(firstLines)

if streamAdapter != nil {
    // Parse into pattern
    pattern, _ := streamAdapter.Parse(bytes.NewReader(output))

    // Render with theme
    cfg := design.UnicodeVibrantTheme()
    fmt.Println(pattern.Render(cfg))
}
```

### With fo CLI (Future)

```bash
# fo will auto-detect JSON output and use appropriate adapter
fo -- go test -json ./...

# Output will be rendered as a TestTable instead of raw JSON
```

## Adding New Adapters

Implement the `StreamAdapter` interface:

```go
type MyToolAdapter struct{}

func (a *MyToolAdapter) Name() string {
    return "my-tool"
}

func (a *MyToolAdapter) Detect(firstLines []string) bool {
    // Check if output matches your tool's format
    for _, line := range firstLines {
        if strings.Contains(line, "my-tool-signature") {
            return true
        }
    }
    return false
}

func (a *MyToolAdapter) Parse(output io.Reader) (design.Pattern, error) {
    // Parse output into appropriate pattern
    // Return TestTable, Leaderboard, Summary, etc.
}
```

Then register it:

```go
registry.Register(&MyToolAdapter{})
```

## Future Adapters

Planned adapters include:

- **Jest/Vitest JSON**: JavaScript test runners → TestTable
- **golangci-lint JSON**: Linting results → Leaderboard (by warning count)
- **Go build**: Binary output → Inventory
- **npm/yarn**: Package installation → Inventory
- **webpack stats**: Build statistics → Comparison + Inventory

## Design Principles

1. **Auto-detection**: Adapters should reliably detect their format from first few lines
2. **Graceful fallback**: If detection fails, fall back to standard output
3. **Semantic preservation**: Extract all meaningful information (don't just parse structure)
4. **Pattern selection**: Choose the most appropriate pattern for the data type
5. **Context preservation**: Include metadata (package names, file paths, etc.)

## Benefits

- **Richer visualization**: Tables, leaderboards, and summaries instead of raw text
- **Automatic**: No special flags needed (beyond tool's native JSON output)
- **Composable**: Parsed patterns can be combined with other patterns
- **Type-safe**: Structured data is parsed into well-defined types
