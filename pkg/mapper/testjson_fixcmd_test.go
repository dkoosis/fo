package mapper

import (
	"testing"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/testjson"
)

func TestFromTestJSON_FixCommand_FailedTest(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{
			Name:   "github.com/acme/fo/pkg/foo",
			Failed: 1,
			FailedTests: []testjson.FailedTest{
				{Name: "TestBar", Output: []string{"fail"}},
			},
		},
	}

	patterns := FromTestJSON(results)
	tbl := findFailTable(t, patterns, "FAIL")
	got := tbl.Results[0].FixCommand
	want := "go test -run ^TestBar$ github.com/acme/fo/pkg/foo -v"
	if got != want {
		t.Fatalf("FixCommand = %q, want %q", got, want)
	}
}

func TestFromTestJSON_FixCommand_Subtest(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{
			Name:   "github.com/acme/fo/pkg/foo",
			Failed: 1,
			FailedTests: []testjson.FailedTest{
				{Name: "TestBar/case_1", Output: []string{"fail"}},
			},
		},
	}

	patterns := FromTestJSON(results)
	tbl := findFailTable(t, patterns, "FAIL")
	got := tbl.Results[0].FixCommand
	want := "go test -run ^TestBar$/^case_1$ github.com/acme/fo/pkg/foo -v"
	if got != want {
		t.Fatalf("subtest FixCommand = %q, want %q", got, want)
	}
}

func TestFromTestJSON_FixCommand_BuildError(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{Name: "github.com/acme/fo/pkg/bad", BuildError: "undefined: nope"},
	}

	patterns := FromTestJSON(results)
	tbl := findFailTable(t, patterns, "BUILD FAIL")
	got := tbl.Results[0].FixCommand
	want := "go build github.com/acme/fo/pkg/bad"
	if got != want {
		t.Fatalf("build-error FixCommand = %q, want %q", got, want)
	}
}

func TestFromTestJSON_FixCommand_Panic(t *testing.T) {
	t.Parallel()

	results := []testjson.TestPackageResult{
		{
			Name:        "github.com/acme/fo/pkg/boom",
			Panicked:    true,
			PanicOutput: []string{"panic: boom"},
		},
	}

	patterns := FromTestJSON(results)
	tbl := findFailTable(t, patterns, "PANIC")
	got := tbl.Results[0].FixCommand
	want := "go test github.com/acme/fo/pkg/boom -v"
	if got != want {
		t.Fatalf("panic FixCommand = %q, want %q", got, want)
	}
}

func findFailTable(t *testing.T, patterns []pattern.Pattern, labelPrefix string) *pattern.TestTable {
	t.Helper()
	for _, p := range patterns {
		tbl, ok := p.(*pattern.TestTable)
		if !ok {
			continue
		}
		if len(tbl.Label) >= len(labelPrefix) && tbl.Label[:len(labelPrefix)] == labelPrefix {
			return tbl
		}
	}
	t.Fatalf("no TestTable with label prefix %q in %d patterns", labelPrefix, len(patterns))
	return nil
}
