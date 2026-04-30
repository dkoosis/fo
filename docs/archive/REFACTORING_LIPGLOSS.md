# Lipgloss Refactoring Plan (Issue #137)

## Overview

This document tracks the refactoring of fo to use lipgloss idiomatically for all styling, eliminating manual ANSI escape code handling and consolidating the dual color system.

## Current State Analysis

### Dual Color System
- **Config.Colors**: String fields storing raw ANSI escape codes (`"\x1b[38;5;111m"`)
- **DesignTokens.Colors**: Already uses `lipgloss.Color` but not fully integrated
- **NormalizeANSIEscape()**: Manual function to handle YAML parsing edge cases

### Manual Color Concatenation
- Pattern: `color + text + reset` throughout codebase
- Found in: `render.go`, `patterns.go`, `progress.go`
- ~162 color-related usages to refactor

### Custom Border Drawing
- Character-by-character border construction
- Manual width calculations
- Should use `lipgloss.Border()` types

## Implementation Phases

### Phase 1: Color System ✅ **COMPLETE**
**Goal**: Change `Config.Colors` from `string` to `lipgloss.Color`

**Tasks**:
- [x] Change `Config.Colors` struct fields from `string` to `lipgloss.Color`
- [x] Update YAML unmarshaling to handle color values as strings (lipgloss.Color is a string type)
- [x] Deprecate `NormalizeANSIEscape()` (kept as no-op for compatibility)
- [x] Update `GetColor()` and `ResetColor()` methods to return `lipgloss.Color`
- [x] Update all call sites to convert `lipgloss.Color` to `string` when writing (Phase 1)
- [x] Update default theme colors to use ANSI codes stored in `lipgloss.Color`
- [x] Update theme loading in `internal/config/config.go` (normalizeThemeColors is now no-op)
- [x] Update all code to compile (tests may need updates for expected values)

**Status**: Core refactoring complete. Colors are now `lipgloss.Color` type throughout. 
Default values use ANSI codes (for Phase 1 manual concatenation). 
Phase 2 will replace manual concatenation with `lipgloss.Style.Render()`.

**Files to Modify**:
- `pkg/design/config.go` - Core color system
- `pkg/design/render.go` - Color usage
- `pkg/design/patterns.go` - Pattern color usage
- `pkg/design/progress.go` - Progress color usage
- `internal/config/config.go` - Theme loading
- All test files

**Breaking Changes**:
- `.fo.yaml` format changes: `process: "111"` instead of `process: "\x1b[38;5;111m"`
- Public API: `GetColor()` returns `lipgloss.Color` instead of `string`

### Phase 2: Style Rendering ✅ **COMPLETE**
**Goal**: Replace manual color concatenation with `lipgloss.Style.Render()`

**Tasks**:
- [x] Create reusable `lipgloss.Style` instances for common patterns
- [x] Replace `color + text + reset` with `style.Render(text)`
- [x] Use `lipgloss.Style.Render()` for all text rendering
- [x] Update all rendering functions
- [x] Add helper methods: `GetStyle()`, `GetStyleWithBold()`, `GetStyleFromElement()`

**Status**: Complete. All manual color concatenation replaced with `lipgloss.Style.Render()`. 
Helper methods added to `Config` for creating styles from color keys and element definitions.
All patterns and rendering functions now use idiomatic lipgloss styling.

**Files Modified**:
- `pkg/design/config.go` - Added style helper methods
- `pkg/design/render.go` - All rendering uses styles
- `pkg/design/patterns.go` - All patterns use styles

### Phase 3: Border System ✅ **COMPLETE**
**Goal**: Replace custom border handling with `lipgloss.Border()` types

**Tasks**:
- [x] Replace custom border character handling with `lipgloss.Border()` types
- [x] Consolidate BoxLayout to single implementation in `pkg/design/render.go`
- [x] Use lipgloss.Border for border styling with proper corner resolution
- [x] Leverage lipgloss padding/margin for spacing via BorderStyle
- [x] Update box rendering in `fo/console.go` to use design.BoxLayout

**Status**: Complete. BoxLayout consolidated into `pkg/design/render.go` with:
- `BorderChars` struct for corner/edge characters
- `ResolveBorderChars()` for proper corner matching
- `resolveBorderColor()` for theme-aware border colors
- lipgloss.Border integration in NewBoxLayout()

**Files Modified**:
- `pkg/design/render.go` - Consolidated BoxLayout with lipgloss borders
- `fo/console.go` - Uses design.BoxLayout, removed duplicate types

### Phase 4: Theme Definition
**Goal**: Define themes as collections of `lipgloss.Style` instances

**Tasks**:
- [ ] Define themes as collections of `lipgloss.Style` instances
- [ ] Simplify `.fo.yaml` theme format to map to lipgloss primitives
- [ ] Consider lipgloss's built-in adaptive colors for light/dark terminals
- [ ] Update theme documentation

**Files to Modify**:
- `pkg/design/config.go` - Theme definitions
- `docs/guides/THEME_GUIDE.md` - Documentation

## Benefits

1. **Less code**: Remove ~200+ lines of ANSI handling
2. **Better compatibility**: lipgloss auto-detects terminal color support
3. **Simpler themes**: Users specify `"111"` not `"\x1b[38;5;111m"`
4. **Consistent API**: One way to do styling, not two parallel systems
5. **Future-proof**: lipgloss builds on ultraviolet for modern terminal support

## Migration Strategy

### Backward Compatibility
- Support both old and new YAML formats during transition
- Provide migration script/tool if needed
- Document breaking changes clearly

### Testing Strategy
- Update all existing tests
- Add tests for YAML color format parsing
- Test terminal compatibility (color/no-color/CI modes)
- Visual regression tests for themes

## Progress Tracking

- [x] Phase 1: Color System ✅
- [x] Phase 2: Style Rendering ✅
- [x] Phase 3: Border System ✅
- [ ] Phase 4: Theme Definition (future enhancement)

## Related Issues

- #138 - Consider adopting bubbles components (depends on #137)

