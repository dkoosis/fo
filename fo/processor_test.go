package fo

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/design"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessor_When_ValidConfig(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	maxLineLength := 1024 * 1024
	debug := false

	processor := NewProcessor(pm, maxLineLength, debug)

	assert.NotNil(t, processor)
	assert.Equal(t, pm, processor.patternMatcher)
	assert.Equal(t, maxLineLength, processor.maxLineLength)
	assert.Equal(t, debug, processor.debug)
}

func TestProcessor_ProcessOutput_When_PlainOutput(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "Building package...\nBuild complete!"

	processor.ProcessOutput(task, []byte(output), "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 2, len(outputLines), "Expected 2 output lines (one per line)")
	assert.Equal(t, "Building package...", outputLines[0].Content)
	assert.Equal(t, "Build complete!", outputLines[1].Content)
}

func TestProcessor_ProcessOutput_When_EmptyOutput(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	processor.ProcessOutput(task, []byte(""), "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 0, len(outputLines), "Expected no output lines for empty input")
}

func TestProcessor_processLineByLine_When_MultipleLines(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "Line 1\nLine 2\nLine 3"

	processor.processLineByLine(task, output, "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 3, len(outputLines))
	assert.Equal(t, "Line 1", outputLines[0].Content)
	assert.Equal(t, "Line 2", outputLines[1].Content)
	assert.Equal(t, "Line 3", outputLines[2].Content)
}

func TestProcessor_processLineByLine_When_ErrorLines(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "error: undefined variable\nwarning: unused import"

	processor.processLineByLine(task, output, "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 2, len(outputLines))
	// PatternMatcher should classify these appropriately
	assert.NotEmpty(t, outputLines[0].Type)
	assert.NotEmpty(t, outputLines[1].Type)
}

func TestProcessor_processLineByLine_When_EmptyLines(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "Line 1\n\nLine 3"

	processor.processLineByLine(task, output, "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 3, len(outputLines))
	assert.Equal(t, "Line 1", outputLines[0].Content)
	assert.Equal(t, "", outputLines[1].Content) // Empty line
	assert.Equal(t, "Line 3", outputLines[2].Content)
}

func TestProcessor_processLineByLine_When_LongLine(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	maxLineLength := 100 // Small limit for testing
	processor := NewProcessor(pm, maxLineLength, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	// Create a line longer than maxLineLength
	longLine := strings.Repeat("a", maxLineLength+50)
	output := longLine

	processor.processLineByLine(task, output, "go", []string{"build"})

	// Should still process the line (scanner.Buffer allows larger lines)
	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 1, len(outputLines))
	assert.Equal(t, longLine, outputLines[0].Content)
}

func TestProcessor_ProcessOutput_When_DebugEnabled(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	processor := NewProcessor(pm, 1024*1024, true) // Debug enabled

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "Some output"

	// Should not panic when debug is enabled
	processor.ProcessOutput(task, []byte(output), "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 1, len(outputLines))
}
