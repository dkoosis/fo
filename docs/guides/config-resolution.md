# Configuration Resolution Guide

This document explains how fo resolves configuration from multiple sources with explicit priority order.

## Priority Order

Configuration is resolved in the following priority order (highest to lowest):

1. **CLI Flags** (`--theme-file`, `--theme`, `--no-color`, `--ci`, `--no-timer`)
2. **Environment Variables** (`FO_THEME`, `FO_NO_COLOR`, `FO_CI`, `CI`, `NO_COLOR`)
3. **`.fo.yaml` Configuration File** (`active_theme`, `no_color`, `ci`, `no_timer`)
4. **Design Package Defaults** (`UnicodeVibrantTheme`, `ASCIIMinimalTheme`)

## Resolution Process

The `ResolveConfig()` function in `internal/config/resolution.go` is the **single source of truth** for configuration resolution.

### Theme Resolution

Theme is resolved in this order:

1. `--theme-file` flag → Load from file
2. `--theme` flag → Use built-in or file-based theme
3. `FO_THEME` environment variable → Use built-in or file-based theme
4. `.fo.yaml` `active_theme` → Use file-based theme
5. Default → `UnicodeVibrantTheme()`

### Behavioral Settings

Settings like `no_color`, `ci`, `no_timer` follow the same priority:

1. CLI flag (if set)
2. Environment variable (if not set by CLI)
3. `.fo.yaml` file (if not set by CLI/env)
4. Default value

### CI Mode Overrides

When `--ci` flag or `CI=true` environment variable is set:
- `no_color` is automatically set to `true`
- `no_timer` is automatically set to `true`
- Theme is set to monochrome
- Boxes are disabled

## Examples

### Example 1: CLI Overrides File

```yaml
# .fo.yaml
active_theme: ascii_minimal
no_color: false
```

```bash
fo --theme unicode_vibrant --no-color -- go build
```

Result: Uses `unicode_vibrant` theme and `no_color=true` (CLI overrides file)

### Example 2: Environment Overrides File

```yaml
# .fo.yaml
active_theme: ascii_minimal
```

```bash
export FO_THEME=unicode_vibrant
fo -- go build
```

Result: Uses `unicode_vibrant` theme (env overrides file)

### Example 3: File Overrides Default

```yaml
# .fo.yaml
active_theme: ascii_minimal
```

```bash
fo -- go build
```

Result: Uses `ascii_minimal` theme (file overrides default)

## Validation

The resolved configuration is validated for:
- Theme is not nil
- `show_output` is one of: `on-fail`, `always`, `never`
- `max_buffer_size` is positive
- `max_line_length` is positive

Invalid configurations return an error with a helpful message.

## Debugging

Set `FO_DEBUG=1` to see detailed resolution information:

```bash
FO_DEBUG=1 fo --theme ascii_minimal -- go build
```

This shows:
- Which config file was loaded
- Theme resolution source
- Final resolved values

## Configuration Resolution

The `ResolveConfig()` function provides:
- Explicit priority order
- Better error messages
- Validation
- Resolution metadata (for debugging)

## Related

- [Config Package Documentation](../internal/config/)
- [ADR-001: Pattern-Based Architecture](../adr/ADR-001-pattern-based-architecture.md)

