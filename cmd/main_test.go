package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/davidkoosis/fo/cmd/internal/design"
	// No direct import of design needed if we rely on fo's output for asserts
)

// Updated plain text icon constants to match GetIcon's monochrome output
const (
	plainIconStart   = "[START]"
	plainIconSuccess = "[SUCCESS]"
	plainIconWarning = "[WARNING]" // Added for completeness if tests use it
	plainIconFailure = "[FAILED]"
)

var foTestBinaryPath = "./fo_test_binary_generated_by_TestMain" // Relative path for test execution

// TestMain sets up and tears down the test binary.
func TestMain(m *testing.M) {
	// Build the 'fo' binary specifically for testing.
	// Using "." assumes the test is run from the 'cmd' directory.
	cmd := exec.Command("go", "build", "-o", foTestBinaryPath, ".")
	buildOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build test binary '%s': %v\nOutput:\n%s\n", foTestBinaryPath, err, string(buildOutput))
		os.Exit(1)
	}

	// Run the tests
	exitCode := m.Run()

	// Clean up the test binary
	if err := os.Remove(foTestBinaryPath); err != nil {
		// Non-fatal, just log a warning if cleanup fails.
		fmt.Fprintf(os.Stderr, "Warning: Failed to remove test binary '%s': %v\n", foTestBinaryPath, err)
	}
	os.Exit(exitCode)
}

type foResult struct {
	stdout   string
	stderr   string
	exitCode int
	runError error // Error from cmd.Run() itself, if fo process failed to execute/complete
}

// runFo executes the compiled 'fo' test binary with given arguments and returns its output and exit code.
func runFo(t *testing.T, foCmdArgs ...string) foResult {
	t.Helper()
	cmd := exec.Command(foTestBinaryPath, foCmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run() // This blocks until the command exits.
	duration := time.Since(startTime)

	// Determine the exit code of the 'fo' process
	exitCode := 0
	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			// The command ran and exited with a non-zero status.
			exitCode = exitError.ExitCode()
		} else {
			// Other errors (e.g., binary not found, permissions issue for fo itself)
			// fo should still try to os.Exit() with a code if it handles this.
			// If fo itself couldn't start, ProcessState might be nil.
			// Let's default to 1 for generic errors if not an ExitError.
			exitCode = 1
		}
	}
	// If ProcessState is available (command completed, even if with error), it's the most reliable.
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	// Log the execution details for easier debugging of tests.
	logMessage := fmt.Sprintf("Executed: %s %s (took %v)\n"+
		"--- FO STDOUT ---\n%s\n"+
		"--- FO STDERR ---\n%s\n"+
		"--- FO EXITCODE (from fo process): %d ---",
		foTestBinaryPath, strings.Join(foCmdArgs, " "), duration,
		stdout.String(), stderr.String(), exitCode)
	t.Log(logMessage)

	return foResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode, // This is fo's actual exit code as seen by the OS
		runError: runErr,   // This is the error from Go's exec.Command.Run()
	}
}

