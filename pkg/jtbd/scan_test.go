package jtbd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata")
}

func TestScan_FindsAnnotations(t *testing.T) {
	root := testdataDir()
	annotations, err := Scan(root, "example.com/testdata")
	if err != nil {
		t.Fatal(err)
	}

	if len(annotations) != 3 {
		t.Fatalf("expected 3 annotations, got %d", len(annotations))
	}

	byFunc := make(map[string]Annotation)
	for _, a := range annotations {
		byFunc[a.FuncName] = a
	}

	save, ok := byFunc["TestSave"]
	if !ok {
		t.Fatal("TestSave not found")
	}
	if save.Package != "example.com/testdata" {
		t.Errorf("TestSave: expected package example.com/testdata, got %s", save.Package)
	}
	if len(save.JobIDs) != 1 || save.JobIDs[0] != "KG-P1" {
		t.Errorf("TestSave: expected [KG-P1], got %v", save.JobIDs)
	}

	dual, ok := byFunc["TestSaveWithEdges"]
	if !ok {
		t.Fatal("TestSaveWithEdges not found")
	}
	if len(dual.JobIDs) != 2 {
		t.Errorf("TestSaveWithEdges: expected 2 IDs, got %v", dual.JobIDs)
	}

	if _, ok := byFunc["TestHelper"]; ok {
		t.Error("TestHelper should not have annotations")
	}
}

func TestScan_SubdirectoryPackage(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}

	testFile := filepath.Join(subDir, "foo_test.go")
	content := []byte(`package sub

import "testing"

// Serves: FOO-1
func TestBar(t *testing.T) {}
`)
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	annotations, err := Scan(tmpDir, "example.com/proj")
	if err != nil {
		t.Fatal(err)
	}

	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Package != "example.com/proj/sub" {
		t.Errorf("expected package example.com/proj/sub, got %s", annotations[0].Package)
	}
}

func TestScan_EmptyDir(t *testing.T) {
	annotations, err := Scan(t.TempDir(), "example.com/empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(annotations))
	}
}
