// Strategy: extend BulletItem with optional *ClusterRender (per Task 0 orient).
package view

import (
	"regexp"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/theme"
)

var clusterAnsiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return clusterAnsiRE.ReplaceAllString(s, "") }

func TestExpandSet_All(t *testing.T) {
	e := newExpandSet([]string{"all"})
	if !e.wants("cluster-anything") {
		t.Fatal("expected wants(any) when all=true")
	}
}

func TestExpandSet_ByID(t *testing.T) {
	e := newExpandSet([]string{"cluster-a3f2c1", "cluster-b7e009"})
	if !e.wants("cluster-a3f2c1") {
		t.Fatal("expected wants(known-id)")
	}
	if e.wants("cluster-deadbeef") {
		t.Fatal("expected !wants(unknown-id)")
	}
}

func TestExpandSet_Empty(t *testing.T) {
	e := newExpandSet(nil)
	if e.wants("cluster-a3f2c1") {
		t.Fatal("expected !wants on empty set")
	}
}

func TestPartitionTests(t *testing.T) {
	tests := []report.TestResult{
		{Test: "TA", ClusterID: "cluster-1"},
		{Test: "TB"},
		{Test: "TC", ClusterID: "cluster-1"},
		{Test: "TD", ClusterID: "cluster-2"},
		{Test: "TE"},
	}
	clustered, singletons := partitionTests(tests)
	if got := len(clustered["cluster-1"]); got != 2 {
		t.Errorf("cluster-1 size: got %d, want 2", got)
	}
	if got := clustered["cluster-1"][0].Test; got != "TA" {
		t.Errorf("first member of cluster-1: got %q, want TA (source-order)", got)
	}
	if got := len(clustered["cluster-2"]); got != 1 {
		t.Errorf("cluster-2 size: got %d, want 1", got)
	}
	if got := len(singletons); got != 2 {
		t.Errorf("singletons: got %d, want 2", got)
	}
	if singletons[0].Test != "TB" || singletons[1].Test != "TE" {
		t.Errorf("singletons order: got [%s, %s], want [TB, TE]",
			singletons[0].Test, singletons[1].Test)
	}
}

func TestSharedOutput_AllEqual(t *testing.T) {
	members := []report.TestResult{
		{Test: "T1", Output: "got nil, want ErrMissing"},
		{Test: "T2", Output: "got nil, want ErrMissing"},
		{Test: "T3", Output: "got nil, want ErrMissing"},
	}
	got, ok := sharedOutput(members)
	if !ok {
		t.Fatal("expected ok=true when all byte-equal")
	}
	if got != "got nil, want ErrMissing" {
		t.Errorf("got %q", got)
	}
}

func TestSharedOutput_OneDiverges(t *testing.T) {
	members := []report.TestResult{
		{Test: "T1", Output: "got nil, want ErrMissing"},
		{Test: "T2", Output: "got nil, want ErrMissing"},
		{Test: "T3", Output: "got 0 rows, want 1"},
	}
	_, ok := sharedOutput(members)
	if ok {
		t.Fatal("expected ok=false when any member diverges")
	}
}

func TestSharedOutput_EmptyAndSingle(t *testing.T) {
	if _, ok := sharedOutput(nil); ok {
		t.Error("nil → ok must be false")
	}
	if _, ok := sharedOutput([]report.TestResult{}); ok {
		t.Error("empty → ok must be false")
	}
	members := []report.TestResult{{Output: "x"}}
	if got, ok := sharedOutput(members); !ok || got != "x" {
		t.Errorf("single-member: got %q ok=%v, want \"x\" ok=true", got, ok)
	}
}

func TestSharedOutput_WhitespaceMatters(t *testing.T) {
	members := []report.TestResult{
		{Output: "got nil"},
		{Output: "got nil "},
	}
	if _, ok := sharedOutput(members); ok {
		t.Fatal("expected ok=false on whitespace divergence (no normalization)")
	}
}

