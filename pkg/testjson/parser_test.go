package testjson

import (
	"strings"
	"testing"
)

func TestParseStream_Behavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		inputLines      []string
		wantMalformed   int
		wantPackageName string
		wantPassed      int
		wantFailed      int
		wantSkipped     int
		wantStatus      string
		wantCoverage    float64
		wantPanicked    bool
		wantPackages    int
	}{
		{
			name: "pass/fail aggregation and package status",
			inputLines: []string{
				`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"run","Package":"example.com/pkg","Test":"TestB"}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"fail","Package":"example.com/pkg","Test":"TestB","Elapsed":0.2}`,
				`{"Time":"2024-01-01T00:00:00Z","Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantPassed:      1,
			wantFailed:      1,
			wantStatus:      "fail",
		},
		{
			name: "coverage is parsed from output",
			inputLines: []string{
				`{"Action":"run","Package":"example.com/pkg","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/pkg","Test":"TestA","Elapsed":0.1}`,
				`{"Action":"output","Package":"example.com/pkg","Output":"coverage: 85.3% of statements\n"}`,
				`{"Action":"pass","Package":"example.com/pkg","Elapsed":0.5}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantPassed:      1,
			wantCoverage:    85.3,
			wantStatus:      "pass",
		},
		{
			name: "panic output marks package as panicked",
			inputLines: []string{
				`{"Action":"run","Package":"example.com/pkg","Test":"TestBad"}`,
				`{"Action":"output","Package":"example.com/pkg","Test":"TestBad","Output":"panic: runtime error: index out of range\n"}`,
				`{"Action":"fail","Package":"example.com/pkg","Test":"TestBad","Elapsed":0.0}`,
				`{"Action":"fail","Package":"example.com/pkg","Elapsed":0.0}`,
			},
			wantMalformed:   0,
			wantPackages:    1,
			wantPackageName: "example.com/pkg",
			wantFailed:      1,
			wantPanicked:    true,
			wantStatus:      "fail",
		},
		{
			name: "malformed lines are skipped and counted",
			inputLines: []string{
				`not json`,
				`{bad json`,
				`{"Action":"run","Package":"x","Test":"T"}`,
				`{"Action":"pass","Package":"x","Test":"T","Elapsed":0.1}`,
				`{"Action":"pass","Package":"x","Elapsed":0.1}`,
			},
			wantMalformed:   2,
			wantPackages:    1,
			wantPackageName: "x",
			wantPassed:      1,
			wantStatus:      "pass",
		},
		{
			name: "package with no test activity is skipped",
			inputLines: []string{
				`{"Action":"start","Package":"example.com/empty"}`,
			},
			wantMalformed: 0,
			wantPackages:  0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			input := strings.Join(tt.inputLines, "\n") + "\n"
			results, malformed, err := ParseStream(strings.NewReader(input))
			if err != nil {
				t.Fatalf("ParseStream() error = %v", err)
			}
			if malformed != tt.wantMalformed {
				t.Fatalf("malformed = %d, want %d", malformed, tt.wantMalformed)
			}
			if len(results) != tt.wantPackages {
				t.Fatalf("packages = %d, want %d", len(results), tt.wantPackages)
			}
			if tt.wantPackages == 0 {
				return
			}

			got := results[0]
			if got.Name != tt.wantPackageName {
				t.Fatalf("package name = %q, want %q", got.Name, tt.wantPackageName)
			}
			if got.Passed != tt.wantPassed {
				t.Fatalf("passed = %d, want %d", got.Passed, tt.wantPassed)
			}
			if got.Failed != tt.wantFailed {
				t.Fatalf("failed = %d, want %d", got.Failed, tt.wantFailed)
			}
			if got.Skipped != tt.wantSkipped {
				t.Fatalf("skipped = %d, want %d", got.Skipped, tt.wantSkipped)
			}
			if tt.wantStatus != "" && got.Status() != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status(), tt.wantStatus)
			}
			if tt.wantCoverage > 0 && (got.Coverage < tt.wantCoverage-0.01 || got.Coverage > tt.wantCoverage+0.01) {
				t.Fatalf("coverage = %.2f, want %.2f", got.Coverage, tt.wantCoverage)
			}
			if got.Panicked != tt.wantPanicked {
				t.Fatalf("panicked = %t, want %t", got.Panicked, tt.wantPanicked)
			}
		})
	}
}

func TestComputeStats(t *testing.T) {
	t.Parallel()

	results := []TestPackageResult{
		{Name: "a", Passed: 5, Failed: 1, Skipped: 0},
		{Name: "b", Passed: 3, Failed: 0, Skipped: 2},
	}
	s := ComputeStats(results)

	if s.TotalTests != 11 {
		t.Fatalf("total tests = %d, want 11", s.TotalTests)
	}
	if s.Failed != 1 {
		t.Fatalf("failed tests = %d, want 1", s.Failed)
	}
	if s.FailedPkgs != 1 {
		t.Fatalf("failed packages = %d, want 1", s.FailedPkgs)
	}
}

func FuzzParseStream(f *testing.F) {
	f.Add(`{"Action":"run","Package":"x","Test":"T"}` + "\n" + `{"Action":"pass","Package":"x","Test":"T","Elapsed":0.1}` + "\n")
	f.Add(`not-json` + "\n" + `{"Action":"output","Package":"x","Output":"coverage: 80.0% of statements\n"}` + "\n")
	f.Add(`{"Action":"output","Package":"x","Output":"panic: boom\n"}` + "\n" + `{"Action":"fail","Package":"x","Elapsed":0.0}` + "\n")

	f.Fuzz(func(t *testing.T, input string) {
		results, malformed, err := ParseStream(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStream should not fail for arbitrary input: %v", err)
		}
		if malformed < 0 {
			t.Fatalf("malformed should never be negative: %d", malformed)
		}
		_ = ComputeStats(results)
	})
}
