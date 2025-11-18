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

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			task.AddOutputLine(
				"line content",
				TypeDetail,
				LineContext{CognitiveLoad: LoadLow, Importance: 1},
			)
		}(i)
	}

	wg.Wait()

	// All lines should be added without race conditions
	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Equal(t, numGoroutines, len(task.OutputLines))
}

func TestTask_AddOutputLine_When_MultipleTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		lineType    string
		context     LineContext
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := UnicodeVibrantTheme()
			task := NewTask("test", "", "cmd", nil, cfg)
			task.AddOutputLine(tc.content, tc.lineType, tc.context)
			
			task.OutputLinesLock()
			defer task.OutputLinesUnlock()
			assert.Equal(t, 1, len(task.OutputLines))
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
	assert.True(t, task.Duration > 0)
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

	for i := 0; i < numGoroutines; i++ {
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
	
	for i := 0; i < 10; i++ {
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
	
	for i := 0; i < 25; i++ {
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
	
	for i := 0; i < 60; i++ {
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
	
	for i := 0; i < 150; i++ {
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
	
	for i := 0; i < 10; i++ {
		task.AddOutputLine("error", TypeError, LineContext{Importance: 5})
	}
	
	task.UpdateTaskContext()

	assert.Equal(t, LoadHigh, task.Context.CognitiveLoad)
}

func TestTask_UpdateTaskContext_When_WarningsIncreaseLoad(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)
	
	for i := 0; i < 5; i++ {
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

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			task.AddOutputLine("line", TypeDetail, LineContext{Importance: 1})
			task.UpdateTaskContext()
		}()
	}

	wg.Wait()

	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Equal(t, numGoroutines, len(task.OutputLines))
}

func TestTask_OutputLinesLock_When_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	task := NewTask("test", "", "cmd", nil, cfg)
	
	task.AddOutputLine("line1", TypeDetail, LineContext{Importance: 1})
	task.AddOutputLine("line2", TypeDetail, LineContext{Importance: 1})

	task.OutputLinesLock()
	assert.Equal(t, 2, len(task.OutputLines))
	task.OutputLinesUnlock()

	// Should still be accessible after unlock
	task.OutputLinesLock()
	defer task.OutputLinesUnlock()
	assert.Equal(t, 2, len(task.OutputLines))
}

