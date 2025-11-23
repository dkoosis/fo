# fo

A research-backed, pattern-based design system for CLI output visualization. Built on Tufte principles and cognitive load theory to create thoughtful, information-dense build dashboards.

## Overview

`fo` (Format Output) is a presentation layer for command-line build tools, transforming raw output into structured visual patterns that guide attention and reduce cognitive load. Instead of treating output as plain text, `fo` recognizes semantic meaning—test results, errors, warnings, metrics—and renders them using information-dense patterns like sparklines, leaderboards, and comparison tables.

### Design Philosophy

**Tufte-Informed Visualization**
- Maximize data-ink ratio: every character conveys information
- Sparklines for trends, small multiples for comparisons
- Cognitive load-aware rendering adapts to complexity

**Pattern-Based Architecture**
- Semantic patterns (Sparkline, Leaderboard, TestTable, Summary, Comparison, Inventory)
- Theme-independent content: separate what to show from how to show it
- Composable: build dashboards by combining patterns

**Research-Backed Design**
- Cognitive load theory guides information hierarchy
- Error recognition patterns reduce time to understanding
- Density modes optimize for different contexts (detailed, balanced, compact)

## Visual Patterns

### Sparkline
Word-sized trend graphics using Unicode blocks:
```
Build time trend: ▄▃▄▂▂▃▁▁ 1.6s
Test coverage: ▁▂▄▅▆█ 88.0%
```

### Leaderboard
Ranked metrics highlighting optimization targets:
```
Slowest Tests (top 3 of 247)
  1. TestLargeDataProcessing    5.2s
  2. TestComplexQueryExecution  3.8s
  3. TestNetworkIntegration     2.9s
```

### TestTable
Comprehensive test results with density modes:
```
✅ pkg/api       42 tests  2.1s
✅ pkg/database  28 tests  1.8s
❌ pkg/auth      15 tests  0.5s
```

### Comparison
Before/after metrics with directional indicators:
```
Build time:    5.2s → 4.1s  ↓ 1.1s
Binary size:   42MB → 38MB  ↓ 4.0MB
Test coverage: 85% → 88%    ↑ 3.0%
```

See [examples/patterns](examples/patterns/) for complete demonstrations.

## Installation

```bash
go install github.com/dkoosis/fo@latest
```

## Quick Start

### Basic Command Wrapping

```bash
# Wrap any command for formatted output
fo -- go build ./cmd/myapp

# Custom label
fo -l "Building application" -- go build ./cmd/myapp

# Stream mode for interactive commands
fo -s -- go test -v ./...
```

### In Build Scripts

`fo` excels in Makefiles and CI pipelines, creating thoughtful dashboards from build output:

```makefile
.PHONY: build test lint

build:
	@fo -l "Building binary" -- go build -o myapp ./cmd/myapp

test:
	@fo -l "Running tests" -- go test -json ./...

lint:
	@fo -l "Running linter" -- golangci-lint run ./...
```

### Programmatic Usage

Build custom dashboards by composing patterns:

```go
import "github.com/dkoosis/fo/pkg/design"

cfg := design.UnicodeVibrantTheme()

// Sparkline for trends
sparkline := &design.Sparkline{
    Label:  "Build time trend",
    Values: []float64{2.3, 2.1, 2.4, 1.9, 1.8},
    Unit:   "s",
}
fmt.Println(sparkline.Render(cfg))

// Leaderboard for hotspots
leaderboard := &design.Leaderboard{
    Label:      "Slowest Tests",
    ShowRank:   true,
    Items:      slowTests, // []design.LeaderboardItem
}
fmt.Println(leaderboard.Render(cfg))

// TestTable with compact density
testTable := &design.TestTable{
    Results: testResults,
    Density: design.DensityCompact, // 3 columns
}
fmt.Println(testTable.Render(cfg))
```

See [pkg/design/patterns.go](pkg/design/patterns.go) for complete pattern API.

## CLI Reference

### Operation Modes

- **CAPTURE mode** (default): Buffers output, shows summary on completion
- **STREAM mode** (`-s`): Real-time output for interactive commands

### Flags

- `-l, --label <string>`: Task label
- `-s, --stream`: STREAM mode
- `--show-output <mode>`: When to show captured output (`on-fail`|`always`|`never`)
- `--theme <name>`: Visual theme (`unicode_vibrant`|`ascii_minimal`)
- `--no-timer`: Hide duration
- `--no-color`: Disable color/styling
- `--ci`: CI-friendly output (implies --no-color, --no-timer)

### Themes

`fo` ships with multiple themes optimized for different contexts:

- **unicode_vibrant** (default): Rich icons, colors, sparklines
- **ascii_minimal**: Plain ASCII for compatibility
- **Custom themes**: Define your own via `.fo.yaml`

## Configuration

Create `.fo.yaml` in your project root:

```yaml
style:
  use_boxes: true
  density: balanced  # detailed|balanced|compact
  use_inline_progress: true

colors:
  success: "\033[32m"
  error: "\033[31m"
  warning: "\033[33m"

cognitive_load:
  auto_detect: true  # Adapt rendering to output complexity
```

## Design Principles in Practice

### Data-Ink Ratio
Compact modes save 50-66% of lines while maintaining readability:
```
Detailed: ~12 lines
Balanced: ~6 lines (saves 50%)
Compact:  ~4 lines (saves 66%)
```

### Cognitive Load Awareness
High-complexity output (many errors, large output) triggers simplified rendering to reduce cognitive processing overhead.

### Small Multiples
Compose patterns to create comprehensive dashboards:
```go
// Build dashboard
fmt.Println(summary.Render(cfg))
fmt.Println(sparkline.Render(cfg))
fmt.Println(leaderboard.Render(cfg))
fmt.Println(inventory.Render(cfg))
```

## Architecture

**Pattern-Based Rendering**
```
Command Output → Pattern Recognition → Semantic Patterns → Theme Renderer → Visual Output
```

**Key Components**
- `pkg/design/patterns.go`: Pattern types and interfaces
- `pkg/design/config.go`: Theme system
- `pkg/design/render.go`: Rendering engine
- `pkg/design/recognition.go`: Pattern detection

## Development

### Running Examples

```bash
cd examples/patterns
go run main.go          # All patterns
go run compact_demo.go  # Density modes
```

### Running Tests

```bash
go test ./...
```

## Research Foundations

- **Cognitive Load Theory**: Adapts complexity based on output characteristics
- **Tufte Principles**: Data-ink ratio, sparklines, small multiples
- **Information Visualization**: Pattern recognition for semantic meaning

## Related Projects

- [mageconsole](mageconsole/README.md): Go API for programmatic use
- [examples/patterns](examples/patterns/): Pattern demonstrations

## License

MIT License
