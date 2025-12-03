package design

import (
	"strings"
	"testing"
	"time"
)

func TestTaskView_RenderStart(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(60)

	data := TaskData{
		Label:  "Build",
		Status: "running",
	}

	result := view.RenderStart(data)

	// Should contain uppercase label
	if !strings.Contains(result, "BUILD") {
		t.Error("expected start render to contain uppercase label 'BUILD'")
	}

	// Should contain running indicator
	if !strings.Contains(result, "Running") {
		t.Error("expected start render to contain 'Running'")
	}

	// Should have box borders
	if !strings.Contains(result, "╭") && !strings.Contains(result, "┌") {
		t.Error("expected start render to have box borders")
	}
}

func TestTaskView_RenderComplete_Success(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(60)

	data := TaskData{
		Label:    "Test",
		Status:   "success",
		Duration: 150 * time.Millisecond,
	}

	result := view.RenderComplete(data)

	// Should contain label
	if !strings.Contains(result, "TEST") {
		t.Error("expected complete render to contain label")
	}

	// Should contain success indicator
	if !strings.Contains(result, theme.Icons.Success) {
		t.Errorf("expected complete render to contain success icon %q", theme.Icons.Success)
	}

	// Should contain "Complete"
	if !strings.Contains(result, "Complete") {
		t.Error("expected complete render to contain 'Complete'")
	}

	// Should contain duration
	if !strings.Contains(result, "150ms") {
		t.Error("expected complete render to contain duration")
	}
}

func TestTaskView_RenderComplete_Error(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(60)

	data := TaskData{
		Label:    "Build",
		Status:   "error",
		Duration: 2 * time.Second,
	}

	result := view.RenderComplete(data)

	// Should contain error indicator
	if !strings.Contains(result, theme.Icons.Error) {
		t.Errorf("expected error render to contain error icon %q", theme.Icons.Error)
	}

	// Should contain "Failed"
	if !strings.Contains(result, "Failed") {
		t.Error("expected error render to contain 'Failed'")
	}
}

func TestTaskView_RenderComplete_WithLines(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(60)

	data := TaskData{
		Label:     "Test",
		Status:    "warning",
		Duration:  500 * time.Millisecond,
		ShowLines: true,
		Lines: []LineData{
			{Content: "Test 1 passed", Type: "success"},
			{Content: "Test 2 skipped", Type: "warning"},
			{Content: "Test 3 failed", Type: "error"},
		},
	}

	result := view.RenderComplete(data)

	// Should contain all line contents
	if !strings.Contains(result, "Test 1 passed") {
		t.Error("expected result to contain 'Test 1 passed'")
	}
	if !strings.Contains(result, "Test 2 skipped") {
		t.Error("expected result to contain 'Test 2 skipped'")
	}
	if !strings.Contains(result, "Test 3 failed") {
		t.Error("expected result to contain 'Test 3 failed'")
	}
}

func TestTaskView_NoBoxes(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).UseBoxes(false).Width(60)

	data := TaskData{
		Label:    "Build",
		Status:   "success",
		Duration: 100 * time.Millisecond,
	}

	result := view.RenderComplete(data)

	// Should NOT have box borders
	if strings.Contains(result, "╭") || strings.Contains(result, "╰") {
		t.Error("expected no-box render to NOT have borders")
	}

	// Should still contain status
	if !strings.Contains(result, theme.Icons.Success) {
		t.Error("expected no-box render to contain success icon")
	}
}

func TestTaskView_RenderUpdate(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(60)

	data := TaskData{
		Lines: []LineData{
			{Content: "Compiling main.go", Type: "detail"},
			{Content: "Linking binary", Type: "detail"},
		},
	}

	result := view.RenderUpdate(data)

	// Should contain line contents
	if !strings.Contains(result, "Compiling main.go") {
		t.Error("expected update render to contain 'Compiling main.go'")
	}
	if !strings.Contains(result, "Linking binary") {
		t.Error("expected update render to contain 'Linking binary'")
	}
}

func TestFormatDurationCompact(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Microsecond, "500µs"},
		{100 * time.Millisecond, "100ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{90 * time.Second, "1:30s"},
		{125 * time.Second, "2:05s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDurationCompact(tt.d)
			if got != tt.want {
				t.Errorf("formatDurationCompact(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestTaskView_LineTypes(t *testing.T) {
	theme := DefaultTheme()
	view := NewTaskView(theme).Width(80)

	tests := []struct {
		lineType string
		wantIcon string
	}{
		{"error", theme.Icons.Error},
		{"warning", theme.Icons.Warning},
		{"success", theme.Icons.Success},
		{"info", theme.Icons.Info},
	}

	for _, tt := range tests {
		t.Run(tt.lineType, func(t *testing.T) {
			data := TaskData{
				Label:     "Test",
				Status:    "success",
				ShowLines: true,
				Lines: []LineData{
					{Content: "test message", Type: tt.lineType},
				},
			}

			result := view.RenderComplete(data)

			if !strings.Contains(result, tt.wantIcon) {
				t.Errorf("expected line type %q to render with icon %q", tt.lineType, tt.wantIcon)
			}
		})
	}
}
