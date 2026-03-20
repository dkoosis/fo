// Package stream provides real-time streaming display for go test -json output.
package stream

import (
	"fmt"
	"io"
)

// termWriter is the single point of terminal output in streaming mode.
// All output flows through this struct — no other code writes to stdout
// during streaming.
type termWriter struct {
	out         io.Writer
	width       int
	height      int
	footerLines int
}

func newTermWriter(out io.Writer, width, height int) *termWriter {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return &termWriter{out: out, width: width, height: height}
}

// PrintLine writes a line to the scrolling history region.
// Always appends \n.
func (w *termWriter) PrintLine(s string) {
	fmt.Fprintln(w.out, s)
}

// EraseFooter removes the current footer from the terminal.
// No-op if footerLines == 0.
func (w *termWriter) EraseFooter() {
	if w.footerLines == 0 {
		return
	}
	for i := 0; i < w.footerLines; i++ {
		fmt.Fprint(w.out, "\r\033[2K")
		if i < w.footerLines-1 {
			fmt.Fprint(w.out, "\033[1A")
		}
	}
	fmt.Fprint(w.out, "\r")
	w.footerLines = 0
}

// DrawFooter prints footer lines, truncated to terminal width.
// Caps to min(count, max(3, height/3)).
func (w *termWriter) DrawFooter(lines []string) {
	maxLines := w.height / 3
	if maxLines < 3 {
		maxLines = 3
	}

	capped := len(lines) > maxLines
	show := lines
	if capped {
		show = lines[:maxLines-1]
	}

	for _, line := range show {
		fmt.Fprintln(w.out, truncateToWidth(line, w.width))
	}
	if capped {
		overflow := len(lines) - len(show)
		fmt.Fprintln(w.out, truncateToWidth(fmt.Sprintf("  ... and %d more", overflow), w.width))
	}
	w.footerLines = len(show)
	if capped {
		w.footerLines++
	}
}

func truncateToWidth(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
