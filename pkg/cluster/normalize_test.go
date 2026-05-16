package cluster

import (
	"strings"
	"testing"
)

func TestNormalize_Rules(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"uuid", "id=550e8400-e29b-41d4-a716-446655440000 fail", "id=<UUID> fail"},
		{"hash", "sha=2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", "sha=<HASH>"},
		{"addr", "panic 0x14000123180", "panic <ADDR>"},
		{"tmp_posix", "tmp /tmp/foo123/bar.go done", "tmp <TMP> done"},
		{"tmp_macos", "tmp /var/folders/zz/abc/T/x.json", "tmp <TMP>"},
		{"path", "see /Users/me/proj/pkg/foo/compute.go for it", "see <PATH> for it"},
		{"winpath", `at C:\Users\me\proj\pkg\foo\compute.go`, "at <PATH>"},
		{"dur", "took 132ms total", "took <DUR> total"},
		{"ts", "at 2026-05-16T12:34:56Z err", "at <TS> err"},
		{"linecol", "compute.go 42:6 undefined", "compute.go <L:C> undefined"},
		{"num", "got 7 want 3", "got <N> want <N>"},
		{"ws_collapse", "got   7    want   3", "got <N> want <N>"},
		{"trim", "   leading and trailing   ", "leading and trailing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Normalize(tc.in)
			if got != tc.want {
				t.Errorf("Normalize(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalize_RuleOrder_TmpBeforePath(t *testing.T) {
	// /var/folders/... is a valid POSIX path; tmp must win.
	got := Normalize("/var/folders/zz/abc123/T/file.go:42")
	if !strings.HasPrefix(got, "<TMP>") {
		t.Fatalf("expected <TMP> prefix, got %q", got)
	}
}

func TestNormalize_RuleOrder_PathBeforeLineColAndNum(t *testing.T) {
	// /a/b/c.go would otherwise have its digit count attacked by num
	// or be split by linecol; the path rule must consume it whole.
	got := Normalize("/a/b/c.go:42:6: undefined: Bar")
	if !strings.Contains(got, "<PATH>") {
		t.Fatalf("expected <PATH> in result, got %q", got)
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	samples := []string{
		"",
		"plain text",
		"got 7 want 3 at /a/b/c.go:42:6 took 12ms",
		"panic 0x14000abc id=550e8400-e29b-41d4-a716-446655440000",
		"   ws   collapse   ",
		"<UUID> <HASH> <ADDR> <TMP> <PATH> <DUR> <TS> <L:C> <N>",
	}
	for _, s := range samples {
		once := Normalize(s)
		twice := Normalize(once)
		if once != twice {
			t.Errorf("not idempotent for %q:\n  once  %q\n  twice %q", s, once, twice)
		}
	}
}
