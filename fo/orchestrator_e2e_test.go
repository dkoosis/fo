package fo

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleRun_RespectsTimeoutAndCapturesOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	t.Setenv(helperEnvKey, "1")

	var buf bytes.Buffer
	tempDir := t.TempDir()
	designCfg := design.UnicodeVibrantTheme()

	console := NewConsole(ConsoleConfig{
		Design:           designCfg,
		ShowOutputMode:   "always",
		LiveStreamOutput: false,
		Monochrome:       true,
		CaptureDir:       tempDir,
		MaxBufferSize:    1 << 20,
		MaxLineLength:    4096,
		Out:              &buf,
		Err:              &buf,
	})

	command, args := helperCommand(t, "slow-success", "--msg=orchestrator ok", "--sleep-ms=150")

	start := time.Now()
	result, err := console.RunWithContext(ctx, "orchestrator-e2e", command, args...)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, design.StatusSuccess, result.Status)
	assert.Equal(t, 0, result.ExitCode)
	assert.Less(t, result.Duration, 1500*time.Millisecond)
	assert.Less(t, elapsed, 2*time.Second)

	output := buf.String()
	assert.Contains(t, output, designCfg.GetIcon("Success"))
	assert.Contains(t, output, "orchestrator ok")
	assert.NotEmpty(t, result.Lines)
}

func TestConsoleRun_CancelsLongRunningCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	t.Setenv(helperEnvKey, "1")

	var buf bytes.Buffer
	designCfg := design.UnicodeVibrantTheme()

	console := NewConsole(ConsoleConfig{
		Design:           designCfg,
		ShowOutputMode:   "always",
		LiveStreamOutput: false,
		Monochrome:       true,
		MaxBufferSize:    1 << 20,
		MaxLineLength:    4096,
		Out:              &buf,
		Err:              &buf,
	})

	command, args := helperCommand(t, "slow-success", "--sleep-ms=750")

	start := time.Now()
	result, err := console.RunWithContext(ctx, "orchestrator-timeout", command, args...)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, design.StatusError, result.Status)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Less(t, elapsed, 750*time.Millisecond)

	output := buf.String()
	assert.Contains(t, output, designCfg.GetIcon("Error"))
}
