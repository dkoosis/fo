# Pattern Type System

This document formalizes the six standard pattern types in the fo design system, establishing clear contracts between semantic meaning (what to show) and visual presentation (how to show it).

## Architecture Overview

The pattern system follows a CSS-inspired architecture:

```
Semantic Pattern (what) → PatternType (enum) → Theme Config (how) → Visual Output
```

- **Patterns** represent semantic content (data, structure, meaning)
- **PatternType** is the type identifier that enables routing and validation
- **Config** (theme) controls visual presentation (colors, icons, density, borders)
- **Render()** transforms pattern + config → formatted string

## The Six Standard Patterns

### 1. Sparkline (`PatternTypeSparkline`)

**Semantic Purpose**: Word-sized trend graphics showing data progression over time.

**Use Cases**:
- Test duration trends over last N runs
- Coverage percentage changes
- Build size progression
- Error count trends

**Data Contract**:
- `Label`: Human-readable label (e.g., "Build time trend")
- `Values`: Array of numeric data points
- `Min/Max`: Optional scale bounds (0 = auto-detect)
- `Unit`: Optional unit suffix (e.g., "ms", "%", "MB")

**Visual Contract**:
- Uses Unicode block elements: ▁▂▃▄▅▆▇█
- Shows latest value with unit
- Theme controls: colors (label, sparkline, unit), monochrome fallback

**Example**:
```go
sparkline := &design.Sparkline{
    Label:  "Build time trend",
    Values: []float64{2.3, 2.1, 2.4, 1.9, 1.8},
    Unit:   "s",
}
fmt.Println(sparkline.Render(cfg))
// Output: Build time trend: ▄▃▄▂▂▃▁▁ 1.8s
```

### 2. Leaderboard (`PatternTypeLeaderboard`)

**Semantic Purpose**: Ranked lists showing top/bottom N items by a specific metric.

**Use Cases**:
- Slowest N tests (optimization targets)
- Largest N binaries (size analysis)
- Files with most linting warnings (quality hotspots)
- Packages with lowest coverage (test gap identification)

**Data Contract**:
- `Label`: Title (e.g., "Slowest Tests")
- `MetricName`: Name of the metric (e.g., "Duration", "Size", "Warnings")
- `Items`: Array of `LeaderboardItem` (name, metric, value, rank, context)
- `Direction`: "highest" or "lowest"
- `TotalCount`: Total items before filtering to top N
- `ShowRank`: Whether to display rank numbers

**Visual Contract**:
- One item per line with rank, name, metric, optional context
- Theme controls: colors (header, rank, name, metric, context), indentation

**Example**:
```go
leaderboard := &design.Leaderboard{
    Label:      "Slowest Tests",
    MetricName: "Duration",
    Items: []design.LeaderboardItem{
        {Name: "TestLargeDataProcessing", Metric: "5.2s", Value: 5.2, Rank: 1},
        {Name: "TestComplexQueryExecution", Metric: "3.8s", Value: 3.8, Rank: 2},
    },
    ShowRank:   true,
    TotalCount: 247,
}
```

### 3. TestTable (`PatternTypeTestTable`)

**Semantic Purpose**: Comprehensive test results with status and timing.

**Use Cases**:
- Complete test suite results
- Package-level test summaries
- Test execution reports

**Data Contract**:
- `Label`: Title (e.g., "Test Results")
- `Results`: Array of `TestTableItem` (name, status, duration, count, details)
- `Density`: Rendering density mode (detailed, balanced, compact)

**Visual Contract**:
- Detailed: One item per line with full context
- Balanced: 2 columns
- Compact: 3 columns
- Theme controls: colors (status icons, duration), density mode, icons

**Example**:
```go
testTable := &design.TestTable{
    Label: "Test Results",
    Results: []design.TestTableItem{
        {Name: "pkg/api", Status: "pass", Duration: "2.1s", Count: 42},
        {Name: "pkg/db", Status: "fail", Duration: "0.5s", Count: 15},
    },
    Density: design.DensityCompact,
}
```

