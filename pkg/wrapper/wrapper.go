// Package wrapper defines the plugin interface for converting tool output
// into fo-native formats (SARIF or go-test-json).
package wrapper

import "io"

// Format identifies a fo-native output format.
type Format string

const (
	FormatSARIF    Format = "sarif"
	FormatTestJSON Format = "testjson"
)

// Wrapper converts tool-specific output into a fo-native format.
type Wrapper interface {
	// OutputFormat returns the native format this wrapper produces.
	OutputFormat() Format

	// Wrap reads tool output from r, writes fo-native output to w.
	// args contains any flags passed after "fo wrap <name>".
	Wrap(args []string, r io.Reader, w io.Writer) error
}
