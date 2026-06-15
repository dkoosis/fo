package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dkoosis/fo/pkg/metrics"
	"github.com/dkoosis/fo/pkg/report"
	"github.com/dkoosis/fo/pkg/scene"
	"github.com/dkoosis/fo/pkg/state"
	"github.com/dkoosis/fo/pkg/status"
	"github.com/dkoosis/fo/pkg/tally"
	"github.com/dkoosis/fo/pkg/theme"
	"github.com/dkoosis/fo/pkg/view"
)

func renderMode(mode string, r *report.Report, stdout io.Writer, themeName string, expandValues []string) error {
	if mode == formatJSON {
		return writeReportJSON(stdout, r)
	}
	t := resolveTheme(themeName, stdout)
	viewMode := view.ModeHuman
	if mode == formatLLM {
		t = theme.Mono()
		viewMode = view.ModeLLM
	}
	width := termSize(stdout)
	expand := view.NewExpandSet(expandValues)
	if err := view.RenderReportModeWithExpand(stdout, *r, t, width, viewMode, expand); err != nil {
		return err
	}
	if mode == formatLLM {
		writeDiffDetail(stdout, r)
	}
	return nil
}

// renderHygiene dispatches the format switch shared by the hygiene
// renderers (tally/status/metrics/scene). Each caller supplies the
// JSON-encodable value plus closures for the LLM and human writers; the
// helper handles encoding, error reporting, and the exit code. Returns 0
// on success, 2 on writer error.
func renderHygiene(stdout, stderr io.Writer, mode string, jsonValue any, llmFn, humanFn func(io.Writer) error) int {
	switch mode {
	case formatJSON:
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(jsonValue); err != nil {
			fmt.Fprintf(stderr, "fo: %v\n", err)
			return 2
		}
	case formatLLM:
		if err := llmFn(stdout); err != nil {
			fmt.Fprintf(stderr, "fo: %v\n", err)
			return 2
		}
	default:
		if err := humanFn(stdout); err != nil {
			fmt.Fprintf(stderr, "fo: %v\n", err)
			return 2
		}
	}
	return 0
}

// renderTally parses tally-format input and emits the Leaderboard view.
// Bypasses parseToReport/pickview because tallies aren't findings —
// callers explicitly asked for a count-weighted bar chart, not a
// severity-aggregated one. Always exits 0 on success: a tally is
// informational, not pass/fail.
func renderTally(input []byte, stdout io.Writer, stderr io.Writer, mode, themeName string) int {
	t, err := tally.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing tally: %v\n", err)
		return 2
	}
	jsonOut := struct {
		Tool  string      `json:"tool,omitempty"`
		Total float64     `json:"total"`
		Rows  []tally.Row `json:"rows"`
	}{Tool: t.Tool, Rows: t.Rows}
	for _, r := range t.Rows {
		jsonOut.Total += r.Value
	}
	return renderHygiene(stdout, stderr, mode, jsonOut,
		func(w io.Writer) error { return t.RenderLLM(w) },
		func(w io.Writer) error {
			th := resolveTheme(themeName, w)
			width := termSize(w)
			out := view.Render(t.ToLeaderboard(), th, width)
			_, werr := fmt.Fprintln(w, out)
			return werr
		})
}

// castDelay assigns the pause before each beat of a cast recording.
// Narration beats hold longer (there is prose to read); command beats
// pace like a brisk live demo. Tuned for watchability, not realism — a
// recording, not a replay of actual wall-clock timing.
func castDelay(b scene.Beat) time.Duration {
	if b.Kind == scene.BeatNarration {
		return 2500 * time.Millisecond
	}
	return 1200 * time.Millisecond
}

// renderScene parses # fo:scene input and dispatches to the human or
// llm scene renderer (or JSON encoder). Always exits 0 on success —
// scenes are narration, not gates (fo-fl0.4).
func renderScene(input []byte, stdout io.Writer, stderr io.Writer, mode string) int {
	s, err := scene.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing scene: %v\n", err)
		return 2
	}
	if mode == formatCast {
		frames := scene.Cast(s, view.RenderSceneHumanString, castDelay)
		if err := scene.EncodeAsciicast(stdout, frames); err != nil {
			fmt.Fprintf(stderr, "fo: %v\n", err)
			return 2
		}
		return 0
	}
	return renderHygiene(stdout, stderr, mode, s,
		func(w io.Writer) error { return view.RenderSceneLLM(w, s) },
		func(w io.Writer) error { return view.RenderSceneHuman(w, s) })
}

// renderStatus parses status-format input and emits the PASS/FAIL table.
// Always exits 0 on success — status streams are reports, not gates;
// callers decide pass/fail by inspecting the rows themselves (or via the
// parsed json).
func renderStatus(input []byte, stdout io.Writer, stderr io.Writer, mode string) int {
	s, err := status.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing status: %v\n", err)
		return 2
	}
	rows := make([]view.StatusRow, len(s.Rows))
	for i, r := range s.Rows {
		rows[i] = view.StatusRow{State: string(r.State), Label: r.Label, Value: r.Value, Note: r.Note}
	}
	return renderHygiene(stdout, stderr, mode, s,
		func(w io.Writer) error { return view.RenderStatusLLM(w, s.Tool, rows) },
		func(w io.Writer) error { return view.RenderStatusHuman(w, s.Tool, rows) })
}

// renderMetrics parses metrics-format input, computes deltas against
// the sidecar history, renders, and saves the new sample set. Always
// exits 0 on success — metrics streams are informational rollups.
func renderMetrics(input []byte, stdout io.Writer, stderr io.Writer, mode string) int {
	m, err := metrics.Parse(bytes.NewReader(input))
	if err != nil {
		fmt.Fprintf(stderr, "fo: parsing metrics: %v\n", err)
		return 2
	}
	curr := make([]state.MetricSample, len(m.Rows))
	for i, r := range m.Rows {
		curr[i] = state.MetricSample{Tool: m.Tool, Key: r.Key, Value: r.Value, Unit: r.Unit}
	}
	histPath := state.MetricsHistoryPath()
	prev, loadErr := state.LoadMetrics(histPath)
	if loadErr != nil {
		fmt.Fprintf(stderr, "fo: load metrics history: %v\n", loadErr)
	}
	deltas := state.DiffMetrics(prev, curr)

	rows := make([]view.MetricRow, len(deltas))
	for i, d := range deltas {
		rows[i] = view.MetricRow{
			Key: d.Sample.Key, Value: d.Sample.Value, Unit: d.Sample.Unit, Delta: d.Delta, New: d.New,
		}
	}

	jsonOut := struct {
		Tool   string              `json:"tool,omitempty"`
		Deltas []state.MetricDelta `json:"deltas"`
	}{Tool: m.Tool, Deltas: deltas}
	if code := renderHygiene(stdout, stderr, mode, jsonOut,
		func(w io.Writer) error { return view.RenderMetricsLLM(w, m.Tool, rows) },
		func(w io.Writer) error { return view.RenderMetricsHuman(w, m.Tool, rows) }); code != 0 {
		return code
	}

	if err := os.MkdirAll(state.Dir(), 0o755); err != nil {
		fmt.Fprintf(stderr, "fo: save metrics history: %v\n", err)
		return 0
	}
	if err := state.AppendMetrics(histPath, curr); err != nil {
		fmt.Fprintf(stderr, "fo: save metrics history: %v\n", err)
	}
	return 0
}
func writeReportJSON(w io.Writer, r *report.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
