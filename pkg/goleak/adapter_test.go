package goleak_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/goleak"
)

const sampleGoleakOutput = `Errors on successful test run: found unexpected goroutines:
[Goroutine 42 in state chan receive, with github.com/foo/bar.(*Client).readLoop on top of the stack:
goroutine 42 [chan receive]:
github.com/foo/bar.(*Client).readLoop(...)
    /path/to/file.go:123 +0x1a4
created by github.com/foo/bar.NewClient
    /path/to/other.go:45 +0x2b8
]`

func TestParse_ValidOutput(t *testing.T) {
	t.Parallel()

	adapter := goleak.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(sampleGoleakOutput)

	require.NoError(t, err)
	require.Len(t, result.Goroutines, 1)

	g := result.Goroutines[0]
	assert.Equal(t, 42, g.ID)
	assert.Equal(t, "chan receive", g.State)
	assert.Contains(t, g.TopFunction, "readLoop")
	assert.Contains(t, g.CreatedBy, "NewClient")
	assert.Contains(t, g.CreatedAt, "other.go:45")
}

func TestParse_MultipleGoroutines(t *testing.T) {
	t.Parallel()

	input := `found unexpected goroutines:
goroutine 10 [running]:
main.worker()
    /app/main.go:50 +0x100
created by main.startWorkers
    /app/main.go:30 +0x80

goroutine 11 [select]:
main.listener()
    /app/main.go:70 +0x150
created by main.startListeners
    /app/main.go:35 +0x90
`

	adapter := goleak.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Goroutines, 2)

	assert.Equal(t, 10, result.Goroutines[0].ID)
	assert.Equal(t, "running", result.Goroutines[0].State)
	assert.Equal(t, 11, result.Goroutines[1].ID)
	assert.Equal(t, "select", result.Goroutines[1].State)
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()

	adapter := goleak.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString("")

	require.NoError(t, err)
	assert.Empty(t, result.Goroutines)
}

func TestRender_NoLeaks(t *testing.T) {
	t.Parallel()

	adapter := goleak.NewAdapter(design.DefaultConfig())
	result := &goleak.Result{Goroutines: nil}

	output := adapter.Render(result)

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "None")
}

func TestRender_WithLeaks(t *testing.T) {
	t.Parallel()

	adapter := goleak.NewAdapter(design.DefaultConfig())
	result := &goleak.Result{
		Goroutines: []goleak.Goroutine{
			{
				ID:          42,
				State:       "chan receive",
				TopFunction: "github.com/foo/bar.readLoop",
				CreatedBy:   "github.com/foo/bar.NewClient",
			},
		},
	}

	output := adapter.Render(result)

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "goroutine 42")
}

func TestIsGoleakOutput_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "explicit goleak message",
			input: "found unexpected goroutines:\ngoroutine 1 [running]:",
			want:  true,
		},
		{
			name:  "goroutine with created by",
			input: "goroutine 42 [chan receive]:\nmain.foo()\ncreated by main.bar",
			want:  true,
		},
		{
			name:  "full sample output",
			input: sampleGoleakOutput,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, goleak.IsGoleakOutput([]byte(tt.input)))
		})
	}
}

func TestIsGoleakOutput_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty",
			input: "",
		},
		{
			name:  "plain text",
			input: "all tests passed",
		},
		{
			name:  "go test output without leaks",
			input: "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nPASS",
		},
		{
			name:  "goroutine without created by",
			input: "goroutine 1 [running]:\nmain.main()",
		},
		{
			name:  "nilaway json",
			input: `{"posn":"foo.go:1:1","message":"nil risk"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, goleak.IsGoleakOutput([]byte(tt.input)))
		})
	}
}

func TestMapToPatterns(t *testing.T) {
	t.Parallel()

	result := &goleak.Result{
		Goroutines: []goleak.Goroutine{
			{ID: 10, State: "running"},
			{ID: 11, State: "select"},
		},
	}

	patterns := goleak.MapToPatterns(result)

	require.Len(t, patterns, 2)
	assert.Equal(t, design.PatternTypeSummary, patterns[0].PatternType())
	assert.Equal(t, design.PatternTypeTestTable, patterns[1].PatternType())
}

func TestQuickRender(t *testing.T) {
	t.Parallel()

	output, err := goleak.QuickRender(sampleGoleakOutput)

	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "FAIL")
}
