// Package lineread provides a bounded line reader used by fo's input
// pipelines. Replaces bufio.Scanner so that a single oversized line
// (>MaxLineLen) is drained-and-skipped instead of fatally aborting the
// stream — fo-gn0.
package lineread

import (
	"bufio"
	"errors"
)

// MaxLineLen caps a single line at 16 MiB. Lines longer than this are
// drained to the next newline and reported as oversized.
const MaxLineLen = 16 * 1024 * 1024

// Read returns the next line from br (without the trailing '\n'), a flag
// set when the line exceeded MaxLineLen and was discarded, and the
// underlying reader's error (typically io.EOF) when the reader is
// exhausted.
//
// On EOF with a final partial line (no terminating newline), Read returns
// that line with err set.
func Read(br *bufio.Reader) ([]byte, bool, error) {
	var buf []byte
	oversize := false
	for {
		slice, err := br.ReadSlice('\n')
		switch {
		case err == nil:
			return finishLine(buf, slice, oversize)
		case errors.Is(err, bufio.ErrBufferFull):
			buf, oversize = accumulate(buf, slice, oversize)
		default:
			return finishOnError(buf, slice, oversize, err)
		}
	}
}

func finishLine(buf, slice []byte, oversize bool) ([]byte, bool, error) {
	if oversize {
		return nil, true, nil
	}
	if len(buf)+len(slice)-1 > MaxLineLen {
		return nil, true, nil
	}
	return append(buf, dropNL(slice)...), false, nil
}

func accumulate(buf, slice []byte, oversize bool) ([]byte, bool) {
	if oversize || len(buf)+len(slice) > MaxLineLen {
		return nil, true
	}
	return append(buf, slice...), false
}

func finishOnError(buf, slice []byte, oversize bool, err error) ([]byte, bool, error) {
	if oversize {
		return nil, true, err
	}
	if len(slice) > 0 {
		return append(buf, dropNL(slice)...), false, err
	}
	if len(buf) > 0 {
		return buf, false, err
	}
	return nil, false, err
}

func dropNL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		return b[:n-1]
	}
	return b
}
