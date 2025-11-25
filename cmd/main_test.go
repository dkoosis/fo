package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"os/exec"
	"testing"

	config "github.com/dkoosis/fo/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelperProcess is used as a subprocess target for tests that expect os.Exit.
func TestHelperProcess(t *testing.T) {
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
		// Use an isolated FlagSet to avoid leaking state.
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		flag.CommandLine.SetOutput(io.Discard)
		os.Args = append([]string{os.Args[0]}, helperArgs...)
		parseGlobalFlags()
	case "printCommand":
		handlePrintCommand(helperArgs)
	}
	os.Exit(0)
}

func TestConvertAppConfigToLocal_MirrorsValues_When_AppConfigHasOverrides(t *testing.T) {
	t.Parallel()

	appCfg := &config.AppConfig{
		Label:         "custom-label",
		Stream:        true,
		ShowOutput:    "always",
		NoTimer:       true,
		NoColor:       true,
		CI:            true,
		Debug:         true,
		MaxBufferSize: 42,
		MaxLineLength: 7,
	}

	local := convertAppConfigToLocal(appCfg)

	assert.Equal(t, appCfg.Label, local.Label)
	assert.Equal(t, appCfg.Stream, local.Stream)
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Global os.Args use makes tests unsafe for parallel execution.
			originalArgs := os.Args
			os.Args = tc.args
			defer func() { os.Args = originalArgs }()

			got := findCommandArgs()
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestParseGlobalFlags_SetsCliFields_When_ValidInputsProvided(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		assertFunc func(t *testing.T, flags config.CliFlags, version bool)
	}{
		{
			name: "ParsesAllFlags_When_ValuesAreValid",
			args: []string{"fo", "--label", "task", "--stream", "--show-output", "always", "--pattern", "sparkline", "--format", "json", "--no-timer", "--no-color", "--ci", "--debug", "--max-buffer-size", "2", "--max-line-length", "1"},
			assertFunc: func(t *testing.T, flags config.CliFlags, version bool) {
				require.False(t, version)
				assert.Equal(t, "task", flags.Label)
				assert.True(t, flags.Stream)
				assert.True(t, flags.StreamSet)
				assert.Equal(t, "always", flags.ShowOutput)
				assert.True(t, flags.ShowOutputSet)
				assert.Equal(t, "sparkline", flags.Pattern)
				assert.True(t, flags.PatternSet)
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
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// parseGlobalFlags mutates global flag state; isolate per subtest.
			originalArgs := os.Args
			originalCommandLine := flag.CommandLine
			os.Args = tc.args
			flag.CommandLine = flag.NewFlagSet(tc.args[0], flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)

			t.Cleanup(func() {
				os.Args = originalArgs
				flag.CommandLine = originalCommandLine
			})

			flags, version := parseGlobalFlags()
			tc.assertFunc(t, flags, version)
		})
	}
}

func TestParseGlobalFlags_Exits_When_ShowOutputInvalid(t *testing.T) {
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
				assert.Contains(t, stdout, "fo version")
			},
		},
		{
			name:     "FailsWithUsageMessage_When_CommandMissingAfterDelimiter",
			args:     []string{"fo", "--"},
			wantExit: 1,
			checkStderr: func(t *testing.T, stderr string) {
				assert.Contains(t, stderr, "No command specified")
			},
		},
		{
			name:     "RunsCommand_When_ArgumentsProvided",
			args:     []string{"fo", "--", "/bin/echo", "ok"},
			wantExit: 0,
			checkStdout: func(t *testing.T, stdout string) {
				assert.Contains(t, stdout, "Complete")
			},
		},
		{
			name:     "ReturnsJSONOutput_When_FormatIsJSON",
			args:     []string{"fo", "--format", "json", "--", "/bin/echo", "ok"},
			wantExit: 0,
			checkStdout: func(t *testing.T, stdout string) {
				assert.Contains(t, stdout, "\"exit_code\": 0")
			},
		},
		{
			name:     "EmitsDebugLogs_When_DebugFlagEnabled",
			args:     []string{"fo", "--debug", "--", "/bin/true"},
			wantExit: 0,
			checkStderr: func(t *testing.T, stderr string) {
				assert.Contains(t, stderr, "[DEBUG run()]")
			},
		},
	}

	for _, tc := range tests {
		tc := tc
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
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess", "--", "--type", "raw", "--", "Hello")
	cmd.Env = append(os.Environ(), "FO_TEST_HELPER=1", "FO_HELPER_MODE=printCommand")

	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(output), "Hello")
}
