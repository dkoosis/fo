# ADR-002: Refactor Color and Theme System for Extensibility

**Status**: Revised (Superseded by Lip Gloss Integration)
**Date**: 2025-11-24
**Revised**: 2025-01-XX
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

~~Replace switch statement with reflection~~ **SUPERSEDED**: Lip Gloss provides style system

**Original Plan**:
```go
func (c *Config) GetColor(name string) string {
    // Use reflection instead of hardcoded switch
    return reflectGetField(c.Tokens.Colors, name)
}
```

**New Approach**: Use Lip Gloss `lipgloss.Color` and `lipgloss.Style` types directly. Design Tokens (#114) will provide semantic naming layer on top of Lip Gloss.

#### 3. Style Sheet Pattern

~~Separate style definitions from rendering~~ **SUPERSEDED**: Lip Gloss IS a style sheet system

**Original Plan**: Custom style sheet pattern

**New Approach**: Lip Gloss provides `lipgloss.Style` which acts as a style sheet:
```go
style := lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("111")).
    Padding(1, 2)
content := style.Render(innerContent)
```

Design Tokens (#114) will provide semantic naming on top of Lip Gloss styles.

#### 4. Theme Composition

Base theme + overrides:

```go
baseTheme := UnicodeVibrantTheme()
override := LoadTheme("custom.yaml") // Only differences
finalTheme := MergeThemes(baseTheme, override)
```

#### 5. Style Context Inheritance

~~Context-based styling with inheritance~~ **SUPERSEDED**: Lip Gloss handles inheritance

**Original Plan**: Custom context inheritance system

**New Approach**: Lip Gloss provides `lipgloss.Style.Inherit()` for style inheritance:
```go
baseStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
overrideStyle := lipgloss.NewStyle().BorderForeground(lipgloss.Color("111"))
finalStyle := baseStyle.Inherit(overrideStyle)
```

Theme composition (#115) will use this for YAML override merging.

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

## Revision: Lip Gloss Integration

**Date**: 2025-01-XX
**Decision**: Integrate [Lip Gloss](https://github.com/charmbracelet/lipgloss) library instead of custom implementation

### Why Lip Gloss?

After initial implementation planning, we discovered Lip Gloss provides all the core functionality we need:

1. **Correct Rune Width Handling**: `lipgloss.Width()` correctly handles Unicode, emojis, CJK characters
2. **Border Rendering**: Built-in border system with proper corners and styling
3. **Style System**: CSS-like styling with inheritance via `lipgloss.Style.Inherit()`
4. **Composition**: `JoinHorizontal()` / `JoinVertical()` for layout composition
5. **Mature Library**: Well-tested, actively maintained, used by popular CLI tools

### Impact on Original Phases

| Original Phase | New Status | Reason |
|----------------|------------|--------|
| Phase 0: Reflection | Closed #118 | Lip Gloss handles color/style access |
| Phase 1: Design Tokens | Keep #114 | Still needed for semantic naming layer |
| Phase 2: Theme Composition | Keep #115 | Still needed, uses `lipgloss.Inherit()` |
| Phase 3: Style Sheets | Closed #119 | Lip Gloss IS a style sheet system |
| Phase 4: Style Context | Closed #116 | Lip Gloss handles inheritance |
| Phase 5: Cleanup | Closed #120 | Deferred until integration complete |

### Revised Implementation Phases

1. **#121 - Lip Gloss Integration + Width Utilities** ✅
   - Add `lipgloss` dependency
   - Create `PadRight`, `PadLeft`, `VisualWidth` utilities
   - Fix all width specifiers in patterns/render

2. **#122 - BoxLayout Refactor with Lip Gloss** ✅
   - Create `BoxLayout` struct for single-point dimension calculation
   - Integrate Lip Gloss `Border` struct
   - Refactor render functions to use BoxLayout

3. **#114 - Design Tokens (Semantic Layer on Lip Gloss)**
   - Create `DesignTokens` struct using `lipgloss.Color` and `lipgloss.Style`
   - Semantic naming (Spinner not PaleBlue)
   - Theme functions return `*DesignTokens`

4. **#115 - Theme Composition (YAML Overrides)**
   - YAML override schema
   - `LoadThemeOverrides()` function
   - `MergeThemes()` using `lipgloss.Inherit()`

### Benefits of Lip Gloss Approach

- **Faster Implementation**: No need to build custom reflection/style system
- **Proven Solution**: Battle-tested in production CLI tools
- **Better Unicode Support**: Correct handling of wide characters out of the box
- **Less Code**: Leverage existing library instead of custom implementation
- **Community Support**: Active maintenance and community

### Migration Notes

- Original ADR phases 0, 3, 4, 5 are no longer needed
- Phases 1 and 2 remain but adapted to use Lip Gloss types
- All width calculations now use Lip Gloss for correctness
- Box rendering uses Lip Gloss border system

## Next Steps

1. ✅ Complete #121: Lip Gloss integration + width utilities
2. ✅ Complete #122: BoxLayout refactor with Lip Gloss
3. Implement #114: Design Tokens system
4. Implement #115: Theme composition/merging
5. Document final architecture

