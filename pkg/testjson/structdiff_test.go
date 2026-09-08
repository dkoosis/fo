package testjson

import "testing"

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
