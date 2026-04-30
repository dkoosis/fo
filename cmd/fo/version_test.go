package main

import (
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	for _, arg := range []string{"--version", "-version", "version"} {
		t.Run(arg, func(t *testing.T) {
			stdout, _, err := executeCommand(arg)
			if err != nil {
				t.Fatalf("run %s: %v", arg, err)
			}
			if strings.TrimSpace(stdout) == "" {
				t.Fatalf("expected non-empty version on stdout, got %q", stdout)
			}
		})
	}
}

func TestResolveVersionLdflagsWins(t *testing.T) {
	prev := version
	t.Cleanup(func() { version = prev })

	version = "v9.9.9-test"
	if got := resolveVersion(); got != "v9.9.9-test" {
		t.Fatalf("resolveVersion() = %q, want v9.9.9-test", got)
	}
}
