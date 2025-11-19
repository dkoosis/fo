# fo

A command-line utility for standardizing command output with clean formatting, timing, and error reporting.

## Overview

`fo` (Format Output) is a wrapper for executing other commands, providing standardized and visually well-designed output. It's particularly useful in Makefiles and scripts to create consistent, easy-to-scan output with clear success/failure indicators.

## Features

- Wraps command execution with visually distinct start/end status indicators
- Two operation modes:
  - **CAPTURE mode** (default): Buffers command output, shows it only on failure
  - **STREAM mode**: Shows command output in real-time for interactive commands
- Automatic timing of command execution
- Colorized output with emoji indicators (with plain text fallback)
- CI-friendly mode for environments without color support

## Installation

```
go install github.com/davidkoosis/fo@latest
```

## Usage

```
fo [flags] -- <COMMAND> [ARGS...]
```

### Flags

- `-l, --label <string>`: Use a specific label for the task (default: inferred from command name)
- `-s, --stream`: STREAM MODE - print command's stdout/stderr live
- `--show-output <mode>`: Specify when to show captured output (CAPTURE MODE only)
  - `on-fail` (Default): Show only if wrapped command exits non-zero
  - `always`: Show captured output regardless of exit code
  - `never`: Never show captured output, only fo's status line
- `--no-timer`: Disable showing the duration
- `--no-color`: Disable ANSI color/styling output
- `--ci`: Enable CI-friendly, plain-text output (implies --no-color, --no-timer)

## Examples

### Basic usage

```bash
# Run a command with default settings (CAPTURE mode)
fo -- go build ./cmd/myapp

# Use a custom label
fo -l "Building application" -- go build ./cmd/myapp

# Run in STREAM mode (for interactive or verbose commands)
fo -s -- go test -v ./...

# Always show command output even on success
fo --show-output always -- go vet ./...
```

### In Makefiles

```makefile
.PHONY: build test lint

build:
	@fo -l "Building binary" -- env CGO_ENABLED=0 go build -o myapp ./cmd/myapp

test:
	@fo -l "Running tests" -s -- gotestsum --format short -- ./...

lint:
	@fo -l "Running linter" -- golangci-lint run ./...
```

## Programmatic Usage

The `fo` CLI is built on top of the `mageconsole` library. If you want to use the same formatting capabilities programmatically in your Go code (e.g., in Magefiles or build scripts), see the [mageconsole documentation](mageconsole/README.md).

```go
import "github.com/davidkoosis/fo/mageconsole"

console := mageconsole.DefaultConsole()
_, err := console.Run("My Task", "go", "build", "./...")
```

The `mageconsole` package provides all the same features as the CLI in a type-safe Go API.

## License

MIT License