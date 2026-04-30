package wraparchlint_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper/wraparchlint"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestConvert_ProducesExpectedSARIF_When_ParsingArchlintJSON(t *testing.T) {
	t.Parallel()

	type input struct {
		json   string
		writer io.Writer
	}
	type want struct {
		ToolName string
		Results  int
		Message  string
		URI      string
		FixCmd   string
	}

	errWrite := errors.New("write failed")
	tests := []struct {
		name    string
		input   input
		want    want
		wantErr error
		inspect func(*testing.T, sarif.Document)
	}{
		{
			name:    "oversized input returns bounded read error",
			input:   input{json: strings.Repeat("x", boundread.DefaultMax+1)},
			wantErr: boundread.ErrInputTooLarge,
		},
		{
			name:  "clean payload yields zero results",
			input: input{json: `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[]}}`},
			want:  want{ToolName: "go-arch-lint", Results: 0},
			inspect: func(t *testing.T, doc sarif.Document) {
				t.Helper()
				require.Len(t, doc.Runs, 1)
				require.NotEmpty(t, doc.Version)
			},
		},
		{
			name:  "violation payload maps fields into result",
			input: input{json: `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[{"ComponentName":"search","FileRelativePath":"pkg/search/search.go","ResolvedImportName":"embedder"}]}}`},
			want: want{
				ToolName: "go-arch-lint",
				Results:  1,
				Message:  "search → embedder",
				URI:      "pkg/search/search.go",
				FixCmd:   "go-arch-lint check --arch-file .go-arch-lint.yml",
			},
			inspect: func(t *testing.T, doc sarif.Document) {
				t.Helper()
				r := doc.Runs[0].Results[0]
				require.Equal(t, "dependency-violation", r.RuleID)
				require.Equal(t, "error", r.Level)
			},
		},
		{
			name:  "multiple violations preserve one-result-per-input-warning invariant",
			input: input{json: `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[{"ComponentName":"a","FileRelativePath":"a.go","ResolvedImportName":"x"},{"ComponentName":"b","FileRelativePath":"b.go","ResolvedImportName":"y"}]}}`},
			want:  want{ToolName: "go-arch-lint", Results: 2, Message: "a → x", URI: "a.go", FixCmd: "go-arch-lint check --arch-file .go-arch-lint.yml"},
			inspect: func(t *testing.T, doc sarif.Document) {
				t.Helper()
				require.Len(t, doc.Runs[0].Results, 2)
				require.Contains(t, doc.Runs[0].Results[0].Message.Text, "→")
				require.Contains(t, doc.Runs[0].Results[1].Message.Text, "→")
			},
		},
		{
			name:    "writer failure is returned",
			input:   input{json: `{"Type":"models.Check","Payload":{"ArchWarningsDeps":[]}}`, writer: &failingWriter{err: errWrite}},
			wantErr: errWrite,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			writer := tc.input.writer
			if writer == nil {
				writer = &buf
			}

			err := wraparchlint.Convert(strings.NewReader(tc.input.json), writer)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)

			var got sarif.Document
			require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

			require.NotEmpty(t, got.Runs)
			gotRun := got.Runs[0]
			gotWant := want{ToolName: gotRun.Tool.Driver.Name, Results: len(gotRun.Results)}
			if len(gotRun.Results) > 0 {
				gotWant.Message = gotRun.Results[0].Message.Text
				gotWant.URI = gotRun.Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI
				gotWant.FixCmd = gotRun.Results[0].FixCommand()
			}
			if diff := cmp.Diff(tc.want, gotWant); diff != "" {
				t.Errorf("diff (-want +got):\n%s", diff)
			}

			if tc.inspect != nil {
				tc.inspect(t, got)
			}
		})
	}
}

type failingWriter struct{ err error }

func (f *failingWriter) Write(_ []byte) (int, error) {
	return 0, f.err
}
