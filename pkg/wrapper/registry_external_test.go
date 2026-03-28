package wrapper_test

import (
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/wrapper"
)

type fakeWrapper struct {
	format wrapper.Format
}

func (f fakeWrapper) OutputFormat() wrapper.Format                    { return f.format }
func (f fakeWrapper) Wrap(_ []string, _ io.Reader, _ io.Writer) error { return nil }

func TestRegistry_Get_ReturnsNil_When_NameIsUnknown(t *testing.T) {
	if got := wrapper.Get("__missing_wrapper_name__"); got != nil {
		t.Fatalf("Get(unknown) = %T, want nil", got)
	}
}

func TestRegistry_Register_ReturnsRegisteredWrapper_When_NameExists(t *testing.T) {
	// Registry is process-global mutable state. Keep this test non-parallel.
	name := "test-wrapper-register-contract"
	want := fakeWrapper{format: wrapper.FormatSARIF}
	wrapper.Register(name, want)

	got := wrapper.Get(name)
	if got == nil {
		t.Fatalf("Get(%q) = nil, want registered wrapper", name)
	}
	if got.OutputFormat() != wrapper.FormatSARIF {
		t.Fatalf("Get(%q).OutputFormat() = %q, want %q", name, got.OutputFormat(), wrapper.FormatSARIF)
	}
}

func TestRegistry_Register_ReplacesExistingWrapper_When_NameAlreadyRegistered(t *testing.T) {
	// Registry is process-global mutable state. Keep this test non-parallel.
	name := "test-wrapper-overwrite-contract"
	wrapper.Register(name, fakeWrapper{format: wrapper.FormatSARIF})
	wrapper.Register(name, fakeWrapper{format: wrapper.FormatTestJSON})

	got := wrapper.Get(name)
	if got == nil {
		t.Fatalf("Get(%q) = nil, want updated wrapper", name)
	}
	if got.OutputFormat() != wrapper.FormatTestJSON {
		t.Fatalf("OutputFormat() = %q, want %q", got.OutputFormat(), wrapper.FormatTestJSON)
	}
}

func TestRegistry_Names_ReturnsSortedNames_When_RegistryContainsMultipleEntries(t *testing.T) {
	// Registry is process-global mutable state. Keep this test non-parallel.
	for _, name := range []string{
		"test-wrapper-sort-c",
		"test-wrapper-sort-a",
		"test-wrapper-sort-b",
	} {
		wrapper.Register(name, fakeWrapper{format: wrapper.FormatSARIF})
	}

	names := wrapper.Names()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("Names() not sorted: %v", names)
	}

	var gotSubset []string
	for _, name := range names {
		if strings.HasPrefix(name, "test-wrapper-sort-") {
			gotSubset = append(gotSubset, name)
		}
	}
	wantSubset := []string{"test-wrapper-sort-a", "test-wrapper-sort-b", "test-wrapper-sort-c"}
	if len(gotSubset) != len(wantSubset) {
		t.Fatalf("subset size = %d, want %d (%v)", len(gotSubset), len(wantSubset), gotSubset)
	}
	for i := range wantSubset {
		if gotSubset[i] != wantSubset[i] {
			t.Fatalf("subset[%d] = %q, want %q (subset=%v)", i, gotSubset[i], wantSubset[i], gotSubset)
		}
	}
}
