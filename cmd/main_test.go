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
	err := cmd.Run()
	duration := time.Since(startTime)
	t.Logf("Executed: %s %s (took %v)", foExecutable, strings.Join(fullArgs, " "), duration)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Logf("cmd.Run() returned a non-ExitError: %v", err)
			exitCode = -1
		}
	}

	res := foResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
		err:      err,
	}
	t.Logf("fo stdout:\n%s", res.stdout)
	if res.stderr != "" {
		t.Logf("fo stderr:\n%s", res.stderr)
	}
	t.Logf("fo exitCode: %d", res.exitCode)

	return res
}

func setupTestScripts(t *testing.T) {
	t.Helper()
	scriptsDir := "testdata" // Assuming scripts are in cmd/testdata/
	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		// Corrected line 83 area
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
echo "STDOUT: Testing exit code $1"
echo "STDERR: Will exit with $1" >&2
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
		if err := os.Chmod(path, 0755); err != nil { // Ensure executable
			t.Fatalf("Failed to chmod test script %s: %v", name, err)
		}
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestFoCoreExecution(t *testing.T) {
	setupTestScripts(t) // Ensures testdata and scripts are ready
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
			t.Errorf("Expected exit code 42, got %d", res.exitCode)
		}
	})

	t.Run("CommandNotFound", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "a_very_non_existent_command_dhfjs")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code for command not found, got 0")
		}
		if !strings.Contains(res.stdout, "a_very_non_existent_command_dhfjs") {
			t.Errorf("Expected command name in output for command not found")
		}
		if !strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Expected captured output section for command not found")
		}
		if !strings.Contains(res.stdout, "executable file not found") && !strings.Contains(res.stdout, "No such file or directory") {
			t.Errorf("Expected OS error message for command not found in fo's output, got stdout:\n%s", res.stdout)
		}
	})

	t.Run("ArgumentsToWrappedCommand", func(t *testing.T) {
		t.Parallel()
		helperScriptContent := `#!/bin/sh
echo "Args: $1 $2"`
		scriptPath := filepath.Join(t.TempDir(), "args_test.sh")
		// Corrected line 192 area
		if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0755); err != nil {
			t.Fatalf("Failed to write test script %s: %v", scriptPath, err)
		}
		if err := os.Chmod(scriptPath, 0755); err != nil { // Also ensure it's executable
			t.Fatalf("Failed to chmod test script %s: %v", scriptPath, err)
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
	// setupTestScripts(t) // Not strictly needed if scripts aren't dynamic per label test
	t.Run("DefaultLabelInference", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh") // Use script from testdata
		if !strings.Contains(res.stdout, "▶️ testdata/success.sh...") && !strings.Contains(res.stdout, "[START] testdata/success.sh...") {
			t.Errorf("Expected default label 'testdata/success.sh' in start line, got: %s", res.stdout)
		}
		if !strings.Contains(res.stdout, "✅ testdata/success.sh") && !strings.Contains(res.stdout, "[SUCCESS] testdata/success.sh") {
			t.Errorf("Expected default label 'testdata/success.sh' in end line, got: %s", res.stdout)
		}
	})

	t.Run("CustomLabelShortFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-l", "My Custom Task", "--", "testdata/success.sh")
		if !strings.Contains(res.stdout, "My Custom Task...") {
			t.Errorf("Expected custom label 'My Custom Task' in start line, got: %s", res.stdout)
		}
		if !strings.Contains(res.stdout, "My Custom Task") && !strings.Contains(res.stdout, "My Custom Task (") {
			t.Errorf("Expected custom label 'My Custom Task' in end line, got: %s", res.stdout)
		}
	})

	t.Run("CustomLabelLongFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--label", "Another Task", "--", "testdata/success.sh")
		if !strings.Contains(res.stdout, "Another Task...") {
			t.Errorf("Expected custom label 'Another Task' in start line, got: %s", res.stdout)
		}
		if !strings.Contains(res.stdout, "Another Task") && !strings.Contains(res.stdout, "Another Task (") {
			t.Errorf("Expected custom label 'Another Task' in end line, got: %s", res.stdout)
		}
	})
}

