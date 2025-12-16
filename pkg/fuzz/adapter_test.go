package fuzz_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/fuzz"
)

const sampleFuzzProgress = `fuzz: elapsed: 0s, gathering baseline coverage: 0/10 completed
fuzz: elapsed: 0s, gathering baseline coverage: 10/10 completed, now fuzzing with 8 workers
fuzz: elapsed: 3s, execs: 102345 (34115/sec), new interesting: 12 (total: 22)
fuzz: elapsed: 6s, execs: 245678 (47778/sec), new interesting: 3 (total: 25)
PASS`

const sampleFuzzFailure = `fuzz: elapsed: 0s, gathering baseline coverage: 5/5 completed, now fuzzing with 4 workers
fuzz: elapsed: 1s, execs: 5000 (5000/sec), new interesting: 2 (total: 7)
--- FAIL: FuzzParseInput (0.52s)
    --- FAIL: FuzzParseInput (0.00s)
        testing.go:1356: panic: runtime error: index out of range [5] with length 3

    Failing input written to testdata/fuzz/FuzzParseInput/abc123def456
    To re-run:
    go test -run=FuzzParseInput/abc123def456
FAIL`

func TestParse_ProgressOutput(t *testing.T) {
	t.Parallel()

	adapter := fuzz.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(sampleFuzzProgress)

	require.NoError(t, err)
	assert.Equal(t, fuzz.StatusPassed, result.Status)
	assert.Empty(t, result.Failures)
	require.GreaterOrEqual(t, len(result.Progress), 2)

	// Check last progress entry
	last := result.Progress[len(result.Progress)-1]
	assert.Equal(t, int64(245678), last.Executions)
	assert.Equal(t, int64(47778), last.ExecsPerSec)
	assert.Equal(t, 3, last.NewInteresting)
	assert.Equal(t, 25, last.TotalCorpus)
}

func TestParse_FailureOutput(t *testing.T) {
	t.Parallel()

	adapter := fuzz.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(sampleFuzzFailure)

	require.NoError(t, err)
	assert.Equal(t, fuzz.StatusFailed, result.Status)
	// Two failures: parent (0.52s) and crash (0.00s)
	require.Len(t, result.Failures, 2)

	// Check the crash failure (second one has the error details)
	f := result.Failures[1]
	assert.Equal(t, "FuzzParseInput", f.TestName)
	assert.Equal(t, "0.00s", f.Duration)
	assert.Contains(t, f.Error, "index out of range")
	assert.Equal(t, "testdata/fuzz/FuzzParseInput/abc123def456", f.CorpusFile)
	assert.Equal(t, "go test -run=FuzzParseInput/abc123def456", f.RerunCmd)
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()

	adapter := fuzz.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString("")

	require.NoError(t, err)
	assert.Equal(t, fuzz.StatusRunning, result.Status)
	assert.Empty(t, result.Failures)
	assert.Empty(t, result.Progress)
}

func TestRender_Passed(t *testing.T) {
	t.Parallel()

	adapter := fuzz.NewAdapter(design.DefaultConfig())
	result := &fuzz.Result{
		Status: fuzz.StatusPassed,
		Progress: []fuzz.Progress{
			{Executions: 100000, TotalCorpus: 50},
		},
	}

	output := adapter.Render(result)

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "100.0K") // formatted executions
}

func TestRender_Failed(t *testing.T) {
	t.Parallel()

	adapter := fuzz.NewAdapter(design.DefaultConfig())
	result := &fuzz.Result{
		Status: fuzz.StatusFailed,
		Failures: []fuzz.Failure{
			{
				TestName:   "FuzzFoo",
				Duration:   "1.5s",
				Error:      "panic: nil pointer",
				CorpusFile: "testdata/fuzz/FuzzFoo/crash1",
			},
		},
	}

	output := adapter.Render(result)

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "FuzzFoo")
}

func TestIsFuzzOutput_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "progress output",
			input: "fuzz: elapsed: 3s, execs: 1000 (500/sec), new interesting: 1 (total: 5)",
			want:  true,
		},
		{
			name:  "baseline gathering",
			input: "fuzz: elapsed: 0s, gathering baseline coverage: 5/10 completed",
			want:  true,
		},
		{
			name:  "failure with corpus",
			input: "--- FAIL: FuzzInput (0.5s)\nFailing input written to testdata/fuzz/FuzzInput/crash",
			want:  true,
		},
		{
			name:  "full progress sample",
			input: sampleFuzzProgress,
			want:  true,
		},
		{
			name:  "full failure sample",
			input: sampleFuzzFailure,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, fuzz.IsFuzzOutput([]byte(tt.input)))
		})
	}
}

func TestIsFuzzOutput_Invalid(t *testing.T) {
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
			name:  "regular test output",
			input: "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nPASS",
		},
		{
			name:  "race detector",
			input: "WARNING: DATA RACE\nRead at 0x00c000 by goroutine 1:",
		},
		{
			name:  "goleak output",
			input: "found unexpected goroutines:\ngoroutine 1 [running]:\ncreated by main",
		},
		{
			name:  "nilaway json",
			input: `{"posn":"foo.go:1:1","message":"nil risk"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, fuzz.IsFuzzOutput([]byte(tt.input)))
		})
	}
}

func TestMapToPatterns(t *testing.T) {
	t.Parallel()

	result := &fuzz.Result{
		Status: fuzz.StatusFailed,
		Failures: []fuzz.Failure{
			{TestName: "FuzzA"},
			{TestName: "FuzzB"},
		},
	}

	patterns := fuzz.MapToPatterns(result)

	require.Len(t, patterns, 2)
	assert.Equal(t, design.PatternTypeSummary, patterns[0].PatternType())
	assert.Equal(t, design.PatternTypeTestTable, patterns[1].PatternType())
}

func TestQuickRender(t *testing.T) {
	t.Parallel()

	output, err := fuzz.QuickRender(sampleFuzzProgress)

	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "OK")
}