// setupTestScripts creates dummy shell scripts in a 'testdata' directory for tests to use.
func setupTestScripts(t *testing.T) {
	t.Helper()
	scriptsDir := "testdata"
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		if err := os.Mkdir(scriptsDir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", scriptsDir, err)
		}
	}
	// Define scripts as a map for easy iteration
	scripts := map[string]string{
		"success.sh": `#!/bin/sh
echo "STDOUT: Normal output from success.sh"
echo "STDERR: Info output from success.sh" >&2
exit 0`,
		"failure.sh": `#!/bin/sh
echo "STDOUT: Output from failure.sh before failing"
echo "STDERR: Error message from failure.sh" >&2
exit 1`,
		"exit_code.sh": `#!/bin/sh
echo "STDOUT: Script about to exit with $1"
echo "STDERR: Script stderr message before exiting $1" >&2
exit "$1"`,
		"long_running.sh": `#!/bin/sh # For timer tests
echo "STDOUT: Starting long task..."
sleep 0.1 # Short sleep for tests
echo "STDOUT: Long task finished."
exit 0`,
		"only_stdout.sh": `#!/bin/sh
echo "ONLY_STDOUT_CONTENT"
exit 0`,
		"only_stderr.sh": `#!/bin/sh
echo "ONLY_STDERR_CONTENT" >&2
exit 1`,
		"interleaved.sh": `#!/bin/sh # For output order tests
echo "STDOUT: Message 1" 
echo "STDERR: Message 1" >&2
echo "STDOUT: Message 2"
echo "STDERR: Message 2" >&2
exit 0`,
		"large_output.sh": `#!/bin/sh # For buffer limit tests (reduced for speed)
for i in $(seq 1 100); do
  printf "STDOUT: Line %04d - This is test content to generate output.\n" $i
done
echo "Script complete"
exit 0`,
		"signal_test.sh": `#!/bin/sh # For signal propagation tests
echo "Starting signal test script (PID: $$)"
echo "Will sleep for 5 seconds unless interrupted" # Shorter for tests
trap 'echo "Caught signal, exiting cleanly"; exit 42' INT TERM
sleep 5
echo "Finished sleeping"
exit 0`,
	}
	for name, content := range scripts {
		path := filepath.Join(scriptsDir, name)
		// Ensure scripts are executable
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", name, err)
		}
	}
}

// buildPattern creates a regex to match fo's start/end lines.
// expectTimer for end lines controls if the duration part is included in the regex.
func buildPattern(iconString string, label string, isStartLine bool, expectTimer bool) *regexp.Regexp {
	// QuoteMeta escapes regex special characters in iconString and label
	pattern := regexp.QuoteMeta(iconString) + `\s*` + regexp.QuoteMeta(label)
	if isStartLine {
		pattern += `\.\.\.` // Start lines end with "..."
	} else if expectTimer { // For end lines where a timer is expected
		// Matches (optional_whitespace)(duration_format)(optional_whitespace)
		// Duration format example: (123ms), (1.2s), (1:02.345s)
		pattern += `\s*\([\wµ\.:]+\)$` // `\w` includes numbers, `\.` for decimal, `:` for M:SS
	} else { // For end lines where NO timer is expected (e.g., --no-timer or --ci)
		pattern += `$` // Must be the end of the string/line
	}
	return regexp.MustCompile(pattern)
}

// TestFoCoreExecution tests basic command execution, exit codes, and argument passing.
func TestFoCoreExecution(t *testing.T) {
	setupTestScripts(t)
	t.Run("ExitCodePassthroughSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
		}
	})
	t.Run("ExitCodePassthroughFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh")
		if res.exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", res.exitCode)
		}
	})
	t.Run("ExitCodePassthroughSpecific", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/exit_code.sh", "42")
		if res.exitCode != 42 {
			t.Errorf("Expected fo process to exit with code 42, got %d.", res.exitCode)
		}
		if !strings.Contains(res.stdout, "--- Captured output: ---") { // Default on-fail shows output
			t.Error("Expected '--- Captured output: ---' header for a failing script with non-zero exit code.")
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		commandName := "a_very_unique_non_existent_command_askjdfh"
		res := runFo(t, "--no-color", "--", commandName) // Use --no-color for predictable plain icons
		if res.exitCode != 127 {                         // Expect 127 for command not found (from getExitCode)
			t.Errorf("Expected 'fo' to exit with 127 for command not found, got %d. Stderr:\n%s", res.exitCode, res.stderr)
		}

		// Check fo's stdout for start and end lines
		startPattern := buildPattern(plainIconStart, commandName, true, false)
		if !startPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain start line /%s/, got:\n%s", startPattern.String(), res.stdout)
		}
		// For command not found, timer should still be displayed by default (unless --no-timer or --ci)
		endPattern := buildPattern(plainIconFailure, commandName, false, true) // true for expectTimer
		if !endPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain end line /%s/, got:\n%s", endPattern.String(), res.stdout)
		}

		// Check fo's stderr for the direct error message
		expectedStderrPrefix := "Error starting command '" + commandName + "': exec: \"" + commandName + "\":"
		if !strings.HasPrefix(strings.TrimSpace(res.stderr), expectedStderrPrefix) {
			t.Errorf("Expected 'fo' stderr to start with '%s', got:\n%s", expectedStderrPrefix, res.stderr)
		}
		// Ensure "--- Captured output: ---" is NOT in stdout for fo's own startup error
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Did NOT expect '--- Captured output: ---' in stdout for command not found error, got:\n%s", res.stdout)
		}
	})

	t.Run("ArgumentsToWrappedCommand", func(t *testing.T) {
		t.Parallel()
		helperScriptContent := `#!/bin/sh
echo "Args: $1 $2"`
		scriptPath := filepath.Join(t.TempDir(), "args_test.sh")
		if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0755); err != nil {
			t.Fatalf("Failed to write script: %v", err)
		}
		res := runFo(t, "--show-output", "always", "--no-color", "--", scriptPath, "hello", "world")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
		}
		// Captured output should contain the script's output
		if !strings.Contains(res.stdout, "Args: hello world") {
			t.Errorf("Expected 'Args: hello world' in fo's output, got: %s", res.stdout)
		}
	})
}

