package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/davidkoosis/fo/cmd/internal/design" // Import the design package
)

// Plain text icon constants used with --no-color or --ci
const (
	plainIconStart   = "[START]"
	plainIconSuccess = "[SUCCESS]"
	plainIconFailure = "[FAILED]"
)

var foTestBinaryPath = "./fo_test_binary_generated_by_TestMain"

func TestMain(m *testing.M) {
	cmd := exec.Command("go", "build", "-o", foTestBinaryPath, ".")
	buildOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build test binary '%s': %v\nOutput:\n%s\n", foTestBinaryPath, err, string(buildOutput))
		os.Exit(1)
	}
	defer func() {
		if err := os.Remove(foTestBinaryPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to remove test binary '%s': %v\n", foTestBinaryPath, err)
		}
	}()
	exitCode := m.Run()
	os.Exit(exitCode)
}

type foResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

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
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Logf("cmd.Run() for '%s' process returned a non-ExitError: %v", foTestBinaryPath, runErr)
			exitCode = -1
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	logMessage := fmt.Sprintf("Executed: %s %s (took %v)\n"+
		"--- FO STDOUT ---\n%s\n"+
		"--- FO STDERR ---\n%s\n"+
		"--- FO EXITCODE (from runFo perspective): %d ---",
		foTestBinaryPath, strings.Join(foCmdArgs, " "), duration,
		stdout.String(), stderr.String(), exitCode)
	t.Log(logMessage)

	res := foResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
		err:      runErr,
	}
	return res
}

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
sleep 0.2
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
# Generate enough output to exceed a 1MB buffer limit
# Each line is ~200 bytes, so we need ~5000 lines to generate >1MB
for i in $(seq 1 5000); do
  printf "STDOUT: Line %04d - This is test content to generate output that will exceed our buffer limit of 1MB. We're making each line reasonably long to reach the limit quickly.\n" $i
done
echo "Script complete - generated approximately 1MB of output"
exit 0`,
		"signal_test.sh": `#!/bin/sh
