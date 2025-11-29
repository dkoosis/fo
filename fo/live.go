// Package fo provides live rendering with in-place terminal updates.
package fo

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// LiveRenderer handles in-place terminal updates for streaming output.
// It tracks rendered lines and uses ANSI cursor control for updates.
type LiveRenderer struct {
	out          io.Writer
	lineCount    int  // Number of lines currently rendered
	isTTY        bool // Whether output is a terminal
	cursorHidden bool
	mu           sync.Mutex
}

// NewLiveRenderer creates a renderer for the given output.
// If out is not a TTY, updates will append rather than overwrite.
func NewLiveRenderer(out io.Writer) *LiveRenderer {
	isTTY := false
	if f, ok := out.(*os.File); ok {
		isTTY = term.IsTerminal(int(f.Fd()))
	}
	return &LiveRenderer{
		out:   out,
		isTTY: isTTY,
	}
}

// Start begins a live rendering session.
// Hides cursor if TTY to reduce visual noise during updates.
func (r *LiveRenderer) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isTTY && !r.cursorHidden {
		_, _ = fmt.Fprint(r.out, "\033[?25l") // Hide cursor
		r.cursorHidden = true
	}
	r.lineCount = 0
}

// Complete ends the live rendering session.
// Shows cursor and optionally leaves final state visible.
func (r *LiveRenderer) Complete() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cursorHidden {
		_, _ = fmt.Fprint(r.out, "\033[?25h") // Show cursor
		r.cursorHidden = false
	}
}

// Clear removes all rendered lines from the display.
func (r *LiveRenderer) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isTTY || r.lineCount == 0 {
		return
	}

	// Move cursor up and clear each line
	for i := 0; i < r.lineCount; i++ {
		_, _ = fmt.Fprint(r.out, "\033[A\033[K") // Up one line, clear to end
	}
	r.lineCount = 0
}

// Render displays lines, replacing any previous content.
// This is atomic - the entire update happens in one write.
func (r *LiveRenderer) Render(lines []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var sb strings.Builder

	if r.isTTY && r.lineCount > 0 {
		// Move cursor up to overwrite previous content
		for i := 0; i < r.lineCount; i++ {
			sb.WriteString("\033[A") // Move up
		}
		sb.WriteString("\r") // Return to start of line
	}

	// Write all lines
	for i, line := range lines {
		if r.isTTY {
			sb.WriteString("\033[K") // Clear to end of line
		}
		sb.WriteString(line)
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}

	if len(lines) > 0 {
		sb.WriteString("\n")
	}

	// Single atomic write
	_, _ = fmt.Fprint(r.out, sb.String())
	r.lineCount = len(lines)
}

// AppendLine adds a new line without affecting previous content.
// Use this for streaming output that shouldn't be overwritten.
func (r *LiveRenderer) AppendLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, _ = fmt.Fprintln(r.out, line)
	r.lineCount++
}

// UpdateLastLine overwrites just the last rendered line.
// Useful for progress indicators or status updates.
func (r *LiveRenderer) UpdateLastLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isTTY {
		// Non-TTY: just append
		_, _ = fmt.Fprintln(r.out, line)
		r.lineCount++
		return
	}

	if r.lineCount > 0 {
		// Move up one line, clear it, write new content
		_, _ = fmt.Fprintf(r.out, "\033[A\033[K%s\n", line)
	} else {
		_, _ = fmt.Fprintln(r.out, line)
		r.lineCount = 1
	}
}

// IsTTY returns whether the output supports cursor control.
func (r *LiveRenderer) IsTTY() bool {
	return r.isTTY
}
