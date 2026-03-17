package jscpd

import "encoding/json"

// Clone records a single code duplication instance.
type Clone struct {
	Format string
	Lines  int
	FileA  string
	StartA int
	EndA   int
	FileB  string
	StartB int
	EndB   int
}

// Parse decodes jscpd JSON report into a slice of clones.
func Parse(data []byte) ([]Clone, error) {
	var raw struct {
		Duplicates []struct {
			Format     string `json:"format"`
			Lines      int    `json:"lines"`
			FirstFile  struct {
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
		return nil, err
	}

	clones := make([]Clone, 0, len(raw.Duplicates))
	for _, d := range raw.Duplicates {
		clones = append(clones, Clone{
			Format: d.Format, Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line, EndA: d.FirstFile.EndLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return clones, nil
}
