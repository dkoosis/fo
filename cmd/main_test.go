package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/davidkoosis/fo/cmd/internal/design"
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

// foResult holds the output and exit status of an 'fo' command execution.
type foResult struct {
	stdout   string
	stderr   string
	exitCode int
	runError error // Error from cmd.Run() itself, if fo process failed to execute/complete
}

// runFo executes the compiled 'fo' test binary with given arguments.
// It logs execution details and returns the stdout, stderr, exit code, and any execution error.
func runFo(t *testing.T, foCmdArgs ...string) foResult {
	t.Helper() // Marks this function as a test helper.
	cmd := exec.Command(foTestBinaryPath, foCmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run() // This blocks until the command exits.
	duration := time.Since(startTime)

	// Determine the exit code of the 'fo' process.
	exitCode := 0
	if runErr != nil {
		var exitError *exec.ExitError
		if errors.As(runErr, &exitError) {
			// The command ran and exited with a non-zero status.
			exitCode = exitError.ExitCode()
		} else {
			// Other errors (e.g., binary not found, permissions issue for fo itself).
			exitCode = 1 // Default to 1 for generic errors.
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
		exitCode: exitCode, // This is fo's actual exit code as seen by the OS.
		runError: runErr,   // This is the error from Go's exec.Command.Run().
	}
}

// setupTestScripts creates dummy shell scripts in a 'testdata' directory for tests to use.
// setupTestScripts creates dummy shell scripts in a 'testdata' directory for tests to use.
func setupTestScripts(t *testing.T) {
	t.Helper()
	scriptsDir := "testdata"
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		if err := os.Mkdir(scriptsDir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", scriptsDir, err)
		}
	}
	// Define scripts as a map for easy iteration.
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
		// Always ensure a newline at the end of the script content
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		// Write the script file with execute permissions
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", name, err)
		}
	}
}

// buildPattern creates a regex to match fo's start/end lines.
// expectTimer for end lines controls if the duration part is included in the regex.
func buildPattern(iconString, label string, isStartLine, expectTimer bool) *regexp.Regexp {
	// QuoteMeta escapes regex special characters in iconString and label.
	pattern := regexp.QuoteMeta(iconString) + `\s*` + regexp.QuoteMeta(label)
	if isStartLine {
		pattern += `\.\.\.` // Start lines end with "...".
	} else if expectTimer { // For end lines where a timer is expected.
		// Matches (optional_whitespace)(duration_format)(optional_whitespace).
		// Duration format example: (123ms), (1.2s), (1:02.345s).
		pattern += `\s*\([\wµ\.:]+\)$` // `\w` includes numbers, `\.` for decimal, `:` for M:SS.
	} else { // For end lines where NO timer is expected (e.g., --no-timer or --ci).
		pattern += `$` // Must be the end of the string/line.
	}
	return regexp.MustCompile(pattern)
}

