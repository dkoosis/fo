package state

import (
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

func mkReport(findings ...report.Finding) *report.Report {
	return &report.Report{Findings: findings}
}

func mkRun(pairs ...string) Run {
	if len(pairs)%2 != 0 {
		panic("pairs must be fp,sev")
	}
	m := map[string]Severity{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = Severity(pairs[i+1])
	}
	return Run{Findings: m}
}

func TestClassify_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prev        *File
		current     *report.Report
		wantNew     int
		wantResolv  int
		wantRegress int
		wantFlaky   int
		wantPersist int
	}{
		{
			name:    "no prior — all new",
			prev:    nil,
			current: mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}),
			wantNew: 1,
		},
		{
			name:        "all persistent",
			prev:        &File{Version: SchemaVersion, Runs: []Run{mkRun("a", "error", "b", "warning")}},
			current:     mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}, report.Finding{Fingerprint: "b", Severity: report.SeverityWarning}),
			wantPersist: 2,
		},
		{
			name:       "all resolved",
			prev:       &File{Version: SchemaVersion, Runs: []Run{mkRun("a", "error")}},
			current:    mkReport(),
			wantResolv: 1,
		},
		{
			name:        "regressed warning→error",
			prev:        &File{Version: SchemaVersion, Runs: []Run{mkRun("a", "warning")}},
			current:     mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}),
			wantRegress: 1,
		},
		{
			name:        "severity drop counts as persistent",
			prev:        &File{Version: SchemaVersion, Runs: []Run{mkRun("a", "error")}},
			current:     mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityWarning}),
			wantPersist: 1,
		},
		{
			name: "flaky — present at t-2, absent at t-1, present at t",
			prev: &File{Version: SchemaVersion, Runs: []Run{
				mkRun(),            // t-1: gone
				mkRun("a", "error"), // t-2: present
			}},
			current:   mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}),
			wantFlaky: 1,
		},
		{
			name: "with only one prior run, reappearance counts as new not flaky",
			prev: &File{Version: SchemaVersion, Runs: []Run{mkRun()}},
			current: mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}),
			wantNew: 1,
		},
		{
			name: "mixed: 1 new, 1 resolved, 1 regressed, 1 persistent, 1 flaky",
			prev: &File{Version: SchemaVersion, Runs: []Run{
				mkRun("persist", "error", "resolv", "warning", "regress", "warning"),
				mkRun("flake", "error"),
			}},
			current: mkReport(
				report.Finding{Fingerprint: "newone", Severity: report.SeverityError},
				report.Finding{Fingerprint: "persist", Severity: report.SeverityError},
				report.Finding{Fingerprint: "regress", Severity: report.SeverityError},
				report.Finding{Fingerprint: "flake", Severity: report.SeverityError},
			),
			wantNew:     1,
			wantResolv:  1,
			wantRegress: 1,
			wantPersist: 1,
			wantFlaky:   1,
		},
		{
			name:    "findings without fingerprint are ignored",
			prev:    nil,
			current: mkReport(report.Finding{Fingerprint: "", Severity: report.SeverityError}),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := Classify(tc.prev, tc.current)
			if got := len(d.New); got != tc.wantNew {
				t.Errorf("new: got %d want %d", got, tc.wantNew)
			}
			if got := len(d.Resolved); got != tc.wantResolv {
				t.Errorf("resolved: got %d want %d", got, tc.wantResolv)
			}
			if got := len(d.Regressed); got != tc.wantRegress {
				t.Errorf("regressed: got %d want %d", got, tc.wantRegress)
			}
			if got := len(d.Flaky); got != tc.wantFlaky {
				t.Errorf("flaky: got %d want %d", got, tc.wantFlaky)
			}
			if d.PersistentCount != tc.wantPersist {
				t.Errorf("persistent: got %d want %d", d.PersistentCount, tc.wantPersist)
			}
		})
	}
}

func TestClassify_FlakyOver3Runs(t *testing.T) {
	t.Parallel()
	// t-3: present, t-2: gone, t-1: gone, t: present — still flaky
	// (any older run with the fp counts).
	prev := &File{Version: SchemaVersion, Runs: []Run{
		mkRun(),             // t-1
		mkRun(),             // t-2
		mkRun("a", "error"), // t-3
	}}
	d := Classify(prev, mkReport(report.Finding{Fingerprint: "a", Severity: report.SeverityError}))
	if len(d.Flaky) != 1 {
		t.Fatalf("want 1 flaky, got %d", len(d.Flaky))
	}
}

func TestHeadline_Format(t *testing.T) {
	t.Parallel()
	d := Diff{
		New:             []Item{{Fingerprint: "a"}, {Fingerprint: "b"}, {Fingerprint: "c"}},
		Resolved:        []Item{{Fingerprint: "d"}, {Fingerprint: "e"}},
		Regressed:       []Item{{Fingerprint: "f"}},
		Flaky:           []Item{{Fingerprint: "g"}, {Fingerprint: "h"}},
		PersistentCount: 41,
	}
	h := Headline(d)
	want := "3 new · 2 resolved · 1 regressed · 2 flaky · 41 persistent"
	if h != want {
		t.Fatalf("headline mismatch:\n got: %q\nwant: %q", h, want)
	}
}

func TestHeadline_DropsZero(t *testing.T) {
	t.Parallel()
	d := Diff{
		New:             []Item{{Fingerprint: "a"}},
		PersistentCount: 5,
	}
	h := Headline(d)
	if !strings.Contains(h, "1 new") || !strings.Contains(h, "5 persistent") {
		t.Fatalf("missing segments: %q", h)
	}
	if strings.Contains(h, "resolved") || strings.Contains(h, "regressed") || strings.Contains(h, "flaky") {
		t.Fatalf("zero segments leaked into headline: %q", h)
	}
}

func TestHeadline_Empty(t *testing.T) {
	t.Parallel()
	if got := Headline(Diff{}); got != "no changes" {
		t.Fatalf("empty headline: %q", got)
	}
}

func TestEnvelope_NoNilSlices(t *testing.T) {
	t.Parallel()
	e := EnvelopeOf(Diff{})
	if e.New == nil || e.Resolved == nil || e.Regressed == nil || e.Flaky == nil {
		t.Fatalf("envelope slices must be non-nil for stable JSON")
	}
}
