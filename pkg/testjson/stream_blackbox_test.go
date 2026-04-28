package testjson_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/testjson"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestStream_ExpectedBehaviour_When_InputVaries(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	tests := []struct {
		name          string
		input         io.Reader
		cancelBefore  bool
		want          []testjson.TestEvent
		wantMalformed int
		wantErr       error
		inspect       func(*testing.T, []testjson.TestEvent)
	}{
		{
			name:         "returns context cancellation error",
			input:        strings.NewReader(`{"Action":"run","Package":"example.com/p","Test":"TestA"}` + "\n"),
			cancelBefore: true,
			want:         nil,
			wantErr:      context.Canceled,
		},
		{
			name: "propagates reader failure",
			input: &errReader{
				err: errBoom,
			},
			want:    nil,
			wantErr: errBoom,
		},
		{
			name:          "empty input is a no-op boundary",
			input:         strings.NewReader(""),
			want:          []testjson.TestEvent{},
			wantMalformed: 0,
			inspect: func(t *testing.T, got []testjson.TestEvent) {
				t.Helper()
				require.Len(t, got, 0)
			},
		},
		{
			name: "returns events in input order for valid ndjson",
			input: strings.NewReader(strings.Join([]string{
				`{"Action":"run","Package":"example.com/p","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/p","Test":"TestA"}`,
				`{"Action":"pass","Package":"example.com/p"}`,
			}, "\n") + "\n"),
			want: []testjson.TestEvent{
				{Action: "run", Package: "example.com/p", Test: "TestA"},
				{Action: "pass", Package: "example.com/p", Test: "TestA"},
				{Action: "pass", Package: "example.com/p"},
			},
			wantMalformed: 0,
			inspect: func(t *testing.T, got []testjson.TestEvent) {
				t.Helper()
				for i, event := range got {
					require.NotEmpty(t, event.Action, "event %d action should be populated", i)
				}
			},
		},
		{
			name: "counts malformed lines but keeps valid events",
			input: strings.NewReader(strings.Join([]string{
				`{"Action":"run","Package":"example.com/p","Test":"TestA"}`,
				`not json`,
				`{"Action":"fail","Package":"example.com/p","Test":"TestA"}`,
				`{bad`,
			}, "\n") + "\n"),
			want: []testjson.TestEvent{
				{Action: "run", Package: "example.com/p", Test: "TestA"},
				{Action: "fail", Package: "example.com/p", Test: "TestA"},
			},
			wantMalformed: 2,
			inspect: func(t *testing.T, got []testjson.TestEvent) {
				t.Helper()
				require.GreaterOrEqual(t, len(got), 1)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if tc.cancelBefore {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			got := make([]testjson.TestEvent, 0)
			malformed, err := testjson.Stream(ctx, tc.input, func(e testjson.TestEvent) {
				got = append(got, testjson.TestEvent{Action: e.Action, Package: e.Package, Test: e.Test})
			})

			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			if diff := cmp.Diff(tc.wantMalformed, malformed); diff != "" {
				t.Errorf("malformed diff (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("diff (-want +got):\n%s", diff)
			}

			if tc.inspect != nil {
				tc.inspect(t, got)
			}
		})
	}
}

type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (int, error) {
	return 0, r.err
}
