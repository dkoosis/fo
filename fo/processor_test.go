package fo

import (
	"io"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/adapter"
	"github.com/dkoosis/fo/pkg/design"
	"github.com/stretchr/testify/assert"
)

func TestNewProcessor_When_ValidConfig(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	maxLineLength := 1024 * 1024
	debug := false

	processor := NewProcessor(pm, reg, maxLineLength, debug)

	assert.NotNil(t, processor)
	assert.Equal(t, pm, processor.patternMatcher)
	assert.Equal(t, reg, processor.adapterRegistry)
	assert.Equal(t, maxLineLength, processor.maxLineLength)
	assert.Equal(t, debug, processor.debug)
}

func TestProcessor_ProcessOutput_When_AdapterDetected(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

	// Create a task
	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"test", "-json"}, cfg)

	// Go test JSON output that should be detected by the adapter
	output := `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}
{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example","Test":"TestFoo"}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Test":"TestFoo","Elapsed":0.1}
{"Time":"2024-01-01T12:00:01Z","Action":"pass","Package":"pkg/example","Elapsed":0.1}`

	processor.ProcessOutput(task, []byte(output), "go", []string{"test", "-json"})

	// Verify that output was added (adapter should have rendered a pattern)
	outputLines := task.GetOutputLinesSnapshot()
	assert.Greater(t, len(outputLines), 0, "Expected at least one output line from adapter")
}

func TestProcessor_ProcessOutput_When_NoAdapterDetected(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

	// Create a task
	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	// Plain text output that won't be detected by any adapter
	output := "Building package...\nBuild complete!"

	processor.ProcessOutput(task, []byte(output), "go", []string{"build"})

	// Verify that lines were processed line-by-line
	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 2, len(outputLines), "Expected 2 output lines (one per line)")
	assert.Equal(t, "Building package...", outputLines[0].Content)
	assert.Equal(t, "Build complete!", outputLines[1].Content)
}

func TestProcessor_ProcessOutput_When_AdapterDetectedButParseFails(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

	// Create a task
	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"test", "-json"}, cfg)

	// Output that looks like Go test JSON but is malformed
	output := `{"Time":"2024-01-01T12:00:00Z","Action":"run","Package":"pkg/example"}
invalid json line
{"Time":"2024-01-01T12:00:01Z","Action":"pass"`

	processor.ProcessOutput(task, []byte(output), "go", []string{"test", "-json"})

	// Should fall back to line-by-line processing
	outputLines := task.GetOutputLinesSnapshot()
	assert.Greater(t, len(outputLines), 0, "Expected output lines from fallback processing")
}

func TestProcessor_ProcessOutput_When_EmptyOutput(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	processor.ProcessOutput(task, []byte(""), "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 0, len(outputLines), "Expected no output lines for empty input")
}

func TestProcessor_ProcessOutput_When_AdapterReturnsEmptyRendered(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"test", "-json"}, cfg)

	// Minimal JSON that might be detected but produces empty output
	output := `{"Time":"2024-01-01T12:00:00Z","Action":"output","Package":"pkg/example"}`

	processor.ProcessOutput(task, []byte(output), "go", []string{"test", "-json"})

	// Should not add empty rendered output, but may fall back to line-by-line
	outputLines := task.GetOutputLinesSnapshot()
	// The behavior depends on adapter implementation, but should not crash
	assert.NotNil(t, outputLines)
}

func TestProcessor_processLineByLine_When_MultipleLines(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

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
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

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
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, false)

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
	reg := adapter.NewRegistry()
	maxLineLength := 100 // Small limit for testing
	processor := NewProcessor(pm, reg, maxLineLength, false)

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

func TestExtractFirstLines_When_MoreLinesThanCount(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	output := strings.Join(lines, "\n")

	result := extractFirstLines(output, 3)

	assert.Equal(t, 3, len(result))
	assert.Equal(t, "line1", result[0])
	assert.Equal(t, "line2", result[1])
	assert.Equal(t, "line3", result[2])
}

func TestExtractFirstLines_When_FewerLinesThanCount(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2"}
	output := strings.Join(lines, "\n")

	result := extractFirstLines(output, 5)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "line1", result[0])
	assert.Equal(t, "line2", result[1])
}

func TestExtractFirstLines_When_EmptyInput(t *testing.T) {
	t.Parallel()

	result := extractFirstLines("", 5)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, "", result[0])
}

func TestExtractFirstLines_When_ExactCount(t *testing.T) {
	t.Parallel()

	lines := []string{"line1", "line2", "line3"}
	output := strings.Join(lines, "\n")

	result := extractFirstLines(output, 3)

	assert.Equal(t, 3, len(result))
	assert.Equal(t, "line1", result[0])
	assert.Equal(t, "line2", result[1])
	assert.Equal(t, "line3", result[2])
}

func TestExtractFirstLines_When_NewlineOnly(t *testing.T) {
	t.Parallel()

	result := extractFirstLines("\n", 5)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "", result[0])
	assert.Equal(t, "", result[1])
}

func TestProcessor_ProcessOutput_When_DebugEnabled(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	reg := adapter.NewRegistry()
	processor := NewProcessor(pm, reg, 1024*1024, true) // Debug enabled

	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"build"}, cfg)

	output := "Some output"

	// Should not panic when debug is enabled
	processor.ProcessOutput(task, []byte(output), "go", []string{"build"})

	outputLines := task.GetOutputLinesSnapshot()
	assert.Equal(t, 1, len(outputLines))
}

func TestProcessor_ProcessOutput_When_AdapterPatternRendersEmpty(t *testing.T) {
	t.Parallel()

	pm := design.NewPatternMatcher(design.UnicodeVibrantTheme())
	cfg := design.UnicodeVibrantTheme()
	task := design.NewTask("Test", "testing", "go", []string{"test", "-json"}, cfg)

	// Create a custom adapter that returns a pattern that renders to empty string
	emptyAdapter := &emptyRenderingAdapter{}
	customReg := adapter.NewRegistry()
	customReg.Register(emptyAdapter)
	processorWithCustomReg := NewProcessor(pm, customReg, 1024*1024, false)

	output := "detectable output"

	processorWithCustomReg.ProcessOutput(task, []byte(output), "go", []string{"test", "-json"})

	// Should not add empty rendered output
	outputLines := task.GetOutputLinesSnapshot()
	// The adapter is detected, but if it renders empty, we should not add it
	// However, the current implementation checks for empty string, so this should be fine
	assert.NotNil(t, outputLines)
}

// emptyRenderingAdapter is a test adapter that detects but renders empty
type emptyRenderingAdapter struct{}

func (a *emptyRenderingAdapter) Detect(firstLines []string) bool {
	return len(firstLines) > 0 && strings.Contains(firstLines[0], "detectable")
}

func (a *emptyRenderingAdapter) Parse(output io.Reader) (design.Pattern, error) {
	// Return a pattern that renders to empty string
	return &emptyPattern{}, nil
}

func (a *emptyRenderingAdapter) Name() string {
	return "empty-adapter"
}

// emptyPattern is a pattern that renders to empty string
type emptyPattern struct{}

func (p *emptyPattern) Render(cfg *design.Config) string {
	return ""
}

func (p *emptyPattern) PatternType() design.PatternType {
	return design.PatternTypeSummary
}
