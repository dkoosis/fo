package report_test

import (
	"testing"

	"github.com/dkoosis/fo/pkg/report"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestParseSections_ExpectedBehaviour_When_InputVaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     []byte
		want      []report.Section
		wantErr   error
		wantErrAs interface{}
		inspect   func(*testing.T, []report.Section)
	}{
		{name: "error when no delimiters", input: []byte("just output\nwithout delimiters\n"), wantErr: report.ErrNoSections},
		{name: "error on unknown format delimiter", input: []byte("--- tool:lint format:text ---\nbody\n"), wantErrAs: &report.UnknownFormatError{}},
		{
			name:  "boundary empty section content between delimiters",
			input: []byte("--- tool:vet format:sarif ---\n--- tool:test format:testjson ---\n{\"Action\":\"pass\"}\n"),
			want: []report.Section{{Tool: "vet", Format: "sarif", Status: "", Content: nil}, {Tool: "test", Format: "testjson", Status: "", Content: []byte(`{"Action":"pass"}`)}},
			inspect: func(t *testing.T, got []report.Section) { require.Len(t, got[0].Content, 0) },
		},
		{
			name:  "happy path parses two sections with statuses",
			input: []byte("--- tool:vet format:sarif status:clean ---\nline1\nline2\n--- tool:test format:testjson status:error ---\n{\"Action\":\"fail\"}\n"),
			want: []report.Section{{Tool: "vet", Format: "sarif", Status: report.StatusClean, Content: []byte("line1\nline2")}, {Tool: "test", Format: "testjson", Status: report.StatusError, Content: []byte(`{"Action":"fail"}`)}},
			inspect: func(t *testing.T, got []report.Section) {
				for _, sec := range got {
					require.NotEmpty(t, sec.Tool)
					require.Contains(t, report.SupportedFormats, sec.Format)
					require.False(t, len(sec.Content) > 0 && sec.Content[len(sec.Content)-1] == '\n')
				}
			},
		},
		{name: "boundary CRLF input normalized", input: []byte("--- tool:vet format:sarif ---\r\nbody\r\n"), want: []report.Section{{Tool: "vet", Format: "sarif", Status: "", Content: []byte("body")}}, inspect: func(t *testing.T, got []report.Section) { require.NotContains(t, string(got[0].Content), "\r") }},
		{name: "happy path status omitted defaults to empty status", input: []byte("--- tool:fmt format:sarif ---\ncontent\n"), want: []report.Section{{Tool: "fmt", Format: "sarif", Status: "", Content: []byte("content")}}, inspect: func(t *testing.T, got []report.Section) { require.Equal(t, "", got[0].Status) }},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, _, err := report.ParseSections(tc.input)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			if tc.wantErrAs != nil {
				require.ErrorAs(t, err, &tc.wantErrAs)
				return
			}
			require.NoError(t, err)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("diff (-want +got):\n%s", diff)
			}
			if tc.inspect != nil { tc.inspect(t, got) }
		})
	}
}
