# Creating Custom Themes for fo

This guide shows you how to create your own custom theme for fo's output rendering.

## Quick Start

Add a `themes:` section to your `.fo.yaml` file and define your custom theme:

```yaml
active_theme: my_custom_theme  # Set this as the active theme

themes:
  my_custom_theme:
    style:
      use_boxes: true
      indentation: "  "
      show_timestamps: false
      density: balanced  # Options: compact, balanced, detailed
      no_timer: false
      use_inline_progress: false
      no_spinner: false
      spinner_style: dots    # Options: dots, line, arc, star, bounce, grow, arrows, clock, moon, ascii
      spinner_chars: ""      # Custom spinner chars (e.g., "⠋ ⠙ ⠹ ⠸" or "/-\\|")
      spinner_interval: 80
      header_width: 50
    
    icons:
      start: "▶"
      success: "✓"
      warning: "⚠"
      error: "✗"
      info: "ℹ"
      bullet: "•"
    
    colors:
      # Use 256-color numbers (simpler) or ANSI escape codes
      process: "117"              # Light blue (256-color)
      success: "120"              # Light green
      warning: "214"              # Orange
      error: "196"                # Red
      detail: ""                  # Reset (empty = no color)
      muted: "242"                # Dark gray
      reset: ""                   # Reset
      spinner: "111"              # Pale blue (Claude-style spinner)
      white: "231"                # Bright white
      blue_fg: "117"              # Light blue foreground
      green_fg: "120"             # Light green foreground
      blue_bg: "4"                # Blue background
      bold: ""                    # Bold (style, not color)
      italic: ""                  # Italic (style, not color)
    
    border:
      task_style: header_box  # Options: none, left, left_double, header_box
      header_char: "─"
      vertical_char: "│"
      top_corner_char: "╭"
      top_right_char: "╮"
      bottom_corner_char: "╰"
      bottom_right_char: "╛"
      footer_continuation_char: "─"
    
    # Element-specific styling (optional)
    elements:
      Task_Label_Header:
        text_case: upper      # Options: none, upper, lower
        text_style: ["bold"]  # Options: bold, italic
        color_fg: BlueFg      # Color key name
      
      Task_Status_Success_Block:
        icon_key: Success
        text_content: "Complete"
        color_fg: Success
      
      Task_Status_Failed_Block:
        icon_key: Error
        text_content: "Failed"
        color_fg: Error
```

## Complete Theme Structure

Here's a comprehensive example with all available fields:

