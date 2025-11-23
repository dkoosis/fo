# Pattern Composition Examples

This directory demonstrates how to compose multiple patterns to create comprehensive build dashboards.

## Philosophy

Individual patterns are useful, but **composition** tells a complete story. A well-designed dashboard guides the viewer through:

1. **Summary** - What happened? (overall status)
2. **Trends** - How are we doing over time? (sparklines)
3. **Changes** - What improved or regressed? (comparison)
4. **Hotspots** - What needs attention? (leaderboard)
5. **Details** - Show me the data (test table)
6. **Outputs** - What was produced? (inventory)

This follows cognitive load principles: start with high-level overview, then drill into details.

## Examples

### Dashboard Example (`dashboard.go`)

A complete build dashboard that combines 6 different patterns:

```bash
go run dashboard.go
```

This demonstrates:
- **Layered information**: Summary → Trends → Changes → Hotspots → Details → Artifacts
- **Space efficiency**: Using balanced density (2 columns) for test table
- **Actionable insights**: Leaderboard highlights optimization targets
- **Trend awareness**: Sparklines show progress over time
- **Change tracking**: Comparison shows improvements vs previous build

## Composition Strategies

### Small Multiples
Show the same pattern type with different data side-by-side:
```go
// Multiple sparklines for different metrics
fmt.Println(buildTimeTrend.Render(cfg))
fmt.Println(coverageTrend.Render(cfg))
fmt.Println(binarySizeTrend.Render(cfg))
```

### Progressive Disclosure
Start general, get specific:
```go
// High-level first
summary.Render(cfg)      // Overall numbers

// Mid-level context
comparison.Render(cfg)   // What changed
sparkline.Render(cfg)    // Trends

// Detailed last
testTable.Render(cfg)    // All data
leaderboard.Render(cfg)  // Specific hotspots
```

### Consistent Visual Language
Use the same theme across all patterns for cohesion:
```go
cfg := design.UnicodeVibrantTheme()
// All patterns use same colors, icons, spacing
```

## Density Considerations

When composing multiple patterns, density modes help fit more on screen:

- **Detailed mode**: Use for primary focus (e.g., failed test details)
- **Balanced mode**: Use for secondary data (e.g., all test results)
- **Compact mode**: Use when space is constrained (e.g., CI logs)

```go
primaryData := &design.TestTable{
    Results: failedTests,
    Density: design.DensityDetailed, // Show details
}

allData := &design.TestTable{
    Results: allTests,
    Density: design.DensityCompact, // Fit more on screen
}
```

## Use Cases

### CI/CD Pipelines
```go
// Build summary dashboard
summary.Render(cfg)
comparison.Render(cfg)  // vs main branch
testTable.Render(cfg)   // compact mode
inventory.Render(cfg)   // artifacts
```

### Local Development
```go
// Quick feedback dashboard
summary.Render(cfg)
leaderboard.Render(cfg) // slowest tests to optimize
sparkline.Render(cfg)   // coverage trend
```

### Quality Reports
```go
// Quality metrics dashboard
comparison.Render(cfg)   // coverage change
leaderboard.Render(cfg)  // files with most warnings
testTable.Render(cfg)    // test results
sparkline.Render(cfg)    // quality trend
```

## Design Principles

1. **Information Scent**: Each pattern should lead naturally to the next
2. **Data-Ink Ratio**: Every element should convey meaning (Tufte)
3. **Cognitive Load**: Order patterns from simple (summary) to complex (details)
4. **Actionability**: Include patterns that suggest next actions (leaderboard)
5. **Context**: Provide enough context to understand what you're looking at

## Running the Examples

```bash
cd examples/composition
go mod init github.com/dkoosis/fo/examples/composition
go mod edit -replace github.com/dkoosis/fo=../..
go mod tidy
go run dashboard.go
```
