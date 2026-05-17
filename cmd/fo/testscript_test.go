package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"fo": func() {
			os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:           "testdata/script",
		UpdateScripts: os.Getenv("UPDATE_SCRIPTS") == "1",
	})
}
