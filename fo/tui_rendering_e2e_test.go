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

func TestConsoleRunSections_RendersStatusIcons(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	t.Setenv(helperEnvKey, "1")

	var buf bytes.Buffer
	designCfg := design.UnicodeVibrantTheme()

	console := NewConsole(ConsoleConfig{
		Design:           designCfg,
		ShowOutputMode:   "on-fail",
		LiveStreamOutput: false,
		Monochrome:       true,
		Out:              &buf,
		Err:              &buf,
	})

	successCmd, successArgs := helperCommand(t, "success", "--msg=section success")
	failCmd, failArgs := helperCommand(t, "failure")

	sections := []Section{
		{
			Name: "tui-success",
			Run: func() error {
				_, err := console.RunWithContext(ctx, "tui-success", successCmd, successArgs...)
				return err
			},
			Summary: "completed rendering happy path",
		},
		{
			Name: "tui-failure",
			Run: func() error {
				_, err := console.RunWithContext(ctx, "tui-failure", failCmd, failArgs...)
				return err
			},
		},
	}

	results, err := console.RunSections(sections...)
	require.Error(t, err)
	require.Len(t, results, 2)

	output := buf.String()
	assert.Contains(t, output, designCfg.GetIcon("Success"))
	assert.Contains(t, output, designCfg.GetIcon("Error"))
	assert.Contains(t, output, "completed rendering happy path")
}
