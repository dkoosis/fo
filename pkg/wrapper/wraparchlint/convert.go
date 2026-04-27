package wraparchlint

import "io"

// Convert reads go-arch-lint JSON from r and writes SARIF to w.
// Plain-function entry point used by the v2 CLI dispatch — same behavior
// as the plugin (*archlint).Convert, no flags.
func Convert(r io.Reader, w io.Writer) error {
	return (&archlint{}).Convert(r, w)
}
