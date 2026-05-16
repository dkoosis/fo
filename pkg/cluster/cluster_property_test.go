package cluster

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func loadAllFixtureInputs(t *testing.T) [][]Input {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var all [][]Input
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var fx fixture
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatalf("unmarshal %s: %v", p, err)
		}
		if len(fx.Inputs) > 0 {
			all = append(all, fx.Inputs)
		}
	}
	return all
}

func TestProperty_Deterministic_OnShuffle(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, base := range loadAllFixtureInputs(t) {
		want := mustJSON(t, Run(base))
		for trial := range 8 {
			shuffled := append([]Input(nil), base...)
			rng.Shuffle(len(shuffled), func(i, j int) {
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			})
			got := mustJSON(t, Run(shuffled))
			if got != want {
				t.Fatalf("non-deterministic for shuffle trial %d:\n got %s\n want %s", trial, got, want)
			}
		}
	}
}

func TestProperty_Partition_InvariantMemberCount(t *testing.T) {
	for _, in := range loadAllFixtureInputs(t) {
		clusters := Run(in)
		seen := map[string]int{}
		for _, c := range clusters {
			for _, m := range c.Members {
				seen[m]++
			}
		}
		// Inputs are deduped by Key inside Run; recount unique keys.
		want := map[string]bool{}
		for _, x := range in {
			want[x.Key] = true
		}
		if len(seen) != len(want) {
			t.Fatalf("membership cardinality = %d; want %d", len(seen), len(want))
		}
		for k, n := range seen {
			if n != 1 {
				t.Fatalf("key %q appears in %d clusters; want exactly 1", k, n)
			}
		}
	}
}

func TestProperty_Singleton_BothSignalsEmpty(t *testing.T) {
	// An input whose Output yields neither a frame nor a non-empty
	// anchor must land in its own singleton with SignatureKind=="singleton".
	in := []Input{{Key: "lonely", Output: "   \n\n  "}}
	out := Run(in)
	if len(out) != 1 || len(out[0].Members) != 1 {
		t.Fatalf("expected one singleton; got %+v", out)
	}
	if out[0].SignatureKind != "singleton" {
		t.Fatalf("kind = %q; want singleton", out[0].SignatureKind)
	}
}

func TestProperty_Normalize_Idempotent_Random(t *testing.T) {
	// Hand-rolled property: feed Normalize a corpus of pasted-ish
	// strings, assert one more pass changes nothing.
	corpus := []string{
		"plain",
		"got 7 want 3",
		"id=550e8400-e29b-41d4-a716-446655440000",
		"/Users/x/proj/pkg/foo/bar.go:42:6: undefined: Bar",
		"panic 0x1400 in 12.5ms at 2026-05-16T01:02:03Z",
		"<UUID> <PATH> <DUR>",
		"",
		"   leading and    trailing   ",
	}
	for _, s := range corpus {
		once := Normalize(s)
		if Normalize(once) != once {
			t.Errorf("not idempotent for %q", s)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
