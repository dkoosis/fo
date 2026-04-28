// Package wraparchlint converts go-arch-lint JSON output into SARIF 2.1.0.
package wraparchlint

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/internal/boundread"
	"github.com/dkoosis/fo/pkg/sarif"
)

// archlint converts go-arch-lint JSON to SARIF.
type archlint struct{}

func newArchlint() *archlint { return &archlint{} }

// Convert reads go-arch-lint JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for arch-lint reports (typically <100KB).
// Bounded by boundread.DefaultMax to prevent OOM on pathological input (fo-s5x).
func (a *archlint) Convert(r io.Reader, w io.Writer) error {
	data, err := boundread.All(r, 0)
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
	const fixCmd = "go-arch-lint check --arch-file .go-arch-lint.yml"
	for _, d := range raw.Payload.ArchWarningsDeps {
		msg := fmt.Sprintf("%s \u2192 %s", d.ComponentName, d.ResolvedImportName)
		b.AddResultWithFix("dependency-violation", "error", msg, d.FileRelativePath, 0, 0, fixCmd)
	}

	_, err = b.WriteTo(w)
	return err
}
