# Box Rendering Design Assessment

## Problem Statement

The current box rendering implementation suffers from **fragmented responsibility** and **inconsistent calculations**, leading to:
- Misaligned borders (vertical borders don't line up)
- Inconsistent padding calculations
- Duplicated width/border logic across three functions
- Difficulty maintaining alignment when making changes

## Current Architecture Issues

### 1. **Fragmented Rendering Logic**

Three separate functions each independently calculate dimensions:

```go
PrintSectionHeader()  // Calculates: headerWidth, padding, border color
PrintSectionLine()    // Calculates: headerWidth, maxContentWidth, padding, border color  
PrintSectionFooter()  // Calculates: headerWidth, border color
```

**Problems:**
- Each function calls `c.getTerminalWidth()` independently
- Each function calculates border color separately (duplicated `if cfg.ThemeName == "orca"` logic)
- Padding calculations differ between header and content lines
- No shared understanding of box dimensions

### 2. **Inconsistent Width Calculations**

**Header line:**
```go
headerWidth := c.getTerminalWidth()
sb.WriteString(strings.Repeat(headerChar, headerWidth))  // Horizontal line width
remainingWidth := headerWidth - titleVisualLen - 1         // Content padding
```

**Content line:**
```go
headerWidth := c.getTerminalWidth()
maxContentWidth := headerWidth - 3                         // Different formula!
padding := headerWidth - visualWidth - 1                   // Different formula!
```

**Footer line:**
```go
headerWidth := c.getTerminalWidth()
sb.WriteString(strings.Repeat(headerChar, headerWidth))    // Same as header
```

**Result:** Three different formulas for what should be the same box width.

### 3. **Magic Numbers and Hardcoded Values**

- `headerWidth - 3` (where does 3 come from?)
- `headerWidth - visualWidth - 1` (why -1?)
- `"  "` (2 spaces hardcoded)
- Border color logic duplicated 3 times

### 4. **No Single Source of Truth**

There's no shared `BoxLayout` struct that defines:
- Total box width
- Content area width  
- Padding values
- Border characters
- Border colors

## Recommended Solution: Box Layout Model

### Pattern: **Layout Builder / Box Model**

Similar to CSS box model or terminal UI libraries (tview, termui, bubbletea), we need:

1. **A `BoxLayout` struct** that encapsulates all box dimensions
2. **A single rendering context** that maintains box state
3. **Consistent calculation methods** for all box elements

### Proposed Architecture

```go
// BoxLayout defines the dimensions and styling of a rendered box
type BoxLayout struct {
    TotalWidth      int    // Full terminal width
    ContentWidth    int    // Available width for content (TotalWidth - borders - padding)
    LeftPadding     int    // Spaces after left border
    RightPadding    int    // Spaces before right border
    BorderColor     string // ANSI color for borders
    BorderChars     BorderChars // Corner and line characters
}

// BorderChars encapsulates all border characters
type BorderChars struct {
    TopLeft     string
    TopRight    string
    BottomLeft  string
    BottomRight string
    Horizontal  string
    Vertical    string
}

// Console maintains box state
type Console struct {
    // ... existing fields ...
    currentBox *BoxLayout  // Active box being rendered
}
```

### Benefits

1. **Single Source of Truth**: All dimensions calculated once
2. **Consistent Rendering**: All three functions use the same `BoxLayout`
3. **Easier Maintenance**: Change box dimensions in one place
4. **Testability**: Can test `BoxLayout` calculations independently
5. **Extensibility**: Easy to add new box styles or themes

### Implementation Strategy

**Phase 1: Extract Box Layout**
- Create `BoxLayout` struct
- Create `calculateBoxLayout()` method
- Move all width/padding calculations into this method

**Phase 2: Refactor Rendering Functions**
- `PrintSectionHeader()` uses `BoxLayout` for all calculations
- `PrintSectionLine()` uses `BoxLayout` for all calculations  
- `PrintSectionFooter()` uses `BoxLayout` for all calculations

**Phase 3: Unify Border Logic**
- Extract border color calculation to `getBorderColor()` method
- Extract border character selection to `getBorderChars()` method
- Remove duplication

### Example Refactored Code

```go
// Calculate box layout once, use everywhere
func (c *Console) calculateBoxLayout() *BoxLayout {
    cfg := c.designConf
    totalWidth := c.getTerminalWidth()
    
    // Content area = total width - left border (1) - left padding (2) - right padding (1) - right border (1)
    contentWidth := totalWidth - 5
    
    return &BoxLayout{
        TotalWidth:   totalWidth,
        ContentWidth: contentWidth,
        LeftPadding:  2,
        RightPadding: 1,
        BorderColor:  c.getBorderColor(),
        BorderChars:  c.getBorderChars(),
    }
}

// All rendering functions use the same layout
func (c *Console) PrintSectionHeader(name string) {
    box := c.calculateBoxLayout()
    // Use box.TotalWidth, box.BorderColor, box.BorderChars consistently
}

func (c *Console) PrintSectionLine(line string) {
    box := c.calculateBoxLayout()
    // Use box.ContentWidth, box.LeftPadding, box.RightPadding consistently
}
```

## Comparison to Established Patterns

### CSS Box Model
- **Border**: Defines outer edge
- **Padding**: Space between border and content
- **Content**: Actual content area
- **Total width**: Border + Padding + Content

### Terminal UI Libraries (tview, termui)
- **Layout structs**: Define dimensions and constraints
- **Render context**: Maintains state during rendering
- **Consistent calculations**: All elements use same layout

### Our Current Approach
- ❌ No layout struct
- ❌ No shared context
- ❌ Inconsistent calculations
- ❌ Duplicated logic

## Conclusion

The current design violates the **DRY principle** and lacks a **single source of truth** for box dimensions. Implementing a **Box Layout Model** will:

1. Fix alignment issues
2. Reduce code duplication
3. Make future changes easier
4. Improve testability
5. Align with established UI patterns

**Recommendation**: Implement the Box Layout Model as described above.

