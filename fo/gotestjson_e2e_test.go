package fo

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleRun_ProcessesGoTestJSONEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
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

	command, args := helperCommand(t, "go-test-json", "--exit-code=1")

	result, err := console.RunWithContext(ctx, "go-test-json", command, args...)

	require.Error(t, err)
	require.NotNil(t, result)

	assert.Equal(t, design.StatusError, result.Status)
	assert.Equal(t, 1, result.ExitCode)

	output := buf.String()
	assert.Contains(t, output, designCfg.GetIcon("Error"))
	assert.Contains(t, output, "Passed: 1")
	assert.Contains(t, output, "Failed: 1")
	assert.Contains(t, output, "82%")

	// Ensure design.Task state is populated when processing go test JSON payloads.
	task := design.NewTask("go-test-json", "tests", "go", []string{"test"}, designCfg)
	processor := NewProcessor(design.NewPatternMatcher(designCfg), 4096, false)
	processor.ProcessOutput(task, []byte(strings.Join(sampleGoTestJSONLines(), "\n")), "go", []string{"test"})

	assert.True(t, task.IsTestJSON)
	assert.Empty(t, task.OutputLines)

	task.Complete(1)
	assert.Equal(t, design.StatusError, task.Status)
}