// TestFoCoreExecution tests basic command execution, exit codes, and argument passing.
func TestFoCoreExecution(t *testing.T) {
	setupTestScripts(t) // Ensure test scripts are available.

	t.Run("ExitCodePassthroughSuccess", func(t *testing.T) {
		t.Parallel()
		// First verify the script directly
		cmd := exec.Command("testdata/success.sh")
		output, err := cmd.CombinedOutput()
		t.Logf("Direct script execution: testdata/success.sh\nOutput: %s\nError: %v", string(output), err)
		if err != nil {
			t.Fatalf("Script testdata/success.sh failed to execute directly: %v", err)
		}

		// Simple approach - use debug and no-color for cleaner output
		res := runFo(t, "--debug", "--no-color", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			// Detailed error message with all info
			t.Errorf("Expected exit code 0, got %d\nFO STDOUT:\n%s\nFO STDERR:\n%s",
				res.exitCode, res.stdout, res.stderr)
		}
	})

	t.Run("ExitCodePassthroughFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/failure.sh")
		if res.exitCode != 1 {
			t.Errorf("Expected exit code 1, got %d", res.exitCode)
		}
	})

	t.Run("ExitCodePassthroughSpecific", func(t *testing.T) {
		t.Parallel()
		// Use --no-color to ensure consistent output for checking "--- Captured output: ---".
		// The isFoStartupError fix in main.go should ensure captured output appears correctly.
		res := runFo(t, "--no-color", "--show-output", "on-fail", "--", "testdata/exit_code.sh", "42")
		if res.exitCode != 42 {
			t.Errorf("Expected fo process to exit with code 42, got %d.", res.exitCode)
		}
		// Check for captured output header and content.
		if !strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Expected '--- Captured output: ---' header for a failing script with non-zero exit code.")
		}
		if !strings.Contains(res.stdout, "STDOUT: Script about to exit with 42") {
			t.Error("Expected script's STDOUT in captured output.")
		}
		if !strings.Contains(res.stdout, "STDERR: Script stderr message before exiting 42") {
			t.Error("Expected script's STDERR in captured output.")
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		commandName := "a_very_unique_non_existent_command_askjdfh"
		// The --debug flag was previously added to this call in gofix_main_test_20250515_11
		res := runFo(t, "--debug", "--no-color", "--", commandName)
		if res.exitCode != 127 {
			t.Errorf("Expected 'fo' to exit with 127 for command not found, got %d. Stderr:\n%s", res.exitCode, res.stderr)
		}

		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		if len(lines) == 0 {
			t.Fatalf("Expected some output from fo, got empty stdout for CommandNotFound.")
		}

		startPattern := buildPattern(plainIconStart, commandName, true, false)
		if !startPattern.MatchString(lines[0]) {
			t.Errorf("Expected 'fo' stdout to contain start line /%s/, got first line: '%s'\nFull stdout:\n%s", startPattern.String(), lines[0], res.stdout)
		}

		endPattern := buildPattern(plainIconFailure, commandName, false, true) // Timer is expected
		if len(lines) < 2 || !endPattern.MatchString(lines[len(lines)-1]) {
			lastLine := ""
			if len(lines) >= 1 {
				lastLine = lines[len(lines)-1]
			}
			t.Errorf("Expected 'fo' stdout to contain end line /%s/, got last line: '%s'\nFull stdout:\n%s", endPattern.String(), lastLine, res.stdout)
		}

		// MODIFIED ASSERTION: Use strings.Contains because debug output might be prepended.
		expectedStderrContent := "Error starting command '" + commandName + "': exec: \"" + commandName + "\":"
		if !strings.Contains(res.stderr, expectedStderrContent) {
			t.Errorf("Expected 'fo' stderr to contain '%s', got:\n%s", expectedStderrContent, res.stderr)
		}
		// Ensure "--- Captured output: ---" is NOT in stdout for fo's own startup error.
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Did NOT expect '--- Captured output: ---' in stdout for command not found error, got:\n%s", res.stdout)
		}
	})

	t.Run("ArgumentsToWrappedCommand", func(t *testing.T) {
		t.Parallel()
		helperScriptContent := `#!/bin/sh
echo "Args: $1 $2"`
		// Create script in a temporary directory specific to this test run.
		scriptPath := filepath.Join(t.TempDir(), "args_test.sh")
		if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0o755); err != nil {
			t.Fatalf("Failed to write script: %v", err)
		}
		res := runFo(t, "--show-output", "always", "--no-color", "--", scriptPath, "hello", "world")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
		}
		// Captured output should contain the script's output.
		if !strings.Contains(res.stdout, "Args: hello world") {
			t.Errorf("Expected 'Args: hello world' in fo's output, got: %s", res.stdout)
		}
	})
}