func TestFoLabels(t *testing.T) {
	setupTestScripts(t)
	scriptPath := "testdata/success.sh"
	commonTest := func(args []string, label string) {
		res := runFo(t, args...)
		start := buildPattern(plainIconStart, label, true, false)
		end := buildPattern(plainIconSuccess, label, false, true) // Expect timer with --no-color
		if !start.MatchString(res.stdout) {
			t.Errorf("Args: %v. Missing start /%s/ in:\n%s", args, start, res.stdout)
		}
		if !end.MatchString(res.stdout) {
			t.Errorf("Args: %v. Missing end /%s/ in:\n%s", args, end, res.stdout)
		}
	}
	t.Run("DefaultLabelInferenceNoColor", func(t *testing.T) {
		t.Parallel()
		commonTest([]string{"--no-color", "--", scriptPath}, scriptPath)
	})
	t.Run("CustomLabelShortFlagNoColor", func(t *testing.T) {
		t.Parallel()
		label := "My Custom Task"
		commonTest([]string{"--no-color", "-l", label, "--", scriptPath}, label)
	})
	t.Run("CustomLabelLongFlagNoColor", func(t *testing.T) {
		t.Parallel()
		label := "Another Task"
		commonTest([]string{"--no-color", "--label", label, "--", scriptPath}, label)
	})
}

func TestFoCaptureMode(t *testing.T) {
	setupTestScripts(t)
	tests := []struct {
		name                                 string
		foArgs                               []string
		script                               string
		expectExit                           int
		expectHeader, negateOut, negateErr   bool
		expectOutContains, expectErrContains string
	}{
		{"DefaultSuccess", []string{"--no-color"}, "testdata/success.sh", 0, false, true, true, "STDOUT: Normal", "STDERR: Info"},
		{"on-fail Failure", []string{"--no-color", "--show-output", "on-fail"}, "testdata/failure.sh", 1, true, false, false, "STDOUT: Output from failure", "STDERR: Error message"},
		{"always Success", []string{"--no-color", "--show-output", "always"}, "testdata/success.sh", 0, true, false, false, "STDOUT: Normal", "STDERR: Info"},
		{"never Failure", []string{"--no-color", "--show-output", "never"}, "testdata/failure.sh", 1, false, true, true, "STDOUT: Output from failure", "STDERR: Error message"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := append(tt.foArgs, "--", tt.script)
			res := runFo(t, args...)
			if res.exitCode != tt.expectExit {
				t.Errorf("Exit: got %d, want %d", res.exitCode, tt.expectExit)
			}
			hasHeader := strings.Contains(res.stdout, "--- Captured output: ---")
			if tt.expectHeader != hasHeader {
				t.Errorf("'Captured output' header: got %t, want %t:\n%s", hasHeader, tt.expectHeader, res.stdout)
			}

			targetSection := res.stdout
			if hasHeader {
				parts := strings.SplitN(res.stdout, "--- Captured output: ---", 2)
				if len(parts) == 2 {
					targetSection = parts[1]
				}
			}

			outPresent := strings.Contains(targetSection, tt.expectOutContains)
			if tt.negateOut == outPresent {
				t.Errorf("Script stdout '%s' presence: got %t, want %t in section:\n%s", tt.expectOutContains, outPresent, !tt.negateOut, targetSection)
			}
			errPresent := strings.Contains(targetSection, tt.expectErrContains) // Script's stderr merged into fo's stdout display
			if tt.negateErr == errPresent {
				t.Errorf("Script stderr '%s' presence: got %t, want %t in section:\n%s", tt.expectErrContains, errPresent, !tt.negateErr, targetSection)
			}
		})
	}
}

