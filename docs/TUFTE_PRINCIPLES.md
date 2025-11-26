# Tufte Design Principles in fo

This document describes how Edward Tufte's design principles are applied in the fo design system. Tufte's work on information visualization, particularly from "The Visual Display of Quantitative Information" (1983), provides the theoretical foundation for many of fo's design decisions.

## Core Principles

### 1. Data-Ink Ratio

**Principle**: Maximize the proportion of ink (or pixels) that represents actual data, and minimize non-data ink (decoration, borders, unnecessary elements).

**Application in fo**:
- **DensityMode types** (`pkg/design/patterns.go:11-24`): The `DensityDetailed`, `DensityBalanced`, and `DensityCompact` modes directly implement this principle by allowing users to choose how much information density they want.
- **Compact rendering**: TestTable and Inventory patterns support multi-column layouts that pack more data into fewer lines.
- **Minimal borders**: Box rendering uses subtle borders only when necessary, and themes can disable borders entirely for maximum data density.

**Code Reference**:
```go
// pkg/design/patterns.go:11-13
// DensityMode controls the space-efficiency of pattern rendering.
// Based on Tufte's data-ink ratio principle: maximize information per line.
type DensityMode string
```

### 2. Sparklines

**Principle**: "Intense, simple, word-sized graphics" that show trends inline with text, without requiring separate charts or graphs.

**Application in fo**:
- **Sparkline pattern** (`pkg/design/patterns.go:98-149`): Fully implemented with Unicode block elements (▁▂▃▄▅▆▇█) to show trends inline.
- **Use cases**: Test duration trends, coverage changes, build size progression, error count trends.
- **Integration**: Sparklines can be embedded in tables, summaries, and other patterns.

**Code Reference**:
```go
// pkg/design/patterns.go:98-99
// Sparkline represents a word-sized graphic showing trends using Unicode blocks.
// Inspired by Tufte's sparklines - intense, simple, word-sized graphics.
```

### 3. Small Multiples

**Principle**: Display multiple similar graphics side-by-side to enable comparison and pattern recognition.

**Application in fo**:
- **Pattern composition**: Multiple patterns (Summary, Sparkline, Leaderboard, Comparison) can be rendered together in `examples/composition/`.
- **Multi-column layouts**: Density modes enable side-by-side display of test results, metrics, and other data.
- **Future enhancement**: A "Tufte mode" theme could enforce strict small multiples layouts.

**Example**:
```go
// examples/composition/dashboard.go
// Multiple patterns rendered together for comparison
console.RenderSummary(buildSummary)
console.RenderSparkline(coverageTrend)
console.RenderLeaderboard(slowestTests)
```

### 4. Layering and Detail-on-Demand

**Principle**: Show information in layers, with primary information always visible and secondary/tertiary information available on demand.

**Application in fo**:
- **Cognitive load awareness**: The system automatically simplifies output when cognitive load is high (many errors, large output).
- **ShowOutput modes**: `on-fail`, `always`, `never` control when detailed output is displayed.
- **Pattern hierarchy**: Summary patterns show high-level metrics, with detailed patterns available for drill-down.

**Code Reference**:
```go
// pkg/design/system.go:43-50
// TaskContext holds information about the cognitive context of the task
// (e.g., complexity, user's likely cognitive load).
type TaskContext struct {
    CognitiveLoad CognitiveLoadContext
    IsDetailView  bool
    // ...
}
```

### 5. Small Effective Differences

**Principle**: Use subtle visual distinctions rather than bold decorations. Differences should be noticeable but not overwhelming.

**Application in fo**:
- **Theme system**: Colors and styles are configurable, allowing subtle or bold distinctions based on user preference.
- **Status indicators**: Success/warning/error states use color and icons, but the differences are meaningful rather than decorative.
- **Monochrome mode**: When colors are disabled, the system relies on typography and spacing for distinction.

### 6. Chartjunk Elimination

**Principle**: Remove all non-essential decorative elements that don't contribute to understanding.

**Application in fo**:
- **Minimal borders**: Box rendering is optional and can be disabled.
- **No animations**: Static output only (no spinners or progress bars in Tufte mode).
- **Clean typography**: Icons and symbols are semantic, not decorative.

## Implementation Status

### ✅ Fully Implemented

1. **Data-Ink Ratio**: DensityMode types and compact rendering
2. **Sparklines**: Complete implementation with Unicode blocks
3. **Pattern Composition**: Multiple patterns can be combined

### ⚠️ Partially Implemented

1. **Small Multiples**: Supported via composition, but not enforced as a strict layout mode
2. **Layering**: Cognitive load awareness exists, but expandable/collapsible sections not yet implemented
3. **Small Effective Differences**: Theme system supports this, but no "Tufte mode" theme exists

### 🔴 Not Yet Implemented

1. **Strict Tufte Mode**: A theme that enforces all principles strictly
2. **Expandable Sections**: Detail-on-demand with collapsible sections
3. **Grid Layouts**: Automatic small multiples in grid format

## Future Enhancements

### Tufte Mode Theme

A future theme could strictly enforce Tufte principles:

```yaml
# .fo.yaml
themes:
  tufte_strict:
    style:
      density: compact
      borders: false
      icons: minimal
    cognitive_load:
      high_threshold: 3  # Simplify even earlier
      medium_threshold: 1
```

### Small Multiples Grid

Automatic grid layout for multiple patterns:

```go
// Future API
renderer.RenderGrid([]Pattern{
    summaryPattern,
    sparklinePattern,
    leaderboardPattern,
}, GridLayout{Columns: 3})
```

### Expandable Sections

Detail-on-demand with keyboard interaction:

```go
// Future API
section := renderer.NewExpandableSection("Test Details")
section.SetSummary("42 tests, 2 failures")
section.SetDetail(fullTestOutput)
// User can expand/collapse
```

## References

- Tufte, Edward R. (1983). *The Visual Display of Quantitative Information*. Graphics Press.
- Tufte, Edward R. (1990). *Envisioning Information*. Graphics Press.
- Tufte, Edward R. (1997). *Visual Explanations*. Graphics Press.
- Tufte, Edward R. (2006). *Beautiful Evidence*. Graphics Press.

## Related Documentation

- [Research Foundations](./RESEARCH_FOUNDATIONS.md) - Cognitive load theory and other research
- [Pattern Types](./design/pattern-types.md) - Detailed pattern documentation
- [Architecture](./design/architecture.md) - System design overview

