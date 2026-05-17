package view_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/view"
)

const frugalLapwing = "FrugalLapwing"

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestRenderSceneHuman(t *testing.T) {
	s := scene.Scene{
		Title:  "Demo",
		Actors: []string{frugalLapwing, "BraveOtter"},
		Acts: []scene.Act{
			{
				Number: "1",
				Title:  "Setup",
				Beats: []scene.Beat{
					{Kind: scene.BeatNarration, Narration: "Two agents claim territory."},
					{Kind: scene.BeatCommand, Command: scene.Command{
						Actor: frugalLapwing, Cmd: "loto whoami",
						Output: []string{frugalLapwing},
					}},
				},
			},
			{
				Number: "2",
				Title:  "Conflict",
				Beats: []scene.Beat{
					{Kind: scene.BeatCommand, Command: scene.Command{
						Actor: "BraveOtter", Cmd: "loto acquire pkg/x",
						Output: []string{"held by FrugalLapwing"},
						Exit:   1,
					}},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := view.RenderSceneHuman(&buf, s); err != nil {
		t.Fatalf("render: %v", err)
	}
	plain := stripANSI(buf.String())

	for _, want := range []string{
		"Demo",
		"1 · Setup",
		"2 · Conflict",
		"Two agents claim territory.",
		frugalLapwing,
		"BraveOtter",
		"❯ loto whoami",
		"❯ loto acquire pkg/x",
		"  held by FrugalLapwing",
		"(exit 1)",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "(exit 0)") {
		t.Errorf("zero exit should be suppressed, got:\n%s", plain)
	}
}
