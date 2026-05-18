package cluster

import "testing"

func TestExtractTopUserFrame_PanicStack(t *testing.T) {
	out := `panic: runtime error: index out of range [3] with length 2

goroutine 7 [running]:
github.com/dkoosis/fo/pkg/foo.Compute(...)
        /Users/x/proj/pkg/foo/compute.go:42 +0x5c
github.com/dkoosis/fo/pkg/foo.TestCompute(0x14000123180)
        /Users/x/proj/pkg/foo/compute_test.go:17 +0x40
testing.tRunner(0x14000123180, 0x100abc000)
        /usr/local/go/src/testing/testing.go:1689 +0xf4
`
	got := extractTopUserFrame(out, PathTrim)
	want := "pkg/foo/compute.go:42"
	if got != want {
		t.Fatalf("frame = %q; want %q", got, want)
	}
}

func TestExtractTopUserFrame_StdlibSkip(t *testing.T) {
	out := `goroutine 1 [running]:
testing.tRunner(0x14000123180, 0x100abc000)
        /usr/local/go/src/testing/testing.go:1689 +0xf4
`
	got := extractTopUserFrame(out, PathTrim)
	if got != "" {
		t.Fatalf("frame = %q; want empty (stdlib-only)", got)
	}
}

func TestExtractTopUserFrame_TestifySkip(t *testing.T) {
	out := `github.com/stretchr/testify/assert.Fail(...)
        /go/pkg/mod/github.com/stretchr/testify@v1.8.4/assert/assertions.go:760
github.com/dkoosis/fo/pkg/foo.TestCompute(0x14000123180)
        /Users/x/proj/pkg/foo/compute_test.go:17 +0x40
`
	got := extractTopUserFrame(out, PathTrim)
	want := "pkg/foo/compute_test.go:17"
	if got != want {
		t.Fatalf("frame = %q; want %q", got, want)
	}
}

func TestExtractTopUserFrame_TestifyErrorTrace(t *testing.T) {
	out := `    compute_test.go:42:
        Error Trace:    /Users/x/proj/pkg/foo/compute_test.go:42
        Error:          Not equal
`
	got := extractTopUserFrame(out, PathTrim)
	want := "pkg/foo/compute_test.go:42"
	if got != want {
		t.Fatalf("frame = %q; want %q", got, want)
	}
}

func TestExtractTopUserFrame_TErrorfSimpleCite(t *testing.T) {
	out := "    compute_test.go:42: got 7, want 3\n"
	got := extractTopUserFrame(out, PathTrim)
	want := "compute_test.go:42"
	if got != want {
		t.Fatalf("frame = %q; want %q", got, want)
	}
}

func TestExtractTopUserFrame_NoFrame(t *testing.T) {
	if got := extractTopUserFrame("nothing useful here", PathTrim); got != "" {
		t.Fatalf("frame = %q; want empty", got)
	}
}

func TestExtractTopUserFrame_KeepAbsPaths(t *testing.T) {
	out := "    compute_test.go:42:\n        Error Trace:    /Users/x/proj/pkg/foo/compute_test.go:42\n"
	got := extractTopUserFrame(out, PathKeep)
	want := "/Users/x/proj/pkg/foo/compute_test.go:42"
	if got != want {
		t.Fatalf("frame = %q; want %q", got, want)
	}
}
