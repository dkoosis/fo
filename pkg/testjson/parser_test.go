package testjson

import (
	"strings"
	"testing"
)

func TestParseStream_BasicPassFail(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestB"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/pkg","Test":"TestB","Elapsed":0.2}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	results, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 package, got %d", len(results))
	}

	r := results[0]
	if r.Passed != 1 {
		t.Errorf("expected 1 passed, got %d", r.Passed)
	}
	if r.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", r.Failed)
	}
	if r.Status() != "fail" { //nolint:goconst // test data
		t.Errorf("expected status fail, got %s", r.Status())
	}
}

func TestParseStream_Coverage(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg","Output":"coverage: 85.3% of statements\n"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
	}, "\n") + "\n"

	results, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 package, got %d", len(results))
	}
	if results[0].Coverage < 85.0 || results[0].Coverage > 86.0 {
		t.Errorf("expected coverage ~85.3, got %f", results[0].Coverage)
	}
}

func TestParseStream_PanicDetection(t *testing.T) {
	input := strings.Join([]string{
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestBad"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"output","Package":"example.com/pkg","Test":"TestBad","Output":"panic: runtime error: index out of range\n"}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/pkg","Test":"TestBad","Elapsed":0.0}`,
		`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/pkg","Elapsed":0.0}`,
	}, "\n") + "\n"

	results, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 package, got %d", len(results))
	}
	if !results[0].Panicked {
		t.Error("expected panic detected")
	}
}

func TestParseStream_SkipsEmptyPackages(t *testing.T) {
	// A package with only "start" action and no tests should be skipped
	input := `{"Time":"2024-01-01T00:00:00Z","Action":"start","Package":"example.com/empty"}` + "\n"

	results, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 packages, got %d", len(results))
	}
}

func TestParseStream_MalformedLinesSkipped(t *testing.T) {
	input := "not json\n{bad json\n" +
		`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"x","Test":"T"}` + "\n" +
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x","Test":"T","Elapsed":0.1}` + "\n" +
		`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"x","Elapsed":0.1}` + "\n"

	results, err := ParseStream(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 package (skipping malformed), got %d", len(results))
	}
	if results[0].Passed != 1 {
		t.Errorf("expected 1 passed, got %d", results[0].Passed)
	}
}

func TestComputeStats(t *testing.T) {
	results := []TestPackageResult{
		{Name: "a", Passed: 5, Failed: 1, Skipped: 0},
		{Name: "b", Passed: 3, Failed: 0, Skipped: 2},
	}
	s := ComputeStats(results)
	if s.TotalTests != 11 {
		t.Errorf("expected 11 total tests, got %d", s.TotalTests)
	}
	if s.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", s.Failed)
	}
	if s.FailedPkgs != 1 {
		t.Errorf("expected 1 failed pkg, got %d", s.FailedPkgs)
	}
}
