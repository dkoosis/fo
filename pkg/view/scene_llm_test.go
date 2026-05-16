package view_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/view"
)

// canonicalSceneInput is the loto-demo from pkg/scene/scene_test.go::TestParse_happy.
const canonicalSceneInput = `# fo:scene title="loto demo" actors=FrugalLapwing,OddPlover

## 01 · whoami — every session has a handle

> a fresh session lands in the repo.
> first question: what am I called here?

@FrugalLapwing $ loto whoami
  handle: FrugalLapwing
  uuid:   818772ce

> that handle is what peers will see.

## 02 · lock + status

@OddPlover $ loto lock a.go --intent "rename Type"
  ✓ locked count=1
  (exit 0)

@OddPlover $ loto fail
  boom
  (exit 2)
`

func TestRenderSceneLLM_roundTrip(t *testing.T) {
	first, err := scene.Parse(strings.NewReader(canonicalSceneInput))
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	var buf bytes.Buffer
	if err := view.RenderSceneLLM(&buf, first); err != nil {
		t.Fatalf("render: %v", err)
	}
	second, err := scene.Parse(&buf)
	if err != nil {
		t.Fatalf("parse 2: %v\n--- rendered ---\n%s", err, buf.String())
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("round-trip not stable\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestRenderSceneLLM_noANSI(t *testing.T) {
	s, err := scene.Parse(strings.NewReader(canonicalSceneInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := view.RenderSceneLLM(&buf, s); err != nil {
		t.Fatalf("render: %v", err)
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	if ansi.MatchString(buf.String()) {
		t.Errorf("output contains ANSI escapes:\n%q", buf.String())
	}
}

func TestRenderSceneLLM_golden(t *testing.T) {
	s, err := scene.Parse(strings.NewReader(canonicalSceneInput))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var buf bytes.Buffer
	if err := view.RenderSceneLLM(&buf, s); err != nil {
		t.Fatalf("render: %v", err)
	}
	goldenPath := filepath.Join("testdata", "golden", "scene_loto_demo.golden")
	if *update {
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (%s): %v", goldenPath, err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), string(want))
	}
}

func TestRenderSceneLLM_emptyHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := view.RenderSceneLLM(&buf, scene.Scene{}); err != nil {
		t.Fatalf("render: %v", err)
	}
	if got := buf.String(); got != "# fo:scene\n" {
		t.Errorf("empty scene = %q", got)
	}
	// And it must round-trip.
	s2, err := scene.Parse(&buf)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if s2.Title != "" || len(s2.Actors) != 0 || len(s2.Acts) != 0 {
		t.Errorf("reparsed empty = %+v", s2)
	}
}

func TestRenderSceneLLM_emptyNarration(t *testing.T) {
	s := scene.Scene{
		Acts: []scene.Act{{
			Number: "01", Title: "t",
			Beats: []scene.Beat{{Kind: scene.BeatNarration, Narration: ""}},
		}},
	}
	var buf bytes.Buffer
	if err := view.RenderSceneLLM(&buf, s); err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(buf.String(), "\n>\n") {
		t.Errorf("empty narration missing bare '>':\n%s", buf.String())
	}
	s2, err := scene.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(s, s2) {
		t.Errorf("round-trip mismatch:\n%+v\nvs\n%+v", s, s2)
	}
}
