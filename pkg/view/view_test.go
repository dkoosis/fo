package view_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
)

var update = flag.Bool("update", false, "update golden files")

// Color escapes we expect to see when theme.Color() renders a styled
// glyph at color 196 (red), 34 (green), etc. termenv.ANSI256 emits
// these exact byte sequences.
// We assert the color-component substring rather than a full leading
// escape. Lipgloss merges Bold+Foreground into a single SGR sequence
// (e.g. "\x1b[1;38;5;196m"), so a strict prefix check would miss the
// bolded forms used by Error/Fail. Substring match catches both.
// #nosec G101 — these are ANSI SGR color-component substrings, not credentials.
const (
	escRed     = "38;5;196m"
	escGreen   = "38;5;34m"
	escOrange  = "38;5;214m"
	escMagenta = "38;5;201m"
)

func TestMain(m *testing.M) {
	// Force the lipgloss default renderer to ANSI256 so color tests
	// see deterministic escape sequences (instead of TTY-detected
	// behaviour that strips color in `go test` runs).
	lipgloss.SetColorProfile(termenv.ANSI256)
	os.Exit(m.Run())
}

func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, got, string(want))
	}
}

// renderMono wraps Render with theme.Mono() — the structural baseline.
func renderMono(spec view.ViewSpec, width int) string {
	return view.Render(spec, theme.Mono(), width)
}

// renderColor wraps Render with theme.Color() — for escape-presence checks.
func renderColor(spec view.ViewSpec, width int) string {
	return view.Render(spec, theme.Color(), width)
}

// ----- Clean -----

func TestClean_Mono(t *testing.T) {
	out := renderMono(view.Clean{Message: "no findings"}, 80)
	assertGolden(t, "clean", out)
}

func TestClean_Color(t *testing.T) {
	out := renderColor(view.Clean{Message: "no findings"}, 80)
	if !strings.Contains(out, escGreen) {
		t.Errorf("expected green escape %q in clean output, got %q", escGreen, out)
	}
}

// ----- Bullet -----

func sampleBulletItems() []view.BulletItem {
	return []view.BulletItem{
		{Severity: report.SeverityError, Label: "unchecked error", Value: "store.go:42"},
		{Severity: report.SeverityWarning, Label: "shadowed variable", Value: "query.go:117"},
		{Severity: report.SeverityNote, Label: "exported func lacks doc", Value: "api.go:8"},
	}
}

func TestBullet_Simple_Mono(t *testing.T) {
	out := renderMono(view.Bullet{Items: sampleBulletItems()}, 80)
	assertGolden(t, "bullet_simple", out)
}

func TestBullet_WithFix_Mono(t *testing.T) {
	items := []view.BulletItem{
		{Severity: report.SeverityError, Label: "unchecked error", Value: "store.go:42",
			FixCommand: "errcheck ./..."},
		{Severity: report.SeverityWarning, Label: "missing godoc", Value: "api.go:8",
			FixCommand: "godot -w api.go"},
	}
	out := renderMono(view.Bullet{Items: items}, 80)
	assertGolden(t, "bullet_with_fix", out)
}

func TestBullet_Color_HasRed(t *testing.T) {
	out := renderColor(view.Bullet{Items: sampleBulletItems()}, 80)
	if !strings.Contains(out, escRed) {
		t.Errorf("expected red escape for SeverityError glyph, got %q", out)
	}
	if !strings.Contains(out, escOrange) {
		t.Errorf("expected orange escape for SeverityWarning glyph, got %q", out)
	}
}

// ----- Grouped -----

func TestGrouped_Severity_Mono(t *testing.T) {
	g := view.Grouped{
		Sections: []view.GroupedSection{
			{Label: "errors", Items: []view.BulletItem{
				{Severity: report.SeverityError, Label: "unchecked error", Value: "store.go:42"},
				{Severity: report.SeverityError, Label: "nil deref", Value: "query.go:9",
					FixCommand: "go vet ./..."},
			}},
			{Label: "warnings", Items: []view.BulletItem{
				{Severity: report.SeverityWarning, Label: "shadowed", Value: "x.go:3"},
			}},
		},
	}
	out := renderMono(g, 80)
	assertGolden(t, "grouped_severity", out)
}

func TestGrouped_Color_HasRed(t *testing.T) {
	g := view.Grouped{
		Sections: []view.GroupedSection{
			{Label: "errors", Items: []view.BulletItem{
				{Severity: report.SeverityError, Label: "boom", Value: "x.go:1"},
			}},
		},
	}
	out := renderColor(g, 80)
	if !strings.Contains(out, escRed) {
		t.Errorf("expected red escape, got %q", out)
	}
}

