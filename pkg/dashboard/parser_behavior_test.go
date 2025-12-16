package dashboard_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/dashboard"
)

func TestParseManifest_ReturnsError_When_LineIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing colon delimiter",
			input: "build",
		},
		{
			name:  "empty command segment",
			input: "build :",
		},
		{
			name:  "empty task name",
			input: ": go test ./...",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := dashboard.ParseManifest(strings.NewReader(tt.input))
			require.Error(t, err)
		})
	}
}

func TestParseManifest_AssignsTasksGroup_When_NoGroupHeaderPresent(t *testing.T) {
	t.Parallel()

	specs, err := dashboard.ParseManifest(strings.NewReader("format: gofmt -w ."))
	require.NoError(t, err)

	if diff := cmp.Diff([]dashboard.TaskSpec{{Group: "Tasks", Name: "format", Command: "gofmt -w ."}}, specs); diff != "" {
		t.Fatalf("unexpected manifest parse (-want +got):\n%s", diff)
	}
}

func TestParseTaskFlag_ReturnsError_When_CommandOrNameMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		flag string
	}{
		{
			name: "missing command delimiter",
			flag: "group/name",
		},
		{
			name: "empty command content",
			flag: "group/name:  ",
		},
		{
			name: "missing task name in grouped flag",
			flag: "group/: go test ./...",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := dashboard.ParseTaskFlag(tt.flag)
			require.Error(t, err)
		})
	}
}

func TestParseTaskFlag_ParsesComponents_When_GroupPrefixProvided(t *testing.T) {
	t.Parallel()

	spec, err := dashboard.ParseTaskFlag("Quality/lint: golangci-lint run")
	require.NoError(t, err)

	assert.Equal(t, "Quality", spec.Group)
	assert.Equal(t, "lint", spec.Name)
	assert.Equal(t, "golangci-lint run", spec.Command)
}

func TestParseTaskFlag_UsesDefaultGroup_When_GroupPrefixMissing(t *testing.T) {
	t.Parallel()

	spec, err := dashboard.ParseTaskFlag("test: go test ./...")
	require.NoError(t, err)

	assert.Equal(t, "Tasks", spec.Group)
	assert.Equal(t, "test", spec.Name)
	assert.Equal(t, "go test ./...", spec.Command)
}
