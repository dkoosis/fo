package wrapcover

import (
	"bytes"
	"strings"
	"testing"
)

func TestConvert_basic(t *testing.T) {
	in := "github.com/x/y/foo.go:12:\tFoo\t100.0%\n" +
		"github.com/x/y/foo.go:20:\tBar\t75.0%\n" +
		"total:\t\t\t\t(statements)\t87.3%\n"
	var out bytes.Buffer
	if err := Convert(strings.NewReader(in), &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# fo:metrics tool=cover", "github.com/x/y/foo.go:12:Foo 100", "total 87.3 %"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}