// ----- Leaderboard -----

func TestLeaderboard_Top3_Mono(t *testing.T) {
	lb := view.Leaderboard{
		Total: 100,
		Rows: []view.LbRow{
			{Label: "errcheck", Value: 47},
			{Label: "godot", Value: 28},
			{Label: "revive", Value: 14},
			{Label: "gosec", Value: 11},
		},
	}
	out := renderMono(lb, 80)
	assertGolden(t, "leaderboard_top3", out)
}

// ----- Headline -----

func TestHeadline_Mono(t *testing.T) {
	out := renderMono(view.Headline{Title: "PANIC", Detail: "TestStore_Concurrent — runtime error: index out of range"}, 80)
	assertGolden(t, "headline", out)
}

// ----- Alert -----

func TestAlert_Mono(t *testing.T) {
	out := renderMono(view.Alert{
		Severity: report.SeverityError,
		Prefix:   "ERRORS",
		Value:    "47",
		Detail:   "across 12 files",
	}, 80)
	assertGolden(t, "alert", out)
}

func TestAlert_Color_HasRed(t *testing.T) {
	out := renderColor(view.Alert{
		Severity: report.SeverityError,
		Prefix:   "ERRORS",
		Value:    "47",
	}, 80)
	if !strings.Contains(out, escRed) {
		t.Errorf("expected red escape, got %q", out)
	}
}

// ----- Delta -----

func TestDelta_OverBullet_Mono(t *testing.T) {
	inner := view.Bullet{Items: sampleBulletItems()}
	d := view.Delta{
		Inner: inner,
		Buckets: []view.DeltaBucket{
			{Label: "errors", Count: 12, Direction: 1},
			{Label: "warnings", Count: 3, Direction: -1},
			{Label: "notes", Count: 5, Direction: 0},
		},
	}
	out := renderMono(d, 80)
	assertGolden(t, "delta_over_bullet", out)
}

func TestDelta_Color_HasArrowColors(t *testing.T) {
	d := view.Delta{
		Buckets: []view.DeltaBucket{
			{Label: "errors", Count: 12, Direction: 1},
			{Label: "warnings", Count: 3, Direction: -1},
		},
	}
	out := renderColor(d, 80)
	if !strings.Contains(out, escRed) {
		t.Errorf("expected red escape on up arrow, got %q", out)
	}
	if !strings.Contains(out, escGreen) {
		t.Errorf("expected green escape on down arrow, got %q", out)
	}
}

// ----- SmallMultiples -----

func TestSmallMultiples_Mono(t *testing.T) {
	sm := view.SmallMultiples{
		Cells: []view.MultipleCell{
			{Label: "pkg/store", Sparks: []float64{1, 2, 3, 2, 4}, Counters: []view.Counter{
				{Severity: report.SeverityError, Value: 3, Label: "err"},
				{Severity: report.SeverityWarning, Value: 7, Label: "warn"},
			}},
			{Label: "pkg/query", Sparks: []float64{0, 1, 1, 0, 0}, Counters: []view.Counter{
				{Severity: report.SeverityError, Value: 1, Label: "err"},
			}},
			{Label: "pkg/api", Counters: []view.Counter{
				{Severity: report.SeverityWarning, Value: 2, Label: "warn"},
			}},
		},
	}
	out := renderMono(sm, 80)
	assertGolden(t, "small_multiples", out)
}

func TestSmallMultiples_Color_HasRed(t *testing.T) {
	sm := view.SmallMultiples{
		Cells: []view.MultipleCell{
			{Label: "x", Counters: []view.Counter{
				{Severity: report.SeverityError, Value: 1, Label: "err"},
			}},
		},
	}
	out := renderColor(sm, 80)
	if !strings.Contains(out, escRed) {
		t.Errorf("expected red escape, got %q", out)
	}
}

// ----- Render dispatch -----

func TestRender_UnknownVariant(t *testing.T) {
	type fakeSpec struct{ view.ViewSpec }
	out := view.Render(fakeSpec{}, theme.Mono(), 80)
	if !strings.Contains(out, "unknown view") {
		t.Errorf("expected fallback marker, got %q", out)
	}
}

func TestRender_DefaultsWidth(t *testing.T) {
	// width <= 0 should not panic; Leaderboard exercises width budget.
	lb := view.Leaderboard{Total: 10, Rows: []view.LbRow{{Label: "a", Value: 5}}}
	if got := view.Render(lb, theme.Mono(), 0); got == "" {
		t.Error("expected non-empty output for width=0 fallback")
	}
}
