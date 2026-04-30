// Package wrapper is a namespace for SARIF converters that adapt third-party
// tool output to SARIF 2.1.0. Each subpackage exposes a Convert function
// that reads tool output and writes SARIF.
//
// # Contract
//
// Each wrapper subpackage exposes a top-level function:
//
//	func Convert(r io.Reader, w io.Writer) error
//
// Wrappers that need configuration take an options struct as a third
// argument (see wrapdiag.DiagOpts). The function reads the tool's native
// output from r and writes a SARIF 2.1.0 log to w. Errors are returned
// untouched; the caller (cmd/fo) prints them to stderr.
//
// There is no Wrapper interface and no registry. Dispatch lives in
// cmd/fo/main.go: a switch on the wrapper name calls the package's
// Convert directly. This keeps the seam narrow and avoids indirection
// for a list that changes once a quarter.
//
// # Adding a new wrapper
//
//  1. Create pkg/wrapper/wrap<name>/ with convert.go exposing
//     Convert(r io.Reader, w io.Writer) error.
//  2. Produce SARIF 2.1.0; reuse pkg/sarif builders where possible.
//  3. In cmd/fo/main.go: add the name to wrapNames, a description to
//     wrapDescriptions, and a case to the wrap dispatch that calls
//     <pkg>.Convert.
//  4. Add black-box tests in <pkg>_test (stdlib only — no fo internals)
//     so the wrapper stays decoupled from the rest of the tree.
package wrapper
