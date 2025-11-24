# Pattern Composition Examples

This directory demonstrates how to compose multiple visualization patterns together to create comprehensive dashboards for build processes and quality monitoring.

## Philosophy

The `fo` design system is based on **composable patterns**. Each pattern (Sparkline, Leaderboard, Summary, etc.) is independent and can be combined with others to create rich, information-dense dashboards that guide attention and reduce cognitive load.

## Examples

### Build Dashboard (`dashboard.go`)

Demonstrates composing 5 patterns to create a complete build overview:

```bash
go run dashboard.go
```

**Patterns Used:**
- **Summary** - High-level build metrics (packages, tests, coverage, time)
- **Sparkline** - Trends over time (build performance, coverage progression)
- **Leaderboard** - Ranked optimization targets (slowest tests)
- **Comparison** - Before/after sprint metrics with deltas
- **Inventory** - Build artifacts produced

**Key Concepts:**
- Each pattern renders independently
- Patterns share a common theme configuration
- Visual hierarchy guides attention (headers, indentation, colors)
- Data-ink ratio maximized (minimal decoration, maximum information)

### Quality Dashboard (`quality.go`)

Demonstrates composing patterns to create a quality metrics dashboard:

```bash
go run quality.go
```

**Patterns Used:**
- **Comparison** - Coverage changes by package over time
- **Leaderboard** - Slowest tests (optimization targets)
- **Sparkline** - Coverage trend progression
- **Leaderboard** - Largest binaries (size analysis)
- **Summary** - Overall quality metrics

**Use Cases:**
- Sprint retrospectives
- Code quality reviews
- Performance optimization planning
- Binary size analysis

### Makefile Integration (`Makefile`)

Real-world Makefile example showing pattern composition in build workflows:

```bash
make build-dashboard    # Generate build metrics dashboard
make quality-dashboard  # Generate quality metrics dashboard
make all                # Run full pipeline with formatted output
```

**Features:**
- Integrates fo formatting into build steps
- Composes dashboards from multiple patterns
- Demonstrates CI/CD pipeline integration
- Shows practical workflow patterns

## Running the Examples

### Prerequisites

```bash
# From repository root
go mod download
```

### Execute

```bash
cd examples/composition
go run dashboard.go
```

## Creating Your Own Dashboards

### 1. Choose Your Patterns

Select patterns based on what information you need to convey:

| Pattern | Use Case |
|---------|----------|
| `Summary` | Overall metrics, key-value pairs |
| `Sparkline` | Trends over time (build time, coverage, errors) |
| `Leaderboard` | Ranked lists (slowest tests, largest binaries, most warnings) |
| `Comparison` | Before/after comparisons with deltas |
| `Inventory` | Lists of artifacts, files, packages |
| `TestTable` | Comprehensive test results |

### 2. Initialize Theme

```go
cfg := design.UnicodeVibrantTheme()  // Rich Unicode
// or
cfg := design.ASCIIMinimalTheme()    // Plain ASCII for compatibility
```

### 3. Create and Render Patterns

```go
// Create pattern instance
sparkline := &design.Sparkline{
    Label:  "Build time trend",
    Values: buildTimes,
    Unit:   "s",
}

// Render with theme
output := sparkline.Render(cfg)
fmt.Println(output)
```

### 4. Compose Together

Stack patterns vertically with section headers:

```go
fmt.Println("## Performance Metrics")
fmt.Println(sparkline.Render(cfg))
fmt.Println()

fmt.Println("## Optimization Targets")
fmt.Println(leaderboard.Render(cfg))
```

## Design Principles Applied

### Data-Ink Ratio (Tufte)

Every character conveys information. No decorative elements that don't serve a purpose.

### Cognitive Load Awareness

Visual hierarchy (headers, spacing, color) guides attention to important information first.

### Small Multiples

Multiple instances of the same pattern type (e.g., two sparklines for different metrics) allow easy comparison.

### Layering

Section headers create layers of information - scan headers first, dive into details as needed.

## Integration Examples

### In Makefile

```makefile
.PHONY: dashboard
dashboard:
	@go run examples/composition/dashboard.go
```

### In CI Pipeline

```yaml
# .github/workflows/build.yml
- name: Build Dashboard
  run: |
    go run examples/composition/dashboard.go > dashboard.txt
    cat dashboard.txt
```

### As Library

Example integration (note: `BuildResults`, `createSummary()`, `createTrends()`, and `createHotspots()` are placeholders - implement these based on your specific build system):

```go
import "github.com/dkoosis/fo/pkg/design"

func BuildDashboard(buildResults BuildResults) string {
    cfg := design.UnicodeVibrantTheme()

    summary := createSummary(buildResults)
    sparkline := createTrends(buildResults)
    leaderboard := createHotspots(buildResults)

    return summary.Render(cfg) + "\n" +
           sparkline.Render(cfg) + "\n" +
           leaderboard.Render(cfg)
}
```

## Advanced Composition

### Conditional Patterns

Show different patterns based on build state:

```go
if buildFailed {
    // Emphasize errors
    fmt.Println(errorSummary.Render(cfg))
    fmt.Println(failedTestsTable.Render(cfg))
} else {
    // Show comprehensive dashboard
    fmt.Println(summary.Render(cfg))
    fmt.Println(trends.Render(cfg))
}
```

### Density Modes

Adjust pattern density for different contexts:

```go
// CI logs - compact
testTable := &design.TestTable{
    Results: results,
    Density: design.DensityCompact,  // 3 columns
}

// Local development - detailed
testTable := &design.TestTable{
    Results: results,
    Density: design.DensityDetailed,  // 1 column, full context
}
```

## References

- [Pattern API Documentation](../../pkg/design/patterns.go)
- [Theme Configuration](../../pkg/design/config.go)
- [Design Philosophy](../../README.md#design-philosophy)
