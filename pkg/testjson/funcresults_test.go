package testjson

import (
	"testing"
)

const (
	testFooName = "TestFoo"
	pkgAName    = "pkg/a"
	pkgBName    = "pkg/b"
)

func TestFuncResults(t *testing.T) {
	tests := []struct {
		name   string
		events []TestEvent
		want   map[FuncKey]FuncResult
	}{
		{
			name:   "empty",
			events: nil,
			want:   map[FuncKey]FuncResult{},
		},
		{
			name: "pass and fail in different packages",
			events: []TestEvent{
				{Action: "run", Package: pkgAName, Test: testFooName},
				{Action: actionPass, Package: pkgAName, Test: testFooName},
				{Action: "run", Package: pkgBName, Test: testFooName},
				{Action: actionFail, Package: pkgBName, Test: testFooName},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: testFooName}: {Key: FuncKey{Package: pkgAName, Func: testFooName}, Status: FuncPass},
				{Package: pkgBName, Func: testFooName}: {Key: FuncKey{Package: pkgBName, Func: testFooName}, Status: FuncFail},
			},
		},
		{
			name: "subtests filtered out",
			events: []TestEvent{
				{Action: actionPass, Package: pkgAName, Test: testFooName},
				{Action: actionPass, Package: pkgAName, Test: "TestFoo/subtest_one"},
				{Action: actionFail, Package: pkgAName, Test: "TestFoo/subtest_two"},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: testFooName}: {Key: FuncKey{Package: pkgAName, Func: testFooName}, Status: FuncPass},
			},
		},
		{
			name: "package-level events ignored",
			events: []TestEvent{
				{Action: actionPass, Package: pkgAName, Test: ""},
				{Action: actionPass, Package: pkgAName, Test: testFooName},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: testFooName}: {Key: FuncKey{Package: pkgAName, Func: testFooName}, Status: FuncPass},
			},
		},
		{
			name: "last action wins",
			events: []TestEvent{
				{Action: actionPass, Package: pkgAName, Test: testFooName},
				{Action: actionFail, Package: pkgAName, Test: testFooName},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: testFooName}: {Key: FuncKey{Package: pkgAName, Func: testFooName}, Status: FuncFail},
			},
		},
		{
			name: "skip status",
			events: []TestEvent{
				{Action: actionSkip, Package: pkgAName, Test: "TestSkipped"},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: "TestSkipped"}: {Key: FuncKey{Package: pkgAName, Func: "TestSkipped"}, Status: FuncSkip},
			},
		},
		{
			name: "output and run events ignored",
			events: []TestEvent{
				{Action: "run", Package: pkgAName, Test: testFooName},
				{Action: "output", Package: pkgAName, Test: testFooName},
				{Action: actionPass, Package: pkgAName, Test: testFooName},
			},
			want: map[FuncKey]FuncResult{
				{Package: pkgAName, Func: testFooName}: {Key: FuncKey{Package: pkgAName, Func: testFooName}, Status: FuncPass},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FuncResults(tt.events)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d results, want %d", len(got), len(tt.want))
			}
			for key, wantResult := range tt.want {
				gotResult, ok := got[key]
				if !ok {
					t.Errorf("missing key %v", key)
					continue
				}
				if gotResult.Status != wantResult.Status {
					t.Errorf("key %v: got status %d, want %d", key, gotResult.Status, wantResult.Status)
				}
			}
		})
	}
}
