// Package wrapper defines the plugin interface for converting tool output
// into fo-native formats (SARIF or go-test-json).
package wrapper

import (
	"flag"
	"io"
)

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

	// RegisterFlags adds wrapper-specific flags to the provided FlagSet.
	// Called by the framework before flag parsing. Implementations store
	// flag value pointers for use in Convert. No-op if no flags needed.
	RegisterFlags(fs *flag.FlagSet)

	// Convert reads tool output from r and writes fo-native output to w.
	// Must be called after RegisterFlags + FlagSet.Parse — implementations
	// read parsed flag values via stored pointers. Calling Convert without
	// prior registration may panic on nil pointer dereference.
	// Return an error for invalid flag values (e.g. missing required flags)
	// or conversion failures.
	Convert(r io.Reader, w io.Writer) error
}
