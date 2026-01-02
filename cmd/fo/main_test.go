package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	config "github.com/dkoosis/fo/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess is used as a subprocess target for tests that expect os.Exit.
func TestHelperProcess(_ *testing.T) {
	if os.Getenv("FO_TEST_HELPER") == "" {
		return
	}

	helperArgs := os.Args[1:]
	if len(helperArgs) > 0 && helperArgs[0] == "-test.run=TestHelperProcess" {
		helperArgs = helperArgs[1:]
	}
	if len(helperArgs) > 0 && helperArgs[0] == "--" {
		helperArgs = helperArgs[1:]
	}

	mode := os.Getenv("FO_HELPER_MODE")
	switch mode {
	case "parseGlobalFlags":
		// Use parseGlobalFlagsFromArgs with constructed args
		args := append([]string{"fo"}, helperArgs...)
		_, _, _, err := parseGlobalFlagsFromArgs(args)
		if err != nil {
			os.Exit(1)
		}
	case "printCommand":
		exitCode := runPrintSubcommand(helperArgs)
		os.Exit(exitCode)
	}
	os.Exit(0)
}

func TestConvertAppConfigToLocal_MirrorsValues_When_AppConfigHasOverrides(t *testing.T) {
	t.Parallel()

	appCfg := &config.AppConfig{
		Label:            "custom-label",
		LiveStreamOutput: true,
		ShowOutput:       "always",
		NoTimer:          true,
		NoColor:          true,
		CI:               true,
		Debug:            true,
		MaxBufferSize:    42,
		MaxLineLength:    7,
	}

	local := convertAppConfigToLocal(appCfg)

	assert.Equal(t, appCfg.Label, local.Label)
	assert.Equal(t, appCfg.LiveStreamOutput, local.LiveStreamOutput)
	assert.Equal(t, appCfg.ShowOutput, local.ShowOutput)
	assert.Equal(t, appCfg.NoTimer, local.NoTimer)
	assert.Equal(t, appCfg.NoColor, local.NoColor)
	assert.Equal(t, appCfg.CI, local.CI)
	// Debug must start disabled regardless of AppConfig value; only CLI enables it.
	assert.False(t, local.Debug)
	assert.Equal(t, appCfg.MaxBufferSize, local.MaxBufferSize)
	assert.Equal(t, appCfg.MaxLineLength, local.MaxLineLength)
}

func TestFindCommandArgs_ReturnsExpected_When_DelimiterScenariosVary(t *testing.T) {
	t.Parallel() // Now safe for parallel since we don't mutate os.Args

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "ReturnsCommandAndArgs_When_DoubleDashPresent",
			args: []string{"fo", "--", "echo", "hi"},
			want: []string{"echo", "hi"},
		},
		{
			name: "ReturnsEmptySlice_When_NoDelimiterProvided",
			args: []string{"fo", "stream", "value"},
			want: []string{},
		},
		{
			name: "ReturnsEmptySlice_When_DoubleDashLast",
			args: []string{"fo", "--"},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findCommandArgsFromSlice(tc.args)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseGlobalFlags_SetsCliFields_When_ValidInputsProvided(t *testing.T) {
	t.Parallel() // Now safe for parallel since we use FlagSet

	tests := []struct {
		name       string
		args       []string
		assertFunc func(t *testing.T, flags config.CliFlags, version bool, cmdArgs []string)
	}{
		{
			name: "ParsesAllFlags_When_ValuesAreValid",
			args: []string{
				"fo", "--label", "task", "--stream", "--show-output", "always",
				"--pattern", "sparkline", "--format", "json", "--no-timer",
				"--no-color", "--ci", "--debug", "--max-buffer-size", "2",
				"--max-line-length", "1",
			},
			assertFunc: func(t *testing.T, flags config.CliFlags, version bool, _ []string) {
				t.Helper()
				require.False(t, version)
				assert.Equal(t, "task", flags.Label)
				assert.True(t, flags.LiveStreamOutput)
				assert.True(t, flags.LiveStreamOutputSet)
				assert.Equal(t, "always", flags.ShowOutput)
				assert.True(t, flags.ShowOutputSet)
				assert.Equal(t, "sparkline", flags.PatternHint)
				assert.True(t, flags.PatternHintSet)
				assert.Equal(t, "json", flags.Format)
				assert.True(t, flags.NoTimer)
				assert.True(t, flags.NoTimerSet)
				assert.True(t, flags.NoColor)
				assert.True(t, flags.NoColorSet)
				assert.True(t, flags.CI)
				assert.True(t, flags.CISet)
				assert.True(t, flags.Debug)
				assert.True(t, flags.DebugSet)
				assert.Equal(t, int64(2*1024*1024), flags.MaxBufferSize)
				assert.Equal(t, 1*1024, flags.MaxLineLength)
			},
		},
		{
			name: "ParsesCommandArgs_When_DoubleDashPresent",
			args: []string{"fo", "--debug", "--", "echo", "hello"},
			assertFunc: func(t *testing.T, flags config.CliFlags, version bool, cmdArgs []string) {
				t.Helper()
				require.False(t, version)
				assert.True(t, flags.Debug)
				assert.Equal(t, []string{"echo", "hello"}, cmdArgs)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			flags, version, cmdArgs, err := parseGlobalFlagsFromArgs(tc.args)
			require.NoError(t, err)
			tc.assertFunc(t, flags, version, cmdArgs)
		})
	}
}

func TestParseGlobalFlags_Exits_When_ShowOutputInvalid(t *testing.T) {
	// #nosec G204 -- Standard Go test pattern: subprocess call uses os.Args[0] (test binary)
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "--show-output", "sometimes")
	cmd.Env = append(os.Environ(), "FO_TEST_HELPER=1", "FO_HELPER_MODE=parseGlobalFlags")

	err := cmd.Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.NotEqual(t, 0, exitErr.ExitCode())
}

