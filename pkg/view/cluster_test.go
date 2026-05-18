// Strategy: extend BulletItem with optional *ClusterRender (per Task 0 orient).
package view

import "testing"

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
