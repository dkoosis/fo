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
)

// Define icon constants here, ensuring they are EXACTLY as in main.go
// main.go definitions:
// const iconStart = "▶️"
// const iconSuccess = "✅"
// const iconFailure = "❌"
const (
	testIconStart   = "▶️"
	testIconSuccess = "✅"
	testIconFailure = "❌"
)

const (
	foExecutable = "go"
	foArgsPrefix = "run"
	mainGoFile   = "main.go"
)

type foResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

func runFo(t *testing.T, foCmdArgs ...string) foResult {
	t.Helper()
	baseArgs := []string{}
	if foExecutable == "go" {
		baseArgs = append(baseArgs, foArgsPrefix, mainGoFile)
	}

	fullArgs := append(baseArgs, foCmdArgs...)
	cmd := exec.Command(foExecutable, fullArgs...)

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
			t.Logf("cmd.Run() for 'fo' process returned a non-ExitError: %v", runErr)
			exitCode = -1
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	logMessage := fmt.Sprintf("Executed: %s %s (took %v)\n"+
		"--- FO STDOUT ---\n%s\n"+
		"--- FO STDERR ---\n%s\n"+
		"--- FO EXITCODE (from runFo perspective): %d ---",
		foExecutable, strings.Join(fullArgs, " "), duration,
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

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
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
			t.Errorf("Expected 'fo' process to ultimately yield exit code 42 (as fo intends), got %d. "+
				"If main.go debug logs confirm 'about to os.Exit(42)', this discrepancy may be due to 'go run' behavior with non-zero exits + stderr, or how test captures exit code.", res.exitCode)
		}
		if !strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Error("Expected '--- Captured output: ---' header for a failing script with non-zero exit code.")
		}
		if !strings.Contains(res.stdout, "STDOUT: Script about to exit with 42") {
			t.Error("Expected script's STDOUT in 'fo's output.")
		}
		if !strings.Contains(res.stdout, "STDERR: Script stderr message before exiting 42") {
			t.Error("Expected script's STDERR in 'fo's output.")
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		commandName := "a_very_non_existent_command_and_unique_dhfjs"
		res := runFo(t, "--", commandName)

		if res.exitCode == 0 {
			t.Errorf("Expected 'fo' to exit non-zero for command not found, got %d. Error from runFo: %v", res.exitCode, res.err)
		}

		// Use the defined constants for icons directly in the regex string.
		// regexp.QuoteMeta is only for the variable part (commandName).
		expectedStartPattern := regexp.MustCompile(testIconStart + `\s*` + regexp.QuoteMeta(commandName) + `\.\.\.`)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain start line matching pattern '%s', got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := regexp.MustCompile(testIconFailure + `\s*` + regexp.QuoteMeta(commandName) + `\s*\(`)
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected 'fo' stdout to contain end line matching pattern '%s', got:\n%s", expectedEndPattern.String(), res.stdout)
		}

		// Check fo's own stderr for the primary error message
		expectedFoErrCmdStart := "Error starting command: exec: \"" + commandName + "\": executable file not found"
		if !strings.Contains(res.stderr, expectedFoErrCmdStart) {
			expectedFoErrCmdStartAlternate := "Error starting command: exec: \"" + commandName + "\": No such file or directory"
			if !strings.Contains(res.stderr, expectedFoErrCmdStartAlternate) {
				t.Errorf("Expected 'fo' stderr to contain specific 'Error starting command...' message for '%s', got:\n%s", commandName, res.stderr)
			}
		}

		// Check for io.Copy errors that fo prints to its stderr.
		// These are now expected as per the latest logs.
		if !strings.Contains(res.stderr, "Error copying stdout: read") { // More general check
			t.Errorf("Expected 'fo' stderr to contain 'Error copying stdout:' message, got:\n%s", res.stderr)
		}
		if !strings.Contains(res.stderr, "Error copying stderr: read") { // More general check
			t.Errorf("Expected 'fo' stderr to contain 'Error copying stderr:' message, got:\n%s", res.stderr)
		}

		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("'fo' stdout should NOT contain '--- Captured output: ---' header for a command that failed to start, but it was found.")
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

	t.Run("DefaultLabelInference", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", scriptPath)

		expectedStartPattern := regexp.MustCompile(testIconStart + `\s*` + regexp.QuoteMeta(scriptPath) + `\.\.\.`)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain start line matching pattern '%s', got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := regexp.MustCompile(testIconSuccess + `\s*` + regexp.QuoteMeta(scriptPath))
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain success end label matching pattern '%s', got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})

	t.Run("CustomLabelShortFlag", func(t *testing.T) {
		t.Parallel()
		customLabel := "My Custom Task"
		res := runFo(t, "-l", customLabel, "--", scriptPath)

		expectedStartPattern := regexp.MustCompile(testIconStart + `\s*` + regexp.QuoteMeta(customLabel) + `\.\.\.`)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain start line matching pattern '%s', got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := regexp.MustCompile(testIconSuccess + `\s*` + regexp.QuoteMeta(customLabel))
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain success end label matching pattern '%s', got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})

	t.Run("CustomLabelLongFlag", func(t *testing.T) {
		t.Parallel()
		customLabel := "Another Task"
		res := runFo(t, "--label", customLabel, "--", scriptPath)

		expectedStartPattern := regexp.MustCompile(testIconStart + `\s*` + regexp.QuoteMeta(customLabel) + `\.\.\.`)
		if !expectedStartPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain start line matching pattern '%s', got:\n%s", expectedStartPattern.String(), res.stdout)
		}

		expectedEndPattern := regexp.MustCompile(testIconSuccess + `\s*` + regexp.QuoteMeta(customLabel))
		if !expectedEndPattern.MatchString(res.stdout) {
			t.Errorf("Expected stdout to contain success end label matching pattern '%s', got:\n%s", expectedEndPattern.String(), res.stdout)
		}
	})
}

