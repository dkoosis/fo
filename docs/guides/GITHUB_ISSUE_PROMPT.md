# Ideal GitHub Issue Prompt for fo Project

Use this prompt template when assigning a GitHub issue to an AI assistant (like Cursor AI) for the `fo` Go project:

---

## Copy-Paste Template

```
**Issue Type:** [Bug | Feature | Enhancement | Refactoring]

**Problem Description:**
[Clear, concise description of what needs to be fixed or implemented]

**Environment:**
- Go version: `go version`
- fo version: `fo --version`
- OS: [your OS]
- Terminal capabilities: [colors/ANSI support, CI environment, etc.]

**Steps to Reproduce / Context:**
```bash
# Exact commands that demonstrate the issue or feature need
fo [flags] -- <command>
```

**Expected vs Actual Behavior:**
- Expected: [what should happen]
- Actual: [what actually happens or what's missing]

**Relevant Files/Components:**
- Primary file(s): `path/to/file.go`
- Related functions: `FunctionName()` around line X
- Dependencies: [any related packages like `internal/design`, `mageconsole`, `internal/config`]

**Configuration (if applicable):**
```yaml
# .fo.yaml or relevant config
```

**Test Cases:**
- [ ] Should handle [scenario 1]
- [ ] Should handle [scenario 2]
- [ ] Edge case: [if any]

**Additional Context:**
[Any other relevant information, related issues, or constraints]
```

---

## Key Areas of the Codebase

When describing issues, reference these areas:

- **Entry point**: `cmd/main.go` - CLI argument parsing, command dispatch
- **Command execution**: `mageconsole/console.go` - Core command wrapping logic
- **Output rendering**: `internal/design/render.go` - Formatting and display logic
- **Themes/config**: `internal/design/config.go` - Theme definitions and styling
- **Recognition**: `internal/design/recognition.go` - Output parsing and classification
- **Configuration**: `internal/config/config.go` - Config file and flag handling
- **Subcommands**: `fo print` subcommand handling in `cmd/main.go`

---

## Common Issue Categories

### Bug Reports
- Output formatting issues (colors, icons, boxes)
- Command execution problems (streaming, capture mode)
- Configuration file parsing errors
- Theme/display inconsistencies
- Exit code propagation issues

### Feature Requests
- New themes or styling options
- Additional subcommands
- Enhanced output recognition patterns
- New configuration options
- CI/CD improvements

### Enhancements
- Performance optimizations
- Better error messages
- Test coverage improvements
- Documentation updates

---

## Example Issues

### Example 1: Bug Report

```
**Issue Type:** Bug

**Problem Description:**
When using `fo --ci -- go test ./...`, the output still contains ANSI color codes even though CI mode should disable colors.

**Environment:**
- Go version: go1.24.2
- fo version: fo version dev (Commit: abc123)
- OS: macOS 24.5.0
- Terminal: zsh, CI environment (GitHub Actions)

**Steps to Reproduce:**
```bash
fo --ci -- go test ./internal/design/...
```

**Expected vs Actual Behavior:**
- Expected: Plain text output with no ANSI codes
- Actual: Output contains `\033[` escape sequences

**Relevant Files/Components:**
- `internal/config/config.go` - CI flag handling
- `internal/design/config.go` - IsMonochrome logic
- `mageconsole/console.go` - Output generation

**Test Cases:**
- Should work with `--ci` flag
- Should work with `CI=true` env var (if supported)
- Should propagate to subcommands
```

### Example 2: Feature Request

```
**Issue Type:** Feature

**Problem Description:**
Add support for custom success/warning/error recognition patterns via `.fo.yaml` config file.

**Use Case:**
Teams using specialized tools (e.g., custom test runners) need `fo` to recognize their specific output patterns for proper status detection and formatting.

**Proposed Solution:**
Allow regex patterns in `.fo.yaml`:
```yaml
recognition:
  success:
    - "^PASS:"
    - "^✓ Test"
  error:
    - "^FAIL:"
    - "^✗ Test"
```

**Relevant Files/Components:**
- `internal/design/recognition.go` - Pattern matching logic
- `internal/config/config.go` - Config file schema
- `internal/design/config.go` - Recognition pattern definitions

**Test Cases:**
- Should match custom patterns from config
- Should fall back to defaults if config missing
- Should handle invalid regex gracefully
```

---

## Tips for Effective Issue Descriptions

1. **Be specific**: Include exact commands, file paths, and line numbers when possible
2. **Provide context**: Explain why this matters or how it affects users
3. **Include test expectations**: What should the fix verify?
4. **Reference related code**: Point to specific functions or design patterns
5. **Consider edge cases**: Mention special scenarios or constraints
6. **Check existing code**: Look at similar features (themes, modes) for consistency

---

## Quick Command Reference

When testing issues, these commands are helpful:

```bash
# Build and test
mage build
go test ./...

# Run QA checks
mage qa

# Run with debug
fo --debug -- go build ./...

# Test specific scenarios
fo --stream -- go test -v ./internal/design/...
fo --ci -- go vet ./...
fo --theme ascii_minimal -- go fmt ./...

# Check version and config
fo --version
cat .fo.yaml  # if exists
```
