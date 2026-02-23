package jtbd

import (
	"testing"

	"github.com/dkoosis/fo/pkg/testjson"
)

func TestAssemble_Running(t *testing.T) {
	jobs := []Job{{ID: "KG-P1", Layer: "Plumbing", Statement: "save and retrieve"}}
	annotations := []Annotation{{Package: "example.com/store", FuncName: "TestSave", JobIDs: []string{"KG-P1"}}}
	results := []testjson.TestPackageResult{{
		Name:   "example.com/store",
		Passed: 1,
		AllTests: []testjson.TestResult{{Name: "TestSave", Status: "PASS"}},
	}}

	report := Assemble(jobs, annotations, results)

	if report.Running != 1 {
		t.Errorf("expected 1 running, got %d", report.Running)
	}
	if report.Jobs[0].Status != "running" {
		t.Errorf("expected running, got %s", report.Jobs[0].Status)
	}
}

func TestAssemble_Broken(t *testing.T) {
	jobs := []Job{{ID: "KG-P1", Layer: "Plumbing", Statement: "save and retrieve"}}
	annotations := []Annotation{
		{Package: "example.com/store", FuncName: "TestSave", JobIDs: []string{"KG-P1"}},
		{Package: "example.com/store", FuncName: "TestGet", JobIDs: []string{"KG-P1"}},
	}
	results := []testjson.TestPackageResult{{
		Name:   "example.com/store",
		Passed: 1, Failed: 1,
		AllTests: []testjson.TestResult{
			{Name: "TestSave", Status: "PASS"},
			{Name: "TestGet", Status: "FAIL"},
		},
	}}

	report := Assemble(jobs, annotations, results)

	if report.Broken != 1 {
		t.Errorf("expected 1 broken, got %d", report.Broken)
	}
	if report.Jobs[0].Failed != 1 {
		t.Errorf("expected 1 failed, got %d", report.Jobs[0].Failed)
	}
}

func TestAssemble_WIP(t *testing.T) {
	jobs := []Job{{ID: "CORE-5", Layer: "Insight", Statement: "patterns surfaced"}}

	report := Assemble(jobs, nil, nil)

	if report.WIP != 1 {
		t.Errorf("expected 1 WIP, got %d", report.WIP)
	}
	if report.Jobs[0].Status != "wip" {
		t.Errorf("expected wip, got %s", report.Jobs[0].Status)
	}
}

func TestAssemble_SkipsSubtests(t *testing.T) {
	jobs := []Job{{ID: "KG-P1", Layer: "Plumbing", Statement: "save"}}
	annotations := []Annotation{{Package: "example.com/store", FuncName: "TestSave", JobIDs: []string{"KG-P1"}}}
	results := []testjson.TestPackageResult{{
		Name:   "example.com/store",
		Passed: 3,
		AllTests: []testjson.TestResult{
			{Name: "TestSave", Status: "PASS"},
			{Name: "TestSave/subcase_a", Status: "PASS"},
			{Name: "TestSave/subcase_b", Status: "PASS"},
		},
	}}

	report := Assemble(jobs, annotations, results)

	if report.Jobs[0].Total != 1 {
		t.Errorf("expected 1 test, got %d", report.Jobs[0].Total)
	}
}

func TestAssemble_DualAnnotation(t *testing.T) {
	jobs := []Job{
		{ID: "KG-P1", Layer: "Plumbing", Statement: "save"},
		{ID: "CORE-2", Layer: "Memory", Statement: "connections"},
	}
	annotations := []Annotation{
		{Package: "example.com/store", FuncName: "TestSaveWithEdges", JobIDs: []string{"KG-P1", "CORE-2"}},
	}
	results := []testjson.TestPackageResult{{
		Name:   "example.com/store",
		Passed: 1,
		AllTests: []testjson.TestResult{{Name: "TestSaveWithEdges", Status: "PASS"}},
	}}

	report := Assemble(jobs, annotations, results)

	if report.Running != 2 {
		t.Errorf("expected 2 running (both jobs served), got %d", report.Running)
	}
}
