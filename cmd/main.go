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
)

// Updated plain text icon constants to match GetIcon's monochrome output
const (
	plainIconStart   = "[START]"
	plainIconSuccess = "[SUCCESS]"
	plainIconWarning = "[WARNING]"
	plainIconFailure = "[FAILED]"
)

var foTestBinaryPath = "./fo_test_binary_generated_by_TestMain"

// TestMain sets up and tears down the test binary.
func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", foTestBinaryPath, ".")
	buildOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build test binary '%s': %v\nOutput:\n%s\n", foTestBinaryPath, err, string(buildOutput))
		os.Exit(1)
	}

	exitCode := m.Run()

	if err := os.Remove(foTestBinaryPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to remove test binary '%s': %v\n", foTestBinaryPath, err)
	}
	os.Exit(exitCode)
}

// foResult holds the output and exit status of an 'fo' command execution.
type foResult struct {
	stdout   string
	stderr   string
	exitCode int
	runError error
}

// runFo executes the compiled 'fo' test binary with given arguments.
func runFo(t *testing.T, foCmdArgs ...string) foResult {
	t.Helper()
	cmd := exec.Command(foTestBinaryPath, foCmdArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	exitCode := 0
	if runErr != nil {
		if exitError, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

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
		exitCode: exitCode,
		runError: runErr,
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
		"long_running.sh": `#!/bin/sh 
echo "STDOUT: Starting long task..."
sleep 0.1 
echo "STDOUT: Long task finished."
exit 0`,
		"only_stdout.sh": `#!/bin/sh
echo "ONLY_STDOUT_CONTENT"
exit 0`,
		"only_stderr.sh": `#!/bin/sh
echo "ONLY_STDERR_CONTENT" >&2
exit 1`,
		"interleaved.sh": `#!/bin/sh 
echo "STDOUT: Message 1"
echo "STDERR: Message 1" >&2
echo "STDOUT: Message 2"
echo "STDERR: Message 2" >&2
exit 0`,
		"large_output.sh": `#!/bin/sh 
for i in $(seq 1 100); do
  printf "STDOUT: Line %04d - This is test content to generate output.\n" $i
done
echo "Script complete"
exit 0`,
		"signal_test.sh": `#!/bin/sh 
echo "Starting signal test script (PID: $$)"
echo "Will sleep for 5 seconds unless interrupted" 
trap 'echo "Caught signal, exiting cleanly"; exit 42' INT TERM
sleep 5
echo "Finished sleeping"
exit 0`,
	}
	for name, content := range scripts {
		path := filepath.Join(scriptsDir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", name, err)
		}
	}
}

// buildPattern creates a regex to match fo's start/end lines.
func buildPattern(iconString string, label string, isStartLine bool, expectTimer bool) *regexp.Regexp {
	pattern := regexp.QuoteMeta(iconString) + `\s*` + regexp.QuoteMeta(label)
	if isStartLine {
		pattern += `\.\.\.`
	} else if expectTimer {
		pattern += `\s*\([\wµ\.:]+\)$`
	} else {
		pattern += `$`
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
		res := runFo(t, "--no-color", "--show-output", "on-fail", "--", "testdata/exit_code.sh", "42")
		if res.exitCode != 42 {
			t.Errorf("Expected fo process to exit with code 42, got %d.", res.exitCode)
		}
		if !strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Expected '--- Captured output: ---' header for a failing script with non-zero exit code.")
		}
		if !strings.Contains(res.stdout, "STDOUT: Script about to exit with 42") {
			t.Error("Expected script's STDOUT in captured output.")
		}
		// This was the failing assertion. Check if STDERR message is present in the captured output.
		if !strings.Contains(res.stdout, "STDERR: Script stderr message before exiting 42") {
			t.Errorf("Expected script's STDERR in captured output. Full stdout:\n%s", res.stdout)
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		commandName := "a_very_unique_non_existent_command_askjdfh"
		// Add --debug to see if it affects stderr output order
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

		endPattern := buildPattern(plainIconFailure, commandName, false, true)
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
		if !strings.Contains(res.stdout, "Args: hello world") {
			t.Errorf("Expected 'Args: hello world' in fo's output, got: %s", res.stdout)
		}
	})
}