func TestFoStreamMode(t *testing.T) {
	setupTestScripts(t)
	t.Run("StreamSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--no-color", "--", "testdata/success.sh") // Added --no-color for consistency
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Error("Expected script's stdout in fo's stdout")
		}
		// In stream mode, script's stderr should go to fo's stderr
		if !strings.Contains(res.stderr, "STDERR: Info output from success.sh") {
			t.Errorf("Expected script's stderr in fo's stderr. Fo Stderr:\n%s", res.stderr)
		}
	})
	t.Run("StreamOverridesShowOutput", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--show-output", "never", "--no-color", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Error("Expected script's stdout in fo's stdout")
		}
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Did NOT expect 'Captured output' header in stream mode")
		}
	})
}

func TestFoTimer(t *testing.T) {
	setupTestScripts(t)
	// Regex for timer: (any_duration_format) e.g., (123ms), (1.2s), (1m02.345s), (12µs)
	timerRegex := regexp.MustCompile(`\s*\([\d\.:µms]+\)$`)

	t.Run("TimerShownByDefaultNoColor", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/long_running.sh")
		// End line: [SUCCESS] testdata/long_running.sh (duration)
		endLineWithTimerPattern := buildPattern(plainIconSuccess, "testdata/long_running.sh", false, true)
		if !endLineWithTimerPattern.MatchString(res.stdout) {
			t.Errorf("Expected timer in output matching /%s/, but not found. Output:\n%s", endLineWithTimerPattern.String(), res.stdout)
		}
	})

	t.Run("TimerHiddenWithNoTimerFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-timer", "--no-color", "--", "testdata/success.sh")
		// End line: [SUCCESS] testdata/success.sh (no timer part)
		endLineNoTimerPattern := buildPattern(plainIconSuccess, "testdata/success.sh", false, false) // false for expectTimer

		// Check the last line specifically
		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		actualEndLine := ""
		if len(lines) > 0 {
			actualEndLine = lines[len(lines)-1]
		}

		if !endLineNoTimerPattern.MatchString(actualEndLine) {
			t.Errorf("Expected end line /%s/ (no timer), got '%s'. Full stdout:\n%s", endLineNoTimerPattern.String(), actualEndLine, res.stdout)
		}
		// Double check entire output for any timer pattern just in case
		if timerRegex.MatchString(res.stdout) {
			t.Errorf("Expected no timer in any part of output, but found one. Output:\n%s", res.stdout)
		}
	})
}

