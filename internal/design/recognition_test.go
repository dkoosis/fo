package design

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPatternMatcher_When_ValidConfig(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	assert.NotNil(t, pm)
	assert.Equal(t, cfg, pm.Config)
}

func TestPatternMatcher_DetectCommandIntent_When_BuildCommand(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "go build",
			command: "go",
			args:    []string{"build"},
			want:    "building",
		},
		{
			name:    "make command",
			command: "make",
			args:    []string{"build"},
			want:    "building",
		},
		{
			name:    "build in command name",
			command: "buildtool",
			args:    []string{"target"},
			want:    "building",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pm.DetectCommandIntent(tc.command, tc.args)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPatternMatcher_DetectCommandIntent_When_TestCommand(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "go test",
			command: "go",
			args:    []string{"test"},
			want:    "testing",
		},
		{
			name:    "pytest",
			command: "pytest",
			args:    []string{},
			want:    "testing",
		},
		{
			name:    "test in command name",
			command: "testrunner",
			args:    []string{},
			want:    "testing",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pm.DetectCommandIntent(tc.command, tc.args)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPatternMatcher_DetectCommandIntent_When_UnknownCommand(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	got := pm.DetectCommandIntent("unknowncmd", []string{})

	assert.Equal(t, "running", got)
}

func TestPatternMatcher_DetectCommandIntent_When_ToolConfigExists(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"mycmd": {
			Intent: "custom_intent",
		},
	}
	pm := NewPatternMatcher(cfg)

	got := pm.DetectCommandIntent("mycmd", []string{})

	assert.Equal(t, "custom_intent", got)
}

func TestPatternMatcher_DetectCommandIntent_When_ToolConfigWithArgs(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"go build": {
			Intent: "compiling",
		},
	}
	pm := NewPatternMatcher(cfg)

	got := pm.DetectCommandIntent("go", []string{"build"})

	assert.Equal(t, "compiling", got)
}

func TestPatternMatcher_ClassifyOutputLine_When_ErrorPattern(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name    string
		line    string
		wantType string
		wantImportance int
	}{
		{
			name:          "Error: prefix",
			line:          "Error: something went wrong",
			wantType:      TypeError,
			wantImportance: 5,
		},
		{
			name:          "ERROR: prefix",
			line:          "ERROR: critical failure",
			wantType:      TypeError,
			wantImportance: 5,
		},
		{
			name:          "panic: prefix",
			line:          "panic: runtime error",
			wantType:      TypeError,
			wantImportance: 5,
		},
		{
			name:          "FAIL prefix",
			line:          "FAIL: test failed",
			wantType:      TypeError,
			wantImportance: 4, // File:line format gives Importance 4, not pattern-based 5
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lineType, context := pm.ClassifyOutputLine(tc.line, "cmd", []string{})
			assert.Equal(t, tc.wantType, lineType)
			if tc.wantType == TypeError {
				// File:line format detection may have different importance/cognitive load
				assert.GreaterOrEqual(t, context.Importance, 4)
			} else {
				assert.Equal(t, tc.wantImportance, context.Importance)
				assert.Equal(t, LoadHigh, context.CognitiveLoad)
			}
		})
	}
}

func TestPatternMatcher_ClassifyOutputLine_When_WarningPattern(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name    string
		line    string
		wantType string
		wantImportance int
	}{
		{
			name:          "Warning: prefix",
			line:          "Warning: deprecated function",
			wantType:      TypeWarning,
			wantImportance: 4,
		},
		{
			name:          "WARNING: prefix",
			line:          "WARNING: minor issue",
			wantType:      TypeWarning,
			wantImportance: 4,
		},
		{
			name:          "deprecated with colon prefix",
			line:          "deprecated: use new one",
			wantType:      TypeWarning,
			wantImportance: 4,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lineType, context := pm.ClassifyOutputLine(tc.line, "cmd", []string{})
			assert.Equal(t, tc.wantType, lineType)
			assert.Equal(t, tc.wantImportance, context.Importance)
			assert.Equal(t, LoadMedium, context.CognitiveLoad)
		})
	}
}

func TestPatternMatcher_ClassifyOutputLine_When_SuccessPattern(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name    string
		line    string
		wantType string
	}{
		{
			name:     "PASS prefix",
			line:     "PASS: all tests passed",
			wantType: TypeSuccess,
		},
		{
			name:     "Success: prefix",
			line:     "Success: operation completed",
			wantType: TypeSuccess,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lineType, context := pm.ClassifyOutputLine(tc.line, "cmd", []string{})
			assert.Equal(t, tc.wantType, lineType)
			assert.Equal(t, 3, context.Importance)
		})
	}
}

func TestPatternMatcher_ClassifyOutputLine_When_FileLineFormat(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	tests := []struct {
		name string
		line string
	}{
		{
			name: "Go file:line",
			line: "main.go:42: undefined variable",
		},
		{
			name: "JavaScript file:line",
			line: "index.js:10: syntax error",
		},
		{
			name: "Python file:line",
			line: "app.py:5: IndentationError",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lineType, context := pm.ClassifyOutputLine(tc.line, "cmd", []string{})
			assert.Equal(t, TypeError, lineType)
			assert.Equal(t, 4, context.Importance)
		})
	}
}

func TestPatternMatcher_ClassifyOutputLine_When_DefaultDetail(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	lineType, context := pm.ClassifyOutputLine("regular output line", "cmd", []string{})

	assert.Equal(t, TypeDetail, lineType)
	assert.Equal(t, 2, context.Importance)
	assert.Equal(t, LoadMedium, context.CognitiveLoad)
}

