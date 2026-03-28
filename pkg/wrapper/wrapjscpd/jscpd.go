// Package wrapjscpd converts jscpd JSON duplication reports into SARIF 2.1.0.
package wrapjscpd

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/dkoosis/fo/pkg/sarif"
	"github.com/dkoosis/fo/pkg/wrapper"
)

// clone records a single code duplication instance.
type clone struct {
	Format string
	Lines  int
	FileA  string
	StartA int
	EndA   int
	FileB  string
	StartB int
	EndB   int
}

// Jscpd converts jscpd JSON to SARIF.
type Jscpd struct{}

// New returns a new Jscpd wrapper.
func New() *Jscpd { return &Jscpd{} }

func init() {
	wrapper.Register("jscpd", "Convert jscpd JSON duplication report to SARIF", New())
}

// OutputFormat returns FormatSARIF.
func (j *Jscpd) OutputFormat() wrapper.Format { return wrapper.FormatSARIF }

// RegisterFlags is a no-op — jscpd wrapper has no flags.
func (j *Jscpd) RegisterFlags(_ *flag.FlagSet) {}

// Convert reads jscpd JSON from r and writes SARIF to w.
// Reads entire input into memory — fine for jscpd reports (typically <1MB).
func (j *Jscpd) Convert(r io.Reader, w io.Writer) error {
	data, err := io.ReadAll(r)
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
		b.AddResult("code-clone", "warning", msg, c.FileA, c.StartA, 0)
	}

	_, err = b.WriteTo(w)
	return err
}

// parseClones decodes jscpd JSON report into a slice of clones.
func parseClones(data []byte) ([]clone, error) {
	var raw struct {
		Duplicates []struct {
			Format    string `json:"format"`
			Lines     int    `json:"lines"`
			FirstFile struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"firstFile"`
			SecondFile struct {
				Name     string `json:"name"`
				StartLoc struct{ Line int } `json:"startLoc"`
				EndLoc   struct{ Line int } `json:"endLoc"`
			} `json:"secondFile"`
		} `json:"duplicates"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("jscpd: %w", err)
	}

	clones := make([]clone, 0, len(raw.Duplicates))
	for _, d := range raw.Duplicates {
		clones = append(clones, clone{
			Format: d.Format, Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line, EndA: d.FirstFile.EndLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return clones, nil
}
