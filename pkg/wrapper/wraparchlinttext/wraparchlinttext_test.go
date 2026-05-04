package wraparchlinttext

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvert_basic(t *testing.T) {
	in := `[Warning] Component "internal/foo" shouldn't import component "internal/bar"
  internal/foo/x.go: imports forbidden component
[Warning] Component "internal/baz" shouldn't import component "internal/qux"
  internal/baz/y.go: imports forbidden component
total notices: 2
`
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, `"version": "2.1.0"`) {
		t.Errorf("not SARIF: %s", got)
	}
	if !strings.Contains(got, "internal/foo") {
		t.Errorf("missing rule context:\n%s", got)
	}
	if !strings.Contains(got, "internal/foo/x.go") {
		t.Errorf("missing location:\n%s", got)
	}
}

func TestConvert_empty(t *testing.T) {
	var out bytes.Buffer
	if err := Convert(strings.NewReader("total notices: 0\n"), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if !strings.Contains(out.String(), `"version": "2.1.0"`) {
		t.Errorf("expected SARIF envelope even when empty")
	}
}