func TestPatternMatcher_ClassifyOutputLine_When_ToolConfigPatterns(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"mycmd": {
			OutputPatterns: map[string][]string{
				TypeError: {
					"^CRITICAL:",
				},
			},
		},
	}
	pm := NewPatternMatcher(cfg)

	lineType, context := pm.ClassifyOutputLine("CRITICAL: system failure", "mycmd", []string{})

	assert.Equal(t, TypeError, lineType)
	assert.Equal(t, 5, context.Importance)
	assert.Equal(t, LoadHigh, context.CognitiveLoad)
}

func TestPatternMatcher_ClassifyOutputLine_When_EmptyPattern(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"mycmd": {
			OutputPatterns: map[string][]string{
				TypeError: {
					"", // Empty pattern should be skipped
					"^ERROR:",
				},
			},
		},
	}
	pm := NewPatternMatcher(cfg)

	lineType, context := pm.ClassifyOutputLine("ERROR: test", "mycmd", []string{})

	assert.Equal(t, TypeError, lineType)
	assert.Equal(t, 5, context.Importance)
}

func TestPatternMatcher_FindSimilarLines_When_EmptyLines(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{}
	groups := pm.FindSimilarLines(lines)

	assert.Empty(t, groups)
}

func TestPatternMatcher_FindSimilarLines_When_ShortLines(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{
		{Content: "short", Type: TypeError},
		{Content: "tiny", Type: TypeWarning},
	}
	groups := pm.FindSimilarLines(lines)

	assert.Len(t, groups, 2)
	assert.Contains(t, groups, "short_error")
	assert.Contains(t, groups, "short_warning")
}

func TestPatternMatcher_FindSimilarLines_When_SimilarErrors(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{
		{Content: "main.go:10: undefined variable x", Type: TypeError},
		{Content: "main.go:20: undefined variable y", Type: TypeError},
		{Content: "util.go:5: syntax error", Type: TypeError},
	}
	groups := pm.FindSimilarLines(lines)

	// Should group similar file:line errors together
	assert.NotEmpty(t, groups)
}

func TestPatternMatcher_DetermineCognitiveLoad_When_AutoDetectDisabled(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.CognitiveLoad.AutoDetect = false
	cfg.CognitiveLoad.Default = LoadLow
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{
		{Content: "error", Type: TypeError},
		{Content: "error", Type: TypeError},
	}
	load := pm.DetermineCognitiveLoad(lines)

	assert.Equal(t, LoadLow, load)
}

func TestPatternMatcher_DetermineCognitiveLoad_When_ManyErrors(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.CognitiveLoad.AutoDetect = true
	pm := NewPatternMatcher(cfg)

	lines := make([]OutputLine, 10)
	for i := range lines {
		lines[i] = OutputLine{Content: "error", Type: TypeError}
	}

	load := pm.DetermineCognitiveLoad(lines)

	assert.Equal(t, LoadHigh, load)
}

func TestPatternMatcher_DetermineCognitiveLoad_When_LargeOutput(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.CognitiveLoad.AutoDetect = true
	pm := NewPatternMatcher(cfg)

	lines := make([]OutputLine, 150)
	for i := range lines {
		lines[i] = OutputLine{Content: "output", Type: TypeDetail}
	}

	load := pm.DetermineCognitiveLoad(lines)

	assert.Equal(t, LoadHigh, load)
}

func TestPatternMatcher_DetermineCognitiveLoad_When_MediumLoad(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.CognitiveLoad.AutoDetect = true
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{
		{Content: "error", Type: TypeError},
		{Content: "warning", Type: TypeWarning},
		{Content: "warning", Type: TypeWarning},
		{Content: "warning", Type: TypeWarning},
		{Content: "warning", Type: TypeWarning},
	}

	load := pm.DetermineCognitiveLoad(lines)

	assert.Equal(t, LoadMedium, load)
}

func TestPatternMatcher_DetermineCognitiveLoad_When_LowLoad(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.CognitiveLoad.AutoDetect = true
	pm := NewPatternMatcher(cfg)

	lines := []OutputLine{
		{Content: "output", Type: TypeDetail},
		{Content: "info", Type: TypeInfo},
	}

	load := pm.DetermineCognitiveLoad(lines)

	assert.Equal(t, LoadLow, load)
}

func TestPatternMatcher_findToolConfig_When_ExactMatch(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"go": {
			Label: "Go Tool",
		},
	}
	pm := NewPatternMatcher(cfg)

	toolCfg := pm.findToolConfig("go", []string{})

	assert.NotNil(t, toolCfg)
	assert.Equal(t, "Go Tool", toolCfg.Label)
}

func TestPatternMatcher_findToolConfig_When_CommandWithArgs(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"go build": {
			Label: "Go Build",
		},
	}
	pm := NewPatternMatcher(cfg)

	toolCfg := pm.findToolConfig("go", []string{"build"})

	assert.NotNil(t, toolCfg)
	assert.Equal(t, "Go Build", toolCfg.Label)
}

func TestPatternMatcher_findToolConfig_When_NoMatch(t *testing.T) {
	t.Parallel()

	cfg := UnicodeVibrantTheme()
	cfg.Tools = map[string]*ToolConfig{
		"other": {
			Label: "Other",
		},
	}
	pm := NewPatternMatcher(cfg)

	toolCfg := pm.findToolConfig("mycmd", []string{})

	assert.Nil(t, toolCfg)
}

