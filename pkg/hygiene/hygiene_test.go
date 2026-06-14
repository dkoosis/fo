package hygiene_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dkoosis/fo/pkg/hygiene"
)

func TestHasHeader(t *testing.T) {
	tests := []struct {
		name   string
		data   string
		prefix string
		want   bool
	}{
		{"exact", statusHeader, statusHeader, true},
		{"leading whitespace", "  \n\t# fo:status x", statusHeader, true},
		{"with attrs", "# fo:status tool=vet", statusHeader, true},
		{"wrong prefix", "# fo:metrics", statusHeader, false},
		{"empty", "", statusHeader, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hygiene.HasHeader([]byte(tt.data), tt.prefix); got != tt.want {
				t.Errorf("HasHeader(%q) = %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestParseAttr(t *testing.T) {
	tests := []struct {
		tail, key, want string
	}{
		{"tool=vet", toolKey, wantVet},
		{"foo=bar tool=vet", toolKey, wantVet},
		{"tool=vet foo=bar", toolKey, wantVet},
		{"foo=bar", toolKey, ""},
		{"", toolKey, ""},
		{"=novalue", toolKey, ""},
	}
	for _, tt := range tests {
		if got := hygiene.ParseAttr(tt.tail, tt.key); got != tt.want {
			t.Errorf("ParseAttr(%q, %q) = %q, want %q", tt.tail, tt.key, got, tt.want)
		}
	}
}

const (
	testPrefix   = "# fo:test"
	testName     = "test"
	statusHeader = "# fo:status"
	toolKey      = "tool"
	wantVet      = "vet"
)

var (
	errNoHeader = errors.New("test: missing header")
	errNoRows   = errors.New("test: no rows")
	errBadRow   = errors.New("test: bad row")
)

// collect builds a Spec that appends each data line to rows and returns
// the captured slice pointer.
func collect(rows *[]string) hygiene.Spec {
	return hygiene.Spec{
		Prefix:      testPrefix,
		Name:        testName,
		ErrNoHeader: errNoHeader,
		ErrNoRows:   errNoRows,
		OnRow: func(_ int, line string) error {
			*rows = append(*rows, line)
			return nil
		},
	}
}

func TestScan(t *testing.T) {
	t.Run("happy path with attr, comments, blanks", func(t *testing.T) {
		in := testPrefix + " tool=vet\n\na\n# a comment\nb\n"
		var rows []string
		tool, err := hygiene.Scan(strings.NewReader(in), collect(&rows))
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if tool != "vet" {
			t.Errorf("tool = %q, want vet", tool)
		}
		if strings.Join(rows, ",") != "a,b" {
			t.Errorf("rows = %v, want [a b]", rows)
		}
	})

	t.Run("missing header", func(t *testing.T) {
		var rows []string
		_, err := hygiene.Scan(strings.NewReader("a\nb\n"), collect(&rows))
		if !errors.Is(err, errNoHeader) {
			t.Errorf("err = %v, want errNoHeader", err)
		}
	})

	t.Run("header only, no rows", func(t *testing.T) {
		var rows []string
		_, err := hygiene.Scan(strings.NewReader(testPrefix+"\n"), collect(&rows))
		if !errors.Is(err, errNoRows) {
			t.Errorf("err = %v, want errNoRows", err)
		}
	})

	t.Run("OnRow error wrapped with name and line number", func(t *testing.T) {
		spec := hygiene.Spec{
			Prefix:      testPrefix,
			Name:        testName,
			ErrNoHeader: errNoHeader,
			ErrNoRows:   errNoRows,
			OnRow:       func(_ int, _ string) error { return errBadRow },
		}
		// header is line 1, blank line 2, bad row line 3.
		_, err := hygiene.Scan(strings.NewReader(testPrefix+"\n\nboom\n"), spec)
		if !errors.Is(err, errBadRow) {
			t.Fatalf("err = %v, want errBadRow", err)
		}
		if !strings.Contains(err.Error(), "test: line 3:") {
			t.Errorf("err = %q, want name+line-number prefix", err)
		}
	})
}
