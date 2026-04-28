// Package boundread reads from an io.Reader with a hard byte cap so a
// pathological tool can't OOM the wrapper process. Wrappers consume external
// tool output of unknown size; without a bound, a runaway producer (or a
// malicious one) can exhaust memory.
package boundread

import (
	"errors"
	"fmt"
	"io"
)

// DefaultMax is the byte cap applied when callers don't specify one.
// 256 MiB is generous for any realistic SARIF/JSON tool report.
const DefaultMax = 256 << 20

// ErrInputTooLarge is returned when the reader produces more than the cap.
var ErrInputTooLarge = errors.New("input exceeds maximum size")

// All reads up to limit bytes from r. If r produces more, returns
// ErrInputTooLarge wrapped with the cap. limit <= 0 falls back to DefaultMax.
func All(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultMax
	}
	lr := &io.LimitedReader{R: r, N: limit + 1}
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %d bytes", ErrInputTooLarge, limit)
	}
	return data, nil
}