```yaml
active_theme: my_theme

themes:
  my_theme:
    # Style settings
    style:
      use_boxes: true                    # Draw boxes around content
      indentation: "  "                  # Indentation string (spaces or tabs)
      show_timestamps: false             # Show timestamps in output
      density: balanced                  # compact | balanced | detailed
      no_timer: false                    # Hide execution time
      use_inline_progress: false         # Use inline progress indicators
      no_spinner: false                  # Disable spinner animations
      spinner_style: dots                # Spinner animation style
      spinner_chars: ""                  # Custom spinner characters (overrides style)
      spinner_interval: 80               # Spinner update interval (ms)
      header_width: 50                   # Width of header content
    
    # Icons (Unicode characters or emoji)
    icons:
      start: "▶"                         # Task start indicator
      success: "✓"                       # Success indicator
      warning: "⚠"                       # Warning indicator
      error: "✗"                         # Error indicator
      info: "ℹ"                          # Info indicator
      bullet: "•"                        # Bullet point
    
    # Colors (256-color numbers or ANSI escape sequences)
    colors:
      process: "117"                     # Process/task color (light blue)
      success: "120"                     # Success color (green)
      warning: "214"                     # Warning color (orange)
      error: "196"                       # Error color (red)
      detail: ""                         # Detail text color (reset)
      muted: "242"                       # Muted/dim color (gray)
      reset: ""                          # Reset color
      spinner: "111"                     # Spinner animation color (pale blue)
      white: "231"                       # White color
      blue_fg: "117"                     # Blue foreground
      green_fg: "120"                    # Green foreground
      blue_bg: "4"                       # Blue background
      bold: ""                           # Bold style
      italic: ""                         # Italic style
    
    # Border characters
    border:
      task_style: header_box             # none | left | left_double | header_box
      header_char: "─"                    # Horizontal line character
      vertical_char: "│"                  # Vertical line character
      top_corner_char: "╭"                # Top-left corner
      top_right_char: "╮"                # Top-right corner
      bottom_corner_char: "╰"            # Bottom-left corner
      bottom_right_char: "╛"             # Bottom-right corner
      footer_continuation_char: "─"      # Footer continuation character
      # Table border characters (optional)
      table_h_char: "─"                   # Horizontal line for tables
      table_v_char: "│"                   # Vertical line for tables
      table_x_char: "┼"                  # Cross/intersection for tables
      table_corner_tl: "┌"                # Top-left table corner
      table_corner_tr: "┐"                # Top-right table corner
      table_corner_bl: "└"                # Bottom-left table corner
      table_corner_br: "┘"                # Bottom-right table corner
      table_t_down: "┬"                   # T-junction pointing down
      table_t_up: "┴"                     # T-junction pointing up
      table_t_left: "┤"                   # T-junction pointing left
      table_t_right: "├"                  # T-junction pointing right
    
    # Element-specific styling (optional, overrides defaults)
    elements:
      H1_Major_Header:
        text_case: upper
        text_style: ["bold"]
        color_fg: BlueFg
      
      Task_Label_Header:
        text_case: upper
        text_style: ["bold"]
        color_fg: Process
      
      Task_StartIndicator_Line:
        icon_key: Start
        color_fg: Process
      
      Task_Status_Success_Block:
        icon_key: Success
        text_content: "Complete"
        color_fg: Success
      
      Task_Status_Failed_Block:
        icon_key: Error
        text_content: "Failed"
        color_fg: Error
      
      Task_Status_Warning_Block:
        icon_key: Warning
        text_content: "Completed with warnings"
        color_fg: Warning
      
      Task_Status_Duration:
        prefix: "("
        suffix: ")"
        color_fg: Muted
      
      Task_Content_Summary_Heading:
        text_content: "SUMMARY:"
        text_style: ["bold"]
        color_fg: Process
      
      Task_Content_Summary_Item_Error:
        bullet_char: "•"
        color_fg: Error
      
      Task_Content_Summary_Item_Warning:
        bullet_char: "•"
        color_fg: Warning
      
      Stderr_Error_Line_Prefix:
        icon_key: Error
        additional_chars: "  "
        color_fg: Error
      
      Stderr_Warning_Line_Prefix:
        icon_key: Warning
        additional_chars: "  "
        color_fg: Warning
      
      Stdout_Line_Prefix:
        additional_chars: "  "
      
      Table_Header_Cell_Text:
        text_style: ["bold"]
        color_fg: Process
      
      Print_Header_Highlight:
        text_case: none
        text_style: ["bold"]
        color_fg: White
        color_bg: BlueBg
      
      Print_Success_Style:
        color_fg: Success
      
      # Additional element fields available:
      # - text: Custom text content
      # - prefix: Text before content
      # - suffix: Text after content
      # - text_content: Alternative text content
      # - text_case: none | upper | lower
      # - text_style: ["bold", "italic"]
      # - color_fg: Color key name (Process, Success, Error, etc.)
      # - color_bg: Background color key name
      # - icon_key: Icon key name (Start, Success, Warning, Error, Info)
      # - bullet_char: Bullet character
      # - line_char: Line character
      # - line_length_type: Line length calculation type
      # - framing_char_start: Start framing character
      # - framing_char_end: End framing character
      # - additional_chars: Additional characters to prepend
      # - date_time_format: DateTime format string
    
    # Pattern recognition (optional, advanced)
    patterns:
      intent:
        build: ["building", "compiling", "linking"]
        test: ["testing", "running tests"]
      output:
        error: ["error:", "failed", "fatal"]
        warning: ["warning:", "warn"]
    
    # Tool-specific configurations (optional, advanced)
    tools:
      go_build:
        label: "Go Build"
        intent: "build"
        output_patterns:
          error: ["#", "cannot", "undefined"]
    
    # Cognitive load settings (optional)
    cognitive_load:
      auto_detect: true                   # Automatically detect cognitive load
      default: medium                      # low | medium | high
    
    # Complexity thresholds (optional)
    complexity_thresholds:
      very_high: 100                      # Output lines for very high complexity
      high: 50                            # Output lines for high complexity
      medium: 20                          # Output lines for medium complexity
      error_count_high: 5                 # Error count for high cognitive load
      warning_count_medium: 2             # Warning count for medium cognitive load
    
    # Test rendering settings (optional)
    tests:
      sparkbar_filled: "▮"                # Character for filled sparkbar
      sparkbar_empty: "▯"                 # Character for empty sparkbar
      sparkbar_length: 10                 # Length of sparkbar
      show_percentage: false               # Show percentage in test output
      no_test_icon: "○"                   # Icon when no tests found
      no_test_color: "Warning"             # Color key for no tests
      coverage_good_min: 70.0             # Minimum coverage for "good" (green)
      coverage_warning_min: 40.0          # Minimum coverage for "warning" (yellow)
```