### 4. Summary (`PatternTypeSummary`)

**Semantic Purpose**: High-level summaries with key metrics and counts.

**Use Cases**:
- At-a-glance understanding of overall results
- Rollup statistics
- Executive summaries

**Data Contract**:
- `Label`: Title (e.g., "Build Summary")
- `Metrics`: Array of `SummaryItem` (label, value, type)

**Visual Contract**:
- One metric per line with icon and value
- Type-based coloring (success, error, warning, info)
- Theme controls: icons, colors by type, indentation

**Example**:
```go
summary := &design.Summary{
    Label: "Build Summary",
    Metrics: []design.SummaryItem{
        {Label: "Total Tests", Value: "142", Type: "info"},
        {Label: "Passed", Value: "138", Type: "success"},
        {Label: "Failed", Value: "4", Type: "error"},
    },
}
```

### 5. Comparison (`PatternTypeComparison`)

**Semantic Purpose**: Before/after comparisons of metrics.

**Use Cases**:
- Showing changes over time
- Version comparisons
- Delta analysis

**Data Contract**:
- `Label`: Title (e.g., "Build Metrics Comparison")
- `Changes`: Array of `ComparisonItem` (label, before, after, change, unit)

**Visual Contract**:
- Format: `Label: Before → After ↑/↓ ChangeValue Unit`
- Directional indicators (↑/↓/=) with color coding
- Theme controls: colors (before/after muted, change indicator), arrows

**Example**:
```go
comparison := &design.Comparison{
    Label: "Build Metrics",
    Changes: []design.ComparisonItem{
        {Label: "Build time", Before: "5.2s", After: "4.1s", Change: -1.1, Unit: "s"},
        {Label: "Coverage", Before: "85%", After: "88%", Change: 3.0, Unit: "%"},
    },
}
```

### 6. Inventory (`PatternTypeInventory`)

**Semantic Purpose**: Lists of generated artifacts or files.

**Use Cases**:
- Build outputs
- Generated files
- Deployment artifacts
- File listings

**Data Contract**:
- `Label`: Title (e.g., "Build Artifacts")
- `Items`: Array of `InventoryItem` (name, size, path)

**Visual Contract**:
- One item per line with icon, name, size in brackets
- Optional path on second line
- Theme controls: icons, colors (name, size, path), indentation

**Example**:
```go
inventory := &design.Inventory{
    Label: "Build Artifacts",
    Items: []design.InventoryItem{
        {Name: "myapp", Size: "2.3MB", Path: "./bin/myapp"},
        {Name: "myctl", Size: "1.1MB", Path: "./bin/myctl"},
    },
}
```

## Pattern Interface Contract

All patterns implement the `Pattern` interface:

```go
type Pattern interface {
    // Render returns formatted output using the provided theme configuration.
    // Config controls visual presentation; pattern controls semantic content.
    Render(cfg *Config) string

    // PatternType returns the standard type identifier.
    // Enables type-based routing, validation, and theme selection.
    PatternType() PatternType
}
```

## Type System Benefits

1. **Clear Extension Model**: New patterns follow the same contract
2. **Type Safety**: PatternType enum prevents invalid pattern names
3. **Theme Independence**: Same pattern works with any theme
4. **Composability**: Multiple patterns can be combined in dashboards
5. **Validation**: `IsValidPatternType()` enables runtime validation

## Mapping to Config

The `Config` (theme) structure provides visual styling that transforms patterns:

- **Colors**: Per-pattern color schemes (success, error, warning, muted, etc.)
- **Icons**: Pattern-appropriate icons (✅, ❌, ⚠️, etc.)
- **Density**: Controls space efficiency (detailed, balanced, compact)
- **Borders**: Optional box drawing for structured output
- **Indentation**: Hierarchical layout support

## Related Documents

- [ADR-001: Pattern-Based Architecture](adr/ADR-001-pattern-based-architecture.md)
- [Vision Document](../VISION_REVIEW.md)