// TestFoLabels verifies that labels (default and custom) are correctly displayed.
func TestFoLabels(t *testing.T) {
	setupTestScripts(t)
	scriptPath := "testdata/success.sh"

	// Ensure script exists and is executable
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("Test script %s not found: %v", scriptPath, err)
	}

	// Make the script executable - ensure proper permissions
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		t.Fatalf("Failed to make script executable: %v", err)
	}

	// Test by running the script directly to verify it works
	cmd := exec.Command(scriptPath)
	output, err := cmd.CombinedOutput()
	t.Logf("Direct script execution: %s\nOutput: %s\nError: %v", scriptPath, string(output), err)
	if err != nil {
		t.Fatalf("Script %s failed to run directly: %v", scriptPath, err)
	}

	// Get absolute path to avoid working directory issues
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for %s: %v", scriptPath, err)
	}

	// Use absolute path for actual tests to avoid working directory issues
	scriptForTest := absScriptPath

	// commonTest is a helper for testing label scenarios.
	// expectedLabelForPattern should be the exact label string the regex needs to match.
	commonTest := func(tcName string, args []string, expectedLabelForPattern string) {
		t.Run(tcName, func(t *testing.T) {
			t.Parallel() // Run subtests in parallel.
			res := runFo(t, args...)
			lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
			if len(lines) < 2 { // Expect at least a start and an end line.
				t.Fatalf("Args: %v. Expected at least 2 lines of output, got %d:\n%s", args, len(lines), res.stdout)
			}

			// Check the first line for the start pattern.
			startPattern := buildPattern(plainIconStart, expectedLabelForPattern, true, false)
			if !startPattern.MatchString(lines[0]) {
				t.Errorf("Args: %v. Missing start pattern /%s/ in first line '%s'. Full stdout:\n%s", args, startPattern, lines[0], res.stdout)
			}

			// Determine if a timer is expected in the end line based on arguments.
			expectTimer := true // Default with --no-color.
			for _, arg := range args {
				if arg == "--no-timer" || arg == "--ci" { // --ci implies --no-timer.
					expectTimer = false
					break
				}
			}
			// Check the last line for the end pattern.
			endPattern := buildPattern(plainIconSuccess, expectedLabelForPattern, false, expectTimer) // Assumes success for this test.
			if !endPattern.MatchString(lines[len(lines)-1]) {
				t.Errorf("Args: %v. Missing end pattern /%s/ in last line '%s'. Full stdout:\n%s", args, endPattern, lines[len(lines)-1], res.stdout)
			}
		})
	}

	// Test default label inference (should be the basename of the script).
	commonTest("DefaultLabelInferenceNoColor", []string{"--no-color", "--", scriptForTest}, filepath.Base(scriptPath))

	// Test custom label with short flag.
	customLabel1 := "My Custom Task"
	commonTest("CustomLabelShortFlagNoColor", []string{"--no-color", "-l", customLabel1, "--", scriptForTest}, customLabel1)

	// Test custom label with long flag.
	customLabel2 := "Another Task"
	commonTest("CustomLabelLongFlagNoColor", []string{"--no-color", "--label", customLabel2, "--", scriptForTest}, customLabel2)
}

// TestFoStreamMode verifies behavior when -s/--stream flag is used.
func TestFoStreamMode(t *testing.T) {
	setupTestScripts(t)

	t.Run("StreamSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--no-color", "--", "testdata/success.sh") // Added --no-color for consistency.
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0", res.exitCode)
		}
		// In stream mode, script's stdout goes to fo's stdout.
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Error("Expected script's stdout in fo's stdout")
		}
		// In stream mode, script's stderr should go to fo's stderr.
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
		// Output should still be streamed.
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Error("Expected script's stdout in fo's stdout even when --show-output=never is overridden by stream mode")
		}
		// "--- Captured output: ---" header should NOT be present in stream mode.
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Did NOT expect 'Captured output' header in stream mode")
		}
	})
}

