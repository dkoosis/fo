# mageconsole

A Go library for running shell commands with formatted, colorful console output. Designed for use in build scripts, particularly with [Mage](https://magefile.org/).

**Note:** This library is the engine behind the [`fo`](../README.md) CLI. All features available in the `fo` command-line tool are also available programmatically via this library. See the [main README](../README.md) for CLI usage examples.

## Installation

```bash
go get github.com/davidkoosis/fo/mageconsole
```

## Quick Start

```go
package main

import (
    "github.com/davidkoosis/fo/mageconsole"
)

func main() {
    console := mageconsole.DefaultConsole()

    result, err := console.Run("Build", "go", "build", "./...")
    if err != nil {
        // Handle error
    }

    // Check exit code
    if result.ExitCode != 0 {
        // Command failed
    }
}
```

## API Reference

### Console

The main type for running commands.

#### NewConsole(config ConsoleConfig) *Console

Creates a new Console with the specified configuration.

```go
console := mageconsole.NewConsole(mageconsole.ConsoleConfig{
    Stream:         true,        // Stream output live
    ShowOutputMode: "on-fail",   // Show captured output: "always", "on-fail", "never"
    Monochrome:     false,       // Disable colors
})
```

#### DefaultConsole() *Console

Creates a Console with sensible defaults.

```go
console := mageconsole.DefaultConsole()
```

### Running Commands

#### Run(label, command string, args ...string) (*TaskResult, error)

Runs a command with a label and returns detailed results.

```go
result, err := console.Run("Go Test", "go", "test", "-v", "./...")
if err != nil {
    // err is non-nil if command not found or non-zero exit
}
fmt.Printf("Exit code: %d\n", result.ExitCode)
fmt.Printf("Duration: %v\n", result.Duration)
```

#### RunSimple(command string, args ...string) error

Simplified interface that returns only an error.

```go
if err := console.RunSimple("go", "build", "./..."); err != nil {
    return err
}
```

### TaskResult

Returned by `Run()` with command execution details.

```go
type TaskResult struct {
    Label    string        // The label you provided
    Status   string        // "success", "error", "warning"
    ExitCode int           // Command exit code
    Duration time.Duration // Execution time
    Lines    []Line        // Captured output lines (in capture mode)
}
```

### Error Handling

The library exports `ErrNonZeroExit` for checking exit code errors:

```go
result, err := console.Run("Test", "go", "test", "./...")
if errors.Is(err, mageconsole.ErrNonZeroExit) {
    fmt.Printf("Tests failed with exit code %d\n", result.ExitCode)
}
```

For command-not-found errors:

```go
result, err := console.Run("Missing", "nonexistent-command")
if errors.Is(err, exec.ErrNotFound) {
    fmt.Printf("Command not found (exit code %d)\n", result.ExitCode)
}
```

Note: `TaskResult` is always non-nil, even for infrastructure failures. This allows
you to access duration, label, and error details regardless of how the command failed.
Use `result.ExitCode` (127 for command not found) and `result.Err` for details.

For `RunSimple`, you can extract the exit code from errors:

```go
err := console.RunSimple("go", "test", "./...")
if errors.Is(err, mageconsole.ErrNonZeroExit) {
    var exitErr mageconsole.ExitCodeError
    if errors.As(err, &exitErr) {
        fmt.Printf("Exit code: %d\n", exitErr.Code)
    }
}
```

## Configuration Options

### ConsoleConfig Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Stream` | bool | false | Stream output live instead of capturing |
| `ShowOutputMode` | string | "on-fail" | When to show captured output: "always", "on-fail", "never" |
| `Monochrome` | bool | false | Disable ANSI colors |
| `ShowTimer` | bool | true | Show execution duration |
| `ThemeName` | string | "unicode_vibrant" | Visual theme name |
| `UseBoxes` | bool | true | Use box-drawing characters |
| `InlineProgress` | bool | false | Use inline progress instead of multi-line |
| `MaxBufferSize` | int64 | 10MB | Max buffer size for captured output |
| `MaxLineLength` | int | 1MB | Max length for a single output line |
| `Out` | io.Writer | os.Stdout | Output writer |
| `Err` | io.Writer | os.Stderr | Error writer |
| `Debug` | bool | false | Enable debug output |

### Example Configurations

**CI Mode (no colors, no timer):**

```go
console := mageconsole.NewConsole(mageconsole.ConsoleConfig{
    Monochrome: true,
    ShowTimer:  false,
})
```

**Streaming Mode:**

```go
console := mageconsole.NewConsole(mageconsole.ConsoleConfig{
    Stream: true,
})
```

**Custom Writers (for testing):**

```go
var stdout, stderr bytes.Buffer
console := mageconsole.NewConsole(mageconsole.ConsoleConfig{
    Out: &stdout,
    Err: &stderr,
})
```

## Usage Patterns

### Mage Build Script

```go
//go:build mage

package main

import "github.com/davidkoosis/fo/mageconsole"

var console = mageconsole.DefaultConsole()

func Build() error {
    _, err := console.Run("Go Build", "go", "build", "./...")
    return err
}

func Test() error {
    _, err := console.Run("Go Test", "go", "test", "./...")
    return err
}

func QA() error {
    if _, err := console.Run("Format", "go", "fmt", "./..."); err != nil {
        return err
    }
    if _, err := console.Run("Vet", "go", "vet", "./..."); err != nil {
        return err
    }
    return nil
}
```

### Error Aggregation

```go
func RunAll() error {
    var errors []error

    for _, target := range []string{"./cmd/...", "./internal/...", "./pkg/..."} {
        result, err := console.Run("Build "+target, "go", "build", target)
        if err != nil {
            errors = append(errors, fmt.Errorf("%s failed: %w", target, err))
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("build failed: %v", errors)
    }
    return nil
}
```

### Conditional Output

```go
func VerboseBuild(verbose bool) error {
    mode := "on-fail"
    if verbose {
        mode = "always"
    }

    console := mageconsole.NewConsole(mageconsole.ConsoleConfig{
        ShowOutputMode: mode,
    })

    _, err := console.Run("Build", "go", "build", "-v", "./...")
    return err
}
```

## Themes

Available themes:
- `unicode_vibrant` (default) - Colorful with Unicode characters
- `ascii_minimal` - Plain ASCII, suitable for limited terminals

Custom themes can be defined in `.fo.yaml` configuration files.

## License

See the main project license.
