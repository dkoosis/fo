package wrapper_test

import (
	"bytes"
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
			var buf bytes.Buffer
			// No wrapper-specific args — tests the interface contract, not business logic.
			// Wrappers with required flags (e.g. diag --tool) will return an error, which is fine.
			// The contract: empty input must not panic, regardless of flag state.
			_ = w.Wrap([]string{}, strings.NewReader(""), &buf)
		})
	}
}

func TestAllWrappers_GetNilForUnknown(t *testing.T) {
	if w := wrapper.Get("nonexistent"); w != nil {
		t.Error("expected nil for unknown wrapper")
	}
}
