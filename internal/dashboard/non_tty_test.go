package dashboard

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunNonTTYStreamsOutputAndSummary(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	specs := []TaskSpec{
		{Group: "Build", Name: "ok", Command: "printf 'hello\n'"},
		{Group: "Build", Name: "fail", Command: "printf 'oops\n' && exit 1"},
	}

	var buf bytes.Buffer
	exitCode := RunNonTTY(ctx, specs, &buf)
	output := buf.String()

	if !strings.Contains(output, "[Build/ok] hello") {
		t.Fatalf("missing ok output: %s", output)
	}
	if !strings.Contains(output, "[Build/fail] oops") {
		t.Fatalf("missing fail output: %s", output)
	}
	if !strings.Contains(output, "Summary:") {
		t.Fatalf("missing summary: %s", output)
	}
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}
