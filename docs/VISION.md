# Fo Vision

Fo is a research-based presentation layer for the engineering lifecycle.

## Dual Identity

Fo operates in two complementary modes:

### Editor Mode (The McKinsey Associate)

Fo receives messy, inconsistent output from disparate tools and transforms it into clear, beautiful signal. Like an associate who takes 27 messy files and produces a polished deliverable, fo processes batch input through intelligent classification and themed rendering.

```bash
go test -json ./... | fo
cat build.log test.log lint.log | fo
```

The focus is on **output quality** — transforming chaos into clarity.

### Live Mode (The Flight Deck)

Fo provides real-time telemetry as builds and tests unfold. Like a flight deck display, it offers temporal awareness of work in progress — watching the story develop between sessions.

```bash
go test -json ./... | fo -s
fo -s -- mage build
```

The focus is on **temporal experience** — seeing the work happen.

## Core Principles

### Enforce Visual Rigor

Fo provides a standardized library of rich-ASCII components — headlines, leaderboards, sparklines, tables — that prioritize data density and clarity over decoration. These patterns are informed by Edward Tufte's principles and cognitive load theory.

### Normalize Chaos

Fo acts as a translation layer, consuming raw streams (from Go tests, linters, build tools, specialized scripts) and rendering them through a consistent, thoughtful dashboard interface.

### Respect Cognitive Load

Fo uses visual hierarchy (subheads, indentations, muted colors for context) to ensure critical signals — errors, trends, anomalies — stand out immediately against background noise.

## Architecture

Fo is NOT an interactive TUI application. It is a **streaming presentation filter**:
- Data flows in (stdin, command output)
- Classification happens (adapters detect structured formats, pattern matching for lines)
- Formatted data flows out (themed rendering)

This architecture uses ANSI cursor control for in-place updates rather than full-screen event-driven frameworks like Bubble Tea.

## Configuration

### Themes (External YAML)

Visual presentation is controlled by external theme files defining:
- Colors and styles
- Border characters
- Icons and indicators
- Element-specific formatting

### Adapters

Structured format detection and parsing:
- Go test JSON → TestTable pattern
- (Future) golangci-lint, go vet, MCP output, etc.

### Pattern Matching

Line-by-line classification for unstructured output:
- Error detection
- Warning recognition
- Progress indicators
- File/line references

---

*You use fo to ensure your build output respects the user's attention, presenting complex technical data with the clarity of a well-designed instrument panel.*
