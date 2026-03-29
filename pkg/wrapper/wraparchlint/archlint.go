// Package wraparchlint converts go-arch-lint JSON output into SARIF 2.1.0.
package wraparchlint

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

// archlint converts go-arch-lint JSON to SARIF.
type archlint struct{}

func newArchlint() *archlint { return &archlint{} }

func init() {
	wrapper.Register("archlint", "Convert go-arch-lint JSON to SARIF", newArchlint())
}

func (a *archlint) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// RegisterFlags is a no-op — archlint wrapper has no flags.
func (a *archlint) RegisterFlags(_ *flag.FlagSet) {}

// Convert reads go-arch-lint JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for arch-lint reports (typically <100KB).
func (a *archlint) Convert(r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

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
		return fmt.Errorf("archlint: %w", err)
	}

	b := sarif.NewBuilder("go-arch-lint", "")
	for _, d := range raw.Payload.ArchWarningsDeps {
		msg := fmt.Sprintf("%s \u2192 %s", d.ComponentName, d.ResolvedImportName)
		b.AddResult("dependency-violation", "error", msg, d.FileRelativePath, 0, 0)
	}

	_, err = b.WriteTo(w)
	return err
}
