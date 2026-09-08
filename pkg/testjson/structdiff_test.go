package testjson

import (
	"strings"
	"testing"
)

// goCmpFixture mirrors the shape go-cmp's default reporter emits for a
// struct diff: a two-space/tab-indented tree with "- "/"+ " marking the
// mismatched field.
const goCmpFixture = `  pkg.MyStruct{
- 	Field: 1,
+ 	Field: 2,
  	Other: "x",
  }`

// goCmpNestedFixture adds one level of struct nesting so the path-stack
// walk is exercised, not just a flat single-field diff.
const goCmpNestedFixture = `  pkg.Outer{
  	Sub: pkg.Sub{
- 		Name: "a",
+ 		Name: "b",
  	},
  }`

// cupaloyFixture mirrors cupaloy's difflib-style unified diff over a
// snapshot's serialized text.
const cupaloyFixture = `--- Previous
+++ Current
@@ -1,3 +1,3 @@
 unchanged
-Field: 1
+Field: 2
 trailer`

const plainFailureOutput = "assertion failed\n  wanted truthy, got falsy\n"

func TestDetectStructuralDiff_GoCmp(t *testing.T) {
	t.Parallel()
	sd := detectStructuralDiff(goCmpFixture)
	if sd == nil {
		t.Fatal("expected a structural diff, got nil")
	}
	if sd.Kind != "go-cmp" {
		t.Errorf("Kind = %q, want go-cmp", sd.Kind)
	}
	if len(sd.Fields) != 1 {
		t.Fatalf("Fields = %d, want 1: %#v", len(sd.Fields), sd.Fields)
	}
	f := sd.Fields[0]
	if f.Path != "pkg.MyStruct.Field" {
		t.Errorf("Path = %q, want pkg.MyStruct.Field", f.Path)
	}
	if f.Removed != "1" || f.Added != "2" {
		t.Errorf("Removed/Added = %q/%q, want 1/2", f.Removed, f.Added)
	}
}

func TestDetectStructuralDiff_GoCmpNested(t *testing.T) {
	t.Parallel()
	sd := detectStructuralDiff(goCmpNestedFixture)
	if sd == nil {
		t.Fatal("expected a structural diff, got nil")
	}
	if len(sd.Fields) != 1 {
		t.Fatalf("Fields = %d, want 1: %#v", len(sd.Fields), sd.Fields)
	}
	f := sd.Fields[0]
	if f.Path != "pkg.Outer.Sub.Name" {
		t.Errorf("Path = %q, want pkg.Outer.Sub.Name", f.Path)
	}
	if f.Removed != `"a"` || f.Added != `"b"` {
		t.Errorf("Removed/Added = %q/%q, want \"a\"/\"b\"", f.Removed, f.Added)
	}
}

func TestDetectStructuralDiff_Cupaloy(t *testing.T) {
	t.Parallel()
	sd := detectStructuralDiff(cupaloyFixture)
	if sd == nil {
		t.Fatal("expected a structural diff, got nil")
	}
	if sd.Kind != "cupaloy" {
		t.Errorf("Kind = %q, want cupaloy", sd.Kind)
	}
	if len(sd.Fields) != 1 {
		t.Fatalf("Fields = %d, want 1: %#v", len(sd.Fields), sd.Fields)
	}
	f := sd.Fields[0]
	if f.Path != "Field" {
		t.Errorf("Path = %q, want Field", f.Path)
	}
	if f.Removed != "1" || f.Added != "2" {
		t.Errorf("Removed/Added = %q/%q, want 1/2", f.Removed, f.Added)
	}
}

// TestDetectStructuralDiff_PlainFailureFallsThrough is the additive-not-
// regressive guardrail from fo-d84: output with no recognizable diff
// shape must yield nil so callers keep the pre-existing raw-text path.
func TestDetectStructuralDiff_PlainFailureFallsThrough(t *testing.T) {
	t.Parallel()
	cases := []string{
		plainFailureOutput,
		"",
		"only one side:\n- removed\n- also removed",
		"panic: runtime error: index out of range [3] with length 2",
	}
	for _, in := range cases {
		if sd := detectStructuralDiff(in); sd != nil {
			t.Errorf("detectStructuralDiff(%q) = %#v, want nil", in, sd)
		}
	}
}

// TestDetectStructuralDiff_NonAdjacentMarkersNotMisdetected is a code-review
// fix for fo-d84: looksLikeGoCmpShape used to require only "some line
// starts with '-'" and "some line starts with '+'" anywhere in the
// output, with no adjacency — so ordinary output that happens to contain
// two unrelated lines starting with those characters was misdetected as a
// go-cmp diff, replacing the real output with a fabricated single field.
// Requiring an actual removed-then-added pairing closes that.
func TestDetectStructuralDiff_NonAdjacentMarkersNotMisdetected(t *testing.T) {
	t.Parallel()
	cases := []string{
		"- retry attempt 1 failed\nconnecting to db\n+ retry attempt 2 succeeded",
		"+ configuration reloaded\nsome unrelated line\n- shutting down",
	}
	for _, in := range cases {
		if sd := detectStructuralDiff(in); sd != nil {
			t.Errorf("detectStructuralDiff(%q) = %#v, want nil (no adjacent -/+ pairing)", in, sd)
		}
	}
}

