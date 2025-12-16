package racedetect_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/racedetect"
)

const sampleRaceOutput = `==================
WARNING: DATA RACE
Read at 0x00c0001a4018 by goroutine 15:
  github.com/foo/bar.(*Server).handleRequest()
      /path/to/server.go:142 +0x1f4

Previous write at 0x00c0001a4018 by goroutine 7:
  github.com/foo/bar.(*Server).updateState()
      /path/to/server.go:89 +0x124

Goroutine 15 (running) created at:
  github.com/foo/bar.(*Server).Serve()
      /path/to/server.go:67 +0x2a8

Goroutine 7 (running) created at:
  github.com/foo/bar.NewServer()
      /path/to/server.go:34 +0x1b4
==================`

func TestParse_ValidOutput(t *testing.T) {
	t.Parallel()

	adapter := racedetect.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(sampleRaceOutput)

	require.NoError(t, err)
	require.Len(t, result.Races, 1)

	race := result.Races[0]
	require.Len(t, race.Accesses, 2)

	// Check read access
	assert.Equal(t, racedetect.AccessRead, race.Accesses[0].Type)
	assert.Equal(t, 15, race.Accesses[0].Goroutine)
	assert.Contains(t, race.Accesses[0].Function, "handleRequest")
	assert.Equal(t, "/path/to/server.go", race.Accesses[0].File)
	assert.Equal(t, 142, race.Accesses[0].Line)
	assert.False(t, race.Accesses[0].IsPrevious)

	// Check previous write access
	assert.Equal(t, racedetect.AccessWrite, race.Accesses[1].Type)
	assert.Equal(t, 7, race.Accesses[1].Goroutine)
	assert.Contains(t, race.Accesses[1].Function, "updateState")
	assert.True(t, race.Accesses[1].IsPrevious)

	// Check goroutines
	require.Len(t, race.Goroutines, 2)
	assert.Equal(t, 15, race.Goroutines[0].ID)
	assert.Equal(t, "running", race.Goroutines[0].State)
}

func TestParse_MultipleRaces(t *testing.T) {
	t.Parallel()

	input := `==================
WARNING: DATA RACE
Read at 0x00c000100000 by goroutine 10:
  main.reader()
      /app/main.go:50 +0x100

Previous write at 0x00c000100000 by goroutine 11:
  main.writer()
      /app/main.go:60 +0x100
==================
==================
WARNING: DATA RACE
Write at 0x00c000200000 by goroutine 20:
  main.writer2()
      /app/main.go:70 +0x100

Previous read at 0x00c000200000 by goroutine 21:
  main.reader2()
      /app/main.go:80 +0x100
==================`

	adapter := racedetect.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Races, 2)

	// First race: read vs prev write
	assert.Equal(t, racedetect.AccessRead, result.Races[0].Accesses[0].Type)
	assert.Equal(t, racedetect.AccessWrite, result.Races[0].Accesses[1].Type)
	assert.True(t, result.Races[0].Accesses[1].IsPrevious)

	// Second race: write vs prev read
	assert.Equal(t, racedetect.AccessWrite, result.Races[1].Accesses[0].Type)
	assert.Equal(t, racedetect.AccessRead, result.Races[1].Accesses[1].Type)
	assert.True(t, result.Races[1].Accesses[1].IsPrevious)
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()

	adapter := racedetect.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString("")

	require.NoError(t, err)
	assert.Empty(t, result.Races)
}

func TestRender_NoRaces(t *testing.T) {
	t.Parallel()

	adapter := racedetect.NewAdapter(design.DefaultConfig())
	result := &racedetect.Result{Races: nil}

	output := adapter.Render(result)

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "None")
}

func TestRender_WithRaces(t *testing.T) {
	t.Parallel()

	adapter := racedetect.NewAdapter(design.DefaultConfig())
	result := &racedetect.Result{
		Races: []racedetect.Race{
			{
				Accesses: []racedetect.Access{
					{Type: racedetect.AccessRead, Goroutine: 10, Function: "main.reader"},
					{Type: racedetect.AccessWrite, Goroutine: 11, Function: "main.writer", IsPrevious: true},
				},
			},
		},
	}

	output := adapter.Render(result)

	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "1")
	assert.Contains(t, output, "Race #1")
}

func TestIsRaceDetectorOutput_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "full race output",
			input: sampleRaceOutput,
			want:  true,
		},
		{
			name:  "just warning header",
			input: "WARNING: DATA RACE\n",
			want:  true,
		},
		{
			name:  "warning with extra spaces",
			input: "WARNING:  DATA  RACE",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, racedetect.IsRaceDetectorOutput([]byte(tt.input)))
		})
	}
}

func TestIsRaceDetectorOutput_Invalid(t *testing.T) {
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
			name:  "go test output",
			input: "=== RUN   TestFoo\n--- PASS: TestFoo (0.00s)\nPASS",
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
			assert.False(t, racedetect.IsRaceDetectorOutput([]byte(tt.input)))
		})
	}
}

func TestMapToPatterns(t *testing.T) {
	t.Parallel()

	result := &racedetect.Result{
		Races: []racedetect.Race{
			{Accesses: []racedetect.Access{{Type: racedetect.AccessRead}}},
			{Accesses: []racedetect.Access{{Type: racedetect.AccessWrite}}},
		},
	}

	patterns := racedetect.MapToPatterns(result)

	require.Len(t, patterns, 2)
	assert.Equal(t, design.PatternTypeSummary, patterns[0].PatternType())
	assert.Equal(t, design.PatternTypeTestTable, patterns[1].PatternType())
}

func TestQuickRender(t *testing.T) {
	t.Parallel()

	output, err := racedetect.QuickRender(sampleRaceOutput)

	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "FAIL")
}
