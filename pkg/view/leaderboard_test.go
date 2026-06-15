package view

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderLeaderboardLLM(t *testing.T) {
	lb := Leaderboard{Rows: []LbRow{
		{Label: "log.friction", Value: 14332},
		{Label: "journal.day", Value: 2578},
	}}
	var buf bytes.Buffer
	if err := RenderLeaderboardLLM(&buf, lb); err != nil {
		t.Fatalf("RenderLeaderboardLLM: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "log.friction") || !strings.Contains(out, "14332") {
		t.Errorf("output missing data: %q", out)
	}
}
