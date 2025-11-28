package design

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTask_When_ValidInput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test-label", "building", "go", []string{"build"}, cfg)

	assert.Equal(t, "test-label", task.Label)
	assert.Equal(t, "building", task.Intent)
	assert.Equal(t, "go", task.Command)
	assert.Equal(t, []string{"build"}, task.Args)
	assert.Equal(t, StatusRunning, task.Status)
	assert.Equal(t, cfg, task.Config)
	assert.Equal(t, LoadMedium, task.Context.CognitiveLoad)
	assert.Equal(t, 2, task.Context.Complexity)
}

func TestNewTask_When_EmptyLabel(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("", "running", "cmd", nil, cfg)

	assert.Empty(t, task.Label)
	assert.Equal(t, StatusRunning, task.Status)
}

func TestNewTask_When_NilArgs(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("label", "", "cmd", nil, cfg)

	assert.Nil(t, task.Args)
}

func TestTask_AddOutputLine_When_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	const numGoroutines = 100
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			task.AddOutputLine(
				"line content",
				TypeDetail,
				LineContext{CognitiveLoad: LoadLow, Importance: 1},
			)
		}()
	}

	wg.Wait()

	// All lines should be added without race conditions
	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Len(t, task.OutputLines, numGoroutines)
}

func TestTask_AddOutputLine_When_MultipleTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		lineType string
		context  LineContext
	}{
		{
			name:     "error line",
			content:  "error message",
			lineType: TypeError,
			context:  LineContext{CognitiveLoad: LoadHigh, Importance: 5},
		},
		{
			name:     "warning line",
			content:  "warning message",
			lineType: TypeWarning,
			context:  LineContext{CognitiveLoad: LoadMedium, Importance: 4},
		},
		{
			name:     "success line",
			content:  "success message",
			lineType: TypeSuccess,
			context:  LineContext{CognitiveLoad: LoadLow, Importance: 3},
		},
		{
			name:     "detail line",
			content:  "detail message",
			lineType: TypeDetail,
			context:  LineContext{CognitiveLoad: LoadLow, Importance: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := UnicodeVibrantTheme()
			task := NewTask("test", "", "cmd", nil, cfg)
			task.AddOutputLine(tc.content, tc.lineType, tc.context)

			task.OutputLinesLock()
			defer task.OutputLinesUnlock()
			assert.Len(t, task.OutputLines, 1)
			assert.Equal(t, tc.content, task.OutputLines[0].Content)
			assert.Equal(t, tc.lineType, task.OutputLines[0].Type)
		})
	}
}

func TestTask_Complete_When_ExitCodeZero(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	// Add some time to ensure duration > 0
	time.Sleep(10 * time.Millisecond)
	task.Complete(0)

	assert.Equal(t, 0, task.ExitCode)
	assert.Equal(t, StatusSuccess, task.Status)
	assert.Positive(t, task.Duration)
	assert.False(t, task.EndTime.IsZero())
}

func TestTask_Complete_When_ExitCodeNonZero(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	time.Sleep(10 * time.Millisecond)
	task.Complete(1)

	assert.Equal(t, 1, task.ExitCode)
	assert.Equal(t, StatusError, task.Status)
}

func TestTask_Complete_When_ErrorsInOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error occurred", TypeError, LineContext{Importance: 5})
	time.Sleep(10 * time.Millisecond)
	task.Complete(0)

	assert.Equal(t, StatusError, task.Status)
}

func TestTask_Complete_When_WarningsInOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("warning message", TypeWarning, LineContext{Importance: 4})
	time.Sleep(10 * time.Millisecond)
	task.Complete(0)

	assert.Equal(t, StatusWarning, task.Status)
}

func TestTask_Complete_When_ErrorsOverrideWarnings(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error message", TypeError, LineContext{Importance: 5})
	task.AddOutputLine("warning message", TypeWarning, LineContext{Importance: 4})
	time.Sleep(10 * time.Millisecond)
	task.Complete(0)

	assert.Equal(t, StatusError, task.Status)
}

func TestTask_Complete_When_ExitCodeOverridesOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("warning message", TypeWarning, LineContext{Importance: 4})
	time.Sleep(10 * time.Millisecond)
	task.Complete(1)

	assert.Equal(t, StatusError, task.Status)
}

func TestTask_hasOutputIssues_When_NoIssues(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("info message", TypeInfo, LineContext{Importance: 2})
	task.AddOutputLine("detail message", TypeDetail, LineContext{Importance: 1})

	hasErrors, hasWarnings := task.hasOutputIssues()

	assert.False(t, hasErrors)
	assert.False(t, hasWarnings)
}

func TestTask_hasOutputIssues_When_ErrorsPresent(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error message", TypeError, LineContext{Importance: 5})

	hasErrors, hasWarnings := task.hasOutputIssues()

	assert.True(t, hasErrors)
	assert.False(t, hasWarnings)
}

func TestTask_hasOutputIssues_When_WarningsPresent(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("warning message", TypeWarning, LineContext{Importance: 4})

	hasErrors, hasWarnings := task.hasOutputIssues()

	assert.False(t, hasErrors)
	assert.True(t, hasWarnings)
}