## Spinner Styles Reference

Available built-in spinner styles:

| Style | Characters | Description |
|-------|------------|-------------|
| `dots` | ⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ | Claude-style braille dots (default) |
| `line` | -\|/ | Simple ASCII spinner |
| `arc` | ◜◠◝◞◡◟ | Smooth arc animation |
| `star` | ✶✸✹✺✹✸ | Twinkling star |
| `bounce` | ⠁⠂⠄⠂ | Bouncing dot |
| `grow` | ▁▃▄▅▆▇█▇▆▅▄▃ | Growing/shrinking bar |
| `arrows` | ←↖↑↗→↘↓↙ | Rotating arrow |
| `clock` | 🕛🕐🕑🕒🕓🕔🕕🕖🕗🕘🕙🕚 | Clock face animation |
| `moon` | 🌑🌒🌓🌔🌕🌖🌗🌘 | Moon phases |
| `ascii` | -\|/ | ASCII-safe (same as line) |

### Custom Spinner Characters

You can define your own spinner characters using `spinner_chars`:

```yaml
style:
  # Space-separated characters
  spinner_chars: "⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷"

  # Or individual Unicode characters (no spaces)
  spinner_chars: "◐◓◑◒"
```

## ANSI Color Codes Reference

Common ANSI color codes you can use:

```yaml
colors:
  # Standard colors
  black: "\033[0;30m"
  red: "\033[0;31m"
  green: "\033[0;32m"
  yellow: "\033[0;33m"
  blue: "\033[0;34m"
  magenta: "\033[0;35m"
  cyan: "\033[0;36m"
  white: "\033[0;37m"
  
  # Bright colors
  bright_black: "\033[0;90m"
  bright_red: "\033[0;91m"
  bright_green: "\033[0;92m"
  bright_yellow: "\033[0;93m"
  bright_blue: "\033[0;94m"
  bright_magenta: "\033[0;95m"
  bright_cyan: "\033[0;96m"
  bright_white: "\033[0;97m"
  
  # 256-color palette (format: \033[38;5;NUMBERm)
  # Examples:
  light_blue: "\033[38;5;117m"    # Color 117
  light_green: "\033[38;5;120m"   # Color 120
  pale_blue: "\033[38;5;111m"     # Color 111
  
  # Styles
  bold: "\033[1m"
  dim: "\033[2m"
  italic: "\033[3m"
  underline: "\033[4m"
  reset: "\033[0m"
  
  # Background colors
  bg_blue: "\033[44m"
  bg_green: "\033[42m"
  bg_red: "\033[41m"
```

## Option 2: Separate Theme File

You can also create a separate theme file and reference it:

```yaml
# .fo.yaml
active_theme: my_theme
themes:
  my_theme:
    # ... theme definition ...
```

Or use the `--theme-file` flag when running `fo` commands (for CLI usage).

## Tips

1. **Start from an existing theme**: Copy `unicode_vibrant` or `ascii_minimal` theme and modify it
2. **Test incrementally**: Make small changes and test to see the effect
3. **Use color names**: You can reference color keys like `BlueFg`, `Success`, `Error` in element styles
4. **Check terminal support**: Some Unicode characters may not render in all terminals
5. **ANSI codes**: Always use `\033` for the ESC character in YAML strings

## Example: Minimal Dark Theme

```yaml
active_theme: dark_minimal

themes:
  dark_minimal:
    style:
      use_boxes: false
      indentation: "  "
      density: compact
      header_width: 60
    
    icons:
      start: "▶"
      success: "✓"
      warning: "⚠"
      error: "✗"
      info: "ℹ"
      bullet: "•"
    
    colors:
      process: "\033[0;36m"      # Cyan
      success: "\033[0;32m"      # Green
      warning: "\033[0;33m"      # Yellow
      error: "\033[0;31m"        # Red
      detail: "\033[0m"
      muted: "\033[2m"
      reset: "\033[0m"
      bold: "\033[1m"
    
    border:
      task_style: none
```