// TestIsUnifiedDiff_ScatteredMarkersNotAHeader is the fo-d84 review fix:
// isUnifiedDiff used to report true whenever "--- ", "+++ " and "@@" each
// appeared ANYWHERE in the output, in any order — not as one coherent
// header. Requiring the "--- "/"+++ " pair adjacent (with a hunk marker
// following) is what a real unified-diff header looks like; scattered
// occurrences of the same prefixes in unrelated lines must not match.
func TestIsUnifiedDiff_ScatteredMarkersNotAHeader(t *testing.T) {
	t.Parallel()
	scattered := []string{
		"--- unrelated build log line",
		"some middle content here",
		"+++ another unrelated line",
		"more content",
		"@@ not really a hunk, just a line that starts with @@",
	}
	if isUnifiedDiff(scattered) {
		t.Errorf("isUnifiedDiff(%v) = true, want false (markers not adjacent)", scattered)
	}

	coherent := []string{
		"--- Previous",
		"+++ Current",
		"@@ -1,1 +1,1 @@",
		"-old",
		"+new",
	}
	if !isUnifiedDiff(coherent) {
		t.Errorf("isUnifiedDiff(%v) = false, want true (adjacent header + hunk marker)", coherent)
	}
}

// TestDetectStructuralDiff_ScatteredMarkersFallBackToGoCmp is the
// detectStructuralDiff-level companion: once isUnifiedDiff correctly
// rejects a scattered, non-coherent header, detection must fall through
// to go-cmp parsing rather than committing to the cupaloy path (which
// pairs any bare "-"/"+" line it finds, positionally, with no nesting
// context) and mislabeling — or mangling the path of — a real go-cmp
// diff. The scattered "--- "/"+++ " noise lines are themselves seen as
// marked content by the go-cmp parser too (any "-"/"+"-led line is
// content to it) and surface as their own low-value fields; the point of
// this test is that the real field is still found, correctly nested, and
// under the right Kind.
func TestDetectStructuralDiff_ScatteredMarkersFallBackToGoCmp(t *testing.T) {
	t.Parallel()
	output := `--- unrelated build log line
some middle content here
+++ another unrelated line
  pkg.MyStruct{
- 	Field: 1,
+ 	Field: 2,
  }
@@ trailing scattered marker, not really a hunk`

	sd := detectStructuralDiff(output)
	if sd == nil {
		t.Fatal("expected a structural diff, got nil")
	}
	if sd.Kind != "go-cmp" {
		t.Errorf("Kind = %q, want go-cmp (scattered markers must not force the cupaloy path)", sd.Kind)
	}
	var found bool
	for _, f := range sd.Fields {
		if f.Path == "pkg.MyStruct.Field" && f.Removed == "1" && f.Added == "2" {
			found = true
		}
	}
	if !found {
		t.Errorf("Fields = %#v, want an entry Path=pkg.MyStruct.Field Removed=1 Added=2", sd.Fields)
	}
}

// goCmpWholesaleNestedFixture is the shape go-cmp emits when it replaces a
// nested struct wholesale rather than diffing it field by field: the
// opening brace, every line inside, and the closing brace all carry the
// SAME marker on each side.
const goCmpWholesaleNestedFixture = `  pkg.Outer{
- 	Sub: pkg.Sub{
- 		X: 1,
- 	},
+ 	Sub: pkg.Sub{
+ 		X: 2,
+ 	},
  }`

// TestDetectStructuralDiff_GoCmpWholesaleNestedReplace is the fo-d84
// review fix for goCmpParser.step: it used to check for a brace
// open/close suffix BEFORE checking the "-"/"+" marker, so a marked
// closing-brace line ("-\t},") forced any pending "-" leaf to flush as a
// removal-only field before the mirrored "+" block ever arrived —
// producing two unpaired fields at the same path instead of one clean
// paired change.
func TestDetectStructuralDiff_GoCmpWholesaleNestedReplace(t *testing.T) {
	t.Parallel()
	sd := detectStructuralDiff(goCmpWholesaleNestedFixture)
	if sd == nil {
		t.Fatal("expected a structural diff, got nil")
	}
	if len(sd.Fields) != 1 {
		t.Fatalf("Fields = %d, want 1 (one paired change, not two unpaired halves): %#v", len(sd.Fields), sd.Fields)
	}
	f := sd.Fields[0]
	if f.Path != "pkg.Outer.Sub" {
		t.Errorf("Path = %q, want pkg.Outer.Sub", f.Path)
	}
	if f.Removed == "" || f.Added == "" {
		t.Errorf("Removed/Added = %q/%q, want both sides populated (one paired change)", f.Removed, f.Added)
	}
	if !strings.Contains(f.Removed, "1") || !strings.Contains(f.Added, "2") {
		t.Errorf("Removed/Added = %q/%q, want values containing 1 / 2 respectively", f.Removed, f.Added)
	}
}