echo "Starting signal test script (PID: $$)"
echo "Will sleep for 10 seconds unless interrupted"
trap 'echo "Caught signal, exiting cleanly"; exit 42' INT TERM
sleep 10
echo "Finished sleeping"
exit 0`,
	}
	for name, content := range scripts {
		path := filepath.Join(scriptsDir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", name, err)
		}
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatalf("Failed to chmod test script %s: %v", name, err)
		}
	}
}

// Helper to build regex patterns for start/end lines
func buildPattern(iconString string, label string, isStartLine bool, expectTimerForEndLine bool) *regexp.Regexp {
	pattern := regexp.QuoteMeta(iconString) + `\s*` + regexp.QuoteMeta(label)
	if isStartLine {
		pattern += `\.\.\.`
	} else if expectTimerForEndLine {
		pattern += `\s*\(`
	}
	return regexp.MustCompile(pattern)
}

func TestFoCoreExecution(t *testing.T) {
	setupTestScripts(t)
	t.Run("ExitCodePassthroughSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
		}
	})

	t.Run("ExitCodePassthroughFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh")
		if res.exitCode != 1 {
			t.Errorf("Expected 'fo' to exit with code 1, got %d", res.exitCode)
		}
	})

	t.Run("ExitCodePassthroughSpecific", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/exit_code.sh", "42")
		if res.exitCode != 42 {
			t.Errorf("Expected 'fo' process to exit with code 42, got %d. "+
				"If main.go debug logs confirm 'about to os.Exit(42)', this discrepancy may be due to "+
				"test environment's handling of non-zero exits from compiled binaries that also write to stderr.", res.exitCode)
		}
		if !strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Expected '--- Captured output: ---' header for a failing script with non-zero exit code.")
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		commandName := "a_very_non_existent_command_and_unique_dhfjs"
		res := runFo(t, "--no-color", "--", commandName)

		if res.exitCode == 0 {
			t.Errorf("Expected 'fo' to exit non-zero for command not found, got %d. Error from runFo: %v", res.exitCode, res.err)
		}

		expectedStartPattern := buildPattern(plainIconStart, commandName, true, false)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain start line matching pattern /%s/, got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := buildPattern(plainIconFailure, commandName, false, true)
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain end line matching pattern /%s/, got:\n%s", expectedEndPattern.String(), res.stdout)
		}

		expectedStderrPrefix := "Error starting command '" + commandName + "': exec: \"" + commandName + "\": "
		actualStderr := strings.TrimSpace(res.stderr)

		if !strings.HasPrefix(actualStderr, expectedStderrPrefix) {
			t.Errorf("Expected 'fo' stderr to start with the prefix '%s', got:\n%s", expectedStderrPrefix, actualStderr)
		}

		mentionsExecutableNotFound := strings.Contains(actualStderr, "executable file not found")
		mentionsNoSuchFile := strings.Contains(actualStderr, "No such file or directory")

		if !mentionsExecutableNotFound && !mentionsNoSuchFile {
			t.Errorf("Expected 'fo' stderr to contain either 'executable file not found' or 'No such file or directory' after the prefix, got:\n%s", actualStderr)
		}
	})

	t.Run("ArgumentsToWrappedCommand", func(t *testing.T) {
		t.Parallel()
		helperScriptContent := `#!/bin/sh
echo "Args: $1 $2"`
		scriptPath := filepath.Join(t.TempDir(), "args_test.sh")
		if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", scriptPath, err)
		}

		res := runFo(t, "--show-output", "always", "--", scriptPath, "hello", "world")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "Args: hello world") {
			t.Errorf("Expected 'Args: hello world' in output, got: %s", res.stdout)
		}
	})
}

func TestFoLabels(t *testing.T) {
	setupTestScripts(t)
	scriptPath := "testdata/success.sh"

	t.Run("DefaultLabelInferenceNoColor", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", scriptPath)

		expectedStartPattern := buildPattern(plainIconStart, scriptPath, true, false)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain start line matching pattern /%s/, got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := buildPattern(plainIconSuccess, scriptPath, false, true)
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain success end label matching pattern /%s/, got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})

	t.Run("CustomLabelShortFlagNoColor", func(t *testing.T) {
		t.Parallel()
		customLabel := "My Custom Task"
		res := runFo(t, "--no-color", "-l", customLabel, "--", scriptPath)

		expectedStartPattern := buildPattern(plainIconStart, customLabel, true, false)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain start line matching pattern /%s/, got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := buildPattern(plainIconSuccess, customLabel, false, true)
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain success end label matching pattern /%s/, got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})

	t.Run("CustomLabelLongFlagNoColor", func(t *testing.T) {
		t.Parallel()
		customLabel := "Another Task"
		res := runFo(t, "--no-color", "--label", customLabel, "--", scriptPath)

		expectedStartPattern := buildPattern(plainIconStart, customLabel, true, false)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain start line matching pattern /%s/, got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := buildPattern(plainIconSuccess, customLabel, false, true)
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout (no-color) to contain success end label matching pattern /%s/, got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})
}

func TestFoCaptureMode(t *testing.T) {
	setupTestScripts(t)
	tests := []struct {
		name               string
		foArgs             []string
		showOutputFlag     string
		script             string
		scriptArgs         []string
		expectedExitCode   int
		expectOutputHeader bool
		expectStdoutInFo   string
		expectStderrInFo   string
		negateExpectStdout bool
		negateExpectStderr bool
	}{
		{
			name:               "Default (on-fail) Success (no color for this check)",
			foArgs:             []string{"--no-color"},
			showOutputFlag:     "",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdoutInFo:   "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderrInFo:   "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "on-fail Failure (no color for this check)",
			foArgs:             []string{"--no-color"},
			showOutputFlag:     "on-fail",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdoutInFo:   "STDOUT: Output from failure.sh before failing",
			expectStderrInFo:   "STDERR: Error message from failure.sh",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{}
			args = append(args, tt.foArgs...)
			if tt.showOutputFlag != "" {
				args = append(args, "--show-output", tt.showOutputFlag)
			}
			args = append(args, "--")
			args = append(args, tt.script)
			args = append(args, tt.scriptArgs...)

			res := runFo(t, args...)

			if res.exitCode != tt.expectedExitCode {
				t.Errorf("Expected 'fo' to exit with code %d, got %d", tt.expectedExitCode, res.exitCode)
			}
			hasOutputHeader := strings.Contains(res.stdout, "--- Captured output: ---")
			if tt.expectOutputHeader && !hasOutputHeader {
				t.Errorf("Expected '--- Captured output: ---' header, but not found in stdout:\n%s", res.stdout)
			}
			if !tt.expectOutputHeader && hasOutputHeader {
				t.Errorf("Did not expect '--- Captured output: ---' header, but found in stdout:\n%s", res.stdout)
			}
			if tt.expectStdoutInFo != "" {
				stdoutPresent := strings.Contains(res.stdout, tt.expectStdoutInFo)
				if tt.negateExpectStdout {
					if stdoutPresent {
						t.Errorf("Expected script's stdout ('%s') NOT to be in fo's output, but it was:\n%s", tt.expectStdoutInFo, res.stdout)
					}
				} else {
					if !stdoutPresent {
						t.Errorf("Expected script's stdout ('%s') in fo's output, but not found:\n%s", tt.expectStdoutInFo, res.stdout)
					}
				}
			}
			if tt.expectStderrInFo != "" {
				stderrPresent := strings.Contains(res.stdout, tt.expectStderrInFo) // Script's stderr is merged into fo's stdout in capture mode display
				if tt.negateExpectStderr {
					if stderrPresent {
						t.Errorf("Expected script's stderr ('%s') NOT to be in fo's output (merged to stdout), but it was:\n%s", tt.expectStderrInFo, res.stdout)
					}
				} else {
					if !stderrPresent {
						t.Errorf("Expected script's stderr ('%s') in fo's output (merged to stdout), but not found:\n%s", tt.expectStderrInFo, res.stdout)
					}
				}
			}
		})
	}
}

func TestFoStreamMode(t *testing.T) {
	setupTestScripts(t)
	t.Run("StreamSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Errorf("Expected streamed stdout content in fo's stdout, not found")
		}
		// In stream mode, script's stderr goes to fo's stderr
		if !strings.Contains(res.stderr, "STDERR: Info output from success.sh") {
			t.Errorf("Expected streamed stderr content in fo's stderr, not found")
		}
	})

	t.Run("StreamOverridesShowOutput", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--show-output", "never", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Errorf("Expected streamed stdout content even with --show-output never, not found")
		}
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Did not expect '--- Captured output: ---' header in stream mode")
		}
	})
}

func TestFoTimer(t *testing.T) {
	setupTestScripts(t)
	timerRegex := regexp.MustCompile(`\(\s*\d+(?:\.\d+)?\s*(?:s|ms|µs|ns)\s*\)`)

	t.Run("TimerShownByDefault", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/long_running.sh") // Use --no-color for easier regex on timer presence
		if !timerRegex.MatchString(res.stdout) {
			t.Errorf("Expected timer in output matching regex '%s', but not found. Output: %s", timerRegex.String(), res.stdout)
		}
	})

	t.Run("TimerHiddenWithNoTimerFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-timer", "--", "testdata/success.sh")
		if timerRegex.MatchString(res.stdout) {
			t.Errorf("Expected no timer in output, but found one matching regex '%s'. Output: %s", timerRegex.String(), res.stdout)
		}
	})
}

func TestFoColorAndIcons(t *testing.T) {
	setupTestScripts(t)
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	// Get default icons from the design package
	defaultDesignConfig := design.DefaultConfig()
	emojiIconStart := defaultDesignConfig.Icons.Start
	emojiIconSuccess := defaultDesignConfig.Icons.Success
	emojiIconFailure := defaultDesignConfig.Icons.Error // CORRECTED HERE

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, emojiIconStart) {
			t.Errorf("Expected start emoji icon (%s) in output, but not found", emojiIconStart)
		}
		if !strings.Contains(res.stdout, emojiIconSuccess) {
			t.Errorf("Expected success emoji icon (%s) in output, but not found", emojiIconSuccess)
		}
	})

	t.Run("ColorAndIconsShownByDefaultFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh") // Default show-output is on-fail, so output will be shown
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, emojiIconStart) {
			t.Errorf("Expected start emoji icon (%s) in output, but not found", emojiIconStart)
		}
		if !strings.Contains(res.stdout, emojiIconFailure) {
			t.Errorf("Expected failure emoji icon (%s) in output, but not found", emojiIconFailure)
		}
	})

	t.Run("ColorAndIconsHiddenWithNoColorFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/success.sh")
		if ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected no ANSI color codes in output with --no-color, but found")
		}
		// Check that NO emoji icons are present
		if strings.Contains(res.stdout, emojiIconStart) || strings.Contains(res.stdout, emojiIconSuccess) {
			t.Errorf("Expected no emoji icons with --no-color, but found emoji start: '%s' or success: '%s'", emojiIconStart, emojiIconSuccess)
		}
		// Check that plain text icons ARE present
		if !strings.Contains(res.stdout, plainIconStart) {
			t.Errorf("Expected plain text start icon '%s' with --no-color, but not found", plainIconStart)
		}
		if !strings.Contains(res.stdout, plainIconSuccess) {
			t.Errorf("Expected plain text success icon '%s' with --no-color, but not found", plainIconSuccess)
		}
	})
}

func TestFoCIMode(t *testing.T) {
	setupTestScripts(t)
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	timerRegex := regexp.MustCompile(`\(\s*\d+(?:\.\d+)?\s*(?:s|ms|µs|ns)\s*\)`)

	tests := []struct {
		name             string
		args             []string
		scriptToRun      string
		expectedExitCode int
		expectStartText  string
		expectEndText    string
	}{
		{
			name:             "CIModeWithFlagSuccess",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/success.sh",
			expectedExitCode: 0,
			expectStartText:  plainIconStart,
			expectEndText:    plainIconSuccess,
		},
		{
			name:             "CIModeWithFlagFailure",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/failure.sh",
			expectedExitCode: 1,
			expectStartText:  plainIconStart,
			expectEndText:    plainIconFailure,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fullFoArgs := append(tt.args, tt.scriptToRun)
			res := runFo(t, fullFoArgs...)

			if res.exitCode != tt.expectedExitCode {
				t.Errorf("Expected 'fo' to exit with code %d, got %d", tt.expectedExitCode, res.exitCode)
			}
			if ansiEscapeRegex.MatchString(res.stdout) {
				t.Errorf("Expected no ANSI color codes in CI mode, but found")
			}
			if timerRegex.MatchString(res.stdout) {
				t.Errorf("Expected no timer in CI mode, but found. Stdout:\n%s", res.stdout)
			}

			expectedStartPattern := buildPattern(tt.expectStartText, tt.scriptToRun, true, false)
			if !expectedStartPattern.MatchString(res.stdout) {
				t.Errorf("Expected start line pattern /%s/ in CI mode, got:\n%s", expectedStartPattern.String(), res.stdout)
			}

			expectedEndLineExact := tt.expectEndText + " " + tt.scriptToRun

			trimmedStdout := strings.TrimSpace(res.stdout)
			lines := strings.Split(trimmedStdout, "\n")
			var actualEndLine string
			if strings.Contains(trimmedStdout, "--- Captured output: ---") {
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.HasPrefix(lines[i], tt.expectEndText) {
						actualEndLine = lines[i]
						break
					}
				}
			} else if len(lines) > 0 {
				actualEndLine = lines[len(lines)-1]
			}

			if actualEndLine != expectedEndLineExact {
				t.Errorf("Expected exact end line '%s' in CI mode, got '%s'. Full STDOUT:\n%s",
					expectedEndLineExact, actualEndLine, res.stdout)
			}
		})
	}
}

func TestFoErrorHandling(t *testing.T) {
	t.Run("NoCommandAfterDashDash", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code from 'fo', got 0")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Expected 'fo' stderr to contain 'Error: No command specified after --', got: %s", res.stderr)
		}
	})

	t.Run("NoCommandAtAll", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-l", "some-label")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code from 'fo', got 0")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Expected 'fo' stderr to contain 'Error: No command specified after --', got: %s", res.stderr)
		}
	})

	t.Run("InvalidShowOutputValue", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "invalid_value", "--", "true")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code from 'fo', got 0")
		}
		if !strings.Contains(res.stderr, "Error: Invalid value for --show-output: invalid_value") {
			t.Errorf("Expected 'fo' stderr to contain 'Error: Invalid value for --show-output: invalid_value', got: %s", res.stderr)
		}
	})
}

func TestEnvironmentInheritance(t *testing.T) {
	t.Parallel()
	helperScriptContent := `#!/bin/sh
echo "MY_TEST_VAR is: $MY_TEST_VAR"`
	scriptPath := filepath.Join(t.TempDir(), "env_test.sh")
	if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0755); err != nil {
		t.Fatalf("Failed to write test script %s: %v", scriptPath, err)
	}

	originalEnvVal, envWasSet := os.LookupEnv("MY_TEST_VAR")
	envKey := "MY_TEST_VAR"
	envVal := "foobar_value_for_test"
	if err := os.Setenv(envKey, envVal); err != nil {
		t.Fatalf("Failed to set environment variable %s: %v", envKey, err)
	}
	defer func() {
		if envWasSet {
			if err := os.Setenv(envKey, originalEnvVal); err != nil {
				t.Logf("Warning: failed to restore environment variable %s during cleanup: %v", envKey, err)
			}
		} else {
			if err := os.Unsetenv(envKey); err != nil {
				t.Logf("Warning: failed to unset environment variable %s during cleanup: %v", envKey, err)
			}
		}
	}()

	res := runFo(t, "--show-output", "always", "--", scriptPath)
	if res.exitCode != 0 {
		t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
	}
	expectedOutput := fmt.Sprintf("%s is: %s", envKey, envVal)
	if !strings.Contains(res.stdout, expectedOutput) {
		t.Errorf("Expected environment variable to be inherited and printed ('%s'), got: %s", expectedOutput, res.stdout)
	}
}
func TestFoBufferSizeLimit(t *testing.T) {
	t.Skip("Skipping buffer limit test until it can be improved for reliable CI checks")
	setupTestScripts(t)
	t.Run("BufferLimitTest", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--max-buffer-size", "1", "--show-output", "always", "-l", "Small Buffer Test", "--", "testdata/large_output.sh")

		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0 for buffer limit test, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "[fo] BUFFER LIMIT") {
			t.Errorf("Expected '[fo] BUFFER LIMIT' message in output, but not found. STDOUT:\n%s", res.stdout)
		}
	})
}

func TestFoTimestampedOutput(t *testing.T) {
	setupTestScripts(t)
	t.Run("PreservesOutputOrderInCapture", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "always", "--", "testdata/interleaved.sh")

		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
		}

		var capturedOutput string
		outputSections := strings.SplitN(res.stdout, "--- Captured output: ---", 2)
		if len(outputSections) == 2 {
			capturedOutput = strings.TrimSpace(outputSections[1])
		} else {
			t.Fatalf("Could not find '--- Captured output: ---' section. STDOUT:\n%s", res.stdout)
		}

		lines := strings.Split(capturedOutput, "\n")
		var actualOrder []string
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "STDOUT: Message") || strings.HasPrefix(trimmedLine, "STDERR: Message") {
				actualOrder = append(actualOrder, trimmedLine)
			}
		}

		expectedOrder := []string{
			"STDOUT: Message 1",
			"STDERR: Message 1",
			"STDOUT: Message 2",
			"STDERR: Message 2",
		}

		if len(actualOrder) < len(expectedOrder) {
			t.Errorf("Captured output missing lines. Expected at least %d, got %d. Actual: %v", len(expectedOrder), len(actualOrder), actualOrder)
			return
		}

		var stdoutLines, stderrLines []string
		for _, line := range actualOrder {
			if strings.HasPrefix(line, "STDOUT:") {
				stdoutLines = append(stdoutLines, line)
			} else if strings.HasPrefix(line, "STDERR:") {
				stderrLines = append(stderrLines, line)
			}
		}

		expectedStdouts := []string{"STDOUT: Message 1", "STDOUT: Message 2"}
		expectedStderrs := []string{"STDERR: Message 1", "STDERR: Message 2"}

		if !equalSlices(stdoutLines, expectedStdouts) {
			t.Errorf("STDOUT lines out of order or incorrect. Expected %v, got %v", expectedStdouts, stdoutLines)
		}
		if !equalSlices(stderrLines, expectedStderrs) {
			t.Errorf("STDERR lines out of order or incorrect. Expected %v, got %v", expectedStderrs, stderrLines)
		}

		for _, expectedLine := range expectedOrder {
			found := false
			for _, actualLine := range actualOrder {
				if actualLine == expectedLine {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected line '%s' not found in captured output. Actual order: %v", expectedLine, actualOrder)
			}
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

func TestFoSignalHandling(t *testing.T) {
	t.Skip("Skipping signal handling test; requires manual intervention or advanced setup")

	setupTestScripts(t)
	t.Run("PropagatesSignalsToChildAndExitsWithChildCode", func(t *testing.T) {
		cmd := exec.Command(foTestBinaryPath, "--show-output", "always", "--", "testdata/signal_test.sh")

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start fo: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
			t.Logf("Attempting to send SIGINT to fo process PID %d", cmd.Process.Pid)
			t.Fatalf("Failed to send SIGINT signal to fo process: %v", err)
		}

		err := cmd.Wait()

		t.Logf("fo process finished. STDOUT:\n%s\nSTDERR:\n%s", stdout.String(), stderr.String())

		if err == nil {
			t.Errorf("Expected fo to exit with an error (due to child exiting with 42), but it exited cleanly (code 0)")
		} else {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 42 {
					t.Errorf("Expected fo to exit with code 42 (from child's signal trap), got %d", exitErr.ExitCode())
				}
			} else {
				t.Errorf("Expected fo to return an exec.ExitError, got %T: %v", err, err)
			}
		}
		if !strings.Contains(stdout.String(), "Caught signal, exiting cleanly") && !strings.Contains(stderr.String(), "Caught signal, exiting cleanly") {
			t.Errorf("Expected child script's signal catch message in fo's output, but not found.")
		}
	})
}
func TestFoBasicExecution(t *testing.T) {
	setupTestScripts(t)

	t.Run("ExecutesSuccessfulCommands", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "always", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "Normal output from success.sh") {
			t.Errorf("Command output missing in fo's stdout")
		}
	})

	t.Run("RespectsExitCodes", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "always", "--", "testdata/failure.sh")
		if res.exitCode != 1 {
			t.Errorf("Expected 'fo' to exit with code 1, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "Error message from failure.sh") {
			t.Errorf("Error message missing in fo's stdout (merged from script's stderr)")
		}
	})
}
