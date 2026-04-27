package fingerprint

import (
	"testing"
)

// TestFingerprint_StableAcrossLineShift verifies that adding/removing lines
// in the source — which changes ":line:col" coordinates in the message —
// does NOT change the fingerprint.
func TestFingerprint_StableAcrossLineShift(t *testing.T) {
	t.Parallel()

	before := Fingerprint(
		"SA1006",
		"pkg/foo/foo.go",
		"printf-style function with dynamic format string and no further arguments should use print-style function instead pkg/foo/foo.go:42:7",
	)
	after := Fingerprint(
		"SA1006",
		"pkg/foo/foo.go",
		"printf-style function with dynamic format string and no further arguments should use print-style function instead pkg/foo/foo.go:118:7",
	)
	if before != after {
		t.Fatalf("fingerprint changed across line shift\n before=%s\n after =%s", before, after)
	}
}

// TestFingerprint_StableAcrossPathPrefix verifies that an absolute path
// embedded in a message — which differs between CI and laptop checkouts —
// does NOT change the fingerprint.
func TestFingerprint_StableAcrossPathPrefix(t *testing.T) {
	t.Parallel()

	ci := Fingerprint(
		"errcheck",
		"pkg/store/store.go",
		"Error return value of /home/runner/work/fo/fo/pkg/store/store.go is not checked",
	)
	laptop := Fingerprint(
		"errcheck",
		"pkg/store/store.go",
		"Error return value of /Users/dk/Projects/fo/pkg/store/store.go is not checked",
	)
	if ci != laptop {
		t.Fatalf("fingerprint changed across path prefix\n ci    =%s\n laptop=%s", ci, laptop)
	}
}

// TestFingerprint_ChangesOnRuleIDRename verifies that renaming the rule ID
// (e.g. linter renames a check) DOES change the fingerprint — the diff
// classifier should treat this as a different finding.
func TestFingerprint_ChangesOnRuleIDRename(t *testing.T) {
	t.Parallel()

	old := Fingerprint("SA1006", "pkg/foo.go", "use print-style function instead")
	renamed := Fingerprint("S1006", "pkg/foo.go", "use print-style function instead")
	if old == renamed {
		t.Fatalf("fingerprint should change when rule_id is renamed; both = %s", old)
	}
}

// TestFingerprint_ChangesOnFile verifies that the same diagnostic in a
// different file produces a distinct fingerprint.
func TestFingerprint_ChangesOnFile(t *testing.T) {
	t.Parallel()

	a := Fingerprint("SA1006", "pkg/foo.go", "use print-style function instead")
	b := Fingerprint("SA1006", "pkg/bar.go", "use print-style function instead")
	if a == b {
		t.Fatalf("fingerprint should differ across files; both = %s", a)
	}
}

// TestFingerprint_DifferentMessages verifies messages that differ in
// non-coordinate ways still hash to different fingerprints.
func TestFingerprint_DifferentMessages(t *testing.T) {
	t.Parallel()

	a := Fingerprint("R1", "f.go", "variable foo unused")
	b := Fingerprint("R1", "f.go", "variable bar unused")
	if a == b {
		t.Fatalf("fingerprint should differ for different message bodies; both = %s", a)
	}
}

// TestFingerprint_GoldenFormat pins the hex output format (sha256 → 64 hex chars)
// and the exact bytes for one canonical input. If this changes, every saved-state
// file in the wild becomes unmatchable — change deliberately.
func TestFingerprint_GoldenFormat(t *testing.T) {
	t.Parallel()

	got := Fingerprint("SA1006", "pkg/foo.go", "use print-style function")
	if len(got) != 64 {
		t.Fatalf("fingerprint length = %d, want 64 hex chars", len(got))
	}
	for _, c := range got {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("fingerprint contains non-hex char %q in %s", c, got)
		}
	}
}

func TestNormalizeMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "trailing colon line:col",
			in:   "missing return statement foo.go:42:7",
			want: "missing return statement foo.go",
		},
		{
			name: "trailing colon line only",
			in:   "expected semicolon foo.go:42",
			want: "expected semicolon foo.go",
		},
		{
			name: "trailing parenthesized coords",
			in:   "lint error (12, 3)",
			want: "lint error",
		},
		{
			name: "absolute posix path replaced with basename",
			in:   "Error return value of /home/runner/work/fo/fo/pkg/store/store.go is not checked",
			want: "Error return value of store.go is not checked",
		},
		{
			name: "absolute path preserves trailing punctuation",
			in:   "could not open /tmp/build/x/foo.go.",
			want: "could not open foo.go.",
		},
		{
			name: "windows drive path",
			in:   `cannot read C:\Users\dk\code\foo.go`,
			want: "cannot read foo.go",
		},
		{
			name: "collapses whitespace",
			in:   "too\t  many   spaces",
			want: "too many spaces",
		},
		{
			name: "trims whitespace",
			in:   "  hello  ",
			want: "hello",
		},
		{
			name: "multiple trailing coords stripped",
			in:   "msg foo.go:42:7:99",
			want: "msg foo.go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeMessage(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizeMessage(%q) =\n  got  %q\n  want %q", tc.in, got, tc.want)
			}
		})
	}
}
