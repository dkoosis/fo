package main

import (
	"context"
	"io"
	"os"

	"golang.org/x/term"
)

// keyControl wires raw-TTY keyboard input into the watch loop.
//
// When stdin is a TTY, putRawKeys puts it in cbreak/raw mode and spawns a
// goroutine that reads one byte at a time:
//   - 'r' or 'R'             → push a manual trigger on the returned channel
//   - 'q', 'Q', or Ctrl-C    → call cancel to terminate the watch loop
//
// In raw mode the kernel no longer translates Ctrl-C to SIGINT, so byte 0x03
// is treated explicitly. SIGINT/SIGTERM delivered out-of-band (e.g. by a
// parent process) still works because signal.NotifyContext owns ctx.
//
// restore MUST be called on every exit path. It is idempotent.
//
// If stdin is not a TTY (piped/redirected) or any setup step fails, this
// function returns (nil, no-op restore, nil) and the caller falls back to the
// non-keyboard path.
func keyControl(ctx context.Context, in io.Reader, cancel context.CancelFunc) (<-chan struct{}, func()) {
	f, ok := in.(*os.File)
	if !ok {
		return nil, func() {}
	}
	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return nil, func() {}
	}
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, func() {}
	}

	var restored bool
	restore := func() {
		if restored {
			return
		}
		restored = true
		_ = term.Restore(fd, oldState)
	}

	out := make(chan struct{}, 1)
	// Best-effort: a blocking Read on the raw TTY can't be interrupted by ctx
	// alone. Closing the descriptor from another goroutine unblocks it.
	go func() {
		<-ctx.Done()
		_ = f.Close()
	}()
	go readKeys(f, out, cancel)
	return out, restore
}

// readKeys is the goroutine body for keyControl. It exits when the underlying
// reader returns an error (typically caused by Close on ctx cancel).
func readKeys(r io.Reader, out chan<- struct{}, cancel context.CancelFunc) {
	defer close(out)
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		switch buf[0] {
		case 'r', 'R':
			// Non-blocking send: one rerun in flight is enough; further presses
			// during a rerun are dropped rather than queued.
			select {
			case out <- struct{}{}:
			default:
			}
		case 'q', 'Q', 0x03, 0x04: // Ctrl-C, Ctrl-D
			cancel()
			return
		}
	}
}

// fanIn merges two trigger channels into one. The output closes when both
// inputs are closed or ctx is canceled. A nil channel is treated as a
// permanently blocked source (Go's standard idiom).
func fanIn(ctx context.Context, a, b <-chan struct{}) <-chan struct{} {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	out := make(chan struct{})
	go func() {
		defer close(out)
		for a != nil || b != nil {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-a:
				if !ok {
					a = nil
					continue
				}
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				}
			case _, ok := <-b:
				if !ok {
					b = nil
					continue
				}
				select {
				case out <- struct{}{}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
