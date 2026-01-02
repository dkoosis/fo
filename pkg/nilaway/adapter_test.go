package nilaway_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/dkoosis/fo/pkg/nilaway"
)

func TestParse_NestedJSON(t *testing.T) {
	t.Parallel()

	// Actual nilaway -json output format
	input := `{
	"github.com/foo/bar [github.com/foo/bar.test]": {
		"nilaway": [
			{
				"posn": "/path/to/foo.go:42:15",
				"message": "potential nil dereference"
			},
			{
				"posn": "/path/to/bar.go:123:8",
				"message": "nil pointer risk"
			}
		]
	}
}`

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Findings, 2)

	assert.Equal(t, "/path/to/foo.go", result.Findings[0].File)
	assert.Equal(t, 42, result.Findings[0].Line)
	assert.Equal(t, 15, result.Findings[0].Column)
	assert.Equal(t, "potential nil dereference", result.Findings[0].Message)

	assert.Equal(t, "/path/to/bar.go", result.Findings[1].File)
	assert.Equal(t, 123, result.Findings[1].Line)
}

func TestParse_NestedJSON_MultiplePackages(t *testing.T) {
	t.Parallel()

	input := `{
	"pkg/a": {"nilaway": [{"posn": "a.go:10:5", "message": "nil in pkg a"}]},
	"pkg/b": {"nilaway": [{"posn": "b.go:20:3", "message": "nil in pkg b"}]}
}`

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Findings, 2)
}

func TestParse_NDJSON_Fallback(t *testing.T) {
	t.Parallel()

	// Legacy NDJSON format (one JSON object per line)
	input := `{"posn":"internal/foo/bar.go:42:15","message":"accessed field on potentially nil value","reason":"field X may be nil when Y is false"}
{"posn":"pkg/server/handler.go:123:8","message":"nil dereference","reason":"return value not checked"}`

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Findings, 2)

	assert.Equal(t, "internal/foo/bar.go", result.Findings[0].File)
	assert.Equal(t, 42, result.Findings[0].Line)
	assert.Equal(t, 15, result.Findings[0].Column)
	assert.Equal(t, "accessed field on potentially nil value", result.Findings[0].Message)

	assert.Equal(t, "pkg/server/handler.go", result.Findings[1].File)
	assert.Equal(t, 123, result.Findings[1].Line)
}

func TestParse_EmptyInput(t *testing.T) {
	t.Parallel()

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString("")

	require.NoError(t, err)
	assert.Empty(t, result.Findings)
}

func TestParse_MixedContent(t *testing.T) {
	t.Parallel()

	// nilaway might output progress lines before JSON
	input := `analyzing package...
{"posn":"main.go:10:5","message":"potential nil","reason":"unchecked"}
done`

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result, err := adapter.ParseString(input)

	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	assert.Equal(t, "main.go", result.Findings[0].File)
}

func TestRender_NoFindings(t *testing.T) {
	t.Parallel()

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result := &nilaway.Result{Findings: nil}

	output := adapter.Render(result)

	assert.Contains(t, output, "OK")
	assert.Contains(t, output, "None")
}

func TestRender_WithFindings(t *testing.T) {
	t.Parallel()

	adapter := nilaway.NewAdapter(design.DefaultConfig())
	result := &nilaway.Result{
		Findings: []nilaway.Finding{
			{
				File:    "foo.go",
				Line:    42,
				Message: "nil dereference",
				Reason:  "unchecked error",
			},
		},
	}

	output := adapter.Render(result)

	assert.Contains(t, output, "WARN")
	assert.Contains(t, output, "1")   // finding count
	assert.Contains(t, output, ":42") // line number reference
}

func TestIsNilawayOutput_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "nested format",
			input: `{"pkg/foo": {"nilaway": [{"posn":"foo.go:1:1","message":"nil risk"}]}}`,
			want:  true,
		},
		{
			name:  "nested format multiple packages",
			input: `{"pkg/a": {"nilaway": [{"posn":"a.go:1:1","message":"x"}]}, "pkg/b": {"nilaway": [{"posn":"b.go:2:2","message":"y"}]}}`,
			want:  true,
		},
		{
			name:  "ndjson single finding",
			input: `{"posn":"foo.go:1:1","message":"nil risk"}`,
			want:  true,
		},
		{
			name:  "ndjson multiple findings",
			input: "{\"posn\":\"a.go:1:1\",\"message\":\"x\"}\n{\"posn\":\"b.go:2:2\",\"message\":\"y\"}",
			want:  true,
		},
		{
			name:  "with reason field",
			input: `{"posn":"foo.go:1:1","message":"nil risk","reason":"explanation"}`,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, nilaway.IsNilawayOutput([]byte(tt.input)))
		})
	}
}

func TestIsNilawayOutput_Invalid(t *testing.T) {
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
			input: "no errors found",
		},
		{
			name:  "json without posn",
			input: `{"file":"foo.go","message":"something"}`,
		},
		{
			name:  "sarif format",
			input: `{"$schema":"https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"}`,
		},
		{
			name:  "gofmt output",
			input: "internal/foo.go\ninternal/bar.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, nilaway.IsNilawayOutput([]byte(tt.input)))
		})
	}
}

func TestMapToPatterns(t *testing.T) {
	t.Parallel()

	result := &nilaway.Result{
		Findings: []nilaway.Finding{
			{File: "a.go", Line: 10, Message: "nil risk"},
			{File: "b.go", Line: 20, Message: "another nil risk"},
		},
	}

	patterns := nilaway.MapToPatterns(result)

	require.Len(t, patterns, 2)
	assert.Equal(t, design.PatternTypeSummary, patterns[0].PatternType())
	assert.Equal(t, design.PatternTypeTestTable, patterns[1].PatternType())
}

func TestQuickRender(t *testing.T) {
	t.Parallel()

	input := `{"posn":"main.go:5:2","message":"nil pointer"}`

	output, err := nilaway.QuickRender(input)

	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.True(t, strings.Contains(output, "WARN") || strings.Contains(output, "nil pointer"))
}