func TestTask_hasOutputIssues_When_ConcurrentReads(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error message", TypeError, LineContext{Importance: 5})

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			hasErrors, hasWarnings := task.hasOutputIssues()
			assert.True(t, hasErrors)
			assert.False(t, hasWarnings)
		}()
	}

	wg.Wait()
}

func TestTask_UpdateTaskContext_When_NoOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.UpdateTaskContext()

	assert.Equal(t, 2, task.Context.Complexity)
	assert.Equal(t, LoadLow, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_SmallOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 10 {
		task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
	}

	task.UpdateTaskContext()

	assert.Equal(t, 2, task.Context.Complexity)
	assert.Equal(t, LoadLow, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_MediumOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 25 {
		task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
	}

	task.UpdateTaskContext()

	assert.Equal(t, 3, task.Context.Complexity)
	assert.Equal(t, LoadMedium, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_LargeOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 60 {
		task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
	}

	task.UpdateTaskContext()

	assert.Equal(t, 4, task.Context.Complexity)
	// 60 lines > 50, so complexity is 4, which makes cognitive load High
	assert.Equal(t, LoadHigh, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_VeryLargeOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 150 {
		task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
	}

	task.UpdateTaskContext()

	assert.Equal(t, 5, task.Context.Complexity)
	assert.Equal(t, LoadHigh, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_ErrorsIncreaseLoad(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error", TypeError, LineContext{Importance: 5})
	task.AddOutputLine("error", TypeError, LineContext{Importance: 5})

	task.UpdateTaskContext()

	assert.Equal(t, LoadMedium, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_ManyErrors(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 10 {
		task.AddOutputLine("error", TypeError, LineContext{Importance: 5})
	}

	task.UpdateTaskContext()

	assert.Equal(t, LoadHigh, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_WarningsIncreaseLoad(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	for range 5 {
		task.AddOutputLine("warning", TypeWarning, LineContext{Importance: 4})
	}

	task.UpdateTaskContext()

	assert.Equal(t, LoadMedium, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
			task.UpdateTaskContext()
		}()
	}

	wg.Wait()

	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Len(t, task.OutputLines, numGoroutines)
}

func TestTask_OutputLinesLock_When_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("line1", TypeDetail, LineContext{Importance: 1})
	task.AddOutputLine("line2", TypeDetail, LineContext{Importance: 1})

	task.OutputLinesLock()
	assert.Len(t, task.OutputLines, 2)
	task.OutputLinesUnlock()

	// Should still be accessible after unlock
	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Len(t, task.OutputLines, 2)
}

func TestTask_GetOutputLinesSnapshot_When_ReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("line1", TypeDetail, LineContext{Importance: 1})
	task.AddOutputLine("line2", TypeError, LineContext{Importance: 5})

	// Get snapshot
	snapshot := task.GetOutputLinesSnapshot()
	assert.Len(t, snapshot, 2)
	assert.Equal(t, "line1", snapshot[0].Content)
	assert.Equal(t, "line2", snapshot[1].Content)

	// Add more lines to original
	task.AddOutputLine("line3", TypeWarning, LineContext{Importance: 3})

	// Snapshot should not be affected
	assert.Len(t, snapshot, 2, "snapshot should remain unchanged after adding lines")

	// New snapshot should have all 3 lines
	newSnapshot := task.GetOutputLinesSnapshot()
	assert.Len(t, newSnapshot, 3)
}

func TestTask_ProcessOutputLines_When_ProcessesSafely(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	task.AddOutputLine("error1", TypeError, LineContext{Importance: 5})
	task.AddOutputLine("warning1", TypeWarning, LineContext{Importance: 3})
	task.AddOutputLine("detail1", TypeDetail, LineContext{Importance: 1})

	// Count errors using ProcessOutputLines
	errorCount := 0
	task.ProcessOutputLines(func(lines []OutputLine) {
		for _, line := range lines {
			if line.Type == TypeError {
				errorCount++
			}
		}
	})

	assert.Equal(t, 1, errorCount)
}

func TestTask_IncrementalCounters_When_TrackErrorsAndWarnings(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)

	// Initially no errors or warnings
	hasErrors, hasWarnings := task.hasOutputIssues()
	assert.False(t, hasErrors)
	assert.False(t, hasWarnings)

	// Add an error
	task.AddOutputLine("error1", TypeError, LineContext{})
	hasErrors, hasWarnings = task.hasOutputIssues()
	assert.True(t, hasErrors)
	assert.False(t, hasWarnings)

	// Add a warning
	task.AddOutputLine("warning1", TypeWarning, LineContext{})
	hasErrors, hasWarnings = task.hasOutputIssues()
	assert.True(t, hasErrors)
	assert.True(t, hasWarnings)

	// Add more errors and warnings
	task.AddOutputLine("error2", TypeError, LineContext{})
	task.AddOutputLine("warning2", TypeWarning, LineContext{})
	task.AddOutputLine("detail", TypeDetail, LineContext{})

	// UpdateTaskContext should use O(1) counters
	task.UpdateTaskContext()
	// With 2 errors, cognitive load should be medium (not high until >5)
	assert.Equal(t, LoadMedium, task.Context.CognitiveLoad)
}
