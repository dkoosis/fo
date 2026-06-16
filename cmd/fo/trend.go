package main

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/dkoosis/fo/pkg/paint"
	"github.com/dkoosis/fo/pkg/state"
)

// runTrend handles `fo trend <rule-id>` — it charts how often a rule has
// fired across the recorded run history, so a regression that crept in
// across a dozen individually-clean runs becomes visible as a rising line.
func runTrend(args []string, stdout, stderr io.Writer) int {
	rule := ""
	for _, a := range args {
		if a == "-h" || a == flagHelp {
			fmt.Fprintln(stderr, "usage: fo trend <rule-id>   (charts a rule's count across recorded runs)")
			return 0
		}
		if rule == "" && a != "" && a[0] != '-' {
			rule = a
		}
	}
	if rule == "" {
		fmt.Fprintln(stderr, "fo trend: a rule id is required (e.g. fo trend SA1000)")
		return 2
	}

	rl, err := state.LoadRunLog(state.RunLogPath())
	if err != nil {
		fmt.Fprintf(stderr, "fo trend: %v\n", err)
		return 2
	}
	if rl == nil || len(rl.Entries) == 0 {
		fmt.Fprintln(stderr, "fo trend: no run history yet — run fo a few times first")
		return 2
	}

	series := rl.RuleSeries(rule)
	values := make([]float64, len(series))
	var total int
	for i, c := range series {
		values[i] = float64(c)
		total += c
	}
	if total == 0 {
		fmt.Fprintf(stderr, "fo trend: %s never appeared in the last %d run(s)\n", rule, len(series))
		return 2
	}

	first, last := series[0], series[len(series)-1]
	fmt.Fprintf(stdout, "%s  %s  %s\n", rule, paint.Sparkline(values), trendArrow(first, last))
	fmt.Fprintf(stdout, "runs %d  first %d  last %d  peak %d\n", len(series), first, last, maxInt(series))
	return 0
}

// runReplay handles `fo replay [--since=<dur>]` — it lists recent runs with
// their headline counts so a reader can see the shape of activity over time
// without re-running anything.
func runReplay(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fo replay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	since := fs.Duration("since", 0, "Only show runs newer than this (e.g. 1h, 30m); 0 = all")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	rl, err := state.LoadRunLog(state.RunLogPath())
	if err != nil {
		fmt.Fprintf(stderr, "fo replay: %v\n", err)
		return 2
	}
	if rl == nil || len(rl.Entries) == 0 {
		fmt.Fprintln(stderr, "fo replay: no run history yet")
		return 2
	}

	cutoff := replayCutoff(*since, rl.Entries[len(rl.Entries)-1].At)
	shown := 0
	for i := range rl.Entries {
		e := &rl.Entries[i]
		if !e.At.Before(cutoff) {
			fmt.Fprintln(stdout, replayLine(e))
			shown++
		}
	}
	if shown == 0 {
		fmt.Fprintf(stderr, "fo replay: no runs within the last %s\n", *since)
		return 2
	}
	return 0
}

// replayCutoff returns the oldest timestamp to show. A zero duration shows
// everything; otherwise the window is measured back from the newest run's
// time rather than wall clock, so replay is stable regardless of how long
// ago the last run was.
func replayCutoff(since time.Duration, newest time.Time) time.Time {
	if since <= 0 {
		return time.Time{} // zero time → everything is after it
	}
	return newest.Add(-since)
}

func replayLine(e *state.RunLogEntry) string {
	tool := e.Tool
	if tool == "" {
		tool = "-"
	}
	return fmt.Sprintf("%s  %-12s  err %d  warn %d  note %d  fail %d  pass %d",
		e.At.Format("2006-01-02 15:04:05"), tool,
		e.Errors, e.Warnings, e.Notes, e.TestsFailed, e.TestsPassed)
}

func trendArrow(first, last int) string {
	switch {
	case last > first:
		return fmt.Sprintf("worsening +%d", last-first)
	case last < first:
		return fmt.Sprintf("improving -%d", first-last)
	default:
		return "flat"
	}
}

func maxInt(xs []int) int {
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}
