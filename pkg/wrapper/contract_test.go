package wrapper_test

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/wrapper"
	_ "github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapdiag"
	_ "github.com/dkoosis/fo/pkg/wrapper/wrapjscpd"
)

func TestAllWrappers_Registered(t *testing.T) {
	names := wrapper.Names()
	expected := []string{"archlint", "diag", "jscpd"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d wrappers, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range expected {
		if names[i] != name {
			t.Errorf("expected wrapper[%d] = %q, got %q", i, name, names[i])
		}
	}
}

func TestAllWrappers_OutputFormat(t *testing.T) {
	valid := map[wrapper.Format]bool{
		wrapper.FormatSARIF:    true,
		wrapper.FormatTestJSON: true,
	}
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		if w == nil {
			t.Errorf("Get(%q) returned nil", name)
			continue
		}
		if !valid[w.OutputFormat()] {
			t.Errorf("wrapper %q: invalid OutputFormat %q", name, w.OutputFormat())
		}
	}
}

func TestAllWrappers_EmptyInputNoPanic(t *testing.T) {
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		t.Run(name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			w.RegisterFlags(fs)
			_ = fs.Parse([]string{})
			var buf bytes.Buffer
			_ = w.Convert(strings.NewReader(""), &buf)
		})
	}
}

func TestAllWrappers_RegisterFlagsNoPanic(t *testing.T) {
	for _, name := range wrapper.Names() {
		w := wrapper.Get(name)
		t.Run(name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			w.RegisterFlags(fs)
		})
	}
}

func TestAllWrappers_GetNilForUnknown(t *testing.T) {
	if w := wrapper.Get("nonexistent"); w != nil {
		t.Error("expected nil for unknown wrapper")
	}
}
