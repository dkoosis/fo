package cluster

import "testing"

func TestExtractAnchor_Panic(t *testing.T) {
	out := `panic: runtime error: index out of range [3] with length 2

goroutine 7 [running]:
github.com/dkoosis/fo/pkg/foo.Compute(...)
        /Users/x/proj/pkg/foo/compute.go:42 +0x5c
`
	got := extractAnchor(out, 512)
	want := "runtime error: index out of range [3] with length 2"
	if got != want {
		t.Fatalf("anchor\n got  %q\n want %q", got, want)
	}
}

func TestExtractAnchor_Testify(t *testing.T) {
	out := `    compute_test.go:42:
        Error Trace:    /Users/x/proj/pkg/foo/compute_test.go:42
        Error:          Not equal:
                        expected: 1
                        actual  : 2
        Test:           TestCompute
`
	got := extractAnchor(out, 512)
	if got == "" {
		t.Fatal("expected non-empty anchor")
	}
	if got != "Not equal:" {
		t.Fatalf("anchor = %q; want %q", got, "Not equal:")
	}
}

func TestExtractAnchor_TErrorf(t *testing.T) {
	out := "compute_test.go:42: got 7, want 3\n"
	got := extractAnchor(out, 512)
	want := "compute_test.go:42: got 7, want 3"
	if got != want {
		t.Fatalf("anchor = %q; want %q", got, want)
	}
}

func TestExtractAnchor_Empty(t *testing.T) {
	if got := extractAnchor("", 512); got != "" {
		t.Fatalf("anchor of empty = %q; want empty", got)
	}
	if got := extractAnchor("   \n\n  ", 512); got != "" {
		t.Fatalf("anchor of blank = %q; want empty", got)
	}
}

func TestExtractAnchor_FirstNonEmptyFallback(t *testing.T) {
	out := "hello world\n"
	got := extractAnchor(out, 512)
	if got != "hello world" {
		t.Fatalf("anchor = %q; want %q", got, "hello world")
	}
}

func TestExtractAnchor_Truncation(t *testing.T) {
	long := "abcdefghij: extra"
	got := extractAnchor(long, 5)
	if len(got) > 5 {
		t.Fatalf("anchor len = %d; want <= 5; got %q", len(got), got)
	}
}