// TestFoTimer checks if the execution timer is displayed correctly based on flags.
func TestFoTimer(t *testing.T) {
	setupTestScripts(t)
	// Regex for timer: (any_duration_format) e.g., (123ms), (1.2s), (1m02.345s), (12µs).

	t.Run("TimerShownByDefaultNoColor", func(t *testing.T) {
		t.Parallel()
		scriptName := "testdata/long_running.sh"
		expectedLabel := filepath.Base(scriptName) // fo infers basename.
		res := runFo(t, "--no-color", "--", scriptName)

		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		if len(lines) < 1 {
			t.Fatalf("Expected output, got none for TimerShownByDefaultNoColor")
		}
		actualEndLine := lines[len(lines)-1] // End status is the last line.

		// Pattern for end line with timer.
		endLineWithTimerPattern := buildPattern(plainIconSuccess, expectedLabel, false, true)
		if !endLineWithTimerPattern.MatchString(actualEndLine) {
			t.Errorf("Expected timer in output matching /%s/, but not found in line '%s'. Output:\n%s", endLineWithTimerPattern.String(), actualEndLine, res.stdout)
		}
	})

	t.Run("TimerHiddenWithNoTimerFlag", func(t *testing.T) {
		t.Parallel()
		// Check if testdata scripts exist and are executable
		scriptPath := "testdata/success.sh"
		if _, err := os.Stat(scriptPath); err != nil {
			t.Fatalf("Test script %s not found: %v", scriptPath, err)
		}

		// Make the script executable
		if err := os.Chmod(scriptPath, 0o755); err != nil {
			t.Fatalf("Failed to make script executable: %v", err)
		}

		// Run the test directly to verify it works
		cmd := exec.Command(scriptPath)
		output, err := cmd.CombinedOutput()
		t.Logf("Direct script execution: %s\nOutput: %s\nError: %v", scriptPath, string(output), err)
		if err != nil {
			t.Fatalf("Script %s failed to run directly: %v", scriptPath, err)
		}

		// Actual test
		expectedLabel := filepath.Base(scriptPath)
		res := runFo(t, "--no-timer", "--no-color", "--", scriptPath)

		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0", res.exitCode)
		}

		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		if len(lines) < 1 {
			t.Fatalf("Expected output in CI mode, got none for script %s.", scriptPath)
		}

		actualEndLine := ""
		if len(lines) > 0 {
			actualEndLine = lines[len(lines)-1]
		}

		// Check end line pattern (no timer)
		endPattern := buildPattern(plainIconSuccess, expectedLabel, false, false)
		if !endPattern.MatchString(actualEndLine) {
			t.Errorf("Expected end line /%s/ (no timer), got '%s'. Full stdout:\n%s",
				endPattern, actualEndLine, res.stdout)
		}

		// Note: Removed redundant timerRegex check since endPattern already verifies no timer
	})
}