func TestFoColorAndIcons(t *testing.T) {
	setupTestScripts(t)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	// Use a fresh default config to get themed icons, as cliFlagsGlobal might be modified by other tests if not careful
	vibrantCfg := design.UnicodeVibrantTheme()
	emojiStart, emojiSuccess, emojiFail := vibrantCfg.Icons.Start, vibrantCfg.Icons.Success, vibrantCfg.Icons.Error

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh") // No --no-color
		if !ansiRegex.MatchString(res.stdout) {
			t.Error("Expected ANSI color codes, none found")
		}
		if !strings.Contains(res.stdout, emojiStart) {
			t.Errorf("Expected start icon '%s', not found", emojiStart)
		}
		if !strings.Contains(res.stdout, emojiSuccess) {
			t.Errorf("Expected success icon '%s', not found", emojiSuccess)
		}
	})
	t.Run("ColorAndIconsShownByDefaultFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh") // No --no-color
		if !ansiRegex.MatchString(res.stdout) {
			t.Error("Expected ANSI color codes, none found")
		}
		if !strings.Contains(res.stdout, emojiStart) {
			t.Errorf("Expected start icon '%s', not found", emojiStart)
		}
		if !strings.Contains(res.stdout, emojiFail) {
			t.Errorf("Expected failure icon '%s', not found", emojiFail)
		}
	})
	t.Run("ColorAndIconsHiddenWithNoColorFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/success.sh")
		if ansiRegex.MatchString(res.stdout) {
			t.Error("Unexpected ANSI colors with --no-color")
		}
		if strings.Contains(res.stdout, emojiStart) || strings.Contains(res.stdout, emojiSuccess) {
			t.Error("Unexpected emoji icons with --no-color")
		}
		if !strings.Contains(res.stdout, plainIconStart) {
			t.Errorf("Missing plain start icon '%s'", plainIconStart)
		}
		if !strings.Contains(res.stdout, plainIconSuccess) {
			t.Errorf("Missing plain success icon '%s'", plainIconSuccess)
		}
	})
}

func TestFoCIMode(t *testing.T) {
	setupTestScripts(t)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	timerRegex := regexp.MustCompile(`\s*\([\d\.:µms]+\)$`)
	tests := []struct {
		name   string
		script string
		exit   int
		icon   string
	}{
		{"CIModeSuccess", "testdata/success.sh", 0, plainIconSuccess},
		{"CIModeFailure", "testdata/failure.sh", 1, plainIconFailure},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := runFo(t, "--ci", "--", tt.script) // --ci implies --no-color and --no-timer
			if res.exitCode != tt.exit {
				t.Errorf("Exit: got %d, want %d", res.exitCode, tt.exit)
			}
			if ansiRegex.MatchString(res.stdout) {
				t.Error("Unexpected ANSI colors in CI mode")
			}

			lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
			actualEndLine := ""
			if len(lines) > 0 {
				// Find the line that starts with the expected end icon
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], tt.icon) {
						actualEndLine = lines[i]
						break
					}
				}
			}
			if timerRegex.MatchString(actualEndLine) {
				t.Errorf("Unexpected timer in CI mode end line '%s'. Full:\n%s", actualEndLine, res.stdout)
			}

			startPattern := buildPattern(plainIconStart, tt.script, true, false) // No timer for start line
			if !startPattern.MatchString(res.stdout) {
				t.Errorf("Missing start /%s/ in:\n%s", startPattern, res.stdout)
			}

			endPattern := buildPattern(tt.icon, tt.script, false, false) // No timer for end line in CI
			if !endPattern.MatchString(res.stdout) {
				t.Errorf("Missing end /%s/ in:\n%s", endPattern, res.stdout)
			}
		})
	}
}

func TestFoErrorHandling(t *testing.T) {
	t.Run("NoCommandAfterDashDash", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing error msg in stderr:\n%s", res.stderr)
		}
	})
	t.Run("NoCommandAtAll", func(t *testing.T) { // e.g., "fo -l some-label"
		t.Parallel()
		res := runFo(t, "-l", "some-label")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing error msg in stderr:\n%s", res.stderr)
		}
	})
	t.Run("InvalidShowOutputValue", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "bad-value", "--", "true")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit")
		}
		if !strings.Contains(res.stderr, "Error: Invalid value for --show-output: bad-value") {
			t.Errorf("Missing error msg in stderr:\n%s", res.stderr)
		}
	})
}