func TestFoCaptureMode(t *testing.T) {
	// setupTestScripts(t) // Ensures testdata scripts exist
	tests := []struct {
		name               string
		showOutputFlag     string
		script             string
		scriptArgs         []string
		expectedExitCode   int
		expectOutputHeader bool
		expectStdout       string
		expectStderr       string
		negateExpectStdout bool
		negateExpectStderr bool
	}{
		{
			name:               "Default (on-fail) Success",
			showOutputFlag:     "",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdout:       "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderr:       "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "on-fail Explicit Success",
			showOutputFlag:     "on-fail",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdout:       "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderr:       "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "on-fail Failure",
			showOutputFlag:     "on-fail",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdout:       "STDOUT: Output from failure.sh before failing",
			expectStderr:       "STDERR: Error message from failure.sh",
		},
		{
			name:               "always Success",
			showOutputFlag:     "always",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: true,
			expectStdout:       "STDOUT: Normal output from success.sh",
			expectStderr:       "STDERR: Info output from success.sh",
		},
		{
			name:               "always Failure",
			showOutputFlag:     "always",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdout:       "STDOUT: Output from failure.sh before failing",
			expectStderr:       "STDERR: Error message from failure.sh",
		},
		{
			name:               "never Success",
			showOutputFlag:     "never",
			script:             "testdata/success.sh",
			expectedExitCode:   0,
			expectOutputHeader: false,
			expectStdout:       "STDOUT: Normal output from success.sh",
			negateExpectStdout: true,
			expectStderr:       "STDERR: Info output from success.sh",
			negateExpectStderr: true,
		},
		{
			name:               "never Failure",
			showOutputFlag:     "never",
			script:             "testdata/failure.sh",
			expectedExitCode:   1,
			expectOutputHeader: false,
			expectStdout:       "STDOUT: Output from failure.sh before failing",
			negateExpectStdout: true,
			expectStderr:       "STDERR: Error message from failure.sh",
			negateExpectStderr: true,
		},
		{
			name:               "Capture Only Stdout",
			showOutputFlag:     "always",
			script:             "testdata/only_stdout.sh",
			expectedExitCode:   0,
			expectOutputHeader: true,
			expectStdout:       "ONLY_STDOUT_CONTENT",
			expectStderr:       "",
		},
		{
			name:               "Capture Only Stderr",
			showOutputFlag:     "always",
			script:             "testdata/only_stderr.sh",
			expectedExitCode:   1,
			expectOutputHeader: true,
			expectStdout:       "",
			expectStderr:       "ONLY_STDERR_CONTENT",
		},
	}

	for _, tt := range tests {
		tt := tt // Corrected: re-scope tt for loopclosure
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
				t.Errorf("Expected exit code %d, got %d", tt.expectedExitCode, res.exitCode)
			}

			hasOutputHeader := strings.Contains(res.stdout, "--- Captured output: ---")
			if tt.expectOutputHeader && !hasOutputHeader {
				t.Errorf("Expected '--- Captured output: ---' header, but not found")
			}
			if !tt.expectOutputHeader && hasOutputHeader {
				t.Errorf("Did not expect '--- Captured output: ---' header, but found")
			}

			stdoutPresent := strings.Contains(res.stdout, tt.expectStdout)
			if tt.expectStdout != "" {
				if tt.negateExpectStdout {
					if stdoutPresent {
						t.Errorf("Expected script's stdout ('%s') NOT to be in fo's output, but it was", tt.expectStdout)
					}
				} else {
					if !stdoutPresent {
						t.Errorf("Expected script's stdout ('%s') in fo's output, but not found", tt.expectStdout)
					}
				}
			}

			stderrPresent := strings.Contains(res.stdout, tt.expectStderr)
			if tt.expectStderr != "" {
				if tt.negateExpectStderr {
					if stderrPresent {
						t.Errorf("Expected script's stderr ('%s') NOT to be in fo's output, but it was", tt.expectStderr)
					}
				} else {
					if !stderrPresent {
						t.Errorf("Expected script's stderr ('%s') in fo's output, but not found", tt.expectStderr)
					}
				}
			}
		})
	}
}

