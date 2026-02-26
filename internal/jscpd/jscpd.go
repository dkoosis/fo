package jscpd

import "encoding/json"

// Result represents parsed jscpd duplicate detection output.
type Result struct {
	Clones []Clone
}

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

// Parse decodes jscpd JSON report into a Result.
func Parse(data []byte) (*Result, error) {
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

	r := &Result{}
	for _, d := range raw.Duplicates {
		r.Clones = append(r.Clones, Clone{
			Format: d.Format, Lines: d.Lines,
			FileA: d.FirstFile.Name, StartA: d.FirstFile.StartLoc.Line, EndA: d.FirstFile.EndLoc.Line,
			FileB: d.SecondFile.Name, StartB: d.SecondFile.StartLoc.Line, EndB: d.SecondFile.EndLoc.Line,
		})
	}
	return r, nil
}
