package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask_ProvidesStatusBlockData_When_StatusChanges(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := &Task{Label: "build", Config: cfg}

	tests := []struct {
		name      string
		status    string
		wantText  string
		wantColor string
		wantIcon  string
	}{
		{
			name:      "success defaults to complete",
			status:    StatusSuccess,
			wantText:  "Complete",
			wantColor: ColorKeySuccess,
			wantIcon:  cfg.GetIcon("Success"),
		},
		{
			name:      "warning uses warning styling",
			status:    StatusWarning,
			wantText:  "Completed with warnings",
			wantColor: ColorKeyWarning,
			wantIcon:  cfg.GetIcon("Warning"),
		},
		{
			name:      "error falls back to error styling",
			status:    StatusError,
			wantText:  "Failed",
			wantColor: ColorKeyError,
			wantIcon:  cfg.GetIcon("Error"),
		},
		{
			name:      "info uses process color",
			status:    TypeInfo,
			wantText:  "Done",
			wantColor: ColorKeyProcess,
			wantIcon:  cfg.GetIcon("Info"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			task.Status = tc.status
			data := task.getStatusBlockData()

			assert.Equal(t, tc.wantText, data.text)
			assert.Equal(t, tc.wantColor, data.colorKey)
			assert.Equal(t, tc.wantIcon, data.icon)
		})
	}
}

func TestTask_RendersEndLine_When_InMonochromeMode(t *testing.T) {
	t.Parallel()

	cfg := NoColorConfig()
	task := &Task{Label: "package", Config: cfg, Status: StatusSuccess}

	line := task.RenderEndLine()

	assert.Contains(t, line, cfg.GetIcon("Success"))
	assert.NotContains(t, line, "\u001b[")
}

func TestTask_RendersEndLine_When_UsingThemedConfig(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := &Task{Label: "package", Config: cfg, Status: StatusError}

	line := task.RenderEndLine()

	assert.Contains(t, line, cfg.GetIcon("Error"))
	assert.Contains(t, line, cfg.GetColor(ColorKeyError))
}