func TestRun_ManagesExecutionFlow_When_DifferentInputsProvided(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantExit    int
		checkStdout func(t *testing.T, stdout string)
		checkStderr func(t *testing.T, stderr string)
	}{
		{
			name:     "ReturnsVersionInfo_When_VersionFlagProvided",
			args:     []string{"fo", "--version"},
			wantExit: 0,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Contains(t, stdout, "fo version")
			},
		},
		{
			name:     "FailsWithUsageMessage_When_CommandMissingAfterDelimiter",
			args:     []string{"fo", "--"},
			wantExit: 1,
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				assert.Contains(t, stderr, "No command specified")
			},
		},
		{
			name:     "RunsCommand_When_ArgumentsProvided",
			args:     []string{"fo", "--", "echo", "ok"},
			wantExit: 0,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				// Check for success indicator (could be [SUCCESS] or Complete depending on theme)
				assert.True(t,
					strings.Contains(stdout, "[SUCCESS]") ||
						strings.Contains(stdout, "Complete") ||
						strings.Contains(stdout, "[OK]"),
					"Expected success indicator, got: %s", stdout)
			},
		},
		{
			name:     "ReturnsJSONOutput_When_FormatIsJSON",
			args:     []string{"fo", "--format", "json", "--", "echo", "ok"},
			wantExit: 0,
			checkStdout: func(t *testing.T, stdout string) {
				t.Helper()
				assert.Contains(t, stdout, "\"exit_code\": 0")
			},
		},
		{
			name:     "EmitsDebugLogs_When_DebugFlagEnabled",
			args:     []string{"fo", "--debug", "--", "echo", "test"},
			wantExit: 0,
			checkStderr: func(t *testing.T, stderr string) {
				t.Helper()
				assert.Contains(t, stderr, "[DEBUG run()]")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// run() relies on os.Args and global flags; isolate state.
			originalArgs := os.Args
			originalStdout := os.Stdout
			originalStderr := os.Stderr
			originalCommandLine := flag.CommandLine

			stdoutReader, stdoutWriter, _ := os.Pipe()
			stderrReader, stderrWriter, _ := os.Pipe()
			os.Stdout = stdoutWriter
			os.Stderr = stderrWriter
			os.Args = tc.args
			flag.CommandLine = flag.NewFlagSet(tc.args[0], flag.ExitOnError)
			flag.CommandLine.SetOutput(io.Discard)

			t.Cleanup(func() {
				os.Args = originalArgs
				os.Stdout = originalStdout
				os.Stderr = originalStderr
				flag.CommandLine = originalCommandLine
			})

			exitCode := run(tc.args)

			// Close writers to allow reads to finish.
			require.NoError(t, stdoutWriter.Close())
			require.NoError(t, stderrWriter.Close())

			stdoutBuf := new(bytes.Buffer)
			stderrBuf := new(bytes.Buffer)
			_, _ = io.Copy(stdoutBuf, stdoutReader)
			_, _ = io.Copy(stderrBuf, stderrReader)

			assert.Equal(t, tc.wantExit, exitCode)
			if tc.checkStdout != nil {
				tc.checkStdout(t, stdoutBuf.String())
			}
			if tc.checkStderr != nil {
				tc.checkStderr(t, stderrBuf.String())
			}
		})
	}
}

func TestHandlePrintCommand_RendersMessage_When_ArgumentsValid(t *testing.T) {
	// #nosec G204 -- Standard Go test pattern: subprocess call uses os.Args[0] (test binary)
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "--type", "raw", "--", "Hello")
	cmd.Env = append(os.Environ(), "FO_TEST_HELPER=1", "FO_HELPER_MODE=printCommand")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(output), "Hello")
}
