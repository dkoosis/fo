# Mage + fo Example

This directory demonstrates how to use the `fo` library in a Magefile for consistent, beautiful task output.

## Prerequisites

```bash
# Install mage
go install github.com/magefile/mage@latest

# Optional: Install linting tools for full QA suite
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

## Available Commands

### Build & Test
```bash
mage build           # Build the project
mage test            # Run tests
mage testCoverage    # Run tests with coverage
```

### Quality Assurance
```bash
mage qa              # Run comprehensive QA checks (fmt, vet, staticcheck, lint, security)
mage qaFast          # Run essential QA checks quickly (fmt, vet, lint)
mage fmt             # Check code formatting
mage vet             # Run go vet
mage lint            # Run golangci-lint (comprehensive)
mage lintFast        # Run golangci-lint (fast preset)
mage staticCheck     # Run staticcheck
mage security        # Run gosec security scanner
```

### Maintenance
```bash
mage tidy            # Run go mod tidy
mage verify          # Verify go.mod
mage clean           # Remove build artifacts and caches
```

### CI/CD
```bash
mage ci              # Run all CI checks (tidy, verify, qaFast, test, build)
```

## Example Output

When you run `mage qa`, you'll see beautifully formatted output like:

```
🔍 Running comprehensive QA checks...
[BUSY] Go Format Check [Working...]
[OK] Go Format Check [go, 12ms]
[BUSY] Go Vet [Working...]
[OK] Go Vet [go, 145ms]
[BUSY] Staticcheck [Working...]
[OK] Staticcheck [staticcheck, 1.2s]
✅ QA checks completed!
```

## Key Features

- **Consistent UX**: All commands use the same beautiful output format via `fo`
- **No STDIO Conflicts**: Unlike wrapping commands with `fo`, the library approach means no conflicts with make or other tools
- **Type-Safe**: Full Go type safety for your build tasks
- **Composable**: Easy to create complex workflows by composing simple tasks
- **CI-Friendly**: Automatically detects CI environments and adjusts output accordingly

## Integration Example

```go
//go:build mage

package main

import "github.com/dkoosis/fo/fo"

var console = fo.DefaultConsole()

func Build() error {
    _, err := console.Run("My Task", "go", "build", "./...")
    return err
}
```

That's it! The `fo` library handles all the formatting, timing, progress indicators, and output classification automatically.