func TestFoStreamMode(t *testing.T) {
	// setupTestScripts(t)
	t.Run("StreamSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
		}
		if !strings.Contains(res.stdout, "STDOUT: Normal output from success.sh") {
			t.Errorf("Expected streamed stdout content, not found")
		}
		// For stream mode, script's stderr goes to fo's stderr
		if !strings.Contains(res.stderr, "STDERR: Info output from success.sh") {
			// This check assumes 'fo' itself isn't printing other things to its own stderr during a successful stream
			// which is generally true. The start/end lines go to fo's stdout.
			t.Errorf("Expected streamed stderr content in fo's stderr, not found. Stderr:\n%s", res.stderr)
		}
		if strings.Contains(res.stdout, "--- Captured output: ---") {
			t.Errorf("Did not expect '--- Captured output: ---' header in stream mode")
		}
	})

	t.Run("StreamOverridesShowOutput", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-s", "--show-output", "never", "--", "testdata/success.sh")
		if res.exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", res.exitCode)
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
	// setupTestScripts(t)
	timerRegex := regexp.MustCompile(`\(\d+(\.\d+)?[a-z]+\)`)

	t.Run("TimerShownByDefault", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/long_running.sh")
		if !timerRegex.MatchString(res.stdout) {
			t.Errorf("Expected timer in output, but not found. Output: %s", res.stdout)
		}
	})

	t.Run("TimerHiddenWithNoTimerFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-timer", "--", "testdata/success.sh")
		if timerRegex.MatchString(res.stdout) {
			t.Errorf("Expected no timer in output, but found. Output: %s", res.stdout)
		}
	})
}

func TestFoColorAndIcons(t *testing.T) {
	// setupTestScripts(t)
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)

	t.Run("ColorAndIconsShownByDefaultSuccess", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/success.sh")
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, "▶️") || !strings.Contains(res.stdout, "✅") {
			t.Errorf("Expected emoji icons (▶️, ✅) in output, but not found")
		}
	})

	t.Run("ColorAndIconsShownByDefaultFailure", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--", "testdata/failure.sh")
		if !ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected ANSI color codes in output, but not found")
		}
		if !strings.Contains(res.stdout, "▶️") || !strings.Contains(res.stdout, "❌") {
			t.Errorf("Expected emoji icons (▶️, ❌) in output, but not found")
		}
	})

	t.Run("ColorAndIconsHiddenWithNoColorFlag", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--no-color", "--", "testdata/success.sh")
		if ansiEscapeRegex.MatchString(res.stdout) {
			t.Errorf("Expected no ANSI color codes in output, but found")
		}
		if strings.Contains(res.stdout, "▶️") || strings.Contains(res.stdout, "✅") {
			t.Errorf("Expected no emoji icons with --no-color, but found")
		}
		if !strings.Contains(res.stdout, "[START]") || !strings.Contains(res.stdout, "[SUCCESS]") {
			t.Errorf("Expected plain text icons '[START]', '[SUCCESS]' with --no-color, but not found")
		}
	})
}

