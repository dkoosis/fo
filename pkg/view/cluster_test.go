// Strategy: extend BulletItem with optional *ClusterRender (per Task 0 orient).
package view

import (
	"testing"

	"github.com/dkoosis/fo/pkg/report"
)

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
