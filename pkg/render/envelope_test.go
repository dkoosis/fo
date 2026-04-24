package render_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dkoosis/fo/pkg/pattern"
	"github.com/dkoosis/fo/pkg/render"
)

// hash mimics the cmd/fo hash recipe so tests stay decoupled from the binary.
func hash12(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:12]
}

func TestEnvelope_LLM_RendersHashAndTimestamp(t *testing.T) {
	t.Parallel()

	input := []byte("any input bytes")
	meta := render.RunMeta{
		DataHash:    hash12(input),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	out := render.NewLLM().WithMeta(meta).Render(nil)

	if !strings.Contains(out, "data_hash: "+meta.DataHash) {
		t.Fatalf("LLM output missing data_hash: %q", out)
	}
	if !strings.Contains(out, "generated_at: "+meta.GeneratedAt) {
		t.Fatalf("LLM output missing generated_at: %q", out)
	}
	if _, err := time.Parse(time.RFC3339, meta.GeneratedAt); err != nil {
		t.Fatalf("generated_at not RFC3339: %v", err)
	}
}

func TestEnvelope_JSON_RendersHashAndTimestamp(t *testing.T) {
	t.Parallel()

	input := []byte("identical input")
	meta := render.RunMeta{
		DataHash:    hash12(input),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	out := render.NewJSON().WithMeta(meta).Render(nil)

	var got struct {
		DataHash    string `json:"data_hash"`
		GeneratedAt string `json:"generated_at"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got.DataHash != meta.DataHash {
		t.Fatalf("data_hash = %q, want %q", got.DataHash, meta.DataHash)
	}
	if got.GeneratedAt != meta.GeneratedAt {
		t.Fatalf("generated_at = %q, want %q", got.GeneratedAt, meta.GeneratedAt)
	}
	if _, err := time.Parse(time.RFC3339, got.GeneratedAt); err != nil {
		t.Fatalf("generated_at not RFC3339 parseable: %v", err)
	}
}

func TestEnvelope_HashStableAcrossRuns(t *testing.T) {
	t.Parallel()

	input := []byte("deterministic content\nline 2\n")
	dup := append([]byte{}, input...)
	if hash12(input) != hash12(dup) {
		t.Fatalf("hash not stable for identical input")
	}
	if hash12(input) == hash12([]byte("different content")) {
		t.Fatalf("hash collision for distinct inputs")
	}
}

func TestEnvelope_HumanRendererUnchanged(t *testing.T) {
	t.Parallel()

	// Human renderer must NOT carry envelope metadata. NewHuman has no WithMeta.
	r := render.NewHuman(render.MonoTheme())
	out := r.Render([]pattern.Pattern{})
	if strings.Contains(out, "data_hash") || strings.Contains(out, "generated_at") {
		t.Fatalf("human renderer leaked envelope: %q", out)
	}
}
