O: Future Self
FROM: Past Self
DATE: 2025-05-13
RE: Design Specification and Vision for fo (fmtout) Go CLI Utility

1. GOAL:
Create fo, a reusable, command-line utility written in Go, that acts as a wrapper for executing other commands. Its purpose is to standardize and centralize build/script output, making it visually well-designed, easy to scan, and informative, particularly for use in Makefiles.

2. CORE PHILOSOPHY & DEFAULTS:

Executor Model First: The primary use case is wrapping a command, executing it, and reporting status (fo -- command [args]). Pipe/Filter mode is a V2 consideration, if at all.
Sensible Defaults: Plain invocation should provide the most common, preferred behaviour.
Default Behaviour Shall Be:
Mode: CAPTURE (Streaming is OFF).
Output Display: Show the wrapped command's captured Stdout/Stderr only on-fail.
Timer: ON (Show duration).
Label: Infer from the command name (arg[0] after --) if -l flag is not used.
Color/Icons: ON, using a clean, clear theme.
Granularity: The same tool and mechanism will wrap simple (rm) and complex (golangci-lint, gcloud) commands. The defaults (--show-output on-fail, optional streaming) will ensure the appropriate level of detail is presented automatically for most cases.
Error Handling: fo should NOT pre-emptively check if a command exists. If os/exec returns an error (e.g., exec.ErrNotFound), fo will report this as a failure of the task, printing the OS-level error within the fo failure output. fo MUST exit with a non-zero status code if the wrapped command fails.
3. CLI INTERFACE & FLAGS (V1):

Usage: fo [flags] -- <COMMAND> [ARGS...]
Flags:
-l, --label <string>: Use a specific label for the task, overriding the inferred default.
-s, --stream: STREAM MODE. Print command's stdout/stderr live. Use for commands with their own rich/verbose/interactive output (gotestsum, gcloud, go test -v). Overrides --show-output.
--show-output <mode>: (CAPTURE MODE Only) Specify when to show captured output.
on-fail (Default): Show only if wrapped command exits non-zero.
always: Show captured output regardless of exit code.
never: Never show captured output, only fo's status line.
--no-timer: (Default: false) Flag to disable showing the duration.
--no-color: (Default: false) Disable ANSI color/styling output.
--ci: (Default: false) Enable CI-friendly, plain-text output (implies --no-color, --no-timer, uses simpler text prefixes). Should ideally also auto-detect from CI=true env var.
4. BEHAVIOUR - CAPTURE MODE (Default):

Print START line: ▶️ My Label...
Execute command, buffering stdout and stderr.
On command completion, get exit code and duration.
Print END status line, e.g., ✅ My Label (13.4s) or ❌ My Label (5.2s).
If command failed (or if --show-output always), print a header (e.g., --- Captured output: ---) followed by the captured, possibly indented, stdout/stderr.
5. BEHAVIOUR - STREAM MODE (-s):
* Print START line: ▶️ My Label...
* Execute command. Pipe the command's stdout/stderr directly to fo's stdout/stderr in real-time. (Future: Consider adding prefixing/indentation during stream).
* On command completion, get exit code and duration.
* Print END status line: ✅ My Label (5m10.1s) or ❌ My Label (4m55.3s).

6. GO IMPLEMENTATION NOTES:

Packages: Use standard flag or spf13/cobra (if more complexity expected later). Use os/exec, time, bytes, io, fmt, os. Use fatih/color or similar for ANSI codes.
Concurrency: Use goroutines with io.Copy or bufio.Scanner to consume Stdout and Stderr pipes concurrently in Capture mode to avoid subprocess deadlocks. Use a sync.WaitGroup.
Executor:
Use exec.CommandContext(...) to allow for future timeout/cancellation.
Carefully capture the error from Cmd.Wait() and check for *exec.ExitError to get the underlying exit code.
Environment: Ensure the wrapped command inherits the environment. Use env utility in Makefile or Cmd.Env in Go if specific vars needed.
Exit Codes: Ensure fo calls os.Exit(code) where code is 0 for success, or the wrapped command's non-zero exit code, or fo's own error code if fo itself has an issue (e.g. bad flag).
7. USE IN MAKEFILE:

Replace most recipe command invocations and printf formatting.
Selectively add -s or --show-output always as needed, per command.
Makefile retains responsibility for if command -v ... checks for OPTIONAL tools.
# Makefile Examples:
build:
	@fo -l "Building binary" -- env CGO_ENABLED=0 go build ...

lint:
	@fo -l "Running linter" -- golangci-lint run ./...

test:
	@fo -l "Running tests" -s -- gotestsum --format short -- ./...