// Package wrapjscpd converts jscpd JSON duplication reports into SARIF 2.1.0.
package wrapjscpd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper/internal/boundread"
)

// clone records a single code duplication instance.
type clone struct {
	Lines  int
	FileA  string
	StartA int
	FileB  string
	StartB int
	EndB   int
}

// jscpd converts jscpd JSON to SARIF.
type jscpd struct{}

func newJscpd() *jscpd { return &jscpd{} }

// Convert reads jscpd JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for jscpd reports (typically <1MB).
// Bounded by boundread.DefaultMax to prevent OOM on pathological input (fo-s5x).
func (j *jscpd) Convert(r io.Reader, w io.Writer) error {
	data, err := boundread.All(r, 0)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	clones, err := parseClones(data)
	if err != nil {
		return err
	}

	b := sarif.NewBuilder("jscpd", "")
	for _, c := range clones {
		msg := fmt.Sprintf("%d lines duplicated with %s:%d-%d", c.Lines, c.FileB, c.StartB, c.EndB)
		endA := c.StartA + c.Lines - 1
		// Diagnostic hint (not a fix): jump to both ends of the clone pair.
		fixCmd := fmt.Sprintf("# duplicate: %s:%d-%d ↔ %s:%d-%d", c.FileA, c.StartA, endA, c.FileB, c.StartB, c.EndB)
		b.AddResultWithFix("code-clone", "warning", msg, c.FileA, c.StartA, 0, fixCmd)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseClones decodes jscpd JSON report into a slice of clones.
func parseClones(data []byte) ([]clone, error) {
	var raw struct {
		Duplicates []struct {
			Lines     int `json:"lines"`
			FirstFile struct {
				Name     string `json:"name"`
				StartLoc struct {
					Line int `json:"line"`
				} `json:"startLoc"`
				EndLoc struct {
					Line int `json:"line"`
				} `json:"endLoc"`
			} `json:"firstFile"`
			SecondFile struct {
				Name     string `json:"name"`
				StartLoc struct {
					Line int `json:"line"`
				} `json:"startLoc"`
				EndLoc struct {
					Line int `json:"line"`
				} `json:"endLoc"`
			} `json:"secondFile"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("jscpd: %w", err)
	}

	clones := make([]clone, 0, len(raw.Duplicates))
	for _, d := range raw.Duplicates {
		clones = append(clones, clone{
			Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return clones, nil
}