## Complete Field Reference

Here's a complete list of all available theme fields:

### `style` (11 fields)
- `use_boxes` (bool)
- `indentation` (string)
- `show_timestamps` (bool)
- `density` (string: "compact" | "balanced" | "detailed")
- `no_timer` (bool)
- `use_inline_progress` (bool)
- `no_spinner` (bool)
- `spinner_style` (string: "dots" | "line" | "arc" | "star" | "bounce" | "grow" | "arrows" | "clock" | "moon" | "ascii")
- `spinner_chars` (string: custom spinner characters, space-separated or individual chars)
- `spinner_interval` (int, milliseconds)
- `header_width` (int)

### `icons` (6 fields)
- `start` (string)
- `success` (string)
- `warning` (string)
- `error` (string)
- `info` (string)
- `bullet` (string)

### `colors` (14 fields)
- `process` (string, ANSI code or 256-color number)
- `success` (string, ANSI code or 256-color number)
- `warning` (string, ANSI code or 256-color number)
- `error` (string, ANSI code or 256-color number)
- `detail` (string, ANSI code or 256-color number)
- `muted` (string, ANSI code or 256-color number)
- `reset` (string, ANSI code)
- `white` (string, optional)
- `green_fg` (string, optional)
- `blue_fg` (string, optional)
- `blue_bg` (string, optional)
- `spinner` (string, optional) - spinner animation color (default: "111" pale blue)
- `bold` (string, optional)
- `italic` (string, optional)
- `pale_blue` (string, optional)

### `border` (19 fields)
- `task_style` (string: "none" | "left" | "left_double" | "header_box")
- `header_char` (string)
- `vertical_char` (string)
- `top_corner_char` (string)
- `top_right_char` (string)
- `bottom_corner_char` (string)
- `bottom_right_char` (string)
- `footer_continuation_char` (string)
- `table_h_char` (string, optional)
- `table_v_char` (string, optional)
- `table_x_char` (string, optional)
- `table_corner_tl` (string, optional)
- `table_corner_tr` (string, optional)
- `table_corner_bl` (string, optional)
- `table_corner_br` (string, optional)
- `table_t_down` (string, optional)
- `table_t_up` (string, optional)
- `table_t_left` (string, optional)
- `table_t_right` (string, optional)

### `elements` (map of ElementStyleDef)
Each element can have:
- `text` (string)
- `prefix` (string)
- `suffix` (string)
- `text_content` (string)
- `text_case` (string: "none" | "upper" | "lower")
- `text_style` (array of strings: "bold", "italic")
- `color_fg` (string, color key name)
- `color_bg` (string, color key name)
- `icon_key` (string: "Start" | "Success" | "Warning" | "Error" | "Info")
- `bullet_char` (string)
- `line_char` (string)
- `line_length_type` (string)
- `framing_char_start` (string)
- `framing_char_end` (string)
- `additional_chars` (string)
- `date_time_format` (string)

### `patterns` (optional)
- `intent` (map[string][]string)
- `output` (map[string][]string)

### `tools` (optional, map of ToolConfig)
Each tool config has:
- `label` (string)
- `intent` (string)
- `output_patterns` (map[string][]string)

### `cognitive_load` (2 fields)
- `auto_detect` (bool)
- `default` (string: "low" | "medium" | "high")

### `complexity_thresholds` (5 fields)
- `very_high` (int)
- `high` (int)
- `medium` (int)
- `error_count_high` (int)
- `warning_count_medium` (int)

### `tests` (8 fields)
- `sparkbar_filled` (string)
- `sparkbar_empty` (string)
- `sparkbar_length` (int)
- `show_percentage` (bool)
- `no_test_icon` (string)
- `no_test_color` (string, color key name)
- `coverage_good_min` (float64)
- `coverage_warning_min` (float64)

## See Also

- Built-in themes: `unicode_vibrant`, `ascii_minimal`, `orca`
- Source code: `pkg/design/config.go`
- ADR: `docs/adr/ADR-002-color-theme-extensibility.md`

