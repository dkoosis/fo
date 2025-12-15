package fo_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/fo"
)

func TestProfiler_WritesOutput_When_TogglingEnabledState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		enabled    bool
		stageName  string
		metrics    map[string]interface{}
		assertFile func(t *testing.T, path string)
	}{
		{
			name:      "disabled: does not emit file",
			enabled:   false,
			stageName: "setup",
			metrics:   map[string]interface{}{"pattern_matches": 3},
			assertFile: func(t *testing.T, path string) {
				_, err := os.Stat(path)
				require.ErrorIs(t, err, os.ErrNotExist, "disabled profiler should not produce output")
			},
		},
		{
			name:      "enabled: records metrics to file",
			enabled:   true,
			stageName: "parse",
			metrics: map[string]interface{}{
				"pattern_matches": 7,
				"pattern_success": 5,
				"buffer_size":     int64(1024),
				"line_count":      9,
				"memory_alloc":    int64(2048),
			},
			assertFile: func(t *testing.T, path string) {
				f, err := os.Open(path)
				require.NoError(t, err)
				defer f.Close()

				var stageLine string
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "parse\t") {
						stageLine = line
						break
					}
				}
				require.NoError(t, scanner.Err())
				require.NotEmpty(t, stageLine, "stage output should be present")

				columns := strings.Split(stageLine, "\t")
				require.Len(t, columns, 8, "stage output should contain all columns")

				durationMs, err := strconv.ParseInt(columns[2], 10, 64)
				require.NoError(t, err)
				assert.Positive(t, durationMs, "duration should be recorded in milliseconds")

				matches, err := strconv.Atoi(columns[3])
				require.NoError(t, err)
				assert.Equal(t, 7, matches)

				success, err := strconv.Atoi(columns[4])
				require.NoError(t, err)
				assert.Equal(t, 5, success)

				bufferSize, err := strconv.ParseInt(columns[5], 10, 64)
				require.NoError(t, err)
				assert.Equal(t, int64(1024), bufferSize)

				lineCount, err := strconv.Atoi(columns[6])
				require.NoError(t, err)
				assert.Equal(t, 9, lineCount)

				memoryAlloc, err := strconv.ParseInt(columns[7], 10, 64)
				require.NoError(t, err)
				assert.Equal(t, int64(2048), memoryAlloc)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outputPath := filepath.Join(t.TempDir(), "profile.txt")
			profiler := fo.NewProfiler(tc.enabled, outputPath)

			start := time.Now().Add(-25 * time.Millisecond)
			profiler.EndStage(tc.stageName, start, tc.metrics)

			err := profiler.Write()
			require.NoError(t, err)

			tc.assertFile(t, outputPath)
		})
	}
}