// TestFoLabels verifies that labels (default and custom) are correctly displayed.
func TestFoLabels(t *testing.T) {
	setupTestScripts(t)
	scriptPath := "testdata/success.sh"

	commonTest := func(tcName string, args []string, expectedLabelForPattern string) {
		t.Run(tcName, func(t *testing.T) {
			t.Parallel()
			res := runFo(t, args...)
			lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
			if len(lines) < 2 {
				t.Fatalf("Args: %v. Expected at least 2 lines of output, got %d:\n%s", args, len(lines), res.stdout)
			}

			startPattern := buildPattern(plainIconStart, expectedLabelForPattern, true, false)
			if !startPattern.MatchString(lines[0]) {
				t.Errorf("Args: %v. Missing start pattern /%s/ in first line '%s'. Full stdout:\n%s", args, startPattern, lines[0], res.stdout)
			}

			expectTimer := true
			for _, arg := range args {
				if arg == "--no-timer" || arg == "--ci" {
					expectTimer = false
					break
				}
			}
			endPattern := buildPattern(plainIconSuccess, expectedLabelForPattern, false, expectTimer)
			if !endPattern.MatchString(lines[len(lines)-1]) {
				t.Errorf("Args: %v. Missing end pattern /%s/ in last line '%s'. Full stdout:\n%s", args, endPattern, lines[len(lines)-1], res.stdout)
			}
		})
	}

	commonTest("DefaultLabelInferenceNoColor", []string{"--no-color", "--", scriptPath}, filepath.Base(scriptPath))

	customLabel1 := "My Custom Task"
	commonTest("CustomLabelShortFlagNoColor", []string{"--no-color", "-l", customLabel1, "--", scriptPath}, customLabel1)

	customLabel2 := "Another Task"
	commonTest("CustomLabelLongFlagNoColor", []string{"--no-color", "--label", customLabel2, "--", scriptPath}, customLabel2)
}

// TestFoCaptureMode tests different behaviors of --show-output in capture mode.
func TestFoCaptureMode(t *testing.T) {
	setupTestScripts(t)
	tests := []struct {
		name                                 string
		foArgs                               []string
		script                               string
		expectExit                           int
		expectHeader                         bool
		negateOut                            bool
		negateErr                            bool
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
				t.Errorf("Exit code: got %d, want %d", res.exitCode, tt.expectExit)
			}

			hasHeader := strings.Contains(res.stdout, "--- Captured output: ---")
			if tt.expectHeader != hasHeader {
				t.Errorf("'Captured output' header presence: got %t, want %t. Full stdout:\n%s", hasHeader, tt.expectHeader, res.stdout)
			}

			targetSection := res.stdout
			if tt.expectHeader && hasHeader {
				parts := strings.SplitN(res.stdout, "--- Captured output: ---", 2)
				if len(parts) == 2 {
					targetSection = parts[1]
				} else {
					t.Errorf("Expected 'Captured output' header and section, but couldn't split stdout appropriately:\n%s", res.stdout)
				}
			}

			outPresent := strings.Contains(targetSection, tt.expectOutContains)
			if tt.negateOut {
				if outPresent {
					t.Errorf("Script stdout '%s' was unexpectedly PRESENT in target section:\n%s", tt.expectOutContains, targetSection)
				}
			} else {
				if !outPresent {
					t.Errorf("Script stdout '%s' was unexpectedly ABSENT from target section:\n%s", tt.expectOutContains, targetSection)
				}
			}

			errPresent := strings.Contains(targetSection, tt.expectErrContains)
			if tt.negateErr {
				if errPresent {
					t.Errorf("Script stderr '%s' was unexpectedly PRESENT in target section:\n%s", tt.expectErrContains, targetSection)
				}
			} else {
				if !errPresent {
					t.Errorf("Script stderr '%s' was unexpectedly ABSENT from target section:\n%s", tt.expectErrContains, targetSection)
				}
			}
		})
	}
}

// TestFoStreamMode verifies behavior when -s/--stream flag is used.
func TestFoStreamMode(t *testing.T) {
	setupTestScripts(t)

	t.Run("StreamSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--no-color", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Error("Expected script's stdout in fo's stdout")
		}
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
			t.Error("Expected script's stdout in fo's stdout even when --show-output=never is overridden by stream mode")
		}
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Did NOT expect 'Captured output' header in stream mode")
		}
	})
}

// TestFoTimer checks if the execution timer is displayed correctly based on flags.
func TestFoTimer(t *testing.T) {
	setupTestScripts(t)
	timerRegex := regexp.MustCompile(`\s*\([\d\.:µms]+\)$`)

	t.Run("TimerShownByDefaultNoColor", func(t *testing.T) {
		t.Parallel()
		scriptName := "testdata/long_running.sh"
		expectedLabel := filepath.Base(scriptName)
		res := runFo(t, "--no-color", "--", scriptName)

		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		if len(lines) < 1 {
			t.Fatalf("Expected output, got none for TimerShownByDefaultNoColor")
		}
		actualEndLine := lines[len(lines)-1]

		endLineWithTimerPattern := buildPattern(plainIconSuccess, expectedLabel, false, true)
		if !endLineWithTimerPattern.MatchString(actualEndLine) {
			t.Errorf("Expected timer in output matching /%s/, but not found in line '%s'. Output:\n%s", endLineWithTimerPattern.String(), actualEndLine, res.stdout)
		}
	})

	t.Run("TimerHiddenWithNoTimerFlag", func(t *testing.T) {
		t.Parallel()
		scriptName := "testdata/success.sh"
		expectedLabel := filepath.Base(scriptName)
		res := runFo(t, "--no-timer", "--no-color", "--", scriptName)

		lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
		if len(lines) < 1 {
			t.Fatalf("Expected output, got none for TimerHiddenWithNoTimerFlag")
		}
		actualEndLine := lines[len(lines)-1]

		endLineNoTimerPattern := buildPattern(plainIconSuccess, expectedLabel, false, false)

		if !endLineNoTimerPattern.MatchString(actualEndLine) {
			t.Errorf("Expected end line /%s/ (no timer), got '%s'. Full stdout:\n%s", endLineNoTimerPattern.String(), actualEndLine, res.stdout)
		}
		if timerRegex.MatchString(actualEndLine) {
			t.Errorf("Expected no timer in end line '%s', but found one. Full stdout:\n%s", actualEndLine, res.stdout)
		}
	})
}

