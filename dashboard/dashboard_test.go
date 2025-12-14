package dashboard

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func boolPtr(v bool) *bool { return &v }

func TestAllowFailureAggregation(t *testing.T) {
	stdout := &bytes.Buffer{}
	dash := New("suite",
		WithStdout(stdout),
		WithTTY(boolPtr(false)),
	)

	dash.AddTask("Build", "ok", "bash", "-c", "echo success")
	dash.AddTaskSpec(TaskSpec{Group: "Quality", Name: "lint", Command: []string{"bash", "-c", "echo fail && exit 1"}, AllowFailure: true})
	dash.AddTask("Quality", "test", "bash", "-c", "echo broken && exit 2")

	res, err := dash.Run(context.Background())
	if err == nil {
		t.Fatalf("expected aggregated error for non-allowFailure task")
	}

	if len(res.Tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(res.Tasks))
	}

	if res.Tasks["Quality/test"].Status != Failed {
		t.Fatalf("expected Quality/test to fail")
	}

	if res.Tasks["Quality/lint"].Status != Failed {
		t.Fatalf("expected allow-failure task to be marked failed")
	}

	if !strings.Contains(stdout.String(), "Quality/test |") {
		t.Fatalf("expected streamed output for failing task, got %q", stdout.String())
	}
}

func TestTailTruncation(t *testing.T) {
	dash := New("tail", WithTTY(boolPtr(false)), WithMaxTailLines(3))
	dash.AddTask("Tail", "collector", "bash", "-c", "for i in $(seq 1 5); do echo line$i; done")

	res, err := dash.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := res.Tasks["Tail/collector"]
	if len(result.OutputTail) != 3 {
		t.Fatalf("expected 3 tail lines, got %d", len(result.OutputTail))
	}
	if result.OutputTail[0] != "line3" || result.OutputTail[2] != "line5" {
		t.Fatalf("unexpected tail contents: %+v", result.OutputTail)
	}
}

func TestEventsEmission(t *testing.T) {
	var events []Event
	dash := New("events",
		WithTTY(boolPtr(false)),
		WithOnEvent(func(e Event) { events = append(events, e) }),
	)

	dash.AddTask("Group", "task", "bash", "-c", "echo hi")
	if _, err := dash.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) < 2 {
		t.Fatalf("expected at least start and complete events, got %d", len(events))
	}
	if events[0].Type != EventTaskStarted {
		t.Fatalf("first event should be start, got %v", events[0].Type)
	}
	if events[len(events)-1].Type != EventTaskCompleted {
		t.Fatalf("last event should be completion, got %v", events[len(events)-1].Type)
	}
}

func TestPrefixStreaming(t *testing.T) {
	stdout := &bytes.Buffer{}
	dash := New("stream",
		WithStdout(stdout),
		WithTTY(boolPtr(false)),
	)

	dash.AddTask("Demo", "echo", "bash", "-c", "echo demo-line")
	if _, err := dash.Run(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Demo/echo | demo-line") {
		t.Fatalf("expected prefixed output, got %q", stdout.String())
	}
}
