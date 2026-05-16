package cluster

import (
	"regexp"
	"testing"
)

var idShape = regexp.MustCompile(`^F-[0-9a-f]{6}$`)

func TestMakeClusterID_Shape(t *testing.T) {
	id := makeClusterID("pkg/foo/bar.go:42", "F-", 6)
	if !idShape.MatchString(string(id)) {
		t.Fatalf("id %q does not match %s", id, idShape)
	}
}

func TestMakeClusterID_Stable(t *testing.T) {
	a := makeClusterID("same signature", "F-", 6)
	b := makeClusterID("same signature", "F-", 6)
	if a != b {
		t.Fatalf("ids differ: %q vs %q", a, b)
	}
}

func TestMakeClusterID_DifferentSignatures(t *testing.T) {
	a := makeClusterID("sig-one", "F-", 6)
	b := makeClusterID("sig-two", "F-", 6)
	if a == b {
		t.Fatalf("expected distinct ids; got %q twice", a)
	}
}

func TestDisambiguate_CollisionSuffix(t *testing.T) {
	taken := map[ClusterID]struct{}{}
	a := disambiguate(ClusterID("F-aaaaaa"), taken)
	b := disambiguate(ClusterID("F-aaaaaa"), taken)
	c := disambiguate(ClusterID("F-aaaaaa"), taken)
	if a != "F-aaaaaa" || b != "F-aaaaaa-2" || c != "F-aaaaaa-3" {
		t.Fatalf("disambiguation chain: %q %q %q", a, b, c)
	}
}