func TestClusterHeader_Human(t *testing.T) {
	c := report.Cluster{ID: "cluster-a3f2c1", Signature: "pkg/store.(*DB).Get"}
	got := clusterHeader(c, 12, ModeHuman)
	want := "▸ pkg/store.(*DB).Get · 12 tests · --expand=cluster-a3f2c1"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestClusterHeader_LLM(t *testing.T) {
	c := report.Cluster{ID: "cluster-a3f2c1", Signature: "pkg/store.(*DB).Get"}
	got := clusterHeader(c, 3, ModeLLM)
	want := "cluster cluster-a3f2c1 · pkg/store.(*DB).Get · 3 tests"
	if got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}

func TestPickBullet_CollapsedCluster_Human(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TC", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TSolo", Outcome: report.OutcomeFail, Output: "lonely"},
		},
		Clusters: []report.Cluster{
			{ID: "cluster-a3f2c1", Signature: "pkg/store.(*DB).Get", Members: []string{"TA", "TB", "TC"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeHuman, newExpandSet(nil))
	b, ok := spec.(Bullet)
	if !ok {
		t.Fatalf("expected Bullet variant, got %T", spec)
	}
	if got := len(b.Items); got != 2 {
		t.Fatalf("Items len: got %d, want 2 (1 cluster + 1 singleton)", got)
	}
	first := b.Items[0]
	if first.Cluster == nil {
		t.Fatal("first item: expected Cluster != nil")
	}
	if got := len(first.Cluster.Members); got != 1 {
		t.Errorf("collapsed cluster: visible members got %d, want 1", got)
	}
	if first.Cluster.Members[0].Test != "TA" {
		t.Errorf("rep: got %q, want TA (source-order first)", first.Cluster.Members[0].Test)
	}
}

func TestPickBullet_ExpandedCluster_Human(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
		},
		Clusters: []report.Cluster{
			{ID: "cluster-a3f2c1", Signature: "sig", Members: []string{"TA", "TB"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeHuman, newExpandSet([]string{"all"}))
	b, ok := spec.(Bullet)
	if !ok {
		t.Fatalf("expected Bullet variant, got %T", spec)
	}
	first := b.Items[0]
	if first.Cluster == nil {
		t.Fatal("expected Cluster != nil")
	}
	if got := len(first.Cluster.Members); got != 2 {
		t.Errorf("expanded cluster: visible members got %d, want 2", got)
	}
}

func TestPickBullet_LLM_SharedOutput(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "boom"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "boom"},
		},
		Clusters: []report.Cluster{
			{ID: "c1", Signature: "sig", Members: []string{"TA", "TB"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeLLM, newExpandSet(nil))
	b, ok := spec.(Bullet)
	if !ok {
		t.Fatalf("expected Bullet, got %T", spec)
	}
	cr := b.Items[0].Cluster
	if cr == nil {
		t.Fatal("expected Cluster")
	}
	if !cr.UsesSharedRow {
		t.Error("expected UsesSharedRow=true for byte-equal LLM mode")
	}
	if cr.SharedOutput != "boom" {
		t.Errorf("SharedOutput: got %q", cr.SharedOutput)
	}
	// LLM mode always shows all members (no collapse).
	if len(cr.Members) != 2 {
		t.Errorf("LLM members: got %d, want 2", len(cr.Members))
	}
}

func TestRender_ClusterCollapsed_Human(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
			{Test: "TB2", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "boom"},
		},
		Clusters: []report.Cluster{
			{ID: "cluster-a3f2c1", Signature: "pkg/store.(*DB).Get", Members: []string{"TA", "TB", "TB2"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeHuman, newExpandSet(nil))
	got := Render(spec, theme.Mono(), 80)
	// Header is themed in two halves (Heading + Muted) so styled escapes
	// split the literal — check the substrings individually.
	if !strings.Contains(got, "▸ pkg/store.(*DB).Get · 3 tests") {
		t.Errorf("missing cluster header signature in:\n%s", got)
	}
	if !strings.Contains(got, "--expand=cluster-a3f2c1") {
		t.Errorf("missing expand hint in:\n%s", got)
	}
	if !strings.Contains(got, "TA") {
		t.Errorf("missing rep TA in:\n%s", got)
	}
	if strings.Contains(got, "TB ") || strings.Contains(got, "TB2") {
		t.Errorf("collapsed cluster leaked non-rep members:\n%s", got)
	}
}

func TestRender_ClusterExpanded_Human(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "boom"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "boom"},
		},
		Clusters: []report.Cluster{
			{ID: "c1", Signature: "sig", Members: []string{"TA", "TB"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeHuman, newExpandSet([]string{"all"}))
	got := Render(spec, theme.Mono(), 80)
	if !strings.Contains(got, "TA") || !strings.Contains(got, "TB") {
		t.Errorf("expanded cluster missing members in:\n%s", got)
	}
}

func TestRender_ClusterShapeA_LLM(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "got nil, want ErrMissing"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "got nil, want ErrMissing"},
			{Test: "TC", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "got nil, want ErrMissing"},
		},
		Clusters: []report.Cluster{
			{ID: "cluster-a3f2c1", Signature: "pkg/store.(*DB).Get", Members: []string{"TA", "TB", "TC"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeLLM, newExpandSet(nil))
	got := Render(spec, theme.Mono(), 0)
	wantLines := []string{
		"cluster cluster-a3f2c1 · pkg/store.(*DB).Get · 3 tests",
		"  shared: got nil, want ErrMissing",
		"  members: TA, TB, TC",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("missing line %q in:\n%s", line, got)
		}
	}
}

func TestRender_ClusterShapeB_LLM(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "got nil"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "cluster-a3f2c1", Output: "got 0 rows"},
		},
		Clusters: []report.Cluster{
			{ID: "cluster-a3f2c1", Signature: "sig", Members: []string{"TA", "TB"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeLLM, newExpandSet(nil))
	got := Render(spec, theme.Mono(), 0)
	wantLines := []string{
		"cluster cluster-a3f2c1 · sig · 2 tests",
		"  TA: got nil",
		"  TB: got 0 rows",
	}
	for _, line := range wantLines {
		if !strings.Contains(got, line) {
			t.Errorf("missing line %q in:\n%s", line, got)
		}
	}
	if strings.Contains(got, "shared:") {
		t.Errorf("Shape B must not emit `shared:` line:\n%s", got)
	}
}

func TestRender_ClusterThemeParity(t *testing.T) {
	r := report.Report{
		Tests: []report.TestResult{
			{Test: "TA", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "x"},
			{Test: "TB", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "x"},
			{Test: "TC", Outcome: report.OutcomeFail, ClusterID: "c1", Output: "x"},
		},
		Clusters: []report.Cluster{
			{ID: "c1", Signature: "sig", Members: []string{"TA", "TB", "TC"}},
		},
	}
	spec := PickViewModeWithExpand(r, ModeHuman, newExpandSet(nil))
	mono := Render(spec, theme.Mono(), 80)
	color := Render(spec, theme.Color(), 80)
	// Color output must differ from mono only by ANSI escapes — structure
	// (line count, whitespace, glyphs, content) is identical.
	// Structural parity: same line count (theme differs in glyph choice
	// — mono "x" vs color "✗" — not in layout).
	if got, want := len(strings.Split(stripANSI(color), "\n")), len(strings.Split(mono, "\n")); got != want {
		t.Errorf("line count: mono=%d color(stripped)=%d", want, got)
	}
	// Both outputs must contain the cluster header signature and expand hint.
	for _, sub := range []string{"sig · 3 tests", "--expand=c1"} {
		if !strings.Contains(mono, sub) {
			t.Errorf("mono missing %q", sub)
		}
		if !strings.Contains(stripANSI(color), sub) {
			t.Errorf("color(stripped) missing %q", sub)
		}
	}
}
