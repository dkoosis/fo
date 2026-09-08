package view

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/tally"
)

func TestLeaderboardFromTally(t *testing.T) {
	tly := tally.Tally{Rows: []tally.Row{
		{Label: "a", Value: 10},
		{Label: "b", Value: 5},
		{Label: "c", Value: 1},
	}}
	lb := LeaderboardFromTally(tly)
	if lb.Total != 16 {
		t.Errorf("Total = %v, want 16", lb.Total)
	}
	if len(lb.Rows) != 3 || lb.Rows[0].Label != "a" {
		t.Errorf("rows = %+v", lb.Rows)
	}
}

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
