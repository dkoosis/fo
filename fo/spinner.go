package fo

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// SpinnerFrames defines available spinner styles
var SpinnerFrames = map[string][]string{
	"dots":    {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	"line":    {"-", "\\", "|", "/"},
	"arc":     {"◜", "◠", "◝", "◞", "◡", "◟"},
	"star":    {"✶", "✸", "✹", "✺", "✹", "✸"},
	"bounce":  {"⠁", "⠂", "⠄", "⠂"},
	"grow":    {"▁", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃"},
	"arrows":  {"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
	"clock":   {"🕛", "🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚"},
	"moon":    {"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
	"ascii":   {"-", "\\", "|", "/"},
}

// DefaultSpinnerStyle is the Claude-style dots spinner
const DefaultSpinnerStyle = "dots"

// ParseSpinnerChars parses a custom spinner chars string into frames.
// Accepts space-separated strings or individual Unicode characters.
func ParseSpinnerChars(chars string) []string {
	chars = strings.TrimSpace(chars)
	if chars == "" {
		return nil
	}

	// Try space-separated first
	if strings.Contains(chars, " ") {
		return strings.Fields(chars)
	}

	// Otherwise treat as individual runes
	var frames []string
	for _, r := range chars {
		frames = append(frames, string(r))
	}
	return frames
}

// Spinner provides an animated loading indicator
type Spinner struct {
	frames   []string
	interval time.Duration
	message  string
	color    string
	writer   io.Writer

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	frameIdx int
}

// SpinnerConfig configures the spinner
type SpinnerConfig struct {
	Style        string        // "dots", "line", "arc", "star", etc.
	CustomFrames []string      // Custom frames override Style if provided
	Interval     time.Duration // Frame interval (default 80ms)
	Message      string        // Text to show after spinner
	Color        string        // ANSI color code for spinner
	Writer       io.Writer     // Output destination
}

// NewSpinner creates a new spinner with the given config
func NewSpinner(cfg SpinnerConfig) *Spinner {
	// Priority: CustomFrames > Style > Default
	var frames []string
	if len(cfg.CustomFrames) > 0 {
		frames = cfg.CustomFrames
	} else if cfg.Style != "" {
		frames = SpinnerFrames[cfg.Style]
	}
	if frames == nil {
		frames = SpinnerFrames[DefaultSpinnerStyle]
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 80 * time.Millisecond
	}

	return &Spinner{
		frames:   frames,
		interval: interval,
		message:  cfg.Message,
		color:    cfg.Color,
		writer:   cfg.Writer,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	go s.run()
}

// Stop halts the spinner and clears the line
func (s *Spinner) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	<-s.doneCh // Wait for goroutine to finish

	// Clear the spinner line
	s.clearLine()
}

// UpdateMessage changes the spinner message while running
func (s *Spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	s.message = msg
	s.mu.Unlock()
}

func (s *Spinner) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Render initial frame
	s.render()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.frameIdx = (s.frameIdx + 1) % len(s.frames)
			s.mu.Unlock()
			s.render()
		}
	}
}

func (s *Spinner) render() {
	s.mu.Lock()
	frame := s.frames[s.frameIdx]
	msg := s.message
	color := s.color
	s.mu.Unlock()

	// Move to start of line and clear it
	fmt.Fprint(s.writer, "\r\033[K")

	// Render spinner with optional color
	if color != "" {
		fmt.Fprintf(s.writer, "%s%s\033[0m %s", color, frame, msg)
	} else {
		fmt.Fprintf(s.writer, "%s %s", frame, msg)
	}
}

func (s *Spinner) clearLine() {
	fmt.Fprint(s.writer, "\r\033[K")
}
