package wrapjscpd

import "io"

// Convert reads jscpd JSON from r and writes SARIF to w.
// Plain-function entry point used by the v2 CLI dispatch — same behavior
// as the plugin (*jscpd).Convert, no flags.
func Convert(r io.Reader, w io.Writer) error {
	return (&jscpd{}).Convert(r, w)
}