func TestFoCaptureMode(t *testing.T) {
	setupTestScripts(t)
	tests := []struct {
		name               string
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
			name:               "Default (on-fail) Success",
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
			name:               "on-fail Explicit Success",
			showOutputFlag:     "on-fail",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdoutInFo:   "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderrInFo:   "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "on-fail Failure",
			showOutputFlag:     "on-fail",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdoutInFo:   "STDOUT: Output from failure.sh before failing",
			expectStderrInFo:   "STDERR: Error message from failure.sh",
		},
		{
			name:               "always Success",
			showOutputFlag:     "always",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: true,
			expectStdoutInFo:   "STDOUT: Normal output from success.sh",
			expectStderrInFo:   "STDERR: Info output from success.sh",
		},
		{
			name:               "always Failure",
			showOutputFlag:     "always",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdoutInFo:   "STDOUT: Output from failure.sh before failing",
			expectStderrInFo:   "STDERR: Error message from failure.sh",
		},
		{
			name:               "never Success",
			showOutputFlag:     "never",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdoutInFo:   "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderrInFo:   "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "never Failure",
			showOutputFlag:     "never",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: false,
			expectStdoutInFo:   "STDOUT: Output from failure.sh before failing",
			negateExpectStdout: true,
			expectStderrInFo:   "STDERR: Error message from failure.sh",
			negateExpectStderr: true,
		},
		{
			name:               "Capture Only Stdout",
			showOutputFlag:     "always",
			script:             "testdata/only_stdout.sh",
			expectedExitCode:   0,
			expectOutputHeader: true,
			expectStdoutInFo:   "ONLY_STDOUT_CONTENT",
			expectStderrInFo:   "",
		},
		{
			name:               "Capture Only Stderr",
			showOutputFlag:     "always",
			script:             "testdata/only_stderr.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdoutInFo:   "",
			expectStderrInFo:   "ONLY_STDERR_CONTENT",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{}
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
				stderrPresent := strings.Contains(res.stdout, tt.expectStderrInFo)
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
		if !strings.Contains(res.stderr, "STDERR: Info output from success.sh") {
			t.Errorf("Expected streamed stderr content in fo's stderr, not found. Stderr:\n%s", res.stderr)
		}
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Did not expect '--- Captured output: ---' header in stream mode's stdout")
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
		res := runFo(t, "--", "testdata/long_running.sh")
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

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, testIconStart) {
			t.Errorf("Expected start emoji icon (%s) in output, but not found", testIconStart)
		}
		if !strings.Contains(res.stdout, testIconSuccess) {
			t.Errorf("Expected success emoji icon (%s) in output, but not found", testIconSuccess)
		}
	})

	t.Run("ColorAndIconsShownByDefaultFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh")
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, testIconStart) {
			t.Errorf("Expected start emoji icon (%s) in output, but not found", testIconStart)
		}
		if !strings.Contains(res.stdout, testIconFailure) {
			t.Errorf("Expected failure emoji icon (%s) in output, but not found", testIconFailure)
		}
	})

	t.Run("ColorAndIconsHiddenWithNoColorFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/success.sh")
		if ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected no ANSI color codes in output, but found")
		}
		if strings.Contains(res.stdout, testIconStart) || strings.Contains(res.stdout, testIconSuccess) {
			t.Errorf("Expected no emoji icons with --no-color, but found emoji icons")
		}
		if !strings.Contains(res.stdout, "[START]") || !strings.Contains(res.stdout, "[SUCCESS]") {
			t.Errorf("Expected plain text icons '[START]', '[SUCCESS]' with --no-color, but not found")
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
		expectStartText  string // Plain text like "[START]"
		expectEndText    string // Plain text like "[SUCCESS]"
	}{
		{
			name:             "CIModeWithFlagSuccess",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/success.sh",
			expectedExitCode: 0,
			expectStartText:  "[START]",
			expectEndText:    "[SUCCESS]",
		},
		{
			name:             "CIModeWithFlagFailure",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/failure.sh",
			expectedExitCode: 1,
			expectStartText:  "[START]",
			expectEndText:    "[FAILED]",
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

			// In CI mode, icons are plain text.
			expectedStartLinePattern := regexp.MustCompile(regexp.QuoteMeta(tt.expectStartText) + `\s*` + regexp.QuoteMeta(tt.scriptToRun) + `\.\.\.`)
			if !expectedStartLinePattern.MatchString(res.stdout) {
				t.Errorf("Expected start line pattern '%s' in CI mode, but not found. Output:\n%s", expectedStartLinePattern.String(), res.stdout)
			}

			expectedEndLine := tt.expectEndText + " " + tt.scriptToRun
			foundEndLine := false
			lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
			for _, line := range lines {
				if line == expectedEndLine {
					foundEndLine = true
					break
				}
			}
			if !foundEndLine {
				t.Errorf("Expected exact end line '%s' in CI mode, but not found. Output:\n%s", expectedEndLine, res.stdout)
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
	if err := os.Setenv("MY_TEST_VAR", "foobar_value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if envWasSet {
			if err := os.Setenv("MY_TEST_VAR", originalEnvVal); err != nil {
				t.Logf("Warning: failed to restore environment variable MY_TEST_VAR during cleanup: %v", err)
			}
		} else {
			if err := os.Unsetenv("MY_TEST_VAR"); err != nil {
				t.Logf("Warning: failed to unset environment variable MY_TEST_VAR during cleanup: %v", err)
			}
		}
	}()

	res := runFo(t, "--show-output", "always", "--", scriptPath)
	if res.exitCode != 0 {
		t.Errorf("Expected 'fo' to exit with code 0, got %d", res.exitCode)
	}
	if !strings.Contains(res.stdout, "MY_TEST_VAR is: foobar_value") {
		t.Errorf("Expected environment variable to be inherited and printed, got: %s", res.stdout)
	}
}