func TestEnvironmentInheritance(t *testing.T) {
	t.Parallel()
	scriptContent := `#!/bin/sh
echo "VAR is: $MY_TEST_VAR"`
	scriptPath := filepath.Join(t.TempDir(), "env.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Write script: %v", err)
	}
	key, val := "MY_TEST_VAR", "fo_env_test_val_unique"
	origVal, wasSet := os.LookupEnv(key)
	os.Setenv(key, val)
	defer func() {
		if wasSet {
			os.Setenv(key, origVal)
		} else {
			os.Unsetenv(key)
		}
	}()
	res := runFo(t, "--show-output", "always", "--no-color", "--", scriptPath)
	if res.exitCode != 0 {
		t.Errorf("Exit: got %d, want 0", res.exitCode)
	}
	expected := fmt.Sprintf("VAR is: %s", val)
	if !strings.Contains(res.stdout, expected) {
		t.Errorf("Expected '%s' in stdout:\n%s", expected, res.stdout)
	}
}

func TestFoTimestampedOutput(t *testing.T) {
	setupTestScripts(t)
	t.Run("PreservesOutputOrderInCaptureNoColor", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--show-output", "always", "--", "testdata/interleaved.sh")
		if res.exitCode != 0 {
			t.Errorf("Exit: got %d, want 0", res.exitCode)
		}

		capturedSection := ""
		parts := strings.SplitN(res.stdout, "--- Captured output: ---", 2)
		if len(parts) == 2 {
			capturedSection = strings.TrimSpace(parts[1])
		} else {
			t.Fatalf("Missing 'Captured output' section:\n%s", res.stdout)
		}

		lines := strings.Split(capturedSection, "\n")
		var actualOrder []string
		// For --no-color, RenderOutputLine prefixes with "  " or "  > "
		// We are looking for the core message.
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line) // e.g., "STDOUT: Message 1" or "> STDERR: Message 1"
			// Try to extract the core message part after the simple prefix
			if strings.HasPrefix(trimmedLine, "STDOUT: Message") {
				actualOrder = append(actualOrder, trimmedLine)
			}
			if strings.HasPrefix(trimmedLine, "> STDERR: Message") {
				actualOrder = append(actualOrder, strings.TrimSpace(strings.TrimPrefix(trimmedLine, ">")))
			}
		}

		expectedOrder := []string{"STDOUT: Message 1", "STDERR: Message 1", "STDOUT: Message 2", "STDERR: Message 2"}
		if len(actualOrder) != len(expectedOrder) {
			t.Errorf("Line count mismatch. Got %d, want %d.\nActual (parsed): %v\nExpected: %v\nCaptured Section:\n%s", len(actualOrder), len(expectedOrder), actualOrder, expectedOrder, capturedSection)
			return
		}
		// Verify that the elements are present and in the correct relative order for their streams
		var stdouts, stderrs []string
		for _, l := range actualOrder {
			if strings.HasPrefix(l, "STDOUT:") {
				stdouts = append(stdouts, l)
			} else if strings.HasPrefix(l, "STDERR:") {
				stderrs = append(stderrs, l)
			}
		}
		if !equalSlices(stdouts, []string{"STDOUT: Message 1", "STDOUT: Message 2"}) {
			t.Errorf("Stdout mismatch/order. Got %v", stdouts)
		}
		if !equalSlices(stderrs, []string{"STDERR: Message 1", "STDERR: Message 2"}) {
			t.Errorf("Stderr mismatch/order. Got %v", stderrs)
		}
	})
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Signal handling tests are complex to automate reliably across platforms without flakiness.
func TestFoSignalHandling(t *testing.T) {
	t.Skip("Skipping signal handling test; OS-dependent and hard to automate reliably.")
}

// Basic execution tests already covered in TestFoCoreExecution
func TestFoBasicExecution(t *testing.T) {
	t.Skip("Basic execution covered in TestFoCoreExecution and others.")
}
