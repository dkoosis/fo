package stream

import (
	"bytes"
	"strings"
	"testing"
)

func TestTermWriter_PrintLine_AppendsNewline(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.PrintLine("hello")
	got := buf.String()
	if got != "hello\n" {
		t.Errorf("PrintLine output = %q, want %q", got, "hello\n")
	}
}

func TestTermWriter_DrawFooter_TracksLineCount(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.DrawFooter([]string{"line1", "line2", "line3"})
	if tw.footerLines != 3 {
		t.Errorf("footerLines = %d, want 3", tw.footerLines)
	}
}

func TestTermWriter_EraseFooter_WhenZeroLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.EraseFooter()
	if buf.Len() != 0 {
		t.Errorf("EraseFooter with 0 lines wrote %d bytes, want 0", buf.Len())
	}
}

func TestTermWriter_EraseFooter_ClearsLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)
	tw.DrawFooter([]string{"line1", "line2"})
	buf.Reset()

	tw.EraseFooter()
	got := buf.String()
	if !strings.Contains(got, "\033[1A") {
		t.Error("EraseFooter missing cursor-up escape")
	}
	if !strings.Contains(got, "\033[2K") {
		t.Error("EraseFooter missing erase-line escape")
	}
	if tw.footerLines != 0 {
		t.Errorf("footerLines after erase = %d, want 0", tw.footerLines)
	}
}

func TestTermWriter_DrawFooter_TruncatesToWidth(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 20, 24)
	tw.DrawFooter([]string{"this is a very long line that exceeds twenty chars"})
	got := buf.String()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	for _, line := range lines {
		plain := stripANSI(line)
		if len([]rune(plain)) > 20 {
			t.Errorf("footer line %q exceeds width 20 (len=%d)", plain, len([]rune(plain)))
		}
	}
}

func TestTermWriter_DrawFooter_CapsToMaxLines(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 12) // height 12 → max footer = max(3, 12/3) = 4
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "pkg" + string(rune('A'+i))
	}
	tw.DrawFooter(lines)
	if tw.footerLines > 4 {
		t.Errorf("footerLines = %d, want <= 4 (capped by height)", tw.footerLines)
	}
	got := buf.String()
	if !strings.Contains(got, "... and") {
		t.Error("capped footer should contain '... and N more'")
	}
}

func TestTermWriter_EraseAndRedraw_Cycle(t *testing.T) {
	var buf bytes.Buffer
	tw := newTermWriter(&buf, 80, 24)

	tw.DrawFooter([]string{"footer1"})
	tw.EraseFooter()
	tw.PrintLine("history line")
	tw.DrawFooter([]string{"footer2"})

	got := buf.String()
	if !strings.Contains(got, "history line") {
		t.Error("missing history line in output")
	}
	if tw.footerLines != 1 {
		t.Errorf("final footerLines = %d, want 1", tw.footerLines)
	}
}

func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'Z') && (s[j] < 'a' || s[j] > 'z') {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}
