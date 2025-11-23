# Pattern Examples

This directory demonstrates the various visualization patterns available in the `fo` design system.

## Available Patterns

### 1. Sparkline
Compact trend visualization using Unicode block characters. Perfect for showing:
- Build time trends over multiple runs
- Test coverage changes
- Performance metric progressions
- Resource usage trends

```go
sparkline := &design.Sparkline{
    Label:  "Build time trend",
    Values: []float64{2.3, 2.1, 2.4, 1.9, 1.8, 2.0, 1.7, 1.6},
    Unit:   "s",
}
fmt.Println(sparkline.Render(cfg))
```

Output: `Build time trend: ▄▃▄▂▂▃▁▁ 1.6s`

### 2. Leaderboard
Ranked list showing top/bottom N items by a metric. Useful for:
- Slowest tests (optimization targets)
- Largest binaries (size analysis)
- Most warnings (quality hotspots)
- Lowest coverage (test gaps)

```go
leaderboard := &design.Leaderboard{
    Label:      "Slowest Tests",
    ShowRank:   true,
    TotalCount: 247,
    Items: []design.LeaderboardItem{
        {Name: "TestLargeData", Metric: "5.2s", Value: 5.2, Rank: 1},
        {Name: "TestQuery", Metric: "3.8s", Value: 3.8, Rank: 2},
    },
}
fmt.Println(leaderboard.Render(cfg))
```

### 3. TestTable
Comprehensive test results table showing all packages/tests with status and timing.

```go
testTable := &design.TestTable{
    Label: "Unit Test Results",
    Results: []design.TestTableItem{
        {Name: "pkg/api", Status: "pass", Duration: "2.1s", Count: 42},
        {Name: "pkg/auth", Status: "fail", Duration: "0.5s", Count: 15, Details: "timeout"},
    },
}
```

### 4. Summary
High-level metrics and counts for at-a-glance understanding.

```go
summary := &design.Summary{
    Label: "Build Summary",
    Metrics: []design.SummaryItem{
        {Label: "Total Tests", Value: "247", Type: "info"},
        {Label: "Passed", Value: "232", Type: "success"},
        {Label: "Failed", Value: "3", Type: "error"},
    },
}
```

### 5. Comparison
Before/after comparison showing changes with directional indicators.

```go
comparison := &design.Comparison{
    Label: "Performance vs. Previous Build",
    Changes: []design.ComparisonItem{
        {Label: "Build time", Before: "5.2s", After: "4.1s", Change: -1.1, Unit: "s"},
        {Label: "Coverage", Before: "85%", After: "88%", Change: 3, Unit: "%"},
    },
}
```

### 6. Inventory
List of generated artifacts or files with sizes and paths.

```go
inventory := &design.Inventory{
    Label: "Generated Artifacts",
    Items: []design.InventoryItem{
        {Name: "myapp", Size: "38.2MB", Path: "./bin/myapp"},
        {Name: "myapp-linux", Size: "38.5MB", Path: "./dist/myapp-linux"},
    },
}
```

## Running the Examples

```bash
cd examples/patterns
go run main.go
```

## Design Principles

These patterns follow **Tufte-inspired** design principles:

1. **High data-ink ratio**: Maximize information per line, minimize decoration
2. **Small multiples**: Patterns can be composed to create dashboards
3. **Layering**: Visual hierarchy guides attention to important information
4. **Sparklines**: Intense, simple, word-sized graphics for trends

## Composing Patterns

Multiple patterns can be combined to create comprehensive dashboards:

```go
// Build dashboard
fmt.Println(summary.Render(cfg))
fmt.Println(sparkline.Render(cfg))
fmt.Println(leaderboard.Render(cfg))
fmt.Println(inventory.Render(cfg))
```

This creates a complete picture of the build process: overall status, trends, hotspots, and outputs.
