package wrapgobench

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvert_basic(t *testing.T) {
	in := `goos: darwin
pkg: x
BenchmarkFoo-10  1000  1234 ns/op     56 B/op     2 allocs/op
`
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# fo:metrics tool=gobench", "BenchmarkFoo/ns_op 1234 ns/op", "BenchmarkFoo/allocs_op 2 allocs/op"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestConvert_skipsHeaders(t *testing.T) {
	in := "goos: darwin\nPASS\nok  \tx\t1.234s\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if got := out.String(); got != "# fo:metrics tool=gobench\n" {
		t.Errorf("expected header-only output, got: %q", got)
	}
}