func TestFoCIMode(t *testing.T) {
	// setupTestScripts(t)
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[mK]`)
	timerRegex := regexp.MustCompile(`\(\d+(\.\d+)?[a-z]+\)`)

	tests := []struct {
		name             string
		args             []string
		env              []string
		expectStart      string
		expectEnd        string
		scriptToRun      string // Added to specify which script
		expectedExitCode int    // Added for failure case
	}{
		{
			name:             "CIModeWithFlagSuccess",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/success.sh",
			expectStart:      "[START]",
			expectEnd:        "[SUCCESS]",
			expectedExitCode: 0,
		},
		{
			name:             "CIModeWithFlagFailure",
			args:             []string{"--ci", "--"},
			scriptToRun:      "testdata/failure.sh",
			expectStart:      "[START]",
			expectEnd:        "[FAILED]",
			expectedExitCode: 1,
		},
	}

	for _, tt := range tests {
		tt := tt // Corrected: re-scope tt for loopclosure
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmdArgs := append(tt.args, tt.scriptToRun)
			res := runFo(t, cmdArgs...)

			if res.exitCode != tt.expectedExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.expectedExitCode, res.exitCode)
			}
			if ansiEscapeRegex.MatchString(res.stdout) {
				t.Errorf("Expected no ANSI color codes in CI mode, but found")
			}
			if timerRegex.MatchString(res.stdout) {
				t.Errorf("Expected no timer in CI mode, but found")
			}
			// Check if the start line contains the expected prefix and the script name (label)
			expectedStartLine := fmt.Sprintf("%s %s...", tt.expectStart, tt.scriptToRun)
			if !strings.Contains(res.stdout, expectedStartLine) {
				t.Errorf("Expected start line '%s' in CI mode, but not found. Output:\n%s", expectedStartLine, res.stdout)
			}
			// Check if the end line contains the expected prefix and the script name (label)
			// Example: [SUCCESS] testdata/success.sh
			expectedEndLine := fmt.Sprintf("%s %s", tt.expectEnd, tt.scriptToRun) // Timer is not present
			if !strings.Contains(res.stdout, expectedEndLine) {
				t.Errorf("Expected end line containing '%s' in CI mode, but not found. Output:\n%s", expectedEndLine, res.stdout)
			}
		})
	}
}

func TestFoErrorHandling(t *testing.T) {
	t.Run("NoCommandAfterDashDash", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code, got 0")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Expected error message 'Error: No command specified after --' in stderr, got: %s", res.stderr)
		}
	})

	t.Run("NoCommandAtAll", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "-l", "some-label")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code, got 0")
		}
		if !strings.Contains(res.stderr, "Error: No command specified after --") {
			t.Errorf("Expected error message 'Error: No command specified after --' in stderr, got: %s", res.stderr)
		}
	})

	t.Run("InvalidShowOutputValue", func(t *testing.T) {
		t.Parallel()
		res := runFo(t, "--show-output", "invalid_value", "--", "true")
		if res.exitCode == 0 {
			t.Errorf("Expected non-zero exit code, got 0")
		}
		if !strings.Contains(res.stderr, "Error: Invalid value for --show-output: invalid_value") {
			t.Errorf("Expected error message for invalid --show-output value in stderr, got: %s", res.stderr)
		}
	})
}

func TestEnvironmentInheritance(t *testing.T) {
	t.Parallel()
	helperScriptContent := `#!/bin/sh
echo "MY_TEST_VAR is: $MY_TEST_VAR"`
	scriptPath := filepath.Join(t.TempDir(), "env_test.sh")
	// Corrected line 615 area
	if err := os.WriteFile(scriptPath, []byte(helperScriptContent), 0755); err != nil {
		t.Fatalf("Failed to write test script %s: %v", scriptPath, err)
	}
	if err := os.Chmod(scriptPath, 0755); err != nil {
		t.Fatalf("Failed to chmod test script %s: %v", scriptPath, err)
	}

	// Corrected line 619 & 620 area
	if err := os.Setenv("MY_TEST_VAR", "foobar_value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("MY_TEST_VAR"); err != nil {
			t.Logf("Warning: failed to unset environment variable during cleanup: %v", err)
		}
	}()

	res := runFo(t, "--show-output", "always", "--", scriptPath)
	if res.exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", res.exitCode)
	}
	if !strings.Contains(res.stdout, "MY_TEST_VAR is: foobar_value") {
		t.Errorf("Expected environment variable to be inherited and printed, got: %s", res.stdout)
	}
}
