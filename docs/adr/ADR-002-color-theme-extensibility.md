# ADR-002: Refactor Color and Theme System for Extensibility

**Status**: Proposed
**Date**: 2025-11-24
**Deciders**: fo project team

## Context

The current color and theme system has significant extensibility limitations:

### Current Problems

1. **Hardcoded Switch Statements**: Adding a new color requires:
   - Adding field to `Colors` struct
   - Adding to both theme initializations (UnicodeVibrant + ASCIIMinimal)
   - Adding case to `resolveColorName()` switch statement
   - Manual application in formatting code

2. **Duplicated Initialization**: Both themes duplicate struct initialization code, requiring updates in multiple places for any change.

3. **No Dynamic Property Access**: Cannot access colors by name dynamically; must hardcode every color in switch statements.

4. **Tight Coupling**: Formatting code manually applies colors rather than using configuration-driven styling.

5. **No Element-Level Color Control**: Element styles cannot specify colors for their sub-components (e.g., spinner color).

### Impact

- **High maintenance cost**: Simple changes require touching 4+ files
- **Error-prone**: Easy to miss locations when adding features
- **Not scalable**: Adding colors/themes becomes increasingly difficult
- **Poor separation of concerns**: Formatting logic mixed with color resolution

### Example: Adding PaleBlue Color

Required changes:
1. Add `PaleBlue` field to `Colors` struct
2. Initialize in `UnicodeVibrantTheme()`
3. Initialize in `ASCIIMinimalTheme()`
4. Add case to `resolveColorName()` switch
5. Manually apply in `progress.go` formatting code

**Result**: 5 locations changed for a single color addition.

## Decision

Refactor the color and theme system to use established UI/design system patterns:

### Core Principles

1. **Design Tokens**: Centralized semantic values with type-safe access
2. **Reflection-Based Access**: Dynamic property lookup (no hardcoded switches)
3. **Style Sheets**: Declarative style definitions separate from rendering
4. **Theme Composition**: Base theme + overrides = final theme
5. **Style Context**: Inherited style state for parent-child relationships

### Architecture

#### 1. Design Tokens System

Replace hardcoded color values with semantic tokens:

```go
type DesignTokens struct {
    Colors struct {
        Process  Token
        Success  Token
        Warning  Token
        Error    Token
        Spinner  Token  // Semantic naming
        // ... extensible via map
    }
    Spacing struct {
        Progress int
        // ...
    }
}

// Type-safe access
tokens.Colors.Spinner.Get() // Returns ANSI code
```

#### 2. Reflection-Based Color Resolution

Replace switch statement with reflection:

```go
func (c *Config) GetColor(name string) string {
    // Use reflection instead of hardcoded switch
    return reflectGetField(c.Tokens.Colors, name)
}
```

**Benefits**:
- No switch statement maintenance
- Works for any color automatically
- Extensible without code changes

#### 3. Style Sheet Pattern

Separate style definitions from rendering:

```go
// Style sheet (declarative)
Styles["Task_Progress_Line"] = Style{
    Spinner: StyleComponent{
        Chars: "·✻✽✶✳✢",
        Color: "Spinner", // References token
    },
    Text: StyleComponent{
        Color: "Process",
    },
}

// Renderer applies styles
renderer.ApplyStyle(element, Styles["Task_Progress_Line"])
```

#### 4. Theme Composition

Base theme + overrides:

```go
baseTheme := UnicodeVibrantTheme()
override := LoadTheme("custom.yaml") // Only differences
finalTheme := MergeThemes(baseTheme, override)
```

#### 5. Style Context Inheritance

Context-based styling with inheritance:

```go
type StyleContext struct {
    Tokens *DesignTokens
    Overrides map[string]interface{}
}

func Render(element Element, ctx StyleContext) {
    // Element can override ctx for children
    childCtx := ctx.WithOverride(element.Style)
    Render(child, childCtx)
}
```

## Consequences

### Positive

- **Extensibility**: Add colors/themes without code changes
- **Maintainability**: Single source of truth for design values
- **Type Safety**: Compile-time guarantees with tokens
- **Separation of Concerns**: Styles separate from rendering
- **Scalability**: System grows without complexity explosion
- **Alignment**: Follows established UI/design system patterns

### Negative

- **Refactoring Effort**: Significant initial work required
- **Learning Curve**: Team needs to understand new patterns
- **Migration Risk**: Must maintain backwards compatibility during transition
- **Complexity**: More abstraction layers (but better organized)

### Risks

- **Breaking Changes**: Risk of regressions during refactoring
- **Performance**: Reflection may have slight overhead (mitigated by caching)
- **Over-Engineering**: Risk of adding unnecessary complexity

**Mitigation**:
- Incremental migration (parallel implementation)
- Comprehensive testing at each phase
- Feature flags for gradual rollout
- Performance benchmarks

## Implementation Strategy

### Phased Approach

**Phase 0: Foundation** (Low risk, high value)
- Add reflection-based color access (parallel to existing)
- Keep old system working
- Test both paths

**Phase 1: Design Tokens**
- Centralize all design values
- Semantic naming
- Type-safe access

**Phase 2: Theme Composition**
- Base theme + overrides
- Deep merge strategy
- Backwards compatible

**Phase 3: Style Sheets**
- Separate style definitions
- Element-based styling
- Cleaner separation

**Phase 4: Style Context**
- Context-based styling
- Parent-child inheritance
- Override mechanism

**Phase 5: Cleanup**
- Remove deprecated code
- Remove duplicate initialization
- Final cleanup

### Migration Principles

1. **Incremental**: One phase at a time, complete and test before moving on
2. **Parallel Implementation**: Keep old system working during transition
3. **Feature Flags**: Toggle between old/new for testing
4. **Comprehensive Tests**: Each phase must pass all tests
5. **Documentation**: Update ADR with learnings

## Alternatives Considered

### 1. Keep Current System
- **Rejected**: Extensibility problems will compound over time
- Maintenance burden too high
- Doesn't align with fo's goals of thoughtful design

### 2. Minimal Fixes Only
- **Rejected**: Addresses symptoms, not root cause
- Would require repeated fixes for each new feature
- Technical debt continues to grow

### 3. Complete Rewrite
- **Rejected**: Too risky, too much work at once
- High chance of breaking existing functionality
- No incremental value delivery

### 4. Chosen: Incremental Refactoring
- **Selected**: Balances ambition with risk management
- Delivers value at each phase
- Maintains system stability
- Aligns with fo's thoughtful approach

## Alignment with fo's Goals

This refactoring supports fo's core principles:

1. **Thoughtfulness Over Speed**: Research-backed patterns from established systems
2. **Clarity Over Complexity**: Better organized, easier to understand
3. **Flexibility Within Structure**: More extensible while maintaining structure
4. **Developer Experience First**: Easier to extend and customize

## References

- [ADR-001: Pattern-Based Architecture](ADR-001-pattern-based-architecture.md)
- [Product Philosophy](../design/philosophy.md)
- CSS Cascading and Specificity
- Design Tokens (Material Design, Design Systems)
- React/Vue Component Architecture
- Qt Stylesheets Pattern

## Next Steps

1. Create implementation issues for each phase
2. Begin Phase 0: Reflection-based color access
3. Document learnings and adjust approach as needed

