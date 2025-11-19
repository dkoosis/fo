// Package config handles configuration loading and merging for fo.
//
// # Configuration Precedence
//
// Configuration values are resolved in the following order (highest to lowest priority):
//
//  1. CLI flags (--no-color, --ci, --stream, --theme, etc.)
//  2. Environment variables (FO_NO_COLOR, FO_CI, NO_COLOR, CI)
//  3. YAML config file (.fo.yaml in local directory or ~/.config/fo/.fo.yaml)
//  4. Hardcoded defaults
//
// When a higher-priority source sets a value, it overrides any lower-priority values.
//
// # Key Configuration Options
//
//   - NoColor: Disables all ANSI colors and uses ASCII-only output
//   - CI: Enables CI mode (implies NoColor, NoTimer, and disables interactive features)
//   - Stream: Shows output in real-time instead of capturing it
//   - Theme: Selects the visual theme (unicode_vibrant or ascii_minimal)
//
// # CI Mode Behavior
//
// When CI mode is enabled (via --ci flag, CI=true env var, or ci: true in YAML):
//   - Colors are disabled (monochrome output)
//   - Timer is disabled
//   - Spinner and interactive progress are disabled
//   - Output is optimized for log file readability
//
// # Environment Variables
//
// The following environment variables are recognized:
//
//   - FO_NO_COLOR or NO_COLOR: Set to "true" or "1" to disable colors
//   - FO_CI or CI: Set to "true" or "1" to enable CI mode
//   - FO_DEBUG: Set to any non-empty value to enable debug output
package config
