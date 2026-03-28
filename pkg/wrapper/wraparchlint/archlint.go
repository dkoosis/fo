// Package wraparchlint converts go-arch-lint JSON output into SARIF 2.1.0.
package wraparchlint

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

type violation struct {
	From     string
	To       string
	FileFrom string
}

// Archlint converts go-arch-lint JSON to SARIF.
type Archlint struct{}

// New returns a new Archlint wrapper.
func New() *Archlint { return &Archlint{} }

func init() {
	wrapper.Register("archlint", New())
}

// OutputFormat returns FormatSARIF.
func (a *Archlint) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// Wrap reads go-arch-lint JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for arch-lint reports (typically <100KB).
func (a *Archlint) Wrap(_ []string, r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	violations, err := parseResult(data)
	if err != nil {
		return err
	}

	b := sarif.NewBuilder("go-arch-lint", "")
	for _, v := range violations {
		msg := fmt.Sprintf("%s \u2192 %s", v.From, v.To)
		b.AddResult("dependency-violation", "error", msg, v.FileFrom, 0, 0)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseResult decodes go-arch-lint --json output.
func parseResult(data []byte) ([]violation, error) {
	var raw struct {
		Payload struct {
			ArchWarningsDeps []struct {
				ComponentName      string `json:"ComponentName"`
				FileRelativePath   string `json:"FileRelativePath"`
				ResolvedImportName string `json:"ResolvedImportName"`
			} `json:"ArchWarningsDeps"`
		} `json:"Payload"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("archlint: %w", err)
	}

	vs := make([]violation, len(raw.Payload.ArchWarningsDeps))
	for i, d := range raw.Payload.ArchWarningsDeps {
		vs[i] = violation{
			From:     d.ComponentName,
			To:       d.ResolvedImportName,
			FileFrom: d.FileRelativePath,
		}
	}
	return vs, nil
}
