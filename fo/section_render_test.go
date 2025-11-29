package fo

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/stretchr/testify/assert"
)

// stripANSICodes removes ANSI escape sequences from a string to calculate visual width.
func stripANSICodes(s string) string {
	var result strings.Builder
	inEscape := false
	for i := range len(s) {
		switch {
		case s[i] == '\033':
			inEscape = true
		case inEscape && s[i] == 'm':
			inEscape = false
		case !inEscape:
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

func TestConsole_RendersAlignedSectionContentLine_When_IconAndTextProvided(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	console := NewConsole(ConsoleConfig{Out: &buf})

	icon := "✓"
	text := "Build complete"

	console.PrintSectionContentLine(ContentLine{
		Icon:      icon,
		IconColor: console.GetColor("Success"),
		Text:      text,
	})

	output := strings.TrimSuffix(buf.String(), "\n")
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one rendered line, got %d", len(lines))
	}

	line := lines[0]
	stripped := stripANSICodes(line)
	box := console.calculateBoxLayout()

	if got, want := runewidth.StringWidth(stripped), box.TotalWidth; got != want {
		t.Fatalf("rendered line width mismatch: got %d, want %d", got, want)
	}

	expectedSegment := icon + " " + text
	if !strings.Contains(stripped, expectedSegment) {
		t.Fatalf("expected rendered line to contain %q, got %q", expectedSegment, stripped)
	}

	iconIndex := -1
	for idx, r := range []rune(stripped) {
		if string(r) == icon {
			iconIndex = idx
			break
		}
	}
	if iconIndex < 0 {
		t.Fatalf("icon %q not found in rendered line: %q", icon, stripped)
	}

	if got, want := iconIndex, 1+box.LeftPadding; got != want {
		t.Fatalf("icon position mismatch: got %d, want %d", got, want)
	}
}

func TestConsole_PreservesSectionWidth_When_TextFillsContentArea(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	console := NewConsole(ConsoleConfig{Out: &buf})
	box := console.calculateBoxLayout()

	icon := "✓"
	iconWidth := runewidth.StringWidth(icon) + 1 // icon + trailing space
	availableTextWidth := (box.TotalWidth - 2) - box.LeftPadding - box.RightPadding - iconWidth
	if availableTextWidth <= 0 {
		t.Fatalf("unexpected non-positive available text width: %d", availableTextWidth)
	}

	text := strings.Repeat("X", availableTextWidth)
	console.PrintSectionContentLine(ContentLine{Icon: icon, Text: text})

	output := strings.TrimSuffix(buf.String(), "\n")
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one rendered line, got %d", len(lines))
	}

	line := lines[0]
	stripped := stripANSICodes(line)

	if got, want := runewidth.StringWidth(stripped), box.TotalWidth; got != want {
		t.Fatalf("rendered line width mismatch: got %d, want %d", got, want)
	}

	if !strings.HasSuffix(stripped, box.BorderChars.Vertical) {
		t.Fatalf("expected rendered line to end with border %q, got %q", box.BorderChars.Vertical, stripped)
	}
}

func TestLiveSection_AddRow_When_NewRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRow("row1", "Content 1")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "row1", rows[0].ID)
	assert.Equal(t, "Content 1", rows[0].Content)
	assert.False(t, rows[0].Expanded)
}

func TestLiveSection_UpdateRow_When_ExistingRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRow("row1", "Content 1")
	ls.UpdateRow("row1", "Updated Content")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "Updated Content", rows[0].Content)
}

func TestLiveSection_RemoveRow_When_ExistingRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRow("row1", "Content 1")
	ls.AddRow("row2", "Content 2")
	ls.RemoveRow("row1")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "row2", rows[0].ID)
}

func TestLiveSection_GetRows_When_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })

	// Add rows concurrently
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			ls.AddRow(fmt.Sprintf("row%d", id), fmt.Sprintf("Content %d", id))
		}(i)
	}
	wg.Wait()

	rows := ls.GetRows()
	assert.Len(t, rows, numGoroutines)
}

func TestLiveSection_AddRowWithExpansion_When_ExpandedContentProvided(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	expandedContent := []string{"Detail 1", "Detail 2", "Detail 3"}
	ls.AddRowWithExpansion("row1", "Summary", expandedContent)

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, "Summary", rows[0].Content)
	assert.False(t, rows[0].Expanded)
	assert.Equal(t, expandedContent, rows[0].ExpandedContent)
}

func TestLiveSection_ExpandRow_When_ExistingRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRowWithExpansion("row1", "Summary", []string{"Detail 1"})
	ls.ExpandRow("row1")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.True(t, rows[0].Expanded)
}

func TestLiveSection_CollapseRow_When_ExpandedRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRowWithExpansion("row1", "Summary", []string{"Detail 1"})
	ls.ExpandRow("row1")
	ls.CollapseRow("row1")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.False(t, rows[0].Expanded)
}

func TestLiveSection_ToggleRowExpansion_When_ExistingRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRowWithExpansion("row1", "Summary", []string{"Detail 1"})
	
	// Toggle from false to true
	ls.ToggleRowExpansion("row1")
	rows := ls.GetRows()
	assert.True(t, rows[0].Expanded)

	// Toggle from true to false
	ls.ToggleRowExpansion("row1")
	rows = ls.GetRows()
	assert.False(t, rows[0].Expanded)
}

func TestLiveSection_SetExpandedContent_When_ExistingRow(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	ls.AddRow("row1", "Summary")
	ls.SetExpandedContent("row1", []string{"New Detail 1", "New Detail 2"})

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, []string{"New Detail 1", "New Detail 2"}, rows[0].ExpandedContent)
}

func TestLiveSection_GetRows_When_ExpandedContent(t *testing.T) {
	t.Parallel()

	ls := NewLiveSection("Test Section", func(*LiveSection) error { return nil })
	expandedContent := []string{"Detail 1", "Detail 2"}
	ls.AddRowWithExpansion("row1", "Summary", expandedContent)
	ls.ExpandRow("row1")

	rows := ls.GetRows()
	assert.Len(t, rows, 1)
	assert.True(t, rows[0].Expanded)
	assert.Equal(t, expandedContent, rows[0].ExpandedContent)
	// Verify it's a deep copy (modifying original shouldn't affect snapshot)
	expandedContent[0] = "Modified"
	assert.Equal(t, []string{"Detail 1", "Detail 2"}, rows[0].ExpandedContent)
}
