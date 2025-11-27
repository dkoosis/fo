package fo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
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
