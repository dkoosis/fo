package testjson

import (
	"testing"
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
				{Action: "run", Package: "pkg/a", Test: "TestFoo"},
				{Action: "pass", Package: "pkg/a", Test: "TestFoo"},
				{Action: "run", Package: "pkg/b", Test: "TestFoo"},
				{Action: "fail", Package: "pkg/b", Test: "TestFoo"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/a", Func: "TestFoo"}, Status: FuncPass},
				{Package: "pkg/b", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/b", Func: "TestFoo"}, Status: FuncFail},
			},
		},
		{
			name: "subtests filtered out",
			events: []TestEvent{
				{Action: "pass", Package: "pkg/a", Test: "TestFoo"},
				{Action: "pass", Package: "pkg/a", Test: "TestFoo/subtest_one"},
				{Action: "fail", Package: "pkg/a", Test: "TestFoo/subtest_two"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/a", Func: "TestFoo"}, Status: FuncPass},
			},
		},
		{
			name: "package-level events ignored",
			events: []TestEvent{
				{Action: "pass", Package: "pkg/a", Test: ""},
				{Action: "pass", Package: "pkg/a", Test: "TestFoo"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/a", Func: "TestFoo"}, Status: FuncPass},
			},
		},
		{
			name: "last action wins",
			events: []TestEvent{
				{Action: "pass", Package: "pkg/a", Test: "TestFoo"},
				{Action: "fail", Package: "pkg/a", Test: "TestFoo"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/a", Func: "TestFoo"}, Status: FuncFail},
			},
		},
		{
			name: "skip status",
			events: []TestEvent{
				{Action: "skip", Package: "pkg/a", Test: "TestSkipped"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestSkipped"}: {Key: FuncKey{Package: "pkg/a", Func: "TestSkipped"}, Status: FuncSkip},
			},
		},
		{
			name: "output and run events ignored",
			events: []TestEvent{
				{Action: "run", Package: "pkg/a", Test: "TestFoo"},
				{Action: "output", Package: "pkg/a", Test: "TestFoo"},
				{Action: "pass", Package: "pkg/a", Test: "TestFoo"},
			},
			want: map[FuncKey]FuncResult{
				{Package: "pkg/a", Func: "TestFoo"}: {Key: FuncKey{Package: "pkg/a", Func: "TestFoo"}, Status: FuncPass},
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