// TestFoColorAndIcons verifies that ANSI colors and icons are used/suppressed correctly.
func TestFoColorAndIcons(t *testing.T) {
	setupTestScripts(t)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`) // Regex to detect ANSI escape codes.
	// Use a fresh default config to get themed icons.
	vibrantCfg := design.UnicodeVibrantTheme()
	emojiStart, emojiSuccess, emojiFail := vibrantCfg.Icons.Start, vibrantCfg.Icons.Success, vibrantCfg.Icons.Error

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh") // No --no-color.
		if !ansiRegex.MatchString(res.stdout) {
			t.Error("Expected ANSI color codes, none found")
		}
		if !strings.Contains(res.stdout, emojiStart) {
			t.Errorf("Expected start icon '%s', not found in stdout:\n%s", emojiStart, res.stdout)
		}
		if !strings.Contains(res.stdout, emojiSuccess) {
			t.Errorf("Expected success icon '%s', not found in stdout:\n%s", emojiSuccess, res.stdout)
		}
	})

	t.Run("ColorAndIconsShownByDefaultFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh") // No --no-color.
		if !ansiRegex.MatchString(res.stdout) {
			t.Error("Expected ANSI color codes, none found")
		}
		if !strings.Contains(res.stdout, emojiStart) {
			t.Errorf("Expected start icon '%s', not found in stdout:\n%s", emojiStart, res.stdout)
		}
		if !strings.Contains(res.stdout, emojiFail) {
			t.Errorf("Expected failure icon '%s', not found in stdout:\n%s", emojiFail, res.stdout)
		}
	})

	t.Run("ColorAndIconsHiddenWithNoColorFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/success.sh")
		if ansiRegex.MatchString(res.stdout) {
			t.Error("Unexpected ANSI colors with --no-color")
		}
		// With --no-color, emoji icons should be replaced by plain text versions.
		if strings.Contains(res.stdout, emojiStart) || strings.Contains(res.stdout, emojiSuccess) {
			t.Error("Unexpected emoji icons with --no-color")
		}
		// Check for plain text icons.
		if !strings.Contains(res.stdout, plainIconStart) {
			t.Errorf("Missing plain start icon '%s' in stdout:\n%s", plainIconStart, res.stdout)
		}
		if !strings.Contains(res.stdout, plainIconSuccess) {
			t.Errorf("Missing plain success icon '%s' in stdout:\n%s", plainIconSuccess, res.stdout)
		}
	})
}

// TestFoCIMode checks behavior with the --ci flag (implies no color, no timer).
func TestFoCIMode(t *testing.T) {
	setupTestScripts(t)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	tests := []struct {
		name       string
		scriptPath string // Full path to the script being run by fo.
		exit       int    // Expected exit code from fo.
		endIcon    string // Expected plain text end icon (e.g., plainIconSuccess).
	}{
		{"CIModeSuccess", "testdata/success.sh", 0, plainIconSuccess},
		{"CIModeFailure", "testdata/failure.sh", 1, plainIconFailure},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable.
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectedLabel := filepath.Base(tt.scriptPath) // fo uses base name for label.
			res := runFo(t, "--ci", "--", tt.scriptPath)

			if res.exitCode != tt.exit {
				t.Errorf("Exit code: got %d, want %d", res.exitCode, tt.exit)
			}
			// ADDED: Hex dump for debugging potential hidden ANSI codes.
			if ansiRegex.MatchString(res.stdout) {
				t.Errorf("Unexpected ANSI colors in CI mode. Stdout:\n%s\nHex:\n%x", res.stdout, res.stdout)
			}

			lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
			if len(lines) == 0 {
				t.Fatalf("Expected output in CI mode, got none for script %s.", tt.scriptPath)
			}

			actualStartLine := lines[0]
			actualEndLine := ""
			if len(lines) > 0 {
				actualEndLine = lines[len(lines)-1]
			}

			// Check start line pattern.
			startPattern := buildPattern(plainIconStart, expectedLabel, true, false) // No timer for start line.
			if !startPattern.MatchString(actualStartLine) {
				t.Errorf("Missing start pattern /%s/ in start line '%s'. Full stdout:\n%s", startPattern, actualStartLine, res.stdout)
			}

			// Check end line pattern (no timer).
			endPattern := buildPattern(tt.endIcon, expectedLabel, false, false)
			if !endPattern.MatchString(actualEndLine) {
				t.Errorf("Missing end pattern /%s/ in end line '%s'. Full stdout:\n%s", endPattern, actualEndLine, res.stdout)
			}
		})
	}
}

// TestFoErrorHandling tests fo's own error reporting for invalid usage.
func TestFoErrorHandling(t *testing.T) {
	t.Run("NoCommandAfterDashDash", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--")  // Only "--" provided.
		if res.exitCode == 0 { // Should fail.
			t.Error("Expected non-zero exit when no command is specified after --")
		}
		// Check for the specific error message in fo's stderr.
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing expected error message in stderr. Got:\n%s", res.stderr)
		}
	})

	t.Run("NoCommandAtAll", func(t *testing.T) { // e.g., "fo -l some-label" without "--" or command.
		t.Parallel()
		res := runFo(t, "-l", "some-label")
		if res.exitCode == 0 { // Should fail.
			t.Error("Expected non-zero exit when no command is specified at all")
		}
		// Should also report that no command was found after an expected "--".
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing expected error message in stderr. Got:\n%s", res.stderr)
		}
	})

	t.Run("InvalidShowOutputValue", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "bad-value", "--", "true") // "true" is a dummy command.
		if res.exitCode == 0 {                                      // Should fail due to bad flag value.
			t.Error("Expected non-zero exit for invalid --show-output value")
		}
		// Check for the specific error message.
		if !strings.Contains(res.stderr, "Error: Invalid value for --show-output: bad-value") {
			t.Errorf("Missing expected error message for invalid --show-output. Got:\n%s", res.stderr)
		}
	})
}

// TestEnvironmentInheritance checks if the wrapped command inherits environment variables.
func TestEnvironmentInheritance(t *testing.T) {
	t.Parallel()
	scriptContent := `#!/bin/sh
echo "VAR is: $MY_TEST_VAR"` // Script that prints an environment variable.
	scriptPath := filepath.Join(t.TempDir(), "env.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to write environment test script: %v", err)
	}

	key, val := "MY_TEST_VAR", "fo_env_test_val_unique"
	origVal, wasSet := os.LookupEnv(key) // Store original value, if any.

	// Set the environment variable for the scope of this test.
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("Failed to set environment variable %s to %s: %v", key, val, err)
	}
	// Defer restoration of the original environment variable state.
	defer func() {
		if wasSet {
			if err := os.Setenv(key, origVal); err != nil {
				t.Logf("Warning: Failed to restore environment variable %s to %s: %v", key, origVal, err)
			}
		} else {
			if err := os.Unsetenv(key); err != nil {
				t.Logf("Warning: Failed to unset environment variable %s: %v", key, err)
			}
		}
	}()

	// Run fo with the script, expecting it to see the set environment variable.
	res := runFo(t, "--show-output", "always", "--no-color", "--", scriptPath)
	if res.exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", res.exitCode)
	}
	expectedOutput := fmt.Sprintf("VAR is: %s", val)
	if !strings.Contains(res.stdout, expectedOutput) {
		t.Errorf("Expected '%s' in fo's stdout, indicating environment inheritance. Got:\n%s", expectedOutput, res.stdout)
	}
}

// TestFoTimestampedOutput verifies that interleaved stdout/stderr from the script
// are captured and displayed, preserving their content.
func TestFoTimestampedOutput(t *testing.T) {
	setupTestScripts(t)
	t.Run("PreservesOutputOrderInCaptureNoColor", func(t *testing.T) {
		t.Parallel()
		// ADDED --debug to the fo command invocation for this test.
		res := runFo(t, "--debug", "--no-color", "--show-output", "always", "--", "testdata/interleaved.sh")
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0. Stdout:\n%s\nStderr:\n%s", res.exitCode, res.stdout, res.stderr)
		}

		// Extract the "Captured output" section from fo's stdout.
		capturedSection := ""
		parts := strings.SplitN(res.stdout, "--- Captured output: ---", 2)
		if len(parts) == 2 {
			foEndLinePattern := regexp.MustCompile(`(?m)^\[(SUCCESS|FAILED|WARNING)\].*$`)
			endLineMatch := foEndLinePattern.FindStringIndex(parts[1])
			if endLineMatch != nil {
				capturedSection = strings.TrimSpace(parts[1][:endLineMatch[0]])
			} else {
				capturedSection = strings.TrimSpace(parts[1])
			}
		} else {
			t.Fatalf("Missing '--- Captured output: ---' section in stdout:\n%s", res.stdout)
		}

		expectedMessages := []string{
			"STDOUT: Message 1",
			"STDERR: Message 1",
			"STDOUT: Message 2",
			"STDERR: Message 2",
		}

		for _, msg := range expectedMessages {
			if !strings.Contains(capturedSection, msg) {
				t.Errorf("Expected captured output to contain '%s', but it was missing. Captured section:\n%s", msg, capturedSection)
			}
		}
	})
}

// TestFoSignalHandling is skipped as it's hard to automate reliably.
func TestFoSignalHandling(t *testing.T) {
	t.Skip("Skipping signal handling test; OS-dependent and hard to automate reliably.")
}

// TestFoBasicExecution is skipped as its functionality is covered by other, more specific tests.
func TestFoBasicExecution(t *testing.T) {
	t.Skip("Basic execution covered in TestFoCoreExecution and others.")
}
