# fo Architecture: Conceptual Model

This document provides conceptual diagrams showing how fo processes command output and transforms it into structured visual patterns.

## Information Flow

```mermaid
graph TD
    A[Command Execution] --> B[Capture Output]
    B --> C[Output Classification]
    C --> D[Pattern Recognition]
    D --> E[Pattern Selection]
    E --> F[Theme Application]
    F --> G[Visual Rendering]
    G --> H[Terminal Output]
    
    C --> C1[Error Detection]
    C --> C2[Warning Detection]
    C --> C3[Success Detection]
    C --> C4[Info Classification]
    
    D --> D1[Sparkline Pattern]
    D --> D2[Leaderboard Pattern]
    D --> D3[TestTable Pattern]
    D --> D4[Summary Pattern]
    D --> D5[Comparison Pattern]
    D --> D6[Inventory Pattern]
    
    F --> F1[Color Scheme]
    F --> F2[Icon Selection]
    F --> F3[Density Mode]
    F --> F4[Border Style]
```

## Pattern Type Hierarchy

```mermaid
graph TD
    A[Pattern Interface] --> B[PatternType Enum]
    
    B --> C[Sparkline]
    B --> D[Leaderboard]
    B --> E[TestTable]
    B --> F[Summary]
    B --> G[Comparison]
    B --> H[Inventory]
    
    A --> I[Render Method]
    A --> J[PatternType Method]
    
    I --> K[Config Theme]
    K --> L[Visual Output]
```

## Config Resolution Flow

```mermaid
graph TD
    A[CLI Flags] --> E[Final Config]
    B[.fo.yaml File] --> E
    C[Environment Variables] --> E
    D[Default Theme] --> E
    
    E --> F[Theme Selection]
    F --> G[Color/Icon Resolution]
    G --> H[Style Application]
    H --> I[Rendered Output]
    
    style A fill:#e1f5ff
    style B fill:#fff4e1
    style C fill:#e8f5e9
    style D fill:#f3e5f5
    style E fill:#ffebee
```

## Error Propagation Paths

```mermaid
graph TD
    A[Command Execution] --> B{Exit Code}
    B -->|0| C[Success Path]
    B -->|Non-zero| D[Error Path]
    
    C --> E[Success Pattern]
    E --> F[Green/Success Theme]
    
    D --> G[Error Classification]
    G --> H[Error Pattern]
    H --> I[Red/Error Theme]
    
    J[Internal fo Error] --> K[Stderr Channel]
    K --> L[fo Prefix]
    L --> M[Distinct Formatting]
    
    style J fill:#ffcdd2
    style K fill:#ffcdd2
    style L fill:#ffcdd2
    style M fill:#ffcdd2
```

## Pattern Recognition Process

```mermaid
sequenceDiagram
    participant Cmd as Command Output
    participant Classifier as Output Classifier
    participant Matcher as Pattern Matcher
    participant Selector as Pattern Selector
    participant Renderer as Theme Renderer
    participant Terminal as Terminal
    
    Cmd->>Classifier: Raw output lines
    Classifier->>Classifier: Detect errors/warnings
    Classifier->>Matcher: Classified lines
    Matcher->>Matcher: Match intent patterns
    Matcher->>Selector: Suggested patterns
    Selector->>Selector: Choose best pattern
    Selector->>Renderer: Pattern + Data
    Renderer->>Renderer: Apply theme
    Renderer->>Terminal: Formatted output
```

## System Boundaries

```mermaid
graph LR
    subgraph "fo CLI"
        A[Command Wrapper]
        B[Output Capture]
        C[Pattern System]
        D[Theme System]
    end
    
    subgraph "External"
        E[User Commands]
        F[Terminal]
    end
    
    subgraph "Configuration"
        G[.fo.yaml]
        H[CLI Flags]
        I[Environment]
    end
    
    E --> A
    A --> B
    B --> C
    C --> D
    D --> F
    
    G --> D
    H --> D
    I --> D
```

## Key Concepts

### 1. Separation of Concerns

- **Semantic Layer (Patterns)**: What to show (data, structure, meaning)
- **Visual Layer (Theme)**: How to show it (colors, icons, density, borders)
- **Execution Layer (CLI)**: Command wrapping and output capture

### 2. Pattern-Based Architecture

Patterns are composable units that can be combined to create dashboards. Each pattern has:
- A semantic contract (data structure)
- A visual contract (rendering behavior)
- A type identifier (PatternType enum)

### 3. Theme Independence

The same pattern can be rendered with different themes:
- `unicode_vibrant`: Rich icons, colors, sparklines
- `ascii_minimal`: Plain ASCII for compatibility
- Custom themes: User-defined via `.fo.yaml`

### 4. Cognitive Load Awareness

The system adapts rendering complexity based on:
- Output size (line count)
- Error/warning density
- Pattern complexity
- User preferences

## Related Documentation

- [Pattern Types Specification](pattern-types.md)
- [ADR-001: Pattern-Based Architecture](../adr/ADR-001-pattern-based-architecture.md)
- [Product Vision](../VISION_REVIEW.md)

