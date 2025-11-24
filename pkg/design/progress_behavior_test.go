package design

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInlineProgress_FormatsMessage_When_RunningInMonochromeMode(t *testing.T) {
	t.Parallel()

	cfg := NoColorConfig()
	task := &Task{Label: "build", Command: "make", Config: cfg}
	progress := NewInlineProgress(task, false, &bytes.Buffer{})

	message := progress.formatProgressMessage(StatusRunning)

	assert.Contains(t, message, "[BUSY]")
	assert.Contains(t, message, "build")
	assert.Contains(t, message, "Working")
}

func TestInlineProgress_RendersCompletedMessage_When_UsingThemedTerminal(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := &Task{Label: "deploy", Command: "/bin/deploy", Duration: 1500 * time.Millisecond, Config: cfg}
	progress := NewInlineProgress(task, false, &bytes.Buffer{})
	progress.isTerminal = true // simulate interactive terminal

	message := progress.formatProgressMessage(StatusSuccess)

	assert.Contains(t, message, cfg.GetIcon("Success"))
	assert.Contains(t, message, "deploy")
	assert.Contains(t, message, "1.5s")
}

func TestInlineProgress_WritesWithNewline_When_NotTerminal(t *testing.T) {
	t.Parallel()

	cfg := NoColorConfig()
	task := &Task{Label: "archive", Command: "tar", Duration: 2500 * time.Millisecond, Config: cfg}
	buffer := &bytes.Buffer{}
	progress := NewInlineProgress(task, false, buffer)
	progress.isTerminal = false

	progress.RenderProgress(StatusSuccess)

	output := buffer.String()
	assert.Contains(t, output, "[OK]")
	assert.True(t, bytes.HasSuffix([]byte(output), []byte("\n")))
}

func TestInlineProgress_StartsTracking_When_SpinnerDisabled(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := &Task{Label: "compile", Command: "go", Config: cfg}
	buffer := &bytes.Buffer{}
	progress := NewInlineProgress(task, false, buffer)
	progress.isTerminal = false

	progress.Start(context.Background(), false)

	progress.mutex.Lock()
	isActive := progress.IsActive
	startTime := progress.StartTime
	progress.mutex.Unlock()

	assert.True(t, isActive)
	assert.False(t, startTime.IsZero())
	assert.NotEmpty(t, buffer.String())
}

func TestInlineProgress_CompletesRendering_When_TaskFinishes(t *testing.T) {
	t.Parallel()

	cfg := NoColorConfig()
	task := &Task{Label: "package", Command: "tar", Config: cfg}
	buffer := &bytes.Buffer{}
	progress := NewInlineProgress(task, false, buffer)
	progress.isTerminal = false

	progress.Complete(StatusSuccess)

	progress.mutex.Lock()
	isActive := progress.IsActive
	progress.mutex.Unlock()

	assert.False(t, isActive)
	assert.Contains(t, buffer.String(), "[OK]")
}

func TestInlineProgress_StopsSpinner_When_ContextCancelled(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Style.SpinnerInterval = 1
	cfg.Elements["Task_Progress_Line"] = ElementStyleDef{AdditionalChars: "<>"}

	task := &Task{Label: "sync", Command: "/usr/bin/sync", Config: cfg}
	buffer := &bytes.Buffer{}
	progress := NewInlineProgress(task, false, buffer)
	progress.isTerminal = true

	ctx, cancel := context.WithCancel(context.Background())
	progress.IsActive = true

	done := make(chan struct{})
	go func() {
		progress.runSpinner(ctx)
		close(done)
	}()

	// Wait for at least one tick to advance the spinner.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("spinner did not stop after context cancellation")
	}

	progress.mutex.Lock()
	spinnerIndex := progress.SpinnerIndex
	progress.mutex.Unlock()

	assert.GreaterOrEqual(t, spinnerIndex, 0)
	assert.NotEmpty(t, buffer.String())
}