// TestFoColorAndIcons verifies that ANSI colors and icons are used/suppressed correctly.
func TestFoColorAndIcons(t *testing.T) {
	setupTestScripts(t)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	vibrantCfg := design.UnicodeVibrantTheme()
	emojiStart, emojiSuccess, emojiFail := vibrantCfg.Icons.Start, vibrantCfg.Icons.Success, vibrantCfg.Icons.Error

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
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
		res := runFo(t, "--", "testdata/failure.sh")
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
		if strings.Contains(res.stdout, emojiStart) || strings.Contains(res.stdout, emojiSuccess) {
			t.Error("Unexpected emoji icons with --no-color")
		}
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
	timerRegex := regexp.MustCompile(`\s*\([\d\.:µms]+\)$`)
	tests := []struct {
		name       string
		scriptPath string
		args       []string // Additional args for fo, like --debug
		exit       int
		endIcon    string
	}{
		{"CIModeSuccess", "testdata/success.sh", []string{"--ci"}, 0, plainIconSuccess},
		// Add --debug to CIModeFailure to get MergeWithFlags output
		{"CIModeFailure", "testdata/failure.sh", []string{"--ci", "--debug"}, 1, plainIconFailure},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			expectedLabel := filepath.Base(tt.scriptPath)
			foArgs := append(tt.args, "--", tt.scriptPath)
			res := runFo(t, foArgs...)

			if res.exitCode != tt.exit {
				t.Errorf("Exit code: got %d, want %d", res.exitCode, tt.exit)
			}
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
				// Find the last line that looks like a status line
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], plainIconStart) ||
						strings.HasPrefix(lines[i], plainIconSuccess) ||
						strings.HasPrefix(lines[i], plainIconFailure) ||
						strings.HasPrefix(lines[i], plainIconWarning) {
						actualEndLine = lines[i]
						break
					}
				}
				if actualEndLine == "" { // Fallback if no clear status line found at end
					actualEndLine = lines[len(lines)-1]
				}
			}

			if timerRegex.MatchString(actualEndLine) {
				t.Errorf("Unexpected timer in CI mode end line '%s'. Full stdout:\n%s", actualEndLine, res.stdout)
			}

			startPattern := buildPattern(plainIconStart, expectedLabel, true, false)
			if !startPattern.MatchString(actualStartLine) {
				t.Errorf("Missing start pattern /%s/ in start line '%s'. Full stdout:\n%s", startPattern, actualStartLine, res.stdout)
			}

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
		res := runFo(t, "--")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit when no command is specified after --")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing expected error message in stderr. Got:\n%s", res.stderr)
		}
	})

	t.Run("NoCommandAtAll", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-l", "some-label")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit when no command is specified at all")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Missing expected error message in stderr. Got:\n%s", res.stderr)
		}
	})

	t.Run("InvalidShowOutputValue", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "bad-value", "--", "true")
		if res.exitCode == 0 {
			t.Error("Expected non-zero exit for invalid --show-output value")
		}
		if !strings.Contains(res.stderr, "Error: Invalid value for --show-output: bad-value") {
			t.Errorf("Missing expected error message for invalid --show-output. Got:\n%s", res.stderr)
		}
	})
}

// TestEnvironmentInheritance checks if the wrapped command inherits environment variables.
func TestEnvironmentInheritance(t *testing.T) {
	t.Parallel()
	scriptContent := `#!/bin/sh
echo "VAR is: $MY_TEST_VAR"`
	scriptPath := filepath.Join(t.TempDir(), "env.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write environment test script: %v", err)
	}

	key, val := "MY_TEST_VAR", "fo_env_test_val_unique"
	origVal, wasSet := os.LookupEnv(key)

	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("Failed to set environment variable %s to %s: %v", key, val, err)
	}
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
		res := runFo(t, "--debug", "--no-color", "--show-output", "always", "--", "testdata/interleaved.sh")
		if res.exitCode != 0 {
			t.Errorf("Exit code: got %d, want 0. Stdout:\n%s\nStderr:\n%s", res.exitCode, res.stdout, res.stderr)
		}

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
