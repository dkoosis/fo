package dashboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTask_StartTasks_UpdatesStatusAndOutput_When_CommandsSucceedAndFail(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	specs := []TaskSpec{
		{Group: "GroupA", Name: "success", Command: "printf 'stdout\n'"},
		{Group: "GroupB", Name: "failure", Command: "printf 'stderr\n' 1>&2; exit 2"},
	}

	tasks, updates := StartTasks(ctx, specs)
	// Consume all updates; runTask already updates Status/ExitCode
	for update := range updates {
		task := tasks[update.Index]
		if update.Line != "" {
			// appendLine is needed since runTask doesn't call it
			task.appendLine(update.Line)
		}
		// Don't update Status/ExitCode - runTask already does that
	}

	require.Len(t, tasks, 2)

	assert.Equal(t, TaskSuccess, tasks[0].Status)
	assert.Equal(t, 0, tasks[0].ExitCode)
	assert.Contains(t, tasks[0].GetOutput(), "stdout")

	assert.Equal(t, TaskFailed, tasks[1].Status)
	assert.Equal(t, 2, tasks[1].ExitCode)
	assert.Contains(t, tasks[1].GetOutput(), "stderr")
}

func TestTask_AppendsOutputWithinBounds_When_LinesExceedBuffer(t *testing.T) {
	t.Parallel()

	task := &Task{}
	total := defaultBufferLines + 10
	for i := 0; i < total; i++ {
		task.appendLine(fmt.Sprintf("line-%d", i))
	}

	output := task.GetOutput()
	assert.Equal(t, defaultBufferLines, len(output))
	assert.Equal(t, fmt.Sprintf("line-%d", total-defaultBufferLines), output[0])
	assert.Equal(t, fmt.Sprintf("line-%d", total-1), output[len(output)-1])

	originalLast := output[len(output)-1]
	output[0] = "mutated"
	assert.Equal(t, originalLast, task.GetOutput()[len(output)-1], "returned slice should be a copy")
}

func TestTask_DurationReflectsState_When_UnfinishedAndFinished(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-500 * time.Millisecond)
	task := &Task{StartedAt: start}

	require.InDelta(t, 500*time.Millisecond, task.Duration(), float64(400*time.Millisecond))

	task.FinishedAt = start.Add(750 * time.Millisecond)
	assert.Equal(t, 750*time.Millisecond, task.Duration())
}
