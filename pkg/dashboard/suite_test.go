package dashboard

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuite_HandlesResults_When_RunningWithNonTTYOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		specs     []TaskSpec
		wantErr   error
		wantCode  int
		wantLines []string
	}{
		{
			name: "success: returns nil when all tasks succeed",
			specs: []TaskSpec{
				{Group: "Build", Name: "fmt", Command: "printf 'done\n'"},
				{Group: "Lint", Name: "noop", Command: "printf 'ok\n'"},
			},
			wantCode:  0,
			wantLines: []string{"[Build/fmt] done", "[Lint/noop] ok", "✓ Build/fmt", "✓ Lint/noop"},
		},
		{
			name: "error: returns SuiteError when any task fails",
			specs: []TaskSpec{
				{Group: "Test", Name: "pass", Command: "printf 'pass\n'"},
				{Group: "Test", Name: "fail", Command: "printf 'fail\n' && exit 3"},
			},
			wantErr:   &SuiteError{ExitCode: 1},
			wantCode:  1,
			wantLines: []string{"[Test/pass] pass", "[Test/fail] fail", "✓ Test/pass", "✗ Test/fail"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			var buf bytes.Buffer
			suite := NewSuite("non-tty")
			for _, spec := range tc.specs {
				suite.AddTask(spec.Group, spec.Name, spec.Command)
			}

			err := suite.RunWithOutput(ctx, &buf)
			output := buf.String()

			if tc.wantErr != nil {
				var suiteErr *SuiteError
				require.ErrorAs(t, err, &suiteErr)
				assert.Equal(t, tc.wantCode, suiteErr.ExitCode)
			} else {
				require.NoError(t, err)
			}

			for _, line := range tc.wantLines {
				assert.Contains(t, output, line, "expected output to include %q", line)
			}
		})
	}
}

func TestSuite_SkipsExecution_When_NoTasksProvided(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var buf bytes.Buffer
	suite := NewSuite("empty")

	err := suite.RunWithOutput(ctx, &buf)

	require.NoError(t, err)
	assert.Empty(t, buf.String())
}
