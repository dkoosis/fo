package cluster

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateGoldens = flag.Bool("update", false, "update golden files in testdata/golden/")

type fixture struct {
	Inputs []Input        `json:"inputs"`
	Config *fixtureConfig `json:"config,omitempty"`
}

type fixtureConfig struct {
	Mode string `json:"mode"`
}

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
		t.Fatalf("Run([]) = %v; want empty non-nil slice", got)
	}
}

func TestCluster_EmptyKeySkipped(t *testing.T) {
	// Empty-Key inputs would otherwise collapse together via map dedup
	// and silently merge unrelated failures. RunWith must drop them so
	// only valid-keyed inputs survive into Cluster.Members.
	inputs := []Input{
		{Key: "", Output: "panic: runtime error A"},
		{Key: "", Output: "panic: runtime error B"},
		{Key: "k1", Output: "panic: runtime error C"},
	}
	got := RunWith(inputs, Config{})
	for _, c := range got {
		for _, m := range c.Members {
			if m == "" {
				t.Fatalf("empty Key leaked into Cluster.Members: %+v", c)
			}
		}
	}
	// After empty-Key skip, only k1 remains → at most one cluster.
	if len(got) > 1 {
		t.Fatalf("got %d clusters from one valid input, want ≤1: %+v", len(got), got)
	}
}

func TestCluster_Fixtures(t *testing.T) {
	fixtures, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, f := range fixtures {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			var fx fixture
			if err := json.Unmarshal(raw, &fx); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
			cfg := Config{}
			if fx.Config != nil {
				switch fx.Config.Mode {
				case "and":
					cfg.Mode = ModeAnd
				case "frame":
					cfg.Mode = ModeFrameOnly
				case "norm":
					cfg.Mode = ModeNormOnly
				}
			}
			got := RunWith(fx.Inputs, cfg)
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			goldenPath := filepath.Join("testdata", "golden", name+".json")
			if *updateGoldens {
				if err := os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			wantRaw, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to regenerate): %v", err)
			}
			want := strings.TrimRight(string(wantRaw), "\n")
			gotStr := strings.TrimRight(string(gotJSON), "\n")
			if want != gotStr {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", name, gotStr, want)
			}
		})
	}
}
