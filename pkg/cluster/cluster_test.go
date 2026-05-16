package cluster

import "testing"

func TestCluster_EmptyInputs(t *testing.T) {
	got := Run(nil)
	if got == nil {
		t.Fatal("Run(nil) returned nil; want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("Run(nil) len = %d; want 0", len(got))
	}
	got = Run([]Input{})
	if got == nil || len(got) != 0 {
		t.Fatalf("Cluster([]) = %v; want empty non-nil slice", got)
	}
}
