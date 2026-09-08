package scene

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/dkoosis/fo/internal/textwidth"
)

// clearHome resets the terminal before each frame: clear the whole screen
// (CSI 2J) then move the cursor home (CSI H). The cast is a full-redraw
// stream (see Cast / docs/design/cast-rail-visual.md), so every frame
// repaints from a clean slate rather than appending a delta.
const clearHome = "\x1b[2J\x1b[H"

// EncodeAsciicast writes frames to w as an asciinema v2 recording.
//
// Line 1 is the JSON header (version 2 + terminal geometry); each frame
// becomes one "o" (output) event at a cumulative timestamp, the sum of
// frame Delays up to and including it. Every event repaints the screen
// (clear + home) before the frame Content — the full-redraw model, so the
// recording needs no delta bookkeeping and plays on any terminal.
//
// Geometry is measured from the frames: width is the widest visible line
// (ANSI stripped), height the tallest frame's line count. A player uses
// these as the initial viewport.
func EncodeAsciicast(w io.Writer, frames []Frame) error {
	width, height := castDimensions(frames)

	header := struct {
		Version int `json:"version"`
		Width   int `json:"width"`
		Height  int `json:"height"`
	}{Version: 2, Width: width, Height: height}

	enc := json.NewEncoder(w)
	if err := enc.Encode(header); err != nil {
		return err
	}

	// Accumulate in integer nanoseconds (time.Duration) and convert to
	// seconds once per event. Summing the float Seconds() directly would
	// drift — e.g. 9.9 surfaces as 9.899999999999999 in the cast.
	var elapsed time.Duration
	for _, f := range frames {
		elapsed += f.Delay
		event := []any{elapsed.Seconds(), "o", clearHome + f.Content}
		if err := enc.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

// castDimensions returns the viewport that fits every frame: the widest
// visible line across all frames and the most lines in any single frame.
// Both have a floor of 1 so an empty scene still yields a valid header.
func castDimensions(frames []Frame) (width, height int) {
	width, height = 1, 1
	for _, f := range frames {
		lines := strings.Split(strings.TrimRight(f.Content, "\n"), "\n")
		if n := len(lines); n > height {
			height = n
		}
		for _, line := range lines {
			if v := textwidth.Visible(line); v > width {
				width = v
			}
		}
	}
	return width, height
}
