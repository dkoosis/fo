package jtbd

import (
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

func TestScan_EmptyDir(t *testing.T) {
	annotations, err := Scan(t.TempDir(), "example.com/empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(annotations))
	}
}
