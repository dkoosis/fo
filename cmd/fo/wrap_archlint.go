package main

import (
	"fmt"
	"io"
)

func runWrapArchlint(_ io.Reader, _, stderr io.Writer) int {
	fmt.Fprintf(stderr, "fo wrap archlint: not yet implemented\n")
	return 2
}
