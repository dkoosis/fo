package cluster

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// orWinsInputs loads the canonical fixture that demonstrates why OR
// beats AND: 3 failures from 3 different test files share an
// assertion-text ("helper exploded: bad input") but no two share a
// stack frame. OR merges them; AND splits them.
func orWinsInputs(t *testing.T) []Input {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fixtures", "or_wins_over_and.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx fixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return fx.Inputs
}

func TestCluster_ModeOr_MergesSharedAssertion(t *testing.T) {
	got := RunWith(orWinsInputs(t), Config{Mode: ModeOr})
	if len(got) != 1 {
		t.Fatalf("ModeOr produced %d clusters; want 1", len(got))
	}
	if len(got[0].Members) != 3 {
		t.Fatalf("cluster size = %d; want 3", len(got[0].Members))
	}
}

func TestCluster_ModeAnd_SplitsWhenFramesDiffer(t *testing.T) {
	got := RunWith(orWinsInputs(t), Config{Mode: ModeAnd})
	if len(got) != 3 {
		t.Fatalf("ModeAnd produced %d clusters; want 3 (frames differ → no merge)", len(got))
	}
}

func TestCluster_ModeFrameOnly_SplitsAllUniqueFrames(t *testing.T) {
	got := RunWith(orWinsInputs(t), Config{Mode: ModeFrameOnly})
	if len(got) != 3 {
		t.Fatalf("ModeFrameOnly produced %d clusters; want 3", len(got))
	}
}

func TestCluster_ModeNormOnly_MergesSharedAssertion(t *testing.T) {
	got := RunWith(orWinsInputs(t), Config{Mode: ModeNormOnly})
	if len(got) != 1 {
		t.Fatalf("ModeNormOnly produced %d clusters; want 1", len(got))
	}
}
